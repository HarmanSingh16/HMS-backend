package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/HarmanSingh16/HMS-backend/internal/db"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PatientService struct {
	querier db.Querier
	pool    *pgxpool.Pool
	cfg     *config.Config
}

func NewPatientService(querier db.Querier, pool *pgxpool.Pool, cfg *config.Config) *PatientService {
	return &PatientService{
		querier: querier,
		pool:    pool,
		cfg:     cfg,
	}
}
func (s *PatientService) RegisterPatient(ctx context.Context, req domain.RegisterPatientRequest, actorID int32) (*domain.RegisterPatientResponse, error) {
	parsedDOB, err := req.Validate()
	if err != nil {
		return nil, err
	}

	var createdPatient domain.Patient

	txErr := db.WithTx(ctx, s.pool, func(q db.Querier) error {
		// Check if email exists
		_, err := q.GetAccountByEmail(ctx, req.Email)
		if err == nil {
			return domain.ErrEmailAlreadyExists
		}

		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		// create account with role patient
		account, err := q.CreateAccount(ctx, db.CreateAccountParams{
			Email:        req.Email,
			PasswordHash: pgtype.Text{Valid: false},
			Role:         db.UserRolePatient,
			Status:       db.AccountStatusInvited,
		})

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return domain.ErrEmailAlreadyExists
			}
			return err
		}

		// create patient demographic record
		patientRow, err := q.CreatePatient(ctx, db.CreatePatientParams{
			AccountID:   account.ID,
			FirstName:   req.FirstName,
			LastName:    req.LastName,
			DateOfBirth: pgtype.Date{Time: parsedDOB, Valid: true},
			PhoneNumber: req.PhoneNumber,
			Gender:      toPgText(req.Gender),
			Address:     toPgText(req.Address),
		})
		if err != nil {
			return err
		}

		// Generate token
		rawToken, err := generateSecureToken()
		if err != nil {
			return err
		}
		tokenHash := hashToken(rawToken)

		// Create invite
		invite, err := q.CreateInvite(ctx, db.CreateInviteParams{
			AccountID: account.ID,
			TokenHash: tokenHash,
			ExpiresAt: pgtype.Timestamptz{
				Time:  time.Now().Add(s.cfg.InviteTokenTTL),
				Valid: true,
			},
		})
		if err != nil {
			return err
		}

		notificationPayload, err := json.Marshal(map[string]any{
			"email":        req.Email,
			"first_name":   req.FirstName,
			"last_name":    req.LastName,
			"invite_token": rawToken,
			"role":         string(domain.RolePatient),
		})
		if err != nil {
			return err
		}

		_, err = q.CreateNotification(ctx, db.CreateNotificationParams{
			RecipientID:   account.ID,
			InviteID:      pgtype.Int4{Int32: invite.ID, Valid: true},
			AppointmentID: pgtype.Int4{Valid: false},
			Type:          "account_invite",
			Status:        "pending",
			Payload:       notificationPayload,
			ScheduledFor:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
		if err != nil {
			return err
		}

		// 7. Record intake in audit log
		auditDetails, _ := json.Marshal(map[string]any{
			"email":        req.Email,
			"patient_name": req.FirstName + " " + req.LastName,
		})

		_, _ = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
			Action:     "patient_registered",
			EntityType: "patient",
			EntityID:   pgtype.Int4{Int32: account.ID, Valid: true},
			Details:    auditDetails,
		})

		createdPatient = domain.Patient{
			AccountID:   patientRow.AccountID,
			Email:       account.Email,
			Role:        domain.RolePatient,
			Status:      string(account.Status),
			FirstName:   patientRow.FirstName,
			LastName:    patientRow.LastName,
			DateOfBirth: patientRow.DateOfBirth.Time.Format("2006-01-02"),
			PhoneNumber: patientRow.PhoneNumber,
			Gender:      fromPgText(patientRow.Gender),
			Address:     fromPgText(patientRow.Address),
			CreatedAt:   patientRow.CreatedAt.Time,
			UpdatedAt:   patientRow.UpdatedAt.Time,
		}
		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	return &domain.RegisterPatientResponse{
		Patient: createdPatient,
	}, nil
}

