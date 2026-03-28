package atsruntime

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	atsdomain "github.com/movebigrocks/platform/extensions/ats/runtime/domain"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(engine *gin.Engine, handler *Handler) {
	engine.GET("/extensions/ats/api/jobs", handler.ListJobs)
	engine.POST("/extensions/ats/api/jobs", handler.CreateJob)
	engine.POST("/extensions/ats/api/jobs/:id/publish", handler.PublishJob)
	engine.POST("/extensions/ats/api/jobs/:id/close", handler.CloseJob)
	engine.POST("/extensions/ats/api/jobs/:id/reopen", handler.ReopenJob)
	engine.GET("/extensions/ats/api/jobs/:id/applications", handler.ListApplications)
	engine.GET("/extensions/ats/api/defaults", handler.GetWorkspaceDefaults)
	engine.POST("/extensions/ats/api/applications/:id/stage", handler.ChangeCandidateStage)
	engine.POST("/extensions/ats/api/applications/:id/notes", handler.AddRecruiterNote)
	engine.POST("/careers/applications", handler.SubmitApplication)
	engine.GET("/extensions/ats/health", handler.Health)
	engine.HEAD("/extensions/ats/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
}

func (h *Handler) ListJobs(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	vacancies, err := h.service.ListJobs(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": vacancies})
}

func (h *Handler) CreateJob(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		Slug           string `json:"slug"`
		Title          string `json:"title"`
		Team           string `json:"team"`
		Location       string `json:"location"`
		WorkMode       string `json:"workMode"`
		EmploymentType string `json:"employmentType"`
		Summary        string `json:"summary"`
		Description    string `json:"description"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	job, err := h.service.CreateJob(c.Request.Context(), CreateJobInput{
		WorkspaceID:    workspaceID,
		Slug:           request.Slug,
		Title:          request.Title,
		Team:           request.Team,
		Location:       request.Location,
		WorkMode:       atsdomain.WorkMode(strings.TrimSpace(request.WorkMode)),
		EmploymentType: atsdomain.EmploymentType(strings.TrimSpace(request.EmploymentType)),
		Summary:        request.Summary,
		Description:    request.Description,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, job)
}

func (h *Handler) PublishJob(c *gin.Context) {
	h.mutateJob(c, func(workspaceID string) (*Vacancy, error) {
		return h.service.PublishJob(c.Request.Context(), workspaceID, c.Param("id"), occurredAt(timeNow()))
	})
}

func (h *Handler) CloseJob(c *gin.Context) {
	h.mutateJob(c, func(workspaceID string) (*Vacancy, error) {
		return h.service.CloseJob(c.Request.Context(), workspaceID, c.Param("id"), occurredAt(timeNow()))
	})
}

func (h *Handler) ReopenJob(c *gin.Context) {
	h.mutateJob(c, func(workspaceID string) (*Vacancy, error) {
		return h.service.ReopenJob(c.Request.Context(), workspaceID, c.Param("id"), occurredAt(timeNow()))
	})
}

func (h *Handler) ListApplications(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	profiles, err := h.service.ListCandidates(c.Request.Context(), workspaceID, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"applications": profiles})
}

func (h *Handler) GetWorkspaceDefaults(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	defaults, err := h.service.WorkspaceDefaults(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, defaults)
}

func (h *Handler) SubmitApplication(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	input := SubmitApplicationInput{
		WorkspaceID: workspaceID,
		Submission: atsdomain.CandidateSubmission{
			FullName:           strings.TrimSpace(c.PostForm("fullName")),
			Email:              strings.TrimSpace(c.PostForm("email")),
			Phone:              strings.TrimSpace(c.PostForm("phone")),
			Location:           strings.TrimSpace(c.PostForm("location")),
			LinkedInURL:        strings.TrimSpace(c.PostForm("linkedinUrl")),
			PortfolioURL:       strings.TrimSpace(c.PostForm("portfolioUrl")),
			CoverNote:          strings.TrimSpace(c.PostForm("coverNote")),
			ResumeAttachmentID: strings.TrimSpace(c.PostForm("resumeAttachmentId")),
			Source:             "careers_runtime",
		},
	}
	input.VacancySlug = strings.TrimSpace(c.PostForm("vacancySlug"))
	if input.VacancySlug == "" {
		input.VacancySlug = strings.TrimSpace(c.PostForm("roleSlug"))
	}
	if c.ContentType() == "application/json" {
		var request struct {
			VacancySlug        string `json:"vacancySlug"`
			RoleSlug           string `json:"roleSlug"`
			FullName           string `json:"fullName"`
			Email              string `json:"email"`
			Phone              string `json:"phone"`
			Location           string `json:"location"`
			LinkedInURL        string `json:"linkedinUrl"`
			PortfolioURL       string `json:"portfolioUrl"`
			CoverNote          string `json:"coverNote"`
			ResumeAttachmentID string `json:"resumeAttachmentId"`
			Source             string `json:"source"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		input.VacancySlug = firstNonBlank(request.VacancySlug, request.RoleSlug)
		input.Submission = atsdomain.CandidateSubmission{
			FullName:           request.FullName,
			Email:              request.Email,
			Phone:              request.Phone,
			Location:           request.Location,
			LinkedInURL:        request.LinkedInURL,
			PortfolioURL:       request.PortfolioURL,
			CoverNote:          request.CoverNote,
			ResumeAttachmentID: request.ResumeAttachmentID,
			Source:             firstNonBlank(request.Source, "careers_runtime"),
		}
	}
	result, err := h.service.SubmitApplication(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *Handler) ChangeCandidateStage(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		Stage     string `json:"stage"`
		ActorName string `json:"actorName"`
		ActorType string `json:"actorType"`
		Reason    string `json:"reason"`
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	application, err := h.service.ChangeCandidateStage(c.Request.Context(), workspaceID, c.Param("id"), StageChangeInput{
		Stage:     atsdomain.ApplicationStage(strings.TrimSpace(request.Stage)),
		ActorName: request.ActorName,
		ActorType: request.ActorType,
		Reason:    request.Reason,
		Note:      request.Note,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, application)
}

func (h *Handler) AddRecruiterNote(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		Body       string `json:"body"`
		AuthorName string `json:"authorName"`
		AuthorType string `json:"authorType"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	note, err := h.service.AddRecruiterNote(c.Request.Context(), workspaceID, c.Param("id"), request.Body, request.AuthorName, request.AuthorType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, note)
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "ats runtime ready",
	})
}

func (h *Handler) workspaceID(c *gin.Context) (string, bool) {
	workspaceID := strings.TrimSpace(c.GetString("workspace_id"))
	if workspaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace context is required"})
		return "", false
	}
	return workspaceID, true
}

func (h *Handler) mutateJob(c *gin.Context, fn func(workspaceID string) (*Vacancy, error)) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	vacancy, err := fn(workspaceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, vacancy)
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func timeNow() time.Time {
	return time.Now().UTC()
}
