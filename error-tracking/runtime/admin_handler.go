package sql

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"
	"github.com/movebigrocks/extension-sdk/runtimehttp"

	middleware "github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/httpx"
	platformadmin "github.com/movebigrocks/extension-sdk/extensionhost/platform/adminui"
	platformservices "github.com/movebigrocks/extension-sdk/extensionhost/platform/services"
	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	storecontracts "github.com/movebigrocks/extensions/error-tracking/runtime/storecontracts"
)

const (
	errorTrackingApplicationsBasePath = "/extensions/error-tracking/applications"
	errorTrackingIssuesBasePath       = "/extensions/error-tracking/issues"
)

type ErrorTrackingAdminHandler struct {
	workspaceService *platformservices.WorkspaceManagementService
	userService      *platformservices.UserManagementService
	extensionService *platformservices.ExtensionService
	issueService     adminIssueService
	projectService   adminProjectService
	apiBaseURL       string
}

type adminIssueService interface {
	ListWorkspaceIssues(ctx context.Context, workspaceID string, limit int) ([]*observabilitydomain.Issue, int, error)
	ListAllIssues(ctx context.Context, filters storecontracts.IssueFilters) ([]*observabilitydomain.Issue, int, error)
	GetIssueInWorkspace(ctx context.Context, workspaceID, issueID string) (*observabilitydomain.Issue, error)
	GetIssueWithProject(ctx context.Context, issueID string) (*observabilitydomain.Issue, *observabilitydomain.Project, error)
	GetIssueEvents(ctx context.Context, issueID string, limit int) ([]*observabilitydomain.ErrorEvent, error)
}

type adminProjectService interface {
	ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*observabilitydomain.Project, error)
	ListAllProjects(ctx context.Context) ([]*observabilitydomain.Project, error)
	GetProjectsByIDs(ctx context.Context, projectIDs []string) ([]*observabilitydomain.Project, error)
	GetProject(ctx context.Context, projectID string) (*observabilitydomain.Project, error)
	CreateProject(ctx context.Context, extensionInstallID string, project *observabilitydomain.Project) error
	UpdateProject(ctx context.Context, project *observabilitydomain.Project) error
	DeleteProject(ctx context.Context, workspaceID, projectID string) error
}

type ApplicationsPageData struct {
	platformadmin.BasePageData
	Applications         []ApplicationListItem
	TotalApplications    int
	ApplicationsBasePath string
}

type ApplicationListItem struct {
	ID            string
	Name          string
	Slug          string
	Platform      string
	Environment   string
	Status        string
	EventCount    int64
	WorkspaceID   string
	WorkspaceName string
}

type ApplicationDetailPageData struct {
	platformadmin.BasePageData
	Application          ApplicationDetailItem
	Workspaces           []platformadmin.WorkspaceOption
	IsNew                bool
	ApplicationsBasePath string
}

type ApplicationDetailItem struct {
	ID             string
	Name           string
	Slug           string
	Platform       string
	Environment    string
	Status         string
	DSN            string
	PublicKey      string
	WorkspaceID    string
	WorkspaceName  string
	EventsPerHour  int
	StorageQuotaMB int
	RetentionDays  int
	EventCount     int64
	CreatedAt      time.Time
}

type IssuesPageData struct {
	platformadmin.BasePageData
	Issues         []*observabilitydomain.Issue
	TotalIssues    int
	ProjectNames   map[string]string
	IssuesBasePath string
}

type IssueDetailPageData struct {
	platformadmin.BasePageData
	Issue          *observabilitydomain.Issue
	Events         []*observabilitydomain.ErrorEvent
	WorkspaceName  string
	ProjectName    string
	IssuesBasePath string
}

func NewErrorTrackingAdminHandler(
	workspaceService *platformservices.WorkspaceManagementService,
	userService *platformservices.UserManagementService,
	extensionService *platformservices.ExtensionService,
	issueService adminIssueService,
	projectService adminProjectService,
	apiBaseURL string,
) *ErrorTrackingAdminHandler {
	return &ErrorTrackingAdminHandler{
		workspaceService: workspaceService,
		userService:      userService,
		extensionService: extensionService,
		issueService:     issueService,
		projectService:   projectService,
		apiBaseURL:       strings.TrimSpace(apiBaseURL),
	}
}

