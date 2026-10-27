package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/HarmanSingh16/HMS-backend/internal/api/middleware"
	"github.com/HarmanSingh16/HMS-backend/internal/api/response"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/HarmanSingh16/HMS-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type DoctorHandler struct {
	scheduleService *service.ScheduleService
}

func NewDoctorHandler(scheduleService *service.ScheduleService) *DoctorHandler {
	return &DoctorHandler{
		scheduleService: scheduleService,
	}
}

// ListDoctors handles GET /api/v1/doctors?department_id=1
func (h *DoctorHandler) ListDoctors(c *gin.Context) {
	var deptID *int32
	if deptParam := c.Query("department_id"); deptParam != "" {
		parsed, err := strconv.ParseInt(deptParam, 10, 32)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_DEPARTMENT_ID", "department_id must be an integer")
			return
		}
		val := int32(parsed)
		deptID = &val
	}

	doctors, err := h.scheduleService.ListDoctors(c.Request.Context(), deptID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve doctors")
		return
	}

	response.Success(c, http.StatusOK, doctors)
}

// GetDoctorSchedule handles GET /api/v1/doctors/:id/schedule
func (h *DoctorHandler) GetDoctorSchedule(c *gin.Context) {
	doctorID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Doctor ID must be an integer")
		return
	}

	schedule, err := h.scheduleService.GetDoctorSchedule(c.Request.Context(), int32(doctorID))
	if err != nil {
		if errors.Is(err, domain.ErrDoctorNotFound) {
			response.Error(c, http.StatusNotFound, "DOCTOR_NOT_FOUND", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve doctor schedule")
		return
	}

	response.Success(c, http.StatusOK, schedule)
}

// UpdateOfficeHours handles PUT /api/v1/doctors/:id/office-hours?preview=true
func (h *DoctorHandler) UpdateOfficeHours(c *gin.Context) {
	doctorID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Doctor ID must be an integer")
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}

	callerRole, ok := middleware.GetUserRole(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User role not found in context")
		return
	}

	// RBAC Invariant: Doctors can only update their own schedule; Admins can update any
	if callerRole == domain.RoleDoctor && callerID != int32(doctorID) {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "Doctors can only modify their own office hours")
		return
	}

	var req domain.UpdateOfficeHoursRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	preview := c.DefaultQuery("preview", "false") == "true"

	res, err := h.scheduleService.UpdateOfficeHours(c.Request.Context(), int32(doctorID), req, preview, callerID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDoctorNotFound):
			response.Error(c, http.StatusNotFound, "DOCTOR_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrInvalidWeekday):
			response.Error(c, http.StatusBadRequest, "INVALID_WEEKDAY", err.Error())
		case errors.Is(err, domain.ErrInvalidTimeRange):
			response.Error(c, http.StatusBadRequest, "INVALID_TIME_RANGE", err.Error())
		case errors.Is(err, domain.ErrInvalidTimeFormat):
			response.Error(c, http.StatusBadRequest, "INVALID_TIME_FORMAT", err.Error())
		case errors.Is(err, domain.ErrOverlappingOfficeHours):
			response.Error(c, http.StatusBadRequest, "OVERLAPPING_OFFICE_HOURS", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update office hours")
		}
		return
	}

	response.Success(c, http.StatusOK, res)
}

// CreateTimeOff handles POST /api/v1/doctors/:id/time-off
func (h *DoctorHandler) CreateTimeOff(c *gin.Context) {
	doctorID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Doctor ID must be an integer")
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}

	callerRole, ok := middleware.GetUserRole(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User role not found in context")
		return
	}

	// RBAC Invariant: Doctors can only book their own time-off; Admins can book for any
	if callerRole == domain.RoleDoctor && callerID != int32(doctorID) {
		response.Error(c, http.StatusForbidden, "FORBIDDEN", "Doctors can only book their own time-off")
		return
	}

	var req domain.CreateTimeOffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	res, err := h.scheduleService.CreateTimeOff(c.Request.Context(), int32(doctorID), req, callerID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDoctorNotFound):
			response.Error(c, http.StatusNotFound, "DOCTOR_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrInvalidTimeRange):
			response.Error(c, http.StatusBadRequest, "INVALID_TIME_RANGE", err.Error())
		case errors.Is(err, domain.ErrTimeOffInPast):
			response.Error(c, http.StatusBadRequest, "TIME_OFF_IN_PAST", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create time off")
		}
		return
	}

	response.Success(c, http.StatusCreated, res)
}

// GetAvailableSlots handles GET /api/v1/doctors/:id/available-slots?date=YYYY-MM-DD
func (h *DoctorHandler) GetAvailableSlots(c *gin.Context) {
	doctorID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Doctor ID must be an integer")
		return
	}

	var targetDate *time.Time
	if dateStr := c.Query("date"); dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_DATE_FORMAT", "date must be in YYYY-MM-DD format")
			return
		}
		targetDate = &parsedDate
	}

	res, err := h.scheduleService.ComputeAvailableSlots(c.Request.Context(), int32(doctorID), targetDate, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDoctorNotFound):
			response.Error(c, http.StatusNotFound, "DOCTOR_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrDoctorNotActive):
			response.Error(c, http.StatusBadRequest, "DOCTOR_NOT_ACTIVE", err.Error())
		case errors.Is(err, domain.ErrBookingWindowExceeded):
			response.Error(c, http.StatusBadRequest, "BOOKING_WINDOW_EXCEEDED", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to compute available slots")
		}
		return
	}

	response.Success(c, http.StatusOK, res)
}
