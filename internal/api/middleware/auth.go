package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HarmanSingh16/HMS-backend/internal/api/response"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserIDKey = "user_id"
	ContextRoleKey   = "user_role"
	ContextEmailKey  = "user_email"
)

func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "MISSING_AUTH_HEADER", "Authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, "INVALID_AUTH_HEADER", "Authorization header format must be Bearer {token}")
			c.Abort()
			return
		}

		tokenString := parts[1]

		token, err := jwt.ParseWithClaims(tokenString, &domain.UserClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, domain.ErrInvalidToken
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			if errors.Is(err, jwt.ErrTokenExpired) {
				response.Error(c, http.StatusUnauthorized, "TOKEN_EXPIRED", "Token has expired")
			} else {
				response.Error(c, http.StatusUnauthorized, "INVALID_TOKEN", "Token is invalid")
			}
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*domain.UserClaims)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "INVALID_CLAIMS", "Failed to extract token claims")
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextRoleKey, claims.Role)
		c.Set(ContextEmailKey, claims.Email)
		c.Next()
	}
}

// Helper functions
func GetUserID(c *gin.Context) (int32, bool) {
	val, exists := c.Get(ContextUserIDKey)
	if !exists {
		return 0, false
	}

	id, ok := val.(int32)
	return id, ok
}

func GetUserRole(c *gin.Context) (domain.Role, bool) {
	val, exists := c.Get(ContextRoleKey)
	if !exists {
		return "", false
	}
	role, ok := val.(domain.Role)
	return role, ok
}