// GetPatientById
func (s *PatientService) GetPatientByID(ctx context.Context, patientID int32, callerID int32, callerRole domain.Role) (*domain.Patient, error) {
	// Privacy: patient can only view their own data
	if callerRole == domain.RolePatient && callerID != patientID {
		return nil, domain.ErrForbidden
	}

	row, err := s.querier.GetPatientByAccountID(ctx, patientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPatientNotFound
		}
		return nil, err
	}

	if callerRole == domain.RoleDoctor {
		details, _ := json.Marshal(map[string]any{
			"patient_name": row.FirstName + " " + row.LastName,
		})

		_, _ = s.querier.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorID:    pgtype.Int4{Int32: callerID, Valid: true},
			Action:     "patient_demographics_viewed",
			EntityType: "patient",
			EntityID:   pgtype.Int4{Int32: patientID, Valid: true},
			Details:    details,
		})
	}

	patient := &domain.Patient{
		AccountID:   row.AccountID,
		Email:       row.Email,
		Role:        domain.Role(row.Role),
		Status:      string(row.AccountStatus),
		FirstName:   row.FirstName,
		LastName:    row.LastName,
		DateOfBirth: row.DateOfBirth.Time.Format("2006-01-02"),
		PhoneNumber: row.PhoneNumber,
		Gender:      fromPgText(row.Gender),
		Address:     fromPgText(row.Address),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}

	return patient, nil
}

func (s *PatientService) SearchPatients(ctx context.Context, query domain.PatientSearchQuery, callerRole domain.Role) (*domain.PaginatedPatientsResponse, error) {
	// only staff may search across patient records
	// WHY ARE WE DOING THIS HERE?? When requireRole middleware already does this

	if callerRole == domain.RolePatient {
		return nil, domain.ErrForbidden
	}

	page := query.Page
	page = max(page, 1)

	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit
	searchTerm := strings.TrimSpace(query.Query)

	rows, err := s.querier.SearchPatients(ctx, db.SearchPatientsParams{
		Column1: searchTerm,
		Limit:   limit,
		Offset:  offset,
	})

	if err != nil {
		return nil, err
	}

	totalCount, err := s.querier.CountPatients(ctx, searchTerm)
	if err != nil {
		return nil, err
	}

	patients := make([]domain.Patient, 0, len(rows))
	for _, r := range rows {
		patients = append(patients, domain.Patient{
			AccountID:   r.AccountID,
			Email:       r.Email,
			Role:        domain.Role(r.Role),
			Status:      string(r.AccountStatus),
			FirstName:   r.FirstName,
			LastName:    r.LastName,
			DateOfBirth: r.DateOfBirth.Time.Format("2006-01-02"),
			PhoneNumber: r.PhoneNumber,
			Gender:      fromPgText(r.Gender),
			Address:     fromPgText(r.Address),
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		})
	}

	totalPages := int32(math.Ceil(float64(totalCount) / float64(limit)))

	return &domain.PaginatedPatientsResponse{
		Patients:   patients,
		TotalCount: totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (s *PatientService) UpdatePatient(ctx context.Context, patientID int32, req domain.UpdatePatientRequest, callerID int32, callerRole domain.Role) (*domain.Patient, error) {
	if callerRole == domain.RolePatient && callerID != patientID {
		return nil, domain.ErrForbidden
	}
	parsedDOB, err := req.Validate()
	if err != nil {
		return nil, err
	}
	// Verify patient exists
	existing, err := s.querier.GetPatientByAccountID(ctx, patientID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPatientNotFound
		}
		return nil, err
	}
	updatedRow, err := s.querier.UpdatePatientDemographics(ctx, db.UpdatePatientDemographicsParams{
		AccountID:   patientID,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		DateOfBirth: pgtype.Date{Time: parsedDOB, Valid: true},
		PhoneNumber: req.PhoneNumber,
		Gender:      toPgText(req.Gender),
		Address:     toPgText(req.Address),
	})
	if err != nil {
		return nil, err
	}
	// Audit trail for modifications
	auditDetails, _ := json.Marshal(map[string]any{
		"previous_phone": existing.PhoneNumber,
		"new_phone":      updatedRow.PhoneNumber,
	})
	_, _ = s.querier.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorID:    pgtype.Int4{Int32: callerID, Valid: true},
		Action:     "patient_demographics_updated",
		EntityType: "patient",
		EntityID:   pgtype.Int4{Int32: patientID, Valid: true},
		Details:    auditDetails,
	})
	return &domain.Patient{
		AccountID:   updatedRow.AccountID,
		Email:       existing.Email,
		Role:        domain.Role(existing.Role),
		Status:      string(existing.AccountStatus),
		FirstName:   updatedRow.FirstName,
		LastName:    updatedRow.LastName,
		DateOfBirth: updatedRow.DateOfBirth.Time.Format("2006-01-02"),
		PhoneNumber: updatedRow.PhoneNumber,
		Gender:      fromPgText(updatedRow.Gender),
		Address:     fromPgText(updatedRow.Address),
		CreatedAt:   updatedRow.CreatedAt.Time,
		UpdatedAt:   updatedRow.UpdatedAt.Time,
	}, nil
}

// Helper functions:
func toPgText(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func fromPgText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