func (h *ErrorTrackingAdminHandler) ShowApplications(c *gin.Context) {
	ctx := c.Request.Context()
	pageSubtitle := "View all monitored applications across workspaces"
	workspaceNames := make(map[string]string)

	var (
		projects []*observabilitydomain.Project
		err      error
	)
	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		projects, err = h.projectService.ListWorkspaceProjects(ctx, workspaceID)
		pageSubtitle = "View monitored applications for " + workspaceName
		workspaceNames[workspaceID] = workspaceName
	} else {
		projects, err = h.projectService.ListAllProjects(ctx)
		if err == nil {
			workspaceIDs := make([]string, 0, len(projects))
			for _, p := range projects {
				workspaceIDs = append(workspaceIDs, p.WorkspaceID)
			}
			workspaceNames = h.getWorkspaceNamesMap(ctx, workspaceIDs)
		}
	}
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", platformadmin.ErrorPageData{Error: "Failed to load applications"})
		return
	}

	apps := make([]ApplicationListItem, 0, len(projects))
	for _, project := range projects {
		apps = append(apps, ApplicationListItem{
			ID:            project.ID,
			Name:          project.Name,
			Slug:          project.Slug,
			Platform:      project.Platform,
			Environment:   project.Environment,
			Status:        project.Status,
			EventCount:    project.EventCount,
			WorkspaceID:   project.WorkspaceID,
			WorkspaceName: workspaceNames[project.WorkspaceID],
		})
	}

	c.HTML(http.StatusOK, "applications.html", ApplicationsPageData{
		BasePageData:         h.buildBasePageData(c, "applications", "Monitored Applications", pageSubtitle),
		Applications:         apps,
		TotalApplications:    len(apps),
		ApplicationsBasePath: errorTrackingApplicationsBasePath,
	})
}

func (h *ErrorTrackingAdminHandler) ShowApplicationDetail(c *gin.Context) {
	ctx := c.Request.Context()

	appID := c.Param("id")
	var currentWorkspaceID, currentWorkspaceName string
	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		currentWorkspaceID = workspaceID
		currentWorkspaceName = workspaceName
	}

	workspaceOptions := make([]platformadmin.WorkspaceOption, 0)
	if currentWorkspaceID != "" {
		workspaceOptions = append(workspaceOptions, platformadmin.WorkspaceOption{
			ID:   currentWorkspaceID,
			Name: currentWorkspaceName,
		})
	} else if h.workspaceService != nil {
		allWorkspaces, err := h.workspaceService.ListAllWorkspaces(ctx)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", platformadmin.ErrorPageData{Error: "Failed to load workspaces"})
			return
		}
		for _, ws := range allWorkspaces {
			workspaceOptions = append(workspaceOptions, platformadmin.WorkspaceOption{
				ID:   ws.ID,
				Name: ws.Name,
			})
		}
	}

	isNewApplication := appID == "new" || strings.HasSuffix(c.Request.URL.Path, "/new")
	if isNewApplication {
		c.HTML(http.StatusOK, "application_detail.html", ApplicationDetailPageData{
			BasePageData:         h.buildBasePageData(c, "applications", "New Application", "Create a new monitored application"),
			Workspaces:           workspaceOptions,
			IsNew:                true,
			ApplicationsBasePath: errorTrackingApplicationsBasePath,
		})
		return
	}

	appID = middleware.ValidateUUIDParam(c, "id")
	if appID == "" {
		return
	}

	project, err := h.projectService.GetProject(ctx, appID)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", platformadmin.ErrorPageData{Error: "Application not found"})
		return
	}
	if currentWorkspaceID != "" && project.WorkspaceID != currentWorkspaceID {
		c.HTML(http.StatusNotFound, "error.html", platformadmin.ErrorPageData{Error: "Application not found"})
		return
	}

	workspaceName := currentWorkspaceName
	if workspaceName == "" && h.workspaceService != nil {
		if ws, err := h.workspaceService.GetWorkspace(ctx, project.WorkspaceID); err == nil {
			workspaceName = ws.Name
		}
	}

	c.HTML(http.StatusOK, "application_detail.html", ApplicationDetailPageData{
		BasePageData: h.buildBasePageData(c, "applications", project.Name, "Manage application settings"),
		Application: ApplicationDetailItem{
			ID:             project.ID,
			Name:           project.Name,
			Slug:           project.Slug,
			Platform:       project.Platform,
			Environment:    project.Environment,
			Status:         project.Status,
			DSN:            project.DSN,
			PublicKey:      project.PublicKey,
			WorkspaceID:    project.WorkspaceID,
			WorkspaceName:  workspaceName,
			EventsPerHour:  project.EventsPerHour,
			StorageQuotaMB: project.StorageQuotaMB,
			RetentionDays:  project.RetentionDays,
			EventCount:     project.EventCount,
			CreatedAt:      project.CreatedAt,
		},
		Workspaces:           workspaceOptions,
		ApplicationsBasePath: errorTrackingApplicationsBasePath,
	})
}

