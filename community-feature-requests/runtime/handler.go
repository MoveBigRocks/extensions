package communityrequests

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/movebigrocks/platform/extensions/common/runtimehttp"
)

const voterCookieName = "mbr_feature_request_voter"

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ShowBoard(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.String(http.StatusBadRequest, "workspace context is required")
		return
	}

	ideas, err := h.store.ListIdeas(c.Request.Context(), workspaceID, ListOptions{
		PublicOnly: true,
		Status:     c.Query("status"),
		Search:     c.Query("q"),
		Sort:       c.DefaultQuery("sort", "top"),
	})
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.HTML(http.StatusOK, "board.html", gin.H{
		"Ideas":         ideas,
		"Status":        c.Query("status"),
		"Query":         c.Query("q"),
		"Sort":          c.DefaultQuery("sort", "top"),
		"StatusOptions": ValidStatuses,
	})
}

func (h *Handler) ShowDetail(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.String(http.StatusBadRequest, "workspace context is required")
		return
	}

	idea, err := h.store.GetIdeaBySlug(c.Request.Context(), workspaceID, c.Param("slug"), true)
	if err != nil {
		c.String(http.StatusNotFound, "idea not found")
		return
	}

	c.HTML(http.StatusOK, "detail.html", gin.H{
		"Idea": idea,
	})
}

func (h *Handler) SubmitIdea(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.String(http.StatusBadRequest, "workspace context is required")
		return
	}

	idea, err := h.store.CreateIdea(c.Request.Context(), CreateIdeaInput{
		WorkspaceID:         workspaceID,
		Title:               c.PostForm("title"),
		DescriptionMarkdown: c.PostForm("description"),
		SubmitterName:       c.PostForm("name"),
		SubmitterEmail:      c.PostForm("email"),
		IsPublic:            true,
	})
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	c.Redirect(http.StatusSeeOther, "/community/ideas/"+idea.Slug)
}

func (h *Handler) VoteIdea(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.String(http.StatusBadRequest, "workspace context is required")
		return
	}

	voterKey, err := resolveVoterKey(c)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	idea, _, err := h.store.AddVote(c.Request.Context(), workspaceID, c.Param("slug"), voterKey)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	c.Redirect(http.StatusSeeOther, "/community/ideas/"+idea.Slug)
}

func (h *Handler) ShowAdminDashboard(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.String(http.StatusBadRequest, "workspace context is required")
		return
	}

	ideas, err := h.store.ListIdeas(c.Request.Context(), workspaceID, ListOptions{
		PublicOnly: false,
		Sort:       "status",
	})
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	data := runtimehttp.BuildBasePageData(c, "community-feature-requests", "Community Feature Requests", "Review incoming ideas and update public status.")
	data["Ideas"] = ideas
	data["StatusOptions"] = ValidStatuses
	c.HTML(http.StatusOK, "admin_dashboard.html", data)
}

func (h *Handler) UpdateIdea(c *gin.Context) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.String(http.StatusBadRequest, "workspace context is required")
		return
	}

	_, err := h.store.UpdateIdea(c.Request.Context(), workspaceID, c.Param("id"), UpdateIdeaInput{
		Status:   c.PostForm("status"),
		IsPublic: c.PostForm("isPublic") == "on",
	})
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	c.Redirect(http.StatusSeeOther, "/extensions/community-feature-requests")
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "community feature requests runtime ready",
	})
}

func resolveVoterKey(c *gin.Context) (string, error) {
	if existing, err := c.Cookie(voterCookieName); err == nil && strings.TrimSpace(existing) != "" {
		return strings.TrimSpace(existing), nil
	}
	key := uuid.NewString()
	c.SetCookie(voterCookieName, key, 60*60*24*365, "/", "", false, true)
	return key, nil
}
