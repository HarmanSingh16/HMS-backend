package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/HarmanSingh16/HMS-backend/internal/db"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountService struct {
	querier db.Querier
	pool    *pgxpool.Pool
	cfg     *config.Config
}

func NewAccountService(querier db.Querier, pool *pgxpool.Pool, cfg *config.Config) *AccountService {
	return &AccountService{
		querier: querier,
		pool:    pool,
		cfg:     cfg,
	}
}

func (s *AccountService) ListDepartments(ctx context.Context) ([]domain.Department, error) {
	rows, err := s.querier.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}

	departments := make([]domain.Department, 0, len(rows))
	for _, r := range rows {
		departments = append(departments, domain.Department{
			ID:   r.ID,
			Name: r.Name,
		})
	}

	return departments, nil
}

// create department, also make an audit log entry
func (s *AccountService) CreateDepartment(ctx context.Context, req domain.CreateDepartmentRequest, actorID int32) (*domain.Department, error) {
	name := strings.TrimSpace(req.Name)

	_, err := s.querier.GetDepartmentByName(ctx, name)
	if err == nil {
		return nil, domain.ErrDepartmentNameTaken
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	dept, err := s.querier.CreateDepartment(ctx, name)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrDepartmentNameTaken
		}
		return nil, err
	}

	details, _ := json.Marshal(map[string]any{
		"name": dept.Name,
	})

	_, _ = s.querier.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
		Action:     "department_created",
		EntityType: "department",
		EntityID:   pgtype.Int4{Int32: dept.ID, Valid: true},
		Details:    details,
	})

	return &domain.Department{
		ID:   dept.ID,
		Name: dept.Name,
	}, nil
}

// update department name and create an audit log entry
func (s *AccountService) UpdateDepartment(ctx context.Context, id int32, req domain.UpdateDepartmentRequest, actorID int32) (*domain.Department, error) {
	name := strings.TrimSpace(req.Name)

	existing, err := s.querier.GetDepartmentById(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDepartmentNotFound
		}
		return nil, err
	}

	duplicate, err := s.querier.GetDepartmentByName(ctx, name)
	if err == nil && duplicate.ID != id {
		return nil, domain.ErrDepartmentNameTaken
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	updated, err := s.querier.UpdateDepartment(ctx, db.UpdateDepartmentParams{
		ID:   id,
		Name: name,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrDepartmentNameTaken
		}
		return nil, err
	}

	details, _ := json.Marshal(map[string]any{
		"previous_name": existing.Name,
		"new_name":      updated.Name,
	})

	_, _ = s.querier.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorID:    pgtype.Int4{Int32: actorID, Valid: true},
		Action:     "department_updated",
		EntityType: "department",
		EntityID:   pgtype.Int4{Int32: updated.ID, Valid: true},
		Details:    details,
	})

	return &domain.Department{
		ID: updated.ID, 
		Name: updated.Name,
	}, nil
}
