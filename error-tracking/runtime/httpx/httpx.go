package httpx

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RespondWithError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": strings.TrimSpace(message)})
}

func ValidateUUIDParam(c *gin.Context, name string) string {
	value := strings.TrimSpace(c.Param(name))
	if _, err := uuid.Parse(value); err != nil {
		RespondWithError(c, http.StatusBadRequest, "Invalid "+name)
		return ""
	}
	return value
}
