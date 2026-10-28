package domain

import "time"

// DoctorProfile represents the doctor directory view
type DoctorProfile struct {
	ID             int32     `json:"id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Email          string    `json:"email"`
	DepartmentID   *int32    `json:"department_id,omitempty"`
	DepartmentName *string   `json:"department_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// OfficeHourInterval represents a discrete shift window, e.g. "09:00" to "17:00"
type OfficeHourInterval struct {
	StartTime string `json:"start_time" binding:"required"` // Format: "15:04" (HH:MM)
	EndTime   string `json:"end_time" binding:"required"`   // Format: "15:04" (HH:MM)
}

// WeekdaySchedule groups intervals by day of week (0 = Sunday, 1 = Monday, ..., 6 = Saturday)
type WeekdaySchedule struct {
	Weekday   int                  `json:"weekday" binding:"min=0,max=6"`
	Intervals []OfficeHourInterval `json:"intervals" binding:"required"`
}

// UpdateOfficeHoursRequest is the payload for PUT /api/v1/doctors/:id/office-hours
type UpdateOfficeHoursRequest struct {
	Schedules []WeekdaySchedule `json:"schedules" binding:"required"`
}

// OfficeHourResponse represents persisted office hours
type OfficeHourResponse struct {
	ID        int32  `json:"id"`
	DoctorID  int32  `json:"doctor_id"`
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// CreateTimeOffRequest is the payload for POST /api/v1/doctors/:id/time-off
type CreateTimeOffRequest struct {
	StartsAt time.Time `json:"starts_at" binding:"required"`
	EndsAt   time.Time `json:"ends_at" binding:"required"`
	Reason   string    `json:"reason"`
}

// TimeOffResponse represents a scheduled leave range
type TimeOffResponse struct {
	ID       int32     `json:"id"`
	DoctorID int32     `json:"doctor_id"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Reason   string    `json:"reason"`
}

// DoctorScheduleResponse is the full schedule returned by GET /api/v1/doctors/:id/schedule
type DoctorScheduleResponse struct {
	DoctorID    int32                `json:"doctor_id"`
	OfficeHours []OfficeHourResponse `json:"office_hours"`
	TimeOff     []TimeOffResponse    `json:"upcoming_time_off"`
}

// Slot represents a discrete 15-minute appointment slot
type Slot struct {
	StartsAt         time.Time `json:"starts_at"`
	EndsAt           time.Time `json:"ends_at"`
	IsWalkInEligible bool      `json:"is_walk_in_eligible"` // True if this slot is the ongoing active slot
}

// DoctorAvailableSlotsResponse is returned by GET /api/v1/doctors/:id/available-slots
type DoctorAvailableSlotsResponse struct {
	DoctorID int32  `json:"doctor_id"`
	Slots    []Slot `json:"slots"`
}

// AffectedAppointment is returned in preview mode for conflicts
type AffectedAppointment struct {
	ID        int32     `json:"id"`
	PatientID int32     `json:"patient_id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
}

// UpdateOfficeHoursPreviewResponse is returned when ?preview=true is passed
type UpdateOfficeHoursPreviewResponse struct {
	ConflictsCount       int                   `json:"conflicts_count"`
	AffectedAppointments []AffectedAppointment `json:"affected_appointments"`
}
