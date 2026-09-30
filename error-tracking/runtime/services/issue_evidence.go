package observabilityservices

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	"github.com/movebigrocks/extensions/error-tracking/runtime/storecontracts"
)

const IssueReadContract = "error-tracking/v1"

var ErrInvalidIssueRead = errors.New("invalid issue read")

type IssueSummary struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspaceId"`
	ProjectID   string    `json:"projectId"`
	Title       string    `json:"title"`
	Culprit     string    `json:"culprit"`
	Status      string    `json:"status"`
	Level       string    `json:"level"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
	EventCount  int64     `json:"eventCount"`
}
type IssuePage struct {
	Items      []IssueSummary `json:"items"`
	NextOffset *int           `json:"nextOffset,omitempty"`
}
type IssueEventEvidence struct {
	EventID     string           `json:"eventId"`
	Timestamp   time.Time        `json:"timestamp"`
	Environment string           `json:"environment"`
	Release     string           `json:"release"`
	Exceptions  []IssueException `json:"exceptions"`
	Frames      []IssueFrame     `json:"frames"`
}
type IssueException struct {
	Type   string       `json:"type"`
	Value  string       `json:"value"`
	Frames []IssueFrame `json:"frames"`
}
type IssueFrame struct {
	Filename string `json:"filename"`
	Function string `json:"function"`
	Line     int    `json:"line"`
	InApp    bool   `json:"inApp"`
}
type IssueEvidence struct {
	Issue       IssueSummary        `json:"issue"`
	LatestEvent *IssueEventEvidence `json:"latestEvent,omitempty"`
}

func validReadID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}
func boundedIssueText(s string, limit int) string {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return s
}
func summarizeIssue(i *domain.Issue) IssueSummary {
	return IssueSummary{ID: i.ID, WorkspaceID: i.WorkspaceID, ProjectID: i.ProjectID, Title: boundedIssueText(i.Title, 1024), Culprit: boundedIssueText(i.Culprit, 512), Status: i.Status, Level: i.Level, FirstSeen: i.FirstSeen, LastSeen: i.LastSeen, EventCount: i.EventCount}
}
func issueFrames(stack *domain.StacktraceData) []IssueFrame {
	out := []IssueFrame{}
	if stack == nil {
		return out
	}
	frames := stack.Frames
	if len(frames) > 50 {
		frames = frames[len(frames)-50:]
	}
	for _, f := range frames {
		out = append(out, IssueFrame{Filename: boundedIssueText(f.Filename, 512), Function: boundedIssueText(f.Function, 256), Line: f.LineNumber, InApp: f.InApp})
	}
	return out
}

// ReadIssuePage always supplies a workspace predicate, including for the
// instance-scoped runtime's administrative database role. Pagination is a live
// last-seen view; callers should deduplicate IDs while new events are arriving.
func (s *IssueService) ReadIssuePage(ctx context.Context, ws, project, status, level string, limit, offset int) (IssuePage, error) {
	out := IssuePage{Items: []IssueSummary{}}
	if !validReadID(ws) || (project != "" && !validReadID(project)) || limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return out, ErrInvalidIssueRead
	}
	switch status {
	case "", "unresolved", "resolved", "ignored", "muted":
	default:
		return out, ErrInvalidIssueRead
	}
	switch level {
	case "", "fatal", "error", "warning", "info", "debug":
	default:
		return out, ErrInvalidIssueRead
	}
	issues, total, err := s.issueStore.ListIssues(ctx, storecontracts.IssueFilters{WorkspaceID: ws, ProjectID: project, Status: status, Level: level, Limit: limit, Offset: offset})
	if err != nil {
		return out, err
	}
	for _, issue := range issues {
		if issue == nil || issue.WorkspaceID != ws {
			return IssuePage{}, errors.New("invalid scoped issue result")
		}
		out.Items = append(out.Items, summarizeIssue(issue))
	}
	if offset+len(issues) < total {
		next := offset + len(issues)
		out.NextOffset = &next
	}
	return out, nil
}

// ReadIssueEvidence deliberately excludes request headers, user details, tags,
// local variables, breadcrumbs and arbitrary context objects from agent reads.
func (s *IssueService) ReadIssueEvidence(ctx context.Context, ws, id string) (IssueEvidence, error) {
	var out IssueEvidence
	if !validReadID(ws) || !validReadID(id) {
		return out, ErrInvalidIssueRead
	}
	issue, err := s.GetIssueInWorkspace(ctx, ws, id)
	if err != nil {
		return out, err
	}
	out.Issue = summarizeIssue(issue)
	events, err := s.errorEventStore.GetIssueEvents(ctx, id, 1)
	if err != nil {
		return out, err
	}
	if len(events) == 0 {
		return out, nil
	}
	e := events[0]
	if e == nil || e.IssueID != id || e.ProjectID != issue.ProjectID {
		return IssueEvidence{}, errors.New("invalid scoped event result")
	}
	latest := &IssueEventEvidence{EventID: e.EventID, Timestamp: e.Timestamp, Environment: boundedIssueText(e.Environment, 128), Release: boundedIssueText(e.Release, 256), Exceptions: []IssueException{}, Frames: issueFrames(e.Stacktrace)}
	for i, ex := range e.Exception {
		if i == 5 {
			break
		}
		latest.Exceptions = append(latest.Exceptions, IssueException{Type: boundedIssueText(ex.Type, 256), Value: boundedIssueText(ex.Value, 1024), Frames: issueFrames(ex.Stacktrace)})
	}
	out.LatestEvent = latest
	return out, nil
}
