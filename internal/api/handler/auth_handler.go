package handler

import (
	"errors"
	"net/http"

	"github.com/HarmanSingh16/HMS-backend/internal/api/response"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/HarmanSingh16/HMS-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

// Why are doing this?? passing the service as argument again and again
func NewAuthHanlder(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// login - POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	tokens, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		case errors.Is(err, domain.ErrAccountDeactivated):
			response.Error(c, http.StatusForbidden, "ACCOUNT_DEACTIVATED", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occured")
		}
		return
	}

	response.Success(c, http.StatusOK, tokens)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req *domain.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	tokens, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidToken):
			response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
		case errors.Is(err, domain.ErrAccountDeactivated):
			response.Error(c, http.StatusForbidden, "ACCOUNT_DEACTIVATED", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occured")
		}
		return
	}

	response.Success(c, http.StatusOK, tokens)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	response.Success(c, http.StatusOK, gin.H{"message": "Logged Out Successfully"})
}

func (h *AuthHandler) VerifyInvite(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "Invite token in required")
		return
	}

	inviteDetails, err := h.authService.VerifyInvite(c.Request.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInviteNotFound):
			response.Error(c, http.StatusNotFound, "INVITE_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrInviteAlreadyClaimed):
			response.Error(c, http.StatusConflict, "INVITE_ALREADY_CLAIMED", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occured")
		}
	}

	response.Success(c, http.StatusOK, inviteDetails)
}

func (h *AuthHandler) ClaimInvite(c *gin.Context) {
	var req domain.ClaimInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := h.authService.ClaimInvite(c.Request.Context(), req); err != nil {
		switch {
		case errors.Is(err, domain.ErrInviteNotFound):
			response.Error(c, http.StatusNotFound, "INVITE_NOT_FOUND", err.Error())
		case errors.Is(err, domain.ErrInviteAlreadyClaimed):
			response.Error(c, http.StatusConflict, "INVITE_ALREADY_CLAIMED", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred")
		}
		return
	}
	response.Success(c, http.StatusOK, gin.H{"message": "Account activated successfully"})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
		return
	}

	userID, ok := userIDVal.(int32)
	if !ok {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Invalid user ID in context")
		return
	}

	profile, err := h.authService.GetMe(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve user profile")
		return
	}

	response.Success(c, http.StatusOK, profile)
}
