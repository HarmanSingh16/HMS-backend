package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/HarmanSingh16/HMS-backend/internal/db"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppointmentService struct {
	querier db.Querier
	pool    *pgxpool.Pool
	cfg     *config.Config
}

func NewAppointmentService(querier db.Querier, pool *pgxpool.Pool, cfg *config.Config) *AppointmentService {
	return &AppointmentService{
		querier: querier,
		pool:    pool,
		cfg:     cfg,
	}
}

// ----------------------------------------------------------------------
// 1. Book Appointment (Concurrent Safe & Outbox Integrated)
// ----------------------------------------------------------------------
func (s *AppointmentService) BookAppointment(ctx context.Context, actorID int32, req domain.BookAppointmentRequest) (*domain.AppointmentDetailResponse, error) {
	startsAt := req.StartsAt.UTC().Truncate(time.Minute)
	endsAt := startsAt.Add(15 * time.Minute)

	// Invariant: Exactly 15-minute slot boundary alignment
	if startsAt.Minute()%15 != 0 || req.StartsAt.Second() != 0 || req.StartsAt.Nanosecond() != 0 {
		return nil, domain.ErrInvalidSlotDuration
	}

	now := time.Now().UTC()

	// Invariant: Booking window is up to 7 days ahead
	if startsAt.After(now.AddDate(0, 0, 7)) {
		return nil, domain.ErrBookingWindowExceeded
	}

	// Invariant: Walk-in immediate check-in vs Past slot check
	isWalkIn := false
	if startsAt.Before(now) {
		if now.Before(endsAt) {
			// Current ongoing 15m slot -> Walk-in
			isWalkIn = true
		} else {
			return nil, domain.ErrSlotInPast
		}
	}

	initialStatus := db.AppointmentStatusBooked
	if isWalkIn {
		initialStatus = db.AppointmentStatusCheckedIn
	}

	// Verify doctor exists and is active
	doctorProfile, err := s.querier.GetDoctorProfileByID(ctx, req.DoctorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDoctorNotFound
		}
		return nil, err
	}
	if doctorProfile.Status != db.AccountStatusActive {
		return nil, domain.ErrDoctorNotActive
	}

	// Verify patient exists
	_, err = s.querier.GetPatientByAccountID(ctx, req.PatientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPatientNotFound
		}
		return nil, err
	}

	// Invariant: Verify slot falls strictly inside doctor's office hours
	weekday := int16(startsAt.Weekday())
	officeHours, err := s.querier.GetDoctorOfficeHours(ctx, req.DoctorID)
	if err != nil {
		return nil, err
	}

	slotStartMins := startsAt.Hour()*60 + startsAt.Minute()
	slotEndMins := endsAt.Hour()*60 + endsAt.Minute()
	if endsAt.Hour() == 0 && endsAt.Minute() == 0 && endsAt.Day() != startsAt.Day() {
		slotEndMins = 24 * 60
	}

	insideWorkingHours := false
	for _, oh := range officeHours {
		if oh.Weekday != weekday {
			continue
		}
		ohStartH, ohStartM := parsePgTime(oh.StartTime)
		ohEndH, ohEndM := parsePgTime(oh.EndTime)
		ohStartMins := ohStartH*60 + ohStartM
		ohEndMins := ohEndH*60 + ohEndM

		if slotStartMins >= ohStartMins && slotEndMins <= ohEndMins {
			insideWorkingHours = true
			break
		}
	}
	if !insideWorkingHours {
		return nil, domain.ErrOutsideOfficeHours
	}

	// Invariant: Check for doctor scheduled time-off / leave
	leaves, err := s.querier.ListDoctorTimeOffInRange(ctx, db.ListDoctorTimeOffInRangeParams{
		DoctorID: req.DoctorID,
		StartsAt: pgtype.Timestamptz{Time: startsAt, Valid: true},
		EndsAt:   pgtype.Timestamptz{Time: endsAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	if len(leaves) > 0 {
		return nil, domain.ErrDoctorTimeOffConflict
	}

	var createdApptID int32

	// Atomic persistence inside transaction
	err = db.WithTx(ctx, s.pool, func(q db.Querier) error {
		// 1. Insert appointment (protected by PostgreSQL GiST exclusion lock)
		appt, err := q.CreateAppointment(ctx, db.CreateAppointmentParams{
			DoctorID:  req.DoctorID,
			PatientID: req.PatientID,
			StartsAt:  pgtype.Timestamptz{Time: startsAt, Valid: true},
			EndsAt:    pgtype.Timestamptz{Time: endsAt, Valid: true},
			Status:    initialStatus,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
				return domain.ErrSlotAlreadyBooked
			}
			return err
		}
		createdApptID = appt.ID

		// 2. Insert initial status transition event
		reasonText := "scheduled_booking"
		if isWalkIn {
			reasonText = "walk_in_booking"
		}
		_, err = q.CreateAppointmentStatusEvent(ctx, db.CreateAppointmentStatusEventParams{
			AppointmentID: appt.ID,
			FromStatus:    db.NullAppointmentStatus{Valid: false},
			ToStatus:      initialStatus,
			ChangedBy:     pgtype.Int4{Int32: actorID, Valid: true},
			Reason:        pgtype.Text{String: reasonText, Valid: true},
		})
		if err != nil {
			return err
		}

		// 3. Transactional Outbox Notification:
		// Rule: Send reminder 24 hours prior; skip if booked less than 24h ahead
		if startsAt.Sub(now) >= 24*time.Hour {
			scheduledFor := startsAt.Add(-24 * time.Hour)
			payload, _ := json.Marshal(map[string]any{
				"appointment_id": appt.ID,
				"doctor_id":      req.DoctorID,
				"patient_id":     req.PatientID,
				"starts_at":      startsAt,
			})
			_, err = q.CreateNotification(ctx, db.CreateNotificationParams{
				RecipientID:   req.PatientID,
				AppointmentID: pgtype.Int4{Int32: appt.ID, Valid: true},
				Type:          "appointment_reminder",
				Status:        "pending",
				Payload:       payload,
				ScheduledFor:  pgtype.Timestamptz{Time: scheduledFor, Valid: true},
			})
			if err != nil {
				return err
			}
		}

		// 4. Audit Log
		auditAction := "appointment_booked"
		if isWalkIn {
			auditAction = "appointment_walk_in_booked"
		}
		auditDetails, _ := json.Marshal(map[string]any{
			"appointment_id": appt.ID,
			"doctor_id":      req.DoctorID,
			"patient_id":     req.PatientID,
			"starts_at":      startsAt,
			"ends_at":        endsAt,
			"status":         initialStatus,
		})
		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     auditAction,
			EntityType: "appointment",
			EntityID:   pgtype.Int4{Int32: appt.ID, Valid: true},
			Details:    auditDetails,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.getAppointmentDetailsByID(ctx, createdApptID)
}

// ----------------------------------------------------------------------
// 2. Check-In (Receptionist arrival action)
// ----------------------------------------------------------------------
func (s *AppointmentService) CheckInAppointment(ctx context.Context, actorID int32, appointmentID int32) (*domain.AppointmentDetailResponse, error) {
	appt, err := s.querier.GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAppointmentNotFound
		}
		return nil, err
	}

	if appt.Status != db.AppointmentStatusBooked {
		return nil, domain.ErrInvalidStatusTransition
	}

	err = db.WithTx(ctx, s.pool, func(q db.Querier) error {
		_, err := q.UpdateAppointmentStatus(ctx, db.UpdateAppointmentStatusParams{
			ID:                 appointmentID,
			Status:             db.AppointmentStatusCheckedIn,
			CancellationReason: pgtype.Text{Valid: false},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAppointmentStatusEvent(ctx, db.CreateAppointmentStatusEventParams{
			AppointmentID: appointmentID,
			FromStatus:    db.NullAppointmentStatus{AppointmentStatus: db.AppointmentStatusBooked, Valid: true},
			ToStatus:      db.AppointmentStatusCheckedIn,
			ChangedBy:     pgtype.Int4{Int32: actorID, Valid: true},
			Reason:        pgtype.Text{String: "patient_check_in", Valid: true},
		})
		if err != nil {
			return err
		}

		auditDetails, _ := json.Marshal(map[string]any{
			"appointment_id": appointmentID,
			"from_status":    db.AppointmentStatusBooked,
			"to_status":      db.AppointmentStatusCheckedIn,
		})
		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     "appointment_checked_in",
			EntityType: "appointment",
			EntityID:   pgtype.Int4{Int32: appointmentID, Valid: true},
			Details:    auditDetails,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.getAppointmentDetailsByID(ctx, appointmentID)
}

// ----------------------------------------------------------------------
// 3. Complete (Doctor / Receptionist marks consultation finished)
// ----------------------------------------------------------------------
func (s *AppointmentService) CompleteAppointment(ctx context.Context, actorID int32, callerRole domain.Role, appointmentID int32) (*domain.AppointmentDetailResponse, error) {
	appt, err := s.querier.GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAppointmentNotFound
		}
		return nil, err
	}

	// Doctor can only complete consultations assigned to them
	if callerRole == domain.RoleDoctor && appt.DoctorID != actorID {
		return nil, domain.ErrUnauthorizedAppointmentAction
	}

	if appt.Status != db.AppointmentStatusCheckedIn {
		return nil, domain.ErrInvalidStatusTransition
	}

	err = db.WithTx(ctx, s.pool, func(q db.Querier) error {
		_, err := q.UpdateAppointmentStatus(ctx, db.UpdateAppointmentStatusParams{
			ID:                 appointmentID,
			Status:             db.AppointmentStatusCompleted,
			CancellationReason: pgtype.Text{Valid: false},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAppointmentStatusEvent(ctx, db.CreateAppointmentStatusEventParams{
			AppointmentID: appointmentID,
			FromStatus:    db.NullAppointmentStatus{AppointmentStatus: db.AppointmentStatusCheckedIn, Valid: true},
			ToStatus:      db.AppointmentStatusCompleted,
			ChangedBy:     pgtype.Int4{Int32: actorID, Valid: true},
			Reason:        pgtype.Text{String: "consultation_completed", Valid: true},
		})
		if err != nil {
			return err
		}

		auditDetails, _ := json.Marshal(map[string]any{
			"appointment_id": appointmentID,
			"from_status":    db.AppointmentStatusCheckedIn,
			"to_status":      db.AppointmentStatusCompleted,
		})
		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     "appointment_completed",
			EntityType: "appointment",
			EntityID:   pgtype.Int4{Int32: appointmentID, Valid: true},
			Details:    auditDetails,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.getAppointmentDetailsByID(ctx, appointmentID)
}

// ----------------------------------------------------------------------
// 4. Cancel Appointment (Atomic Outbox Invalidation & Cutoff Enforcement)
// ----------------------------------------------------------------------
func (s *AppointmentService) CancelAppointment(ctx context.Context, actorID int32, callerRole domain.Role, appointmentID int32, reason string) (*domain.AppointmentDetailResponse, error) {
	appt, err := s.querier.GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAppointmentNotFound
		}
		return nil, err
	}

	// Invariant: Completed, cancelled, or no-show visits cannot be cancelled
	if appt.Status == db.AppointmentStatusCompleted ||
		appt.Status == db.AppointmentStatusCancelled ||
		appt.Status == db.AppointmentStatusNoShow {
		return nil, domain.ErrCannotCancelCompleted
	}

	// Invariant: Caller permissions & 1-hour cancellation cutoff for patients
	if callerRole == domain.RolePatient {
		if appt.PatientID != actorID {
			return nil, domain.ErrUnauthorizedAppointmentAction
		}
		if time.Until(appt.StartsAt.Time) < 1*time.Hour {
			return nil, domain.ErrCancellationCutoffExceeded
		}
	} else if callerRole == domain.RoleDoctor {
		if appt.DoctorID != actorID {
			return nil, domain.ErrUnauthorizedAppointmentAction
		}
	}

	err = db.WithTx(ctx, s.pool, func(q db.Querier) error {
		// 1. Update status to cancelled
		_, err := q.UpdateAppointmentStatus(ctx, db.UpdateAppointmentStatusParams{
			ID:                 appointmentID,
			Status:             db.AppointmentStatusCancelled,
			CancellationReason: pgtype.Text{String: reason, Valid: true},
		})
		if err != nil {
			return err
		}

		// 2. Append event to lifecycle transition history
		_, err = q.CreateAppointmentStatusEvent(ctx, db.CreateAppointmentStatusEventParams{
			AppointmentID: appointmentID,
			FromStatus:    db.NullAppointmentStatus{AppointmentStatus: appt.Status, Valid: true},
			ToStatus:      db.AppointmentStatusCancelled,
			ChangedBy:     pgtype.Int4{Int32: actorID, Valid: true},
			Reason:        pgtype.Text{String: reason, Valid: true},
		})
		if err != nil {
			return err
		}

		// 3. Atomically revoke pending reminder emails in transactional outbox
		err = q.CancelPendingNotificationsByAppointmentID(ctx, pgtype.Int4{Int32: appointmentID, Valid: true})
		if err != nil {
			return err
		}

		// 4. Audit Log
		auditDetails, _ := json.Marshal(map[string]any{
			"appointment_id":      appointmentID,
			"cancellation_reason": reason,
			"previous_status":     appt.Status,
		})
		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     "appointment_cancelled",
			EntityType: "appointment",
			EntityID:   pgtype.Int4{Int32: appointmentID, Valid: true},
			Details:    auditDetails,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.getAppointmentDetailsByID(ctx, appointmentID)
}

// ----------------------------------------------------------------------
// 5. Revert No-Show (Staff overrides false-alarm auto no-show)
// ----------------------------------------------------------------------
func (s *AppointmentService) RevertNoShow(ctx context.Context, actorID int32, callerRole domain.Role, appointmentID int32) (*domain.AppointmentDetailResponse, error) {
	appt, err := s.querier.GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAppointmentNotFound
		}
		return nil, err
	}

	if callerRole == domain.RoleDoctor && appt.DoctorID != actorID {
		return nil, domain.ErrUnauthorizedAppointmentAction
	}

	if appt.Status != db.AppointmentStatusNoShow {
		return nil, domain.ErrInvalidStatusTransition
	}

	err = db.WithTx(ctx, s.pool, func(q db.Querier) error {
		// Move back to checked_in
		_, err := q.UpdateAppointmentStatus(ctx, db.UpdateAppointmentStatusParams{
			ID:                 appointmentID,
			Status:             db.AppointmentStatusCheckedIn,
			CancellationReason: pgtype.Text{Valid: false},
		})
		if err != nil {
			return err
		}

		_, err = q.CreateAppointmentStatusEvent(ctx, db.CreateAppointmentStatusEventParams{
			AppointmentID: appointmentID,
			FromStatus:    db.NullAppointmentStatus{AppointmentStatus: db.AppointmentStatusNoShow, Valid: true},
			ToStatus:      db.AppointmentStatusCheckedIn,
			ChangedBy:     pgtype.Int4{Int32: actorID, Valid: true},
			Reason:        pgtype.Text{String: "revert_no_show", Valid: true},
		})
		if err != nil {
			return err
		}

		auditDetails, _ := json.Marshal(map[string]any{
			"appointment_id": appointmentID,
			"action":         "revert_no_show",
		})
		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     "appointment_no_show_reverted",
			EntityType: "appointment",
			EntityID:   pgtype.Int4{Int32: appointmentID, Valid: true},
			Details:    auditDetails,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.getAppointmentDetailsByID(ctx, appointmentID)
}

// ----------------------------------------------------------------------
// 6. Get Single Appointment Details (Scoped Authorization)
// ----------------------------------------------------------------------
func (s *AppointmentService) GetAppointment(ctx context.Context, actorID int32, callerRole domain.Role, appointmentID int32) (*domain.AppointmentDetailResponse, error) {
	detail, err := s.getAppointmentDetailsByID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}

	if callerRole == domain.RolePatient && detail.PatientID != actorID {
		return nil, domain.ErrUnauthorizedAppointmentAction
	}
	if callerRole == domain.RoleDoctor && detail.DoctorID != actorID {
		return nil, domain.ErrUnauthorizedAppointmentAction
	}

	return detail, nil
}

// ----------------------------------------------------------------------
// 7. List Appointments with Scoped Filtering & Pagination
// ----------------------------------------------------------------------
func (s *AppointmentService) ListAppointments(ctx context.Context, actorID int32, callerRole domain.Role, filter domain.ListAppointmentsFilter) (*domain.PaginatedAppointmentsResponse, error) {
	// Scoped visibility rules
	var doctorFilter pgtype.Int4
	if callerRole == domain.RoleDoctor {
		doctorFilter = pgtype.Int4{Int32: actorID, Valid: true}
	} else if filter.DoctorID != nil {
		doctorFilter = pgtype.Int4{Int32: *filter.DoctorID, Valid: true}
	}

	var patientFilter pgtype.Int4
	if callerRole == domain.RolePatient {
		patientFilter = pgtype.Int4{Int32: actorID, Valid: true}
	} else if filter.PatientID != nil {
		patientFilter = pgtype.Int4{Int32: *filter.PatientID, Valid: true}
	}

	var statusFilter db.NullAppointmentStatus
	if filter.Status != nil {
		statusFilter = db.NullAppointmentStatus{
			AppointmentStatus: db.AppointmentStatus(*filter.Status),
			Valid:             true,
		}
	}

	var startsAfter pgtype.Timestamptz
	if filter.From != nil {
		startsAfter = pgtype.Timestamptz{Time: *filter.From, Valid: true}
	}

	var endsBefore pgtype.Timestamptz
	if filter.To != nil {
		endsBefore = pgtype.Timestamptz{Time: *filter.To, Valid: true}
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	totalCount, err := s.querier.CountAppointmentsWithDetails(ctx, db.CountAppointmentsWithDetailsParams{
		DoctorID:    doctorFilter,
		PatientID:   patientFilter,
		Status:      statusFilter,
		StartsAfter: startsAfter,
		EndsBefore:  endsBefore,
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.querier.ListAppointmentsWithDetails(ctx, db.ListAppointmentsWithDetailsParams{
		DoctorID:    doctorFilter,
		PatientID:   patientFilter,
		Status:      statusFilter,
		StartsAfter: startsAfter,
		EndsBefore:  endsBefore,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, err
	}

	results := make([]domain.AppointmentDetailResponse, 0, len(rows))
	for _, r := range rows {
		results = append(results, mapListAppointmentRow(r))
	}

	page := int(offset/limit) + 1

	return &domain.PaginatedAppointmentsResponse{
		Appointments: results,
		TotalCount:   totalCount,
		Page:         page,
		Limit:        int(limit),
	}, nil
}

// ----------------------------------------------------------------------
// 8. Get Appointment Transition Timeline
// ----------------------------------------------------------------------
func (s *AppointmentService) GetAppointmentHistory(ctx context.Context, actorID int32, callerRole domain.Role, appointmentID int32) ([]domain.AppointmentStatusEventResponse, error) {
	appt, err := s.querier.GetAppointmentByID(ctx, appointmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAppointmentNotFound
		}
		return nil, err
	}

	if callerRole == domain.RolePatient && appt.PatientID != actorID {
		return nil, domain.ErrUnauthorizedAppointmentAction
	}
	if callerRole == domain.RoleDoctor && appt.DoctorID != actorID {
		return nil, domain.ErrUnauthorizedAppointmentAction
	}

	rows, err := s.querier.ListAppointmentStatusEventsByAppointmentID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}

	events := make([]domain.AppointmentStatusEventResponse, 0, len(rows))
	for _, r := range rows {
		events = append(events, mapStatusEventRow(r))
	}
	return events, nil
}

// ----------------------------------------------------------------------
// Helper Methods & Row Mappings
// ----------------------------------------------------------------------
func (s *AppointmentService) getAppointmentDetailsByID(ctx context.Context, appointmentID int32) (*domain.AppointmentDetailResponse, error) {
	row, err := s.querier.GetAppointmentWithDetailsByID(ctx, appointmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAppointmentNotFound
		}
		return nil, err
	}

	var deptID *int32
	if row.DepartmentID.Valid {
		id := row.DepartmentID.Int32
		deptID = &id
	}
	var deptName *string
	if row.DepartmentName.Valid {
		name := row.DepartmentName.String
		deptName = &name
	}
	var cancelReason *string
	if row.CancellationReason.Valid {
		reason := row.CancellationReason.String
		cancelReason = &reason
	}
	fn := row.PatientFirstName
	ln := row.PatientLastName

	return &domain.AppointmentDetailResponse{
		ID:                 row.ID,
		DoctorID:           row.DoctorID,
		DoctorEmail:        row.DoctorEmail,
		DepartmentID:       deptID,
		DepartmentName:     deptName,
		PatientID:          row.PatientID,
		PatientEmail:       row.PatientEmail,
		PatientFirstName:   &fn,
		PatientLastName:    &ln,
		StartsAt:           row.StartsAt.Time,
		EndsAt:             row.EndsAt.Time,
		Status:             domain.AppointmentStatus(row.Status),
		CancellationReason: cancelReason,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
	}, nil
}

func mapListAppointmentRow(r db.ListAppointmentsWithDetailsRow) domain.AppointmentDetailResponse {
	var deptID *int32
	if r.DepartmentID.Valid {
		id := r.DepartmentID.Int32
		deptID = &id
	}
	var deptName *string
	if r.DepartmentName.Valid {
		name := r.DepartmentName.String
		deptName = &name
	}
	var cancelReason *string
	if r.CancellationReason.Valid {
		reason := r.CancellationReason.String
		cancelReason = &reason
	}
	fn := r.PatientFirstName
	ln := r.PatientLastName

	return domain.AppointmentDetailResponse{
		ID:                 r.ID,
		DoctorID:           r.DoctorID,
		DoctorEmail:        r.DoctorEmail,
		DepartmentID:       deptID,
		DepartmentName:     deptName,
		PatientID:          r.PatientID,
		PatientEmail:       r.PatientEmail,
		PatientFirstName:   &fn,
		PatientLastName:    &ln,
		StartsAt:           r.StartsAt.Time,
		EndsAt:             r.EndsAt.Time,
		Status:             domain.AppointmentStatus(r.Status),
		CancellationReason: cancelReason,
		CreatedAt:          r.CreatedAt.Time,
		UpdatedAt:          r.UpdatedAt.Time,
	}
}

func mapStatusEventRow(r db.ListAppointmentStatusEventsByAppointmentIDRow) domain.AppointmentStatusEventResponse {
	var fromStatus *domain.AppointmentStatus
	if r.FromStatus.Valid {
		fs := domain.AppointmentStatus(r.FromStatus.AppointmentStatus)
		fromStatus = &fs
	}
	var changedBy *int32
	if r.ChangedBy.Valid {
		id := r.ChangedBy.Int32
		changedBy = &id
	}
	var changedByEmail *string
	if r.ChangedByEmail.Valid {
		email := r.ChangedByEmail.String
		changedByEmail = &email
	}
	var reason *string
	if r.Reason.Valid {
		rs := r.Reason.String
		reason = &rs
	}

	return domain.AppointmentStatusEventResponse{
		ID:             r.ID,
		AppointmentID:  r.AppointmentID,
		FromStatus:     fromStatus,
		ToStatus:       domain.AppointmentStatus(r.ToStatus),
		ChangedBy:      changedBy,
		ChangedByEmail: changedByEmail,
		Reason:         reason,
		CreatedAt:      r.CreatedAt.Time,
	}
}