func (h *ErrorTrackingAdminHandler) CreateApplication(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		WorkspaceID string `json:"workspaceID" binding:"required"`
		Platform    string `json:"platform"`
		Environment string `json:"environment"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format: name and workspaceID are required")
		return
	}
	if workspaceID, _, ok := currentWorkspaceScope(c); ok {
		if req.WorkspaceID == "" {
			req.WorkspaceID = workspaceID
		}
		if req.WorkspaceID != workspaceID {
			middleware.RespondWithError(c, http.StatusForbidden, "Workspace mismatch")
			return
		}
	}

	project := observabilitydomain.NewProject(req.WorkspaceID, "", req.Name, slug.Make(req.Name), req.Platform)
	project.DSN = observabilitydomain.BuildProjectDSN(h.apiBaseURL, project.PublicKey, project.ProjectNumber)
	if req.Environment != "" {
		project.Environment = req.Environment
	}

	extensionInstallID := runtimehttp.ExtensionID(c)
	if strings.TrimSpace(extensionInstallID) == "" {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Missing extension identity in runtime context")
		return
	}

	if err := h.projectService.CreateProject(c.Request.Context(), extensionInstallID, project); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to create application")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          project.ID,
		"name":        project.Name,
		"slug":        project.Slug,
		"dsn":         project.DSN,
		"publicKey":   project.PublicKey,
		"workspaceID": project.WorkspaceID,
	})
}

func (h *ErrorTrackingAdminHandler) UpdateApplication(c *gin.Context) {
	appID := middleware.ValidateUUIDParam(c, "id")
	if appID == "" {
		return
	}

	var req struct {
		Name           string `json:"name"`
		Platform       string `json:"platform"`
		Environment    string `json:"environment"`
		Status         string `json:"status"`
		EventsPerHour  int    `json:"eventsPerHour"`
		StorageQuotaMB int    `json:"storageQuotaMB"`
		RetentionDays  int    `json:"retentionDays"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	ctx := c.Request.Context()
	project, err := h.projectService.GetProject(ctx, appID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}
	if workspaceID, _, ok := currentWorkspaceScope(c); ok && project.WorkspaceID != workspaceID {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}

	if req.Name != "" {
		project.Name = req.Name
		project.Slug = slug.Make(req.Name)
	}
	if req.Platform != "" {
		project.Platform = req.Platform
	}
	if req.Environment != "" {
		project.Environment = req.Environment
	}
	if req.Status != "" {
		validStatuses := map[string]bool{"active": true, "paused": true, "disabled": true}
		status := strings.ToLower(req.Status)
		if !validStatuses[status] {
			middleware.RespondWithError(c, http.StatusBadRequest, "Invalid status. Must be: active, paused, or disabled")
			return
		}
		project.Status = status
	}
	if req.EventsPerHour > 0 {
		project.EventsPerHour = req.EventsPerHour
	}
	if req.StorageQuotaMB > 0 {
		project.StorageQuotaMB = req.StorageQuotaMB
	}
	if req.RetentionDays > 0 {
		project.RetentionDays = req.RetentionDays
	}
	if project.DSN == "" || strings.Contains(project.DSN, "@movebigrocks.com/") {
		project.DSN = observabilitydomain.BuildProjectDSN(h.apiBaseURL, project.PublicKey, project.ProjectNumber)
	}

	if err := h.projectService.UpdateProject(ctx, project); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to update application")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Application updated successfully"})
}

