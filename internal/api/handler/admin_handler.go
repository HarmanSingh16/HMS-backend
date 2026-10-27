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

type AdminHandler struct {
	accountService *service.AccountService
}

func NewAdminHandler(accountService *service.AccountService) *AdminHandler {
	return &AdminHandler{
		accountService: accountService,
	}
}

// GET /api/v1/admin/departments
func (h *AdminHandler) ListDepartments(c *gin.Context) {
	departments, err := h.accountService.ListDepartments(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve departments")
		return
	}

	response.Success(c, http.StatusOK, departments)
}

// POST /api/v1/admin/departments
func (h *AdminHandler) CreateDepartment(c *gin.Context) {
	var req domain.CreateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	actorID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}

	dept, err := h.accountService.CreateDepartment(c.Request.Context(), req, actorID)
	if err != nil {
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrDepartmentNameTaken):
				response.Error(c, http.StatusConflict, "DEPARTMENT_NAME_TAKEN", err.Error())
			default:
				response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create department")
			}
			return
		}
	}

	response.Success(c, http.StatusCreated, dept)
}

func (h *AdminHandler) UpdateDepartment(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 32)

	var req domain.UpdateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	actorID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}
	dept, err := h.accountService.UpdateDepartment(c.Request.Context(), int32(id), req, actorID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDepartmentNotFound):
			response.Error(c, http.StatusNotFound, "DEPARTMENT_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrDepartmentNameTaken):
			response.Error(c, http.StatusConflict, "DEPARTMENT_NAME_TAKEN", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update department")
		}
		return
	}
	response.Success(c, http.StatusOK, dept)
}
