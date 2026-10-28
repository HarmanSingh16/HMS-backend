package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/HarmanSingh16/HMS-backend/internal/db"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduleService struct {
	querier db.Querier
	pool    *pgxpool.Pool
	cfg     *config.Config
}

func NewScheduleService(querier db.Querier, pool *pgxpool.Pool, cfg *config.Config) *ScheduleService {
	return &ScheduleService{
		querier: querier,
		pool:    pool,
		cfg:     cfg,
	}
}

// ----------------------------------------------------------------------
// 1. Doctor Directory
// ----------------------------------------------------------------------
func (s *ScheduleService) ListDoctors(ctx context.Context, departmentID *int32) ([]domain.DoctorProfile, error) {
	var deptParam pgtype.Int4
	if departmentID != nil {
		deptParam = pgtype.Int4{Int32: *departmentID, Valid: true}
	}
	rows, err := s.querier.ListActiveDoctors(ctx, deptParam)
	if err != nil {
		return nil, err
	}
	result := make([]domain.DoctorProfile, 0, len(rows))
	for _, r := range rows {
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
		result = append(result, domain.DoctorProfile{
			ID:             r.AccountID,
			FirstName:      r.FirstName,
			LastName:       r.LastName,
			Email:          r.Email,
			DepartmentID:   deptID,
			DepartmentName: deptName,
			CreatedAt:      r.CreatedAt.Time,
		})
	}
	return result, nil
}

// ----------------------------------------------------------------------
// 2. Doctor Schedule (Office Hours + Upcoming Leaves)
// ----------------------------------------------------------------------
func (s *ScheduleService) GetDoctorSchedule(ctx context.Context, doctorID int32) (*domain.DoctorScheduleResponse, error) {
	_, err := s.querier.GetDoctorProfileByID(ctx, doctorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDoctorNotFound
		}
		return nil, err
	}
	officeHoursRows, err := s.querier.GetDoctorOfficeHours(ctx, doctorID)
	if err != nil {
		return nil, err
	}
	officeHours := make([]domain.OfficeHourResponse, 0, len(officeHoursRows))
	for _, oh := range officeHoursRows {
		officeHours = append(officeHours, domain.OfficeHourResponse{
			ID:        oh.ID,
			DoctorID:  oh.DoctorID,
			Weekday:   int(oh.Weekday),
			StartTime: formatPgTime(oh.StartTime),
			EndTime:   formatPgTime(oh.EndTime),
		})
	}
	timeOffRows, err := s.querier.ListUpcomingDoctorTimeOff(ctx, db.ListUpcomingDoctorTimeOffParams{
		DoctorID: doctorID,
		EndsAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return nil, err
	}
	upcomingLeaves := make([]domain.TimeOffResponse, 0, len(timeOffRows))
	for _, to := range timeOffRows {
		upcomingLeaves = append(upcomingLeaves, domain.TimeOffResponse{
			ID:       to.ID,
			DoctorID: to.DoctorID,
			StartsAt: to.StartsAt.Time,
			EndsAt:   to.EndsAt.Time,
			Reason:   to.Reason.String,
		})
	}
	return &domain.DoctorScheduleResponse{
		DoctorID:    doctorID,
		OfficeHours: officeHours,
		TimeOff:     upcomingLeaves,
	}, nil
}

// ----------------------------------------------------------------------
// 3. Dynamic 15-Minute Slot Projection Engine
// ----------------------------------------------------------------------
func (s *ScheduleService) ComputeAvailableSlots(ctx context.Context, doctorID int32, targetDate *time.Time, now time.Time) (*domain.DoctorAvailableSlotsResponse, error) {
	// Verify doctor exists and is active
	doc, err := s.querier.GetDoctorProfileByID(ctx, doctorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDoctorNotFound
		}
		return nil, err
	}
	if doc.Status != db.AccountStatusActive {
		return nil, domain.ErrDoctorNotActive
	}
	// 7-day booking window calculation
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	maxWindowEnd := todayMidnight.AddDate(0, 0, 7)
	var windowStart, windowEnd time.Time
	if targetDate != nil {
		dayStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, now.Location())
		if dayStart.Before(todayMidnight) {
			return &domain.DoctorAvailableSlotsResponse{DoctorID: doctorID, Slots: []domain.Slot{}}, nil
		}
		if dayStart.After(maxWindowEnd) {
			return nil, domain.ErrBookingWindowExceeded
		}
		windowStart = dayStart
		windowEnd = dayStart.AddDate(0, 0, 1)
	} else {
		windowStart = todayMidnight
		windowEnd = maxWindowEnd
	}
	// Fetch recurring office hours
	officeHours, err := s.querier.GetDoctorOfficeHours(ctx, doctorID)
	if err != nil {
		return nil, err
	}
	if len(officeHours) == 0 {
		return &domain.DoctorAvailableSlotsResponse{DoctorID: doctorID, Slots: []domain.Slot{}}, nil
	}
	// Fetch leaves overlapping the requested window
	leaves, err := s.querier.ListDoctorTimeOffInRange(ctx, db.ListDoctorTimeOffInRangeParams{
		DoctorID: doctorID,
		StartsAt: pgtype.Timestamptz{Time: windowStart, Valid: true},
		EndsAt:   pgtype.Timestamptz{Time: windowEnd, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	// Fetch active appointments overlapping the requested window
	appointments, err := s.querier.GetActiveAppointmentsForDoctorInRange(ctx, db.GetActiveAppointmentsForDoctorInRangeParams{
		DoctorID: doctorID,
		StartsAt: pgtype.Timestamptz{Time: windowStart, Valid: true},
		EndsAt:   pgtype.Timestamptz{Time: windowEnd, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	// Slicing into discrete 15-minute intervals in memory
	availableSlots := make([]domain.Slot, 0)
	for currentDay := windowStart; currentDay.Before(windowEnd); currentDay = currentDay.AddDate(0, 0, 1) {
		currentWeekday := int16(currentDay.Weekday())
		for _, oh := range officeHours {
			if oh.Weekday != currentWeekday {
				continue
			}
			startHour, startMin := parsePgTime(oh.StartTime)
			endHour, endMin := parsePgTime(oh.EndTime)
			shiftStart := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), startHour, startMin, 0, 0, now.Location())
			shiftEnd := time.Date(currentDay.Year(), currentDay.Month(), currentDay.Day(), endHour, endMin, 0, 0, now.Location())
			// Slice shift into 15m intervals
			for slotStart := shiftStart; slotStart.Add(15*time.Minute).Before(shiftEnd) || slotStart.Add(15*time.Minute).Equal(shiftEnd); slotStart = slotStart.Add(15 * time.Minute) {
				slotEnd := slotStart.Add(15 * time.Minute)
				// 1. Exclude intervals in the past
				if slotEnd.Before(now) {
					continue
				}
				// 2. Check if overlapping with doctor leave
				isCoveredByLeave := false
				for _, leave := range leaves {
					if slotStart.Before(leave.EndsAt.Time) && slotEnd.After(leave.StartsAt.Time) {
						isCoveredByLeave = true
						break
					}
				}
				if isCoveredByLeave {
					continue
				}
				// 3. Check if overlapping with active appointments
				isBooked := false
				for _, appt := range appointments {
					if slotStart.Before(appt.EndsAt.Time) && slotEnd.After(appt.StartsAt.Time) {
						isBooked = true
						break
					}
				}
				if isBooked {
					continue
				}
				// 4. Walk-in eligibility: If the current time falls inside the slot, it's walk-in eligible
				isWalkIn := now.After(slotStart) && now.Before(slotEnd)
				availableSlots = append(availableSlots, domain.Slot{
					StartsAt:         slotStart,
					EndsAt:           slotEnd,
					IsWalkInEligible: isWalkIn,
				})
			}
		}
	}
	return &domain.DoctorAvailableSlotsResponse{
		DoctorID: doctorID,
		Slots:    availableSlots,
	}, nil
}

// ----------------------------------------------------------------------
// 4. Update Office Hours (Preview Mode & Atomic Changes)
// ----------------------------------------------------------------------
func (s *ScheduleService) UpdateOfficeHours(ctx context.Context, doctorID int32, req domain.UpdateOfficeHoursRequest, preview bool, actorID int32) (*domain.UpdateOfficeHoursPreviewResponse, error) {
	// Validate schedule format
	for _, ws := range req.Schedules {
		if ws.Weekday < 0 || ws.Weekday > 6 {
			return nil, domain.ErrInvalidWeekday
		}
		for i, interval := range ws.Intervals {
			sH, sM, err := parseTimeString(interval.StartTime)
			if err != nil {
				return nil, domain.ErrInvalidTimeFormat
			}
			eH, eM, err := parseTimeString(interval.EndTime)
			if err != nil {
				return nil, domain.ErrInvalidTimeFormat
			}
			startMins := sH*60 + sM
			endMins := eH*60 + eM
			if startMins >= endMins {
				return nil, domain.ErrInvalidTimeRange
			}
			// Check overlap with other intervals on same weekday
			for j := i + 1; j < len(ws.Intervals); j++ {
				other := ws.Intervals[j]
				osH, osM, _ := parseTimeString(other.StartTime)
				oeH, oeM, _ := parseTimeString(other.EndTime)
				oStart := osH*60 + osM
				oEnd := oeH*60 + oeM
				if startMins < oEnd && endMins > oStart {
					return nil, domain.ErrOverlappingOfficeHours
				}
			}
		}
	}
	// Verify doctor exists
	_, err := s.querier.GetDoctorProfileByID(ctx, doctorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDoctorNotFound
		}
		return nil, err
	}
	// Scan future booked appointments (next 30 days) to detect conflicts
	now := time.Now()
	bookedAppts, err := s.querier.GetBookedAppointmentsForDoctorInRange(ctx, db.GetBookedAppointmentsForDoctorInRangeParams{
		DoctorID: doctorID,
		StartsAt: pgtype.Timestamptz{Time: now, Valid: true},
		EndsAt:   pgtype.Timestamptz{Time: now.AddDate(0, 1, 0), Valid: true},
	})
	if err != nil {
		return nil, err
	}
	var conflictingAppts []domain.AffectedAppointment
	for _, appt := range bookedAppts {
		apptStart := appt.StartsAt.Time
		apptEnd := appt.EndsAt.Time
		apptWeekday := int(apptStart.Weekday())
		apptMinsStart := apptStart.Hour()*60 + apptStart.Minute()
		apptMinsEnd := apptEnd.Hour()*60 + apptEnd.Minute()
		fallsWithinNewHours := false
		for _, ws := range req.Schedules {
			if ws.Weekday == apptWeekday {
				for _, interval := range ws.Intervals {
					sH, sM, _ := parseTimeString(interval.StartTime)
					eH, eM, _ := parseTimeString(interval.EndTime)
					if apptMinsStart >= (sH*60+sM) && apptMinsEnd <= (eH*60+eM) {
						fallsWithinNewHours = true
						break
					}
				}
			}
		}
		if !fallsWithinNewHours {
			conflictingAppts = append(conflictingAppts, domain.AffectedAppointment{
				ID:        appt.ID,
				PatientID: appt.PatientID,
				StartsAt:  apptStart,
				EndsAt:    apptEnd,
			})
		}
	}
	// If preview mode, return without modifying database
	if preview {
		return &domain.UpdateOfficeHoursPreviewResponse{
			ConflictsCount:       len(conflictingAppts),
			AffectedAppointments: conflictingAppts,
		}, nil
	}
	// Atomic update inside database transaction
	err = db.WithTx(ctx, s.pool, func(q db.Querier) error {
		// 1. Delete existing office hours
		if err := q.DeleteDoctorOfficeHours(ctx, doctorID); err != nil {
			return err
		}
		// 2. Insert new office hours
		for _, ws := range req.Schedules {
			for _, interval := range ws.Intervals {
				sH, sM, _ := parseTimeString(interval.StartTime)
				eH, eM, _ := parseTimeString(interval.EndTime)
				_, err := q.CreateDoctorOfficeHour(ctx, db.CreateDoctorOfficeHourParams{
					DoctorID:  doctorID,
					Weekday:   int16(ws.Weekday),
					StartTime: pgtype.Time{Microseconds: int64(sH*3600+sM*60) * 1_000_000, Valid: true},
					EndTime:   pgtype.Time{Microseconds: int64(eH*3600+eM*60) * 1_000_000, Valid: true},
				})
				if err != nil {
					return err
				}
			}
		}
		// 3. Cancel conflicting appointments and enqueue 'slot_removed' outbox notifications
		for _, ca := range conflictingAppts {
			_, err := q.CancelAppointmentByID(ctx, db.CancelAppointmentByIDParams{
				ID:                 ca.ID,
				CancellationReason: pgtype.Text{String: "doctor_schedule_change", Valid: true},
			})
			if err != nil {
				return err
			}
			// Record appointment status transition event
			_, _ = q.CreateAppointmentStatusEvent(ctx, db.CreateAppointmentStatusEventParams{
				AppointmentID: ca.ID,
				FromStatus:    db.NullAppointmentStatus{AppointmentStatus: db.AppointmentStatusBooked, Valid: true},
				ToStatus:      db.AppointmentStatusCancelled,
				ChangedBy:     pgtype.Int4{Int32: actorID, Valid: true},
				Reason:        pgtype.Text{String: "doctor_schedule_change", Valid: true},
			})
			// Transactional outbox notification
			payload, _ := json.Marshal(map[string]any{
				"doctor_id":      doctorID,
				"appointment_id": ca.ID,
				"starts_at":      ca.StartsAt,
				"reason":         "doctor_schedule_change",
			})
			_, err = q.CreateNotification(ctx, db.CreateNotificationParams{
				RecipientID:   ca.PatientID,
				AppointmentID: pgtype.Int4{Int32: ca.ID, Valid: true},
				Type:          "slot_removed",
				Status:        "pending",
				Payload:       payload,
				ScheduledFor:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return err
			}
		}
		// 4. Record Audit Log
		auditDetails, _ := json.Marshal(map[string]any{
			"cancelled_appointments_count": len(conflictingAppts),
			"new_schedules":                req.Schedules,
		})
		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     "doctor_schedule_updated",
			EntityType: "doctor",
			EntityID:   pgtype.Int4{Int32: doctorID, Valid: true},
			Details:    auditDetails,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &domain.UpdateOfficeHoursPreviewResponse{
		ConflictsCount:       len(conflictingAppts),
		AffectedAppointments: conflictingAppts,
	}, nil
}

// ----------------------------------------------------------------------
// 5. Book Time Off (Leave & Blackout Dates)
// ----------------------------------------------------------------------
func (s *ScheduleService) CreateTimeOff(ctx context.Context, doctorID int32, req domain.CreateTimeOffRequest, actorID int32) (*domain.TimeOffResponse, error) {
	if !req.StartsAt.Before(req.EndsAt) {
		return nil, domain.ErrInvalidTimeRange
	}
	if req.EndsAt.Before(time.Now()) {
		return nil, domain.ErrTimeOffInPast
	}
	_, err := s.querier.GetDoctorProfileByID(ctx, doctorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDoctorNotFound
		}
		return nil, err
	}
	var createdLeave domain.TimeOffResponse
	err = db.WithTx(ctx, s.pool, func(q db.Querier) error {
		// 1. Insert time off
		toRow, err := q.CreateDoctorTimeOff(ctx, db.CreateDoctorTimeOffParams{
			DoctorID: doctorID,
			StartsAt: pgtype.Timestamptz{Time: req.StartsAt, Valid: true},
			EndsAt:   pgtype.Timestamptz{Time: req.EndsAt, Valid: true},
			Reason:   pgtype.Text{String: req.Reason, Valid: req.Reason != ""},
		})
		if err != nil {
			return err
		}
		createdLeave = domain.TimeOffResponse{
			ID:       toRow.ID,
			DoctorID: toRow.DoctorID,
			StartsAt: toRow.StartsAt.Time,
			EndsAt:   toRow.EndsAt.Time,
			Reason:   toRow.Reason.String,
		}
		// 2. Cancel all booked appointments during this time off
		cancelledAppts, err := q.CancelBookedAppointmentsInRange(ctx, db.CancelBookedAppointmentsInRangeParams{
			DoctorID:           doctorID,
			StartsAt:           pgtype.Timestamptz{Time: req.StartsAt, Valid: true},
			EndsAt:             pgtype.Timestamptz{Time: req.EndsAt, Valid: true},
			CancellationReason: pgtype.Text{String: "doctor_schedule_change", Valid: true},
		})
		if err != nil {
			return err
		}
		// 3. For each cancelled appointment, record status transition & transactional outbox notification
		for _, ca := range cancelledAppts {
			_, _ = q.CreateAppointmentStatusEvent(ctx, db.CreateAppointmentStatusEventParams{
				AppointmentID: ca.ID,
				FromStatus:    db.NullAppointmentStatus{AppointmentStatus: db.AppointmentStatusBooked, Valid: true},
				ToStatus:      db.AppointmentStatusCancelled,
				ChangedBy:     pgtype.Int4{Int32: actorID, Valid: true},
				Reason:        pgtype.Text{String: "doctor_schedule_change", Valid: true},
			})
			payload, _ := json.Marshal(map[string]any{
				"doctor_id":      doctorID,
				"appointment_id": ca.ID,
				"starts_at":      ca.StartsAt.Time,
				"reason":         "doctor_schedule_change",
			})
			_, err = q.CreateNotification(ctx, db.CreateNotificationParams{
				RecipientID:   ca.PatientID,
				AppointmentID: pgtype.Int4{Int32: ca.ID, Valid: true},
				Type:          "slot_removed",
				Status:        "pending",
				Payload:       payload,
				ScheduledFor:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
			if err != nil {
				return err
			}
		}
		// 4. Audit Log
		auditDetails, _ := json.Marshal(map[string]any{
			"starts_at":                    req.StartsAt,
			"ends_at":                      req.EndsAt,
			"reason":                       req.Reason,
			"cancelled_appointments_count": len(cancelledAppts),
		})
		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     "doctor_time_off_created",
			EntityType: "doctor",
			EntityID:   pgtype.Int4{Int32: doctorID, Valid: true},
			Details:    auditDetails,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &createdLeave, nil
}

// ----------------------------------------------------------------------
// Time Conversion Helpers
// ----------------------------------------------------------------------
func parsePgTime(t pgtype.Time) (int, int) {
	totalSeconds := t.Microseconds / 1_000_000
	hour := int(totalSeconds / 3600)
	minute := int((totalSeconds % 3600) / 60)
	return hour, minute
}
func formatPgTime(t pgtype.Time) string {
	h, m := parsePgTime(t)
	return fmt.Sprintf("%02d:%02d", h, m)
}
func parseTimeString(s string) (int, int, error) {
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, errors.New("invalid time format")
	}
	return h, m, nil
}
