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

	// Schedule and Doctor errors
	ErrDoctorNotFound         = errors.New("doctor not found")
	ErrDoctorNotActive        = errors.New("doctor account is not active")
	ErrInvalidTimeRange       = errors.New("end time must be strictly after start time")
	ErrInvalidTimeFormat      = errors.New("time must be in HH:MM format (24-hour)")
	ErrInvalidWeekday         = errors.New("weekday must be between 0 (Sunday) and 6 (Saturday)")
	ErrOverlappingOfficeHours = errors.New("office hour intervals on the same day must not overlap")
	ErrTimeOffInPast          = errors.New("time off cannot be scheduled entirely in the past")
	ErrBookingWindowExceeded  = errors.New("slots can only be viewed up to 7 days in advance")

	// Appointment Lifecycle Errors
	ErrSlotAlreadyBooked             = errors.New("the selected time slot has already been booked")
	ErrAppointmentNotFound           = errors.New("appointment not found")
	ErrInvalidSlotDuration           = errors.New("appointment duration must be exactly 15 minutes")
	ErrSlotInPast                    = errors.New("cannot book an appointment in the past")
	ErrOutsideOfficeHours            = errors.New("appointment slot is outside of doctor's working hours")
	ErrDoctorTimeOffConflict         = errors.New("doctor is on leave during this time slot")
	ErrCancellationCutoffExceeded    = errors.New("patients can only cancel appointments at least 1 hour in advance")
	ErrInvalidStatusTransition       = errors.New("invalid appointment status transition")
	ErrCannotCancelCompleted         = errors.New("cannot cancel an appointment that is completed or cancelled")
	ErrUnauthorizedAppointmentAction = errors.New("unauthorized to perform this action on the appointment")
)
