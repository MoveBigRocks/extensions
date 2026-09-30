package observabilityhandlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/apierrors"
	services "github.com/movebigrocks/extensions/error-tracking/runtime/services"
)

type IssueReadHandler struct{ Service *services.IssueService }

func issueWorkspace(c *gin.Context) (string, bool) {
	ws := c.GetString("workspace_id")
	if ws == "" || c.Query("workspace") != ws {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "forbidden"}})
		return "", false
	}
	return ws, true
}
func issueReadError(c *gin.Context, err error) {
	code, status := "unavailable", http.StatusServiceUnavailable
	if errors.Is(err, services.ErrInvalidIssueRead) {
		code, status = "invalid_request", http.StatusBadRequest
	}
	if errors.Is(err, &apierrors.APIError{Type: apierrors.ErrorTypeNotFound}) {
		code, status = "not_found", http.StatusNotFound
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code}})
}
func (h *IssueReadHandler) List(c *gin.Context) {
	ws, ok := issueWorkspace(c)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil {
		issueReadError(c, services.ErrInvalidIssueRead)
		return
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		issueReadError(c, services.ErrInvalidIssueRead)
		return
	}
	page, err := h.Service.ReadIssuePage(c.Request.Context(), ws, c.Query("project"), c.Query("status"), c.Query("level"), limit, offset)
	if err != nil {
		issueReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": services.IssueReadContract, "workspaceId": ws, "issues": page})
}
func (h *IssueReadHandler) Get(c *gin.Context) {
	ws, ok := issueWorkspace(c)
	if !ok {
		return
	}
	evidence, err := h.Service.ReadIssueEvidence(c.Request.Context(), ws, c.Param("id"))
	if err != nil {
		issueReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": services.IssueReadContract, "workspaceId": ws, "evidence": evidence})
}