func (h *ErrorTrackingAdminHandler) DeleteApplication(c *gin.Context) {
	appID := middleware.ValidateUUIDParam(c, "id")
	if appID == "" {
		return
	}

	ctx := c.Request.Context()
	project, err := h.projectService.GetProject(ctx, appID)
	if err != nil {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}
	if workspaceID, _, ok := currentWorkspaceScope(c); ok && project.WorkspaceID != workspaceID {
		middleware.RespondWithError(c, http.StatusNotFound, "Application not found")
		return
	}

	if err := h.projectService.DeleteProject(ctx, project.WorkspaceID, appID); err != nil {
		middleware.RespondWithError(c, http.StatusInternalServerError, "Failed to delete application")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Application deleted successfully"})
}

func (h *ErrorTrackingAdminHandler) ShowIssues(c *gin.Context) {
	ctx := c.Request.Context()
	pageSubtitle := "View all error tracking issues across workspaces"
	workspaceNames := make(map[string]string)

	var (
		issues []*observabilitydomain.Issue
		err    error
	)
	if workspaceID, workspaceName, ok := currentWorkspaceScope(c); ok {
		issues, _, err = h.issueService.ListWorkspaceIssues(ctx, workspaceID, 100)
		pageSubtitle = "View error issues for " + workspaceName
		workspaceNames[workspaceID] = workspaceName
	} else {
		issues, _, err = h.issueService.ListAllIssues(ctx, storecontracts.IssueFilters{Limit: 100})
	}
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", platformadmin.ErrorPageData{Error: "Failed to load issues"})
		return
	}

	projectNames := make(map[string]string)
	projectIDs := make([]string, 0, len(issues))
	for _, issue := range issues {
		projectIDs = append(projectIDs, issue.ProjectID)
	}
	if len(projectIDs) > 0 {
		projects, _ := h.projectService.GetProjectsByIDs(ctx, projectIDs)
		for _, project := range projects {
			projectNames[project.ID] = project.Name
			if len(workspaceNames) == 0 && h.workspaceService != nil {
				if ws, err := h.workspaceService.GetWorkspace(ctx, project.WorkspaceID); err == nil {
					workspaceNames[project.WorkspaceID] = ws.Name
				}
			}
		}
	}

	c.HTML(http.StatusOK, "issues.html", IssuesPageData{
		BasePageData:   h.buildBasePageData(c, "issues", "Error Issues", pageSubtitle),
		Issues:         issues,
		TotalIssues:    len(issues),
		ProjectNames:   projectNames,
		IssuesBasePath: errorTrackingIssuesBasePath,
	})
}

func (h *ErrorTrackingAdminHandler) ShowIssueDetail(c *gin.Context) {
	ctx := c.Request.Context()
	issueID := middleware.ValidateUUIDParam(c, "id")
	if issueID == "" {
		return
	}

	var (
		issue         *observabilitydomain.Issue
		project       *observabilitydomain.Project
		workspaceName string
		err           error
	)

	if workspaceID, currentWorkspaceName, ok := currentWorkspaceScope(c); ok {
		issue, err = h.issueService.GetIssueInWorkspace(ctx, workspaceID, issueID)
		if err != nil || issue == nil {
			c.HTML(http.StatusNotFound, "error.html", platformadmin.ErrorPageData{Error: "Issue not found"})
			return
		}
		project, _ = h.projectService.GetProject(ctx, issue.ProjectID)
		if project == nil || project.WorkspaceID != workspaceID {
			c.HTML(http.StatusNotFound, "error.html", platformadmin.ErrorPageData{Error: "Issue not found"})
			return
		}
		workspaceName = currentWorkspaceName
	} else {
		issue, project, err = h.issueService.GetIssueWithProject(ctx, issueID)
		if err != nil || issue == nil {
			c.HTML(http.StatusNotFound, "error.html", platformadmin.ErrorPageData{Error: "Issue not found"})
			return
		}
		if project != nil && h.workspaceService != nil {
			if ws, err := h.workspaceService.GetWorkspace(ctx, project.WorkspaceID); err == nil {
				workspaceName = ws.Name
			}
		}
	}

	events, _ := h.issueService.GetIssueEvents(ctx, issueID, 50)
	projectName := ""
	if project != nil {
		projectName = project.Name
	}

	c.HTML(http.StatusOK, "issue_detail.html", IssueDetailPageData{
		BasePageData:   h.buildBasePageData(c, "issues", "Issue: "+issue.ShortID, issue.Title),
		Issue:          issue,
		Events:         events,
		WorkspaceName:  workspaceName,
		ProjectName:    projectName,
		IssuesBasePath: errorTrackingIssuesBasePath,
	})
}

