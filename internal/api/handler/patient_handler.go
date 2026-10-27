package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/HarmanSingh16/HMS-backend/internal/api/middleware"
	"github.com/HarmanSingh16/HMS-backend/internal/api/response"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/HarmanSingh16/HMS-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type PatientHandler struct {
	patientService *service.PatientService
}

func NewPatientHandler(patientService *service.PatientService) *PatientHandler {
	return &PatientHandler{
		patientService: patientService,
	}
}

// RegisterPatient handles POST /api/v1/receptionist/patients
func (h *PatientHandler) RegisterPatient(c *gin.Context) {
	var req domain.RegisterPatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	actorID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}

	result, err := h.patientService.RegisterPatient(c.Request.Context(), req, actorID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmailAlreadyExists):
			response.Error(c, http.StatusConflict, "EMAIL_ALREADY_EXISTS", err.Error())
		case errors.Is(err, domain.ErrInvalidDateOfBirth):
			response.Error(c, http.StatusBadRequest, "INVALID_DATE_OF_BIRTH", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to register patient")
		}
		return
	}

	response.Success(c, http.StatusCreated, result)
}

// GetPatient handles GET /api/v1/patients/:id
func (h *PatientHandler) GetPatient(c *gin.Context) {
	idParam := c.Param("id")
	patientID, err := strconv.ParseInt(idParam, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Patient ID must be an integer")
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

	patient, err := h.patientService.GetPatientByID(c.Request.Context(), int32(patientID), callerID, callerRole)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPatientNotFound):
			response.Error(c, http.StatusNotFound, "PATIENT_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve patient")
		}
		return
	}

	response.Success(c, http.StatusOK, patient)
}

// SearchPatients handles GET /api/v1/patients
func (h *PatientHandler) SearchPatients(c *gin.Context) {
	var query domain.PatientSearchQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	callerRole, ok := middleware.GetUserRole(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User role not found in context")
		return
	}

	results, err := h.patientService.SearchPatients(c.Request.Context(), query, callerRole)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to search patients")
		}
		return
	}

	response.Success(c, http.StatusOK, results)
}

// UpdatePatient handles PUT /api/v1/patients/:id
func (h *PatientHandler) UpdatePatient(c *gin.Context) {
	idParam := c.Param("id")
	patientID, err := strconv.ParseInt(idParam, 10, 32)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Patient ID must be an integer")
		return
	}

	var req domain.UpdatePatientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
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

	updated, err := h.patientService.UpdatePatient(c.Request.Context(), int32(patientID), req, callerID, callerRole)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrPatientNotFound):
			response.Error(c, http.StatusNotFound, "PATIENT_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrInvalidDateOfBirth):
			response.Error(c, http.StatusBadRequest, "INVALID_DATE_OF_BIRTH", err.Error())
		case errors.Is(err, domain.ErrForbidden):
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update patient")
		}
		return
	}

	response.Success(c, http.StatusOK, updated)
}