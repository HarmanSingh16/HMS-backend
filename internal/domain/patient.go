package domain

import (
	"strings"
	"time"
)

type Patient struct {
	AccountID   int32     `json:"account_id"`
	Email       string    `json:"email"`
	Role        Role      `json:"role"`
	Status      string    `json:"status"` // 'invited', 'active', 'deactivated'
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	DateOfBirth string    `json:"date_of_birth"` // Formatted as YYYY-MM-DD
	PhoneNumber string    `json:"phone_number"`
	Gender      *string   `json:"gender,omitempty"`
	Address     *string   `json:"address,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *Patient) FullName() string {
	return strings.TrimSpace(p.FirstName + " " + p.LastName)
}

type RegisterPatientRequest struct {
	Email       string  `json:"email"`
	FirstName   string  `json:"first_name" binding:"required,min=1,max=100"`
	LastName    string  `json:"last_name" binding:"required,min=1,max=100"`
	DateOfBirth string  `json:"date_of_birth" binding:"required"`
	PhoneNumber string  `json:"phone_number" binding:"required,min=10,max=12"`
	Gender      *string `json:"gender,omitempty"`
	Address     *string `json:"address,omitempty"`
}

func (r *RegisterPatientRequest) Validate() (time.Time, error) {
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))
	r.FirstName = strings.TrimSpace(r.FirstName)
	r.LastName = strings.TrimSpace(r.LastName)
	r.PhoneNumber = strings.TrimSpace(r.PhoneNumber)

	if r.Gender != nil {
		g := strings.TrimSpace(*r.Gender)
		r.Gender = &g
	}
	if r.Address != nil {
		a := strings.TrimSpace(*r.Address)
		r.Address = &a
	}

	parsedDOB, err := time.Parse("2006-01-02", r.DateOfBirth)
	if err != nil {
		return time.Time{}, ErrInvalidDateOfBirth
	}

	if !parsedDOB.Before(time.Now()) {
		return time.Time{}, ErrInvalidDateOfBirth
	}

	return parsedDOB, nil
}

type UpdatePatientRequest struct {
	FirstName   string  `json:"first_name" binding:"required,min=1,max=100"`
	LastName    string  `json:"last_name" binding:"required,min=1,max=100"`
	DateOfBirth string  `json:"date_of_birth" binding:"required"`
	PhoneNumber string  `json:"phone_number" binding:"required,min=7,max=30"`
	Gender      *string `json:"gender,omitempty"`
	Address     *string `json:"address,omitempty"`
}

// Validate sanitizes inputs and validates DOB for update requests
func (r *UpdatePatientRequest) Validate() (time.Time, error) {
	r.FirstName = strings.TrimSpace(r.FirstName)
	r.LastName = strings.TrimSpace(r.LastName)
	r.PhoneNumber = strings.TrimSpace(r.PhoneNumber)
	if r.Gender != nil {
		g := strings.TrimSpace(*r.Gender)
		r.Gender = &g
	}
	if r.Address != nil {
		a := strings.TrimSpace(*r.Address)
		r.Address = &a
	}
	parsedDOB, err := time.Parse("2006-01-02", r.DateOfBirth)
	if err != nil {
		return time.Time{}, ErrInvalidDateOfBirth
	}
	if !parsedDOB.Before(time.Now()) {
		return time.Time{}, ErrInvalidDateOfBirth
	}
	return parsedDOB, nil
}

type PatientSearchQuery struct {
	Query string `form:"query"`
	Page  int32  `form:"page,default=1"`
	Limit int32  `form:"limit,default=20"`
}

type PaginatedPatientsResponse struct {
	Patients   []Patient `json:"patients"`
	TotalCount int64     `json:"total_count"`
	Page       int32     `json:"page"`
	Limit      int32     `json:"limit"`
	TotalPages int32     `json:"total_pages"`
}

// See if we really need this, because why can't we just return patient??
type RegisterPatientResponse struct {
	Patient Patient `json:"patient"`
}
