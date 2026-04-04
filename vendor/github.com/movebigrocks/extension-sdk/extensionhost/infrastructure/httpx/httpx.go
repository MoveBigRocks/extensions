package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RespondWithError sends a JSON error response and aborts the request.
func RespondWithError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error":  message,
		"status": statusCode,
	})
	c.Abort()
}

// ValidateUUIDParam validates that the named path parameter is a UUID.
func ValidateUUIDParam(c *gin.Context, paramName string) string {
	value := c.Param(paramName)
	if value == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": paramName + " is required",
		})
		c.Abort()
		return ""
	}
	if _, err := uuid.Parse(value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Bad Request",
			"message": paramName + " must be a valid UUID",
		})
		c.Abort()
		return ""
	}
	return value
}
