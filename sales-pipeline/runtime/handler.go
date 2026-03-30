package salespipeline

import (
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/runtimehttp"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{
		store: store,
	}
}

func (h *Handler) ShowDashboard(c *gin.Context) {
	data := runtimehttp.BuildBasePageData(c, "sales-pipeline", "Sales Pipeline", "Track active opportunities and totals by stage.")
	c.HTML(http.StatusOK, "dashboard.html", data)
}

func (h *Handler) GetBoard(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace context is required"})
		return
	}

	board, err := h.store.Board(c.Request.Context(), workspaceID, h.extensionMode(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, board)
}

func (h *Handler) CreateDeal(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace context is required"})
		return
	}

	var request struct {
		Title            string  `json:"title"`
		OrganizationName string  `json:"organizationName"`
		ContactName      string  `json:"contactName"`
		ContactEmail     string  `json:"contactEmail"`
		LinkedCaseID     string  `json:"linkedCaseId"`
		Value            float64 `json:"value"`
		Currency         string  `json:"currency"`
		CloseDate        string  `json:"closeDate"`
		WinProbability   int     `json:"winProbability"`
		Notes            string  `json:"notes"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var closeDate *time.Time
	if raw := strings.TrimSpace(request.CloseDate); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "closeDate must use YYYY-MM-DD"})
			return
		}
		closeDate = &parsed
	}

	deal, err := h.store.CreateDeal(c.Request.Context(), CreateDealInput{
		WorkspaceID:      workspaceID,
		Title:            request.Title,
		OrganizationName: request.OrganizationName,
		ContactName:      request.ContactName,
		ContactEmail:     request.ContactEmail,
		LinkedCaseID:     request.LinkedCaseID,
		ValueCents:       dollarsToCents(request.Value),
		Currency:         request.Currency,
		CloseDate:        closeDate,
		WinProbability:   request.WinProbability,
		Notes:            request.Notes,
	}, h.extensionMode(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, deal)
}

func (h *Handler) MoveDeal(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace context is required"})
		return
	}

	var request struct {
		StageID string `json:"stageId"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	deal, err := h.store.MoveDeal(c.Request.Context(), workspaceID, c.Param("id"), request.StageID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, deal)
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "sales pipeline runtime ready",
	})
}

func (h *Handler) extensionMode(c *gin.Context) string {
	if configured, ok := runtimehttp.ExtensionConfigString(c, "mode"); ok {
		return strings.ToLower(strings.TrimSpace(configured))
	}
	return "b2b"
}

func dollarsToCents(value float64) int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int64(math.Round(value * 100))
}
