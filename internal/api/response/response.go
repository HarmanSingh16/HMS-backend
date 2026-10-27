package response

import "github.com/gin-gonic/gin"

type APIResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Success(c *gin.Context, statusCode int, data any) {
	c.JSON(statusCode, APIResponse{
		Success: true,
		Data:    data,
	})
}

func Error(c *gin.Context, statusCode int, code string, message string) {
	c.JSON(statusCode, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}
