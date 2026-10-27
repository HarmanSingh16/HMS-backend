package domain

import "errors"

var (
	// Auth errors
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountNotFound    = errors.New("account not found")
	ErrAccountDeactivated = errors.New("account is deactivated")
	ErrEmailAlreadyExists = errors.New("email is already registered")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrMissingAuthHeader  = errors.New("missing or malformed authorization header")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("access forbidden: insufficient permissions")

	// Invite errors
	ErrInviteNotFound       = errors.New("invite not found or expired")
	ErrInviteAlreadyClaimed = errors.New("invite has already been claimed")

	// Department Errors
	ErrDepartmentNotFound  = errors.New("department not found")
	ErrDepartmentNameTaken = errors.New("a department with this name already exists")

	// Patient errors
	ErrPatientNotFound    = errors.New("patient not found")
	ErrInvalidDateOfBirth = errors.New("date of birth must be a valid date in the past (format: YYYY-MM-DD)")
)