func (h *ErrorTrackingAdminHandler) buildBasePageData(c *gin.Context, activePage, title, subtitle string) platformadmin.BasePageData {
	ctxValues := platformadmin.GetContextValues(c)
	workspaceID, workspaceName, _, isWorkspaceScoped := ctxValues.WorkspaceContext()

	return platformadmin.BasePageData{
		ActivePage:         activePage,
		PageTitle:          title,
		PageSubtitle:       subtitle,
		UserName:           ctxValues.UserName,
		UserEmail:          ctxValues.UserEmail,
		UserRole:           ctxValues.UserRole(),
		CanManageUsers:     ctxValues.CanManageUsers(),
		IsWorkspaceScoped:  isWorkspaceScoped,
		ExtensionNav:       h.extensionNavigation(c.Request.Context(), workspaceID),
		ExtensionWidgets:   h.extensionWidgets(c.Request.Context(), workspaceID),
		CurrentWorkspaceID: workspaceID,
		CurrentWorkspace:   workspaceName,
	}
}

func (h *ErrorTrackingAdminHandler) extensionNavigation(ctx context.Context, workspaceID string) []platformadmin.AdminExtensionNavSection {
	if h == nil || h.extensionService == nil {
		return nil
	}

	var (
		items []platformservices.ResolvedExtensionAdminNavigationItem
		err   error
	)
	if strings.TrimSpace(workspaceID) != "" {
		items, err = h.extensionService.ListWorkspaceAdminNavigation(ctx, workspaceID)
	} else {
		items, err = h.extensionService.ListInstanceAdminNavigation(ctx)
	}
	if err != nil || len(items) == 0 {
		return nil
	}

	sectionOrder := make([]string, 0)
	sections := make(map[string][]platformadmin.AdminExtensionNavItem)
	for _, item := range items {
		section := item.Section
		if section == "" {
			section = "Extensions"
		}
		if _, exists := sections[section]; !exists {
			sectionOrder = append(sectionOrder, section)
		}
		sections[section] = append(sections[section], platformadmin.AdminExtensionNavItem{
			Title:      item.Title,
			Icon:       item.Icon,
			Href:       item.Href,
			ActivePage: item.ActivePage,
		})
	}

	result := make([]platformadmin.AdminExtensionNavSection, 0, len(sectionOrder))
	for _, section := range sectionOrder {
		items := sections[section]
		slices.SortStableFunc(items, func(left, right platformadmin.AdminExtensionNavItem) int {
			return strings.Compare(left.Title, right.Title)
		})
		result = append(result, platformadmin.AdminExtensionNavSection{
			Title: section,
			Items: items,
		})
	}
	return result
}

func (h *ErrorTrackingAdminHandler) extensionWidgets(ctx context.Context, workspaceID string) []platformadmin.AdminExtensionWidget {
	if h == nil || h.extensionService == nil {
		return nil
	}

	var (
		widgets []platformservices.ResolvedExtensionDashboardWidget
		err     error
	)
	if strings.TrimSpace(workspaceID) != "" {
		widgets, err = h.extensionService.ListWorkspaceDashboardWidgets(ctx, workspaceID)
	} else {
		widgets, err = h.extensionService.ListInstanceDashboardWidgets(ctx)
	}
	if err != nil || len(widgets) == 0 {
		return nil
	}

	result := make([]platformadmin.AdminExtensionWidget, 0, len(widgets))
	for _, widget := range widgets {
		result = append(result, platformadmin.AdminExtensionWidget{
			Title:       widget.Title,
			Description: widget.Description,
			Icon:        widget.Icon,
			Href:        widget.Href,
		})
	}
	return result
}

func (h *ErrorTrackingAdminHandler) getWorkspaceNamesMap(ctx context.Context, workspaceIDs []string) map[string]string {
	if h == nil || h.workspaceService == nil || len(workspaceIDs) == 0 {
		return map[string]string{}
	}
	uniqueIDs := make([]string, 0, len(workspaceIDs))
	seen := make(map[string]struct{}, len(workspaceIDs))
	for _, id := range workspaceIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	workspaces, err := h.workspaceService.GetWorkspacesByIDs(ctx, uniqueIDs)
	if err != nil {
		return map[string]string{}
	}

	names := make(map[string]string, len(workspaces))
	for _, ws := range workspaces {
		names[ws.ID] = ws.Name
	}
	return names
}

func currentWorkspaceScope(c *gin.Context) (workspaceID, workspaceName string, ok bool) {
	workspaceID, workspaceName, _, ok = platformadmin.GetContextValues(c).WorkspaceContext()
	return workspaceID, workspaceName, ok
}
