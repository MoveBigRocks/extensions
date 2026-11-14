package observabilityhandlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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
	page, err := h.Service.ReadIssuePage(c.Request.Context(), ws, c.Query("project"), c.Query("status"), c.Query("level"), c.Query("environment"), c.Query("release"), limit, offset)
	if err != nil {
		issueReadError(c, err)
		return
	}
	// Echo the event filters so clients can tell an applied filter from an
	// older runtime that ignored an unknown query parameter.
	filters := gin.H{"environment": c.Query("environment"), "release": c.Query("release")}
	c.JSON(http.StatusOK, gin.H{"contract": services.IssueReadContract, "workspaceId": ws, "issues": page, "filters": filters})
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

func queryInt(c *gin.Context, name string, fallback int) (int, bool) {
	raw, present := c.GetQuery(name)
	if !present {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		issueReadError(c, services.ErrInvalidIssueRead)
		return 0, false
	}
	return value, true
}

// Events pages one issue's events: identity, timing, environment, release and
// allowlisted tags only.
func (h *IssueReadHandler) Events(c *gin.Context) {
	ws, ok := issueWorkspace(c)
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	page, err := h.Service.ReadIssueEvents(c.Request.Context(), ws, c.Param("id"), limit, c.Query("after"))
	if err != nil {
		issueReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": services.IssueReadContract, "workspaceId": ws, "issueId": c.Param("id"), "events": page})
}

// FindEvents looks up events by one exact allowlisted tag key and value.
func (h *IssueReadHandler) FindEvents(c *gin.Context) {
	ws, ok := issueWorkspace(c)
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", 20)
	if !ok {
		return
	}
	sinceHours, ok := queryInt(c, "sinceHours", 0)
	if !ok {
		return
	}
	page, err := h.Service.FindEventsByTag(c.Request.Context(), ws, services.EventTagSearch{
		Key: c.Query("tag"), Value: c.Query("value"), Environment: c.Query("environment"), Release: c.Query("release"),
		SinceHours: sinceHours, Limit: limit, After: c.Query("after"),
	}, time.Now())
	if err != nil {
		issueReadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"contract": services.IssueReadContract, "workspaceId": ws, "events": page})
}
