package domain

import (
	"time"
)

// AppointmentStatus represents the lifecycle states of an appointment
type AppointmentStatus string

const (
	AppointmentStatusBooked    AppointmentStatus = "booked"
	AppointmentStatusCheckedIn AppointmentStatus = "checked_in"
	AppointmentStatusCompleted AppointmentStatus = "completed"
	AppointmentStatusCancelled AppointmentStatus = "cancelled"
	AppointmentStatusNoShow    AppointmentStatus = "no_show"
)

// Appointment represents the core domain model
type Appointment struct {
	ID                 int32             `json:"id"`
	DoctorID           int32             `json:"doctor_id"`
	PatientID          int32             `json:"patient_id"`
	StartsAt           time.Time         `json:"starts_at"`
	EndsAt             time.Time         `json:"ends_at"`
	Status             AppointmentStatus `json:"status"`
	CancellationReason *string           `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// AppointmentDetailResponse represents a joined view of appointment with doctor and patient details
type AppointmentDetailResponse struct {
	ID                 int32             `json:"id"`
	DoctorID           int32             `json:"doctor_id"`
	DoctorEmail        string            `json:"doctor_email"`
	DepartmentID       *int32            `json:"department_id,omitempty"`
	DepartmentName     *string           `json:"department_name,omitempty"`
	PatientID          int32             `json:"patient_id"`
	PatientEmail       string            `json:"patient_email"`
	PatientFirstName   *string           `json:"patient_first_name,omitempty"`
	PatientLastName    *string           `json:"patient_last_name,omitempty"`
	StartsAt           time.Time         `json:"starts_at"`
	EndsAt             time.Time         `json:"ends_at"`
	Status             AppointmentStatus `json:"status"`
	CancellationReason *string           `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// AppointmentStatusEventResponse records state transitions in the lifecycle audit timeline
type AppointmentStatusEventResponse struct {
	ID             int64              `json:"id"`
	AppointmentID  int32              `json:"appointment_id"`
	FromStatus     *AppointmentStatus `json:"from_status,omitempty"`
	ToStatus       AppointmentStatus  `json:"to_status"`
	ChangedBy      *int32             `json:"changed_by,omitempty"`
	ChangedByEmail *string            `json:"changed_by_email,omitempty"`
	Reason         *string            `json:"reason,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
}

// BookAppointmentRequest is the payload sent by receptionists to book a slot
type BookAppointmentRequest struct {
	DoctorID int32     `json:"doctor_id" binding:"required"`
	PatientID int32    `json:"patient_id" binding:"required"`
	StartsAt time.Time `json:"starts_at" binding:"required"`
}

// CancelAppointmentRequest is the payload sent when cancelling an appointment
type CancelAppointmentRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

// ListAppointmentsFilter specifies query filtering options
type ListAppointmentsFilter struct {
	DoctorID  *int32
	PatientID *int32
	Status    *AppointmentStatus
	From      *time.Time
	To        *time.Time
	Limit     int32
	Offset    int32
}

// PaginatedAppointmentsResponse provides paginated results for appointment listings
type PaginatedAppointmentsResponse struct {
	Appointments []AppointmentDetailResponse `json:"appointments"`
	TotalCount   int64                       `json:"total_count"`
	Page         int                         `json:"page"`
	Limit        int                         `json:"limit"`
}