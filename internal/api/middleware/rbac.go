package middleware

import (
	"net/http"

	"github.com/HarmanSingh16/HMS-backend/internal/api/response"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/gin-gonic/gin"
)

func RequireRoles(allowedRoles ...domain.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRole(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "User identity not found in context")
			c.Abort()
			return
		}

		for _, allowed := range allowedRoles {
			if role == allowed {
				c.Next()
				return
			}
		}

		response.Error(c, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource")
		c.Abort()
	}
}
