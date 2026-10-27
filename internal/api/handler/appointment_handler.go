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

type AppointmentHandler struct {
	appointmentService *service.AppointmentService
}

func NewAppointmentHandler(appointmentService *service.AppointmentService) *AppointmentHandler {
	return &AppointmentHandler{
		appointmentService: appointmentService,
	}
}

// BookAppointment handles POST /api/v1/appointments
func (h *AppointmentHandler) BookAppointment(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}

	var req domain.BookAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	appt, err := h.appointmentService.BookAppointment(c.Request.Context(), callerID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusCreated, appt)
}

// ListAppointments handles GET /api/v1/appointments
func (h *AppointmentHandler) ListAppointments(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}
	callerRole, _ := middleware.GetUserRole(c)

	filter := domain.ListAppointmentsFilter{
		Limit:  20,
		Offset: 0,
	}

	if docParam := c.Query("doctor_id"); docParam != "" {
		if val, err := strconv.ParseInt(docParam, 10, 32); err == nil {
			id := int32(val)
			filter.DoctorID = &id
		}
	}
	if patParam := c.Query("patient_id"); patParam != "" {
		if val, err := strconv.ParseInt(patParam, 10, 32); err == nil {
			id := int32(val)
			filter.PatientID = &id
		}
	}
	if statusParam := c.Query("status"); statusParam != "" {
		st := domain.AppointmentStatus(statusParam)
		filter.Status = &st
	}
	if fromParam := c.Query("from"); fromParam != "" {
		if t, err := time.Parse(time.RFC3339, fromParam); err == nil {
			filter.From = &t
		}
	}
	if toParam := c.Query("to"); toParam != "" {
		if t, err := time.Parse(time.RFC3339, toParam); err == nil {
			filter.To = &t
		}
	}
	if limitParam := c.Query("limit"); limitParam != "" {
		if val, err := strconv.ParseInt(limitParam, 10, 32); err == nil && val > 0 {
			filter.Limit = int32(val)
		}
	}
	if pageParam := c.Query("page"); pageParam != "" {
		if val, err := strconv.ParseInt(pageParam, 10, 32); err == nil && val > 0 {
			filter.Offset = int32(val-1) * filter.Limit
		}
	}

	result, err := h.appointmentService.ListAppointments(c.Request.Context(), callerID, callerRole, filter)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, result)
}

// GetAppointment handles GET /api/v1/appointments/:id
func (h *AppointmentHandler) GetAppointment(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}
	callerRole, _ := middleware.GetUserRole(c)

	apptID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Appointment ID must be an integer")
		return
	}

	appt, err := h.appointmentService.GetAppointment(c.Request.Context(), callerID, callerRole, int32(apptID))
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, appt)
}

// CheckIn handles POST /api/v1/appointments/:id/check-in
func (h *AppointmentHandler) CheckIn(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}

	apptID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Appointment ID must be an integer")
		return
	}

	appt, err := h.appointmentService.CheckInAppointment(c.Request.Context(), callerID, int32(apptID))
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, appt)
}

// Complete handles POST /api/v1/appointments/:id/complete
func (h *AppointmentHandler) Complete(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}
	callerRole, _ := middleware.GetUserRole(c)

	apptID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Appointment ID must be an integer")
		return
	}

	appt, err := h.appointmentService.CompleteAppointment(c.Request.Context(), callerID, callerRole, int32(apptID))
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, appt)
}

// Cancel handles POST /api/v1/appointments/:id/cancel
func (h *AppointmentHandler) Cancel(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}
	callerRole, _ := middleware.GetUserRole(c)

	apptID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Appointment ID must be an integer")
		return
	}

	var req domain.CancelAppointmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	appt, err := h.appointmentService.CancelAppointment(c.Request.Context(), callerID, callerRole, int32(apptID), req.Reason)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, appt)
}

// RevertNoShow handles POST /api/v1/appointments/:id/revert-no-show
func (h *AppointmentHandler) RevertNoShow(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}
	callerRole, _ := middleware.GetUserRole(c)

	apptID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Appointment ID must be an integer")
		return
	}

	appt, err := h.appointmentService.RevertNoShow(c.Request.Context(), callerID, callerRole, int32(apptID))
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, appt)
}

// GetHistory handles GET /api/v1/appointments/:id/history
func (h *AppointmentHandler) GetHistory(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}
	callerRole, _ := middleware.GetUserRole(c)

	apptID, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Appointment ID must be an integer")
		return
	}

	events, err := h.appointmentService.GetAppointmentHistory(c.Request.Context(), callerID, callerRole, int32(apptID))
	if err != nil {
		h.handleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, events)
}

// handleError standardizes appointment service error mapping
func (h *AppointmentHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrSlotAlreadyBooked):
		response.Error(c, http.StatusConflict, "SLOT_CONFLICT", err.Error())
	case errors.Is(err, domain.ErrAppointmentNotFound):
		response.Error(c, http.StatusNotFound, "APPOINTMENT_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrDoctorNotFound):
		response.Error(c, http.StatusNotFound, "DOCTOR_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrPatientNotFound):
		response.Error(c, http.StatusNotFound, "PATIENT_NOT_FOUND", err.Error())
	case errors.Is(err, domain.ErrDoctorNotActive):
		response.Error(c, http.StatusBadRequest, "DOCTOR_NOT_ACTIVE", err.Error())
	case errors.Is(err, domain.ErrInvalidSlotDuration):
		response.Error(c, http.StatusBadRequest, "INVALID_SLOT_DURATION", err.Error())
	case errors.Is(err, domain.ErrSlotInPast):
		response.Error(c, http.StatusBadRequest, "SLOT_IN_PAST", err.Error())
	case errors.Is(err, domain.ErrBookingWindowExceeded):
		response.Error(c, http.StatusBadRequest, "BOOKING_WINDOW_EXCEEDED", err.Error())
	case errors.Is(err, domain.ErrOutsideOfficeHours):
		response.Error(c, http.StatusUnprocessableEntity, "OUTSIDE_OFFICE_HOURS", err.Error())
	case errors.Is(err, domain.ErrDoctorTimeOffConflict):
		response.Error(c, http.StatusConflict, "DOCTOR_TIME_OFF_CONFLICT", err.Error())
	case errors.Is(err, domain.ErrCancellationCutoffExceeded):
		response.Error(c, http.StatusUnprocessableEntity, "CANCELLATION_CUTOFF_EXCEEDED", err.Error())
	case errors.Is(err, domain.ErrInvalidStatusTransition):
		response.Error(c, http.StatusConflict, "INVALID_STATUS_TRANSITION", err.Error())
	case errors.Is(err, domain.ErrCannotCancelCompleted):
		response.Error(c, http.StatusConflict, "CANNOT_CANCEL_APPOINTMENT", err.Error())
	case errors.Is(err, domain.ErrUnauthorizedAppointmentAction):
		response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
	}
}
