package storecontracts

import (
	"context"
	"time"

	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
)

type ProjectStore interface {
	CreateProject(ctx context.Context, extensionInstallID string, project *observabilitydomain.Project) error
	GetProject(ctx context.Context, projectID string) (*observabilitydomain.Project, error)
	GetProjectInWorkspace(ctx context.Context, workspaceID, projectID string) (*observabilitydomain.Project, error)
	GetProjectByKey(ctx context.Context, projectKey string) (*observabilitydomain.Project, error)
	GetProjectsByIDs(ctx context.Context, projectIDs []string) ([]*observabilitydomain.Project, error)
	UpdateProject(ctx context.Context, project *observabilitydomain.Project) error
	IncrementEventCount(ctx context.Context, workspaceID, projectID string, lastEventAt *time.Time) (int64, error)
	ListWorkspaceProjects(ctx context.Context, workspaceID string) ([]*observabilitydomain.Project, error)
	ListAllProjects(ctx context.Context) ([]*observabilitydomain.Project, error)
	DeleteProject(ctx context.Context, workspaceID, projectID string) error
	GetApplication(ctx context.Context, workspaceID, appID string) (*observabilitydomain.Application, error)
	GetApplicationByKey(ctx context.Context, appKey string) (*observabilitydomain.Application, error)
	ListWorkspaceApplications(ctx context.Context, workspaceID string) ([]*observabilitydomain.Application, error)
}

type IssueStore interface {
	CreateIssue(ctx context.Context, issue *observabilitydomain.Issue) error
	CreateOrUpdateIssueByFingerprint(ctx context.Context, issue *observabilitydomain.Issue) (resultIssue *observabilitydomain.Issue, created bool, err error)
	GetIssue(ctx context.Context, issueID string) (*observabilitydomain.Issue, error)
	GetIssueInWorkspace(ctx context.Context, workspaceID, issueID string) (*observabilitydomain.Issue, error)
	GetIssuesByIDs(ctx context.Context, issueIDs []string) ([]*observabilitydomain.Issue, error)
	GetIssueByFingerprint(ctx context.Context, projectID, fingerprint string) (*observabilitydomain.Issue, error)
	UpdateIssue(ctx context.Context, issue *observabilitydomain.Issue) error
	ListProjectIssues(ctx context.Context, projectID string, filter IssueFilter) ([]*observabilitydomain.Issue, error)
	ListIssues(ctx context.Context, filters IssueFilters) ([]*observabilitydomain.Issue, int, error)
	ListAllIssues(ctx context.Context, filters IssueFilters) ([]*observabilitydomain.Issue, int, error)
	AtomicUpdateIssueStats(ctx context.Context, workspaceID, issueID string, lastEventID string, lastSeen time.Time, incrementUserCount bool) (*observabilitydomain.Issue, error)
}

type ErrorEventStore interface {
	CreateErrorEvent(ctx context.Context, event *observabilitydomain.ErrorEvent) error
	GetErrorEvent(ctx context.Context, eventID string) (*observabilitydomain.ErrorEvent, error)
	GetIssueEvents(ctx context.Context, issueID string, limit int) ([]*observabilitydomain.ErrorEvent, error)
	ListProjectEvents(ctx context.Context, projectID string, filter EventFilter) ([]*observabilitydomain.ErrorEvent, error)
	UpdateEventIssueID(ctx context.Context, workspaceID, eventID, issueID string) error
}

type IssueCaseIntegrationStore interface {
	GetErrorEventsByEmail(ctx context.Context, email string, since time.Time) ([]*observabilitydomain.ErrorEvent, error)
	GetUnresolvedIssuesWithCases(ctx context.Context, workspaceID string) ([]*observabilitydomain.Issue, error)
}

type ErrorAlertStore interface {
	CreateAlert(ctx context.Context, alert *observabilitydomain.Alert) error
	GetAlert(ctx context.Context, alertID string) (*observabilitydomain.Alert, error)
	UpdateAlert(ctx context.Context, alert *observabilitydomain.Alert) error
}

type GitRepoStore interface {
	GetGitRepoByID(ctx context.Context, repoID string) (*observabilitydomain.GitRepo, error)
	ListGitReposByApplication(ctx context.Context, applicationID string) ([]*observabilitydomain.GitRepo, error)
}

type IssueFilter struct {
	Status     string     `json:"status,omitempty"`
	Level      string     `json:"level,omitempty"`
	AssignedTo string     `json:"assigned_to,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	Since      *time.Time `json:"since,omitempty"`
	Until      *time.Time `json:"until,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
}

type EventFilter struct {
	IssueID     string     `json:"issue_id,omitempty"`
	Level       string     `json:"level,omitempty"`
	UserID      string     `json:"user_id,omitempty"`
	Environment string     `json:"environment,omitempty"`
	Release     string     `json:"release,omitempty"`
	Since       *time.Time `json:"since,omitempty"`
	Until       *time.Time `json:"until,omitempty"`
	Limit       int        `json:"limit,omitempty"`
	Offset      int        `json:"offset,omitempty"`
}

type IssueFilters struct {
	WorkspaceID string
	ProjectID   string
	Status      string
	Level       string
	Limit       int
	Offset      int
}
