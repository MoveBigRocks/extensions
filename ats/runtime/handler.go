package atsruntime

import (
	"mime"
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
	engine.PUT("/extensions/ats/api/jobs/:id", handler.UpdateJob)
	engine.POST("/extensions/ats/api/jobs/:id/publish", handler.PublishJob)
	engine.POST("/extensions/ats/api/jobs/:id/close", handler.CloseJob)
	engine.POST("/extensions/ats/api/jobs/:id/reopen", handler.ReopenJob)
	engine.GET("/extensions/ats/api/jobs/:id/applications", handler.ListApplications)
	engine.GET("/extensions/ats/api/applications", handler.ListAllApplications)
	engine.GET("/extensions/ats/api/defaults", handler.GetWorkspaceDefaults)
	engine.GET("/extensions/ats/api/careers", handler.GetCareersBundle)
	engine.PUT("/extensions/ats/api/careers/site", handler.SaveCareersSiteProfile)
	engine.PUT("/extensions/ats/api/careers/team", handler.SaveCareersTeam)
	engine.PUT("/extensions/ats/api/careers/gallery", handler.SaveCareersGallery)
	engine.POST("/extensions/ats/api/careers/assets", handler.UploadCareersMediaAsset)
	engine.POST("/extensions/ats/api/careers/publish", handler.PublishCareersSite)
	engine.GET("/extensions/ats/api/setup", handler.GetSetupStatus)
	engine.PUT("/extensions/ats/api/setup", handler.SaveSetupState)
	engine.POST("/extensions/ats/api/applications/:id/stage", handler.ChangeCandidateStage)
	engine.POST("/extensions/ats/api/applications/:id/notes", handler.AddRecruiterNote)
	engine.POST("/extensions/ats/api/applications/:id/route", handler.RouteCandidate)
	engine.POST("/careers/applications", handler.SubmitApplication)
	engine.POST("/careers/attachments", handler.UploadCareerAttachment)
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
		Slug                    string   `json:"slug"`
		Title                   string   `json:"title"`
		Team                    string   `json:"team"`
		Location                string   `json:"location"`
		WorkMode                string   `json:"workMode"`
		EmploymentType          string   `json:"employmentType"`
		Summary                 string   `json:"summary"`
		Description             string   `json:"description"`
		Language                string   `json:"language"`
		AboutTheJob             string   `json:"aboutTheJob"`
		Responsibilities        []string `json:"responsibilities"`
		ResponsibilitiesHeading string   `json:"responsibilitiesHeading"`
		AboutYou                string   `json:"aboutYou"`
		AboutYouHeading         string   `json:"aboutYouHeading"`
		Profile                 []string `json:"profile"`
		OffersIntro             string   `json:"offersIntro"`
		Offers                  []string `json:"offers"`
		OffersHeading           string   `json:"offersHeading"`
		Quote                   string   `json:"quote"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	job, err := h.service.CreateJob(c.Request.Context(), CreateJobInput{
		WorkspaceID:             workspaceID,
		Slug:                    request.Slug,
		Title:                   request.Title,
		Team:                    request.Team,
		Location:                request.Location,
		WorkMode:                atsdomain.WorkMode(strings.TrimSpace(request.WorkMode)),
		EmploymentType:          atsdomain.EmploymentType(strings.TrimSpace(request.EmploymentType)),
		Summary:                 request.Summary,
		Description:             request.Description,
		Language:                request.Language,
		AboutTheJob:             request.AboutTheJob,
		Responsibilities:        request.Responsibilities,
		ResponsibilitiesHeading: request.ResponsibilitiesHeading,
		AboutYou:                request.AboutYou,
		AboutYouHeading:         request.AboutYouHeading,
		Profile:                 request.Profile,
		OffersIntro:             request.OffersIntro,
		Offers:                  request.Offers,
		OffersHeading:           request.OffersHeading,
		Quote:                   request.Quote,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, job)
}

func (h *Handler) UpdateJob(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		Title                   string   `json:"title"`
		Team                    string   `json:"team"`
		Location                string   `json:"location"`
		WorkMode                string   `json:"workMode"`
		EmploymentType          string   `json:"employmentType"`
		Summary                 string   `json:"summary"`
		Description             string   `json:"description"`
		Language                string   `json:"language"`
		AboutTheJob             string   `json:"aboutTheJob"`
		Responsibilities        []string `json:"responsibilities"`
		ResponsibilitiesHeading string   `json:"responsibilitiesHeading"`
		AboutYou                string   `json:"aboutYou"`
		AboutYouHeading         string   `json:"aboutYouHeading"`
		Profile                 []string `json:"profile"`
		OffersIntro             string   `json:"offersIntro"`
		Offers                  []string `json:"offers"`
		OffersHeading           string   `json:"offersHeading"`
		Quote                   string   `json:"quote"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	job, err := h.service.UpdateJob(c.Request.Context(), workspaceID, c.Param("id"), UpdateJobInput{
		Title:                   request.Title,
		Team:                    request.Team,
		Location:                request.Location,
		WorkMode:                atsdomain.WorkMode(strings.TrimSpace(request.WorkMode)),
		EmploymentType:          atsdomain.EmploymentType(strings.TrimSpace(request.EmploymentType)),
		Summary:                 request.Summary,
		Description:             request.Description,
		Language:                request.Language,
		AboutTheJob:             request.AboutTheJob,
		Responsibilities:        request.Responsibilities,
		ResponsibilitiesHeading: request.ResponsibilitiesHeading,
		AboutYou:                request.AboutYou,
		AboutYouHeading:         request.AboutYouHeading,
		Profile:                 request.Profile,
		OffersIntro:             request.OffersIntro,
		Offers:                  request.Offers,
		OffersHeading:           request.OffersHeading,
		Quote:                   request.Quote,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
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
	profiles, err := h.service.ListCandidates(c.Request.Context(), workspaceID, CandidateListOptions{
		VacancyID: c.Param("id"),
		Scope:     CandidateListScopeJob,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"applications": profiles})
}

func (h *Handler) ListAllApplications(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	scope := CandidateListScope(strings.TrimSpace(strings.ToLower(c.Query("scope"))))
	if scope == "" {
		scope = CandidateListScopeAll
	}
	profiles, err := h.service.ListCandidates(c.Request.Context(), workspaceID, CandidateListOptions{
		VacancyID: strings.TrimSpace(c.Query("jobId")),
		Scope:     scope,
	})
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

func (h *Handler) GetCareersBundle(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	bundle, err := h.service.CareersSiteBundle(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (h *Handler) GetSetupStatus(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	status, err := h.service.SetupStatus(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) SaveSetupState(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		CurrentStep string `json:"currentStep"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	status, err := h.service.SaveSetupState(c.Request.Context(), workspaceID, request.CurrentStep)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) SaveCareersSiteProfile(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		CompanyName        string `json:"companyName"`
		SiteTitle          string `json:"siteTitle"`
		Tagline            string `json:"tagline"`
		MetaDescription    string `json:"metaDescription"`
		HeroEyebrow        string `json:"heroEyebrow"`
		HeroTitle          string `json:"heroTitle"`
		HeroBody           string `json:"heroBody"`
		HeroPrimaryLabel   string `json:"heroPrimaryLabel"`
		HeroPrimaryHref    string `json:"heroPrimaryHref"`
		HeroSecondaryLabel string `json:"heroSecondaryLabel"`
		HeroSecondaryHref  string `json:"heroSecondaryHref"`
		StoryHeading       string `json:"storyHeading"`
		StoryBody          string `json:"storyBody"`
		JobsHeading        string `json:"jobsHeading"`
		JobsIntro          string `json:"jobsIntro"`
		TeamHeading        string `json:"teamHeading"`
		TeamIntro          string `json:"teamIntro"`
		GalleryHeading     string `json:"galleryHeading"`
		GalleryIntro       string `json:"galleryIntro"`
		ContactEmail       string `json:"contactEmail"`
		WebsiteURL         string `json:"websiteUrl"`
		LinkedInURL        string `json:"linkedinUrl"`
		InstagramURL       string `json:"instagramUrl"`
		XURL               string `json:"xUrl"`
		LogoURL            string `json:"logoUrl"`
		HeroImageURL       string `json:"heroImageUrl"`
		OgImageURL         string `json:"ogImageUrl"`
		PrimaryColor       string `json:"primaryColor"`
		AccentColor        string `json:"accentColor"`
		SurfaceColor       string `json:"surfaceColor"`
		BackgroundColor    string `json:"backgroundColor"`
		TextColor          string `json:"textColor"`
		MutedColor         string `json:"mutedColor"`
		StreetAddress      string `json:"streetAddress"`
		AddressLocality    string `json:"addressLocality"`
		AddressRegion      string `json:"addressRegion"`
		PostalCode         string `json:"postalCode"`
		AddressCountry     string `json:"addressCountry"`
		PrivacyPolicyURL   string `json:"privacyPolicyUrl"`
		CustomCSS          string `json:"customCss"`
		CustomCSSEnabled   bool   `json:"customCssEnabled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	profile, err := h.service.SaveCareersSiteProfile(c.Request.Context(), UpsertCareersSiteInput{
		WorkspaceID:        workspaceID,
		CompanyName:        request.CompanyName,
		SiteTitle:          request.SiteTitle,
		Tagline:            request.Tagline,
		MetaDescription:    request.MetaDescription,
		HeroEyebrow:        request.HeroEyebrow,
		HeroTitle:          request.HeroTitle,
		HeroBody:           request.HeroBody,
		HeroPrimaryLabel:   request.HeroPrimaryLabel,
		HeroPrimaryHref:    request.HeroPrimaryHref,
		HeroSecondaryLabel: request.HeroSecondaryLabel,
		HeroSecondaryHref:  request.HeroSecondaryHref,
		StoryHeading:       request.StoryHeading,
		StoryBody:          request.StoryBody,
		JobsHeading:        request.JobsHeading,
		JobsIntro:          request.JobsIntro,
		TeamHeading:        request.TeamHeading,
		TeamIntro:          request.TeamIntro,
		GalleryHeading:     request.GalleryHeading,
		GalleryIntro:       request.GalleryIntro,
		ContactEmail:       request.ContactEmail,
		WebsiteURL:         request.WebsiteURL,
		LinkedInURL:        request.LinkedInURL,
		InstagramURL:       request.InstagramURL,
		XURL:               request.XURL,
		LogoURL:            request.LogoURL,
		HeroImageURL:       request.HeroImageURL,
		OgImageURL:         request.OgImageURL,
		PrimaryColor:       request.PrimaryColor,
		AccentColor:        request.AccentColor,
		SurfaceColor:       request.SurfaceColor,
		BackgroundColor:    request.BackgroundColor,
		TextColor:          request.TextColor,
		MutedColor:         request.MutedColor,
		StreetAddress:      request.StreetAddress,
		AddressLocality:    request.AddressLocality,
		AddressRegion:      request.AddressRegion,
		PostalCode:         request.PostalCode,
		AddressCountry:     request.AddressCountry,
		PrivacyPolicyURL:   request.PrivacyPolicyURL,
		CustomCSS:          request.CustomCSS,
		CustomCSSEnabled:   request.CustomCSSEnabled,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (h *Handler) SaveCareersTeam(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		Team []CareersTeamMember `json:"team"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	team, err := h.service.ReplaceCareersTeamMembers(c.Request.Context(), workspaceID, request.Team)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"team": team})
}

func (h *Handler) SaveCareersGallery(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		Gallery []CareersGalleryItem `json:"gallery"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	gallery, err := h.service.ReplaceCareersGalleryItems(c.Request.Context(), workspaceID, request.Gallery)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"gallery": gallery})
}

func (h *Handler) PublishCareersSite(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	if err := h.service.PublishCareersSite(c.Request.Context(), workspaceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"published": true, "previewUrl": "/careers"})
}

func (h *Handler) UploadCareersMediaAsset(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer file.Close()
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(pathExt(fileHeader.Filename)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	asset, err := h.service.UploadCareersMediaAsset(
		c.Request.Context(),
		workspaceID,
		strings.TrimSpace(c.PostForm("purpose")),
		fileHeader.Filename,
		contentType,
		fileHeader.Size,
		file,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, asset)
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
			SourceKind:         atsdomain.ApplicationSourceKindATSPublic,
			Source:             "careers_runtime",
			SourceRefID:        strings.TrimSpace(c.PostForm("sourceRefId")),
			FormSubmissionID:   strings.TrimSpace(c.PostForm("formSubmissionId")),
		},
	}
	input.VacancySlug = strings.TrimSpace(c.PostForm("vacancySlug"))
	if input.VacancySlug == "" {
		input.VacancySlug = strings.TrimSpace(c.PostForm("roleSlug"))
	}
	if strings.HasPrefix(c.ContentType(), "application/json") {
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
			SourceKind         string `json:"sourceKind"`
			Source             string `json:"source"`
			SourceRefID        string `json:"sourceRefId"`
			FormSubmissionID   string `json:"formSubmissionId"`
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
			SourceKind:         atsdomain.ApplicationSourceKind(firstNonBlank(request.SourceKind, string(atsdomain.ApplicationSourceKindATSPublic))),
			Source:             firstNonBlank(request.Source, "careers_runtime"),
			SourceRefID:        request.SourceRefID,
			FormSubmissionID:   request.FormSubmissionID,
		}
	}
	result, err := h.service.SubmitApplication(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *Handler) UploadCareerAttachment(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer file.Close()
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(pathExt(fileHeader.Filename)))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	attachment, err := h.service.UploadCareerAttachment(
		c.Request.Context(),
		workspaceID,
		fileHeader.Filename,
		contentType,
		"ATS public application resume",
		fileHeader.Size,
		file,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":          attachment.ID,
		"workspaceID": attachment.WorkspaceID,
		"filename":    attachment.Filename,
		"contentType": attachment.ContentType,
		"size":        attachment.Size,
		"status":      attachment.Status,
	})
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

func (h *Handler) RouteCandidate(c *gin.Context) {
	workspaceID, ok := h.workspaceID(c)
	if !ok {
		return
	}
	var request struct {
		Destination string `json:"destination"`
		ActorName   string `json:"actorName"`
		ActorType   string `json:"actorType"`
		Reason      string `json:"reason"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	application, err := h.service.RouteCandidate(c.Request.Context(), workspaceID, c.Param("id"), CandidateRouteInput{
		Destination: request.Destination,
		ActorName:   request.ActorName,
		ActorType:   request.ActorType,
		Reason:      request.Reason,
		Note:        request.Note,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, application)
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

func pathExt(name string) string {
	index := strings.LastIndex(name, ".")
	if index == -1 {
		return ""
	}
	return name[index:]
}
