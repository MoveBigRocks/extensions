package observabilityservices

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
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
	EventID     string            `json:"eventId"`
	Timestamp   time.Time         `json:"timestamp"`
	Environment string            `json:"environment"`
	Release     string            `json:"release"`
	Tags        map[string]string `json:"tags"`
	// TagsTruncated reports that more allowlisted tags existed than returned.
	TagsTruncated bool             `json:"tagsTruncated,omitempty"`
	Exceptions    []IssueException `json:"exceptions"`
	Frames        []IssueFrame     `json:"frames"`
}

// EventSummary is the bounded, agent-facing identity of one stored event.
type EventSummary struct {
	EventID       string            `json:"eventId"`
	IssueID       string            `json:"issueId,omitempty"`
	ProjectID     string            `json:"projectId"`
	Timestamp     time.Time         `json:"timestamp"`
	Environment   string            `json:"environment"`
	Release       string            `json:"release"`
	Tags          map[string]string `json:"tags"`
	TagsTruncated bool              `json:"tagsTruncated,omitempty"`
}
type EventPage struct {
	Items []EventSummary `json:"items"`
	Next  string         `json:"next,omitempty"`
}

// EventTagSearch is an exact tag lookup. Environment and Release are optional
// exact filters; SinceHours bounds the lookup window.
type EventTagSearch struct {
	Key, Value, Environment, Release string
	SinceHours, Limit                int
	After                            string
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

const (
	maxEventPage          = 50
	defaultTagSearchHours = 24 * 7
	maxTagSearchHours     = 24 * 90
	maxEventCursorLength  = 128
)

func publicEventID(e *domain.ErrorEvent) string {
	if e.EventID != "" {
		return boundedIssueText(e.EventID, 64)
	}
	return e.ID
}

func summarizeEvent(e *domain.ErrorEvent) EventSummary {
	tags, truncated := domain.PublicEventTags(e.Tags)
	return EventSummary{EventID: publicEventID(e), IssueID: e.IssueID, ProjectID: e.ProjectID, Timestamp: e.Timestamp, Environment: boundedIssueText(e.Environment, 128), Release: boundedIssueText(e.Release, 256), Tags: tags, TagsTruncated: truncated}
}

// encodeEventCursor binds the next page to the last returned (timestamp, row
// id). The cursor is opaque to clients and carries no workspace data; every
// page re-applies the caller's workspace predicate.
func encodeEventCursor(e *domain.ErrorEvent) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(e.Timestamp.UnixMicro(), 10) + "." + e.ID))
}

func decodeEventCursor(value string) (storecontracts.EventKeysetCursor, error) {
	if value == "" {
		return storecontracts.EventKeysetCursor{}, nil
	}
	if len(value) > maxEventCursorLength {
		return storecontracts.EventKeysetCursor{}, ErrInvalidIssueRead
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return storecontracts.EventKeysetCursor{}, ErrInvalidIssueRead
	}
	micros, id, ok := strings.Cut(string(raw), ".")
	n, err := strconv.ParseInt(micros, 10, 64)
	if !ok || err != nil || n < 0 || !validReadID(id) {
		return storecontracts.EventKeysetCursor{}, ErrInvalidIssueRead
	}
	return storecontracts.EventKeysetCursor{Timestamp: time.UnixMicro(n).UTC(), ID: id}, nil
}

// eventPage asks the store for one extra row so Next is only issued when a
// further page exists.
func eventPage(events []*domain.ErrorEvent, limit int, valid func(*domain.ErrorEvent) bool) (EventPage, error) {
	out := EventPage{Items: []EventSummary{}}
	for i, e := range events {
		if e == nil || !valid(e) {
			return EventPage{}, errors.New("invalid scoped event result")
		}
		if i == limit {
			out.Next = encodeEventCursor(events[limit-1])
			break
		}
		out.Items = append(out.Items, summarizeEvent(e))
	}
	return out, nil
}

func validEventText(value string, limit int) bool {
	if value == "" || len([]rune(value)) > limit {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
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
// Environment and release keep issues with at least one matching event.
func (s *IssueService) ReadIssuePage(ctx context.Context, ws, project, status, level, environment, release string, limit, offset int) (IssuePage, error) {
	out := IssuePage{Items: []IssueSummary{}}
	if !validReadID(ws) || (project != "" && !validReadID(project)) || limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return out, ErrInvalidIssueRead
	}
	if (environment != "" && !validEventText(environment, 128)) || (release != "" && !validEventText(release, 256)) {
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
	issues, total, err := s.issueStore.ListIssues(ctx, storecontracts.IssueFilters{WorkspaceID: ws, ProjectID: project, Status: status, Level: level, Environment: environment, Release: release, Limit: limit, Offset: offset})
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

// ReadIssueEvidence deliberately excludes request headers, user details, local
// variables, breadcrumbs and arbitrary context objects from agent reads. Tags
// pass only through the bounded allowlist in domain.PublicEventTags.
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
	tags, truncated := domain.PublicEventTags(e.Tags)
	latest := &IssueEventEvidence{EventID: e.EventID, Timestamp: e.Timestamp, Environment: boundedIssueText(e.Environment, 128), Release: boundedIssueText(e.Release, 256), Tags: tags, TagsTruncated: truncated, Exceptions: []IssueException{}, Frames: issueFrames(e.Stacktrace)}
	for i, ex := range e.Exception {
		if i == 5 {
			break
		}
		latest.Exceptions = append(latest.Exceptions, IssueException{Type: boundedIssueText(ex.Type, 256), Value: boundedIssueText(ex.Value, 1024), Frames: issueFrames(ex.Stacktrace)})
	}
	out.LatestEvent = latest
	return out, nil
}

// ReadIssueEvents pages one issue's events newest first with only identity,
// timing, environment, release and allowlisted tags.
func (s *IssueService) ReadIssueEvents(ctx context.Context, ws, id string, limit int, after string) (EventPage, error) {
	if !validReadID(ws) || !validReadID(id) || limit < 1 || limit > maxEventPage {
		return EventPage{Items: []EventSummary{}}, ErrInvalidIssueRead
	}
	cursor, err := decodeEventCursor(after)
	if err != nil {
		return EventPage{Items: []EventSummary{}}, err
	}
	issue, err := s.GetIssueInWorkspace(ctx, ws, id)
	if err != nil {
		return EventPage{Items: []EventSummary{}}, err
	}
	events, err := s.errorEventStore.ListIssueEventEvidence(ctx, storecontracts.IssueEventPageQuery{WorkspaceID: ws, IssueID: id, After: cursor, Limit: limit + 1})
	if err != nil {
		return EventPage{Items: []EventSummary{}}, err
	}
	return eventPage(events, limit, func(e *domain.ErrorEvent) bool { return e.IssueID == id && e.ProjectID == issue.ProjectID })
}

// FindEventsByTag finds events in the workspace carrying an exact allowlisted
// tag. Denied keys are refused rather than answered, so a lookup can never act
// as an oracle for personal data such as an email address.
func (s *IssueService) FindEventsByTag(ctx context.Context, ws string, q EventTagSearch, now time.Time) (EventPage, error) {
	empty := EventPage{Items: []EventSummary{}}
	if !validReadID(ws) || !domain.IsPublicTagKey(q.Key) || !domain.IsPublicTagValue(q.Value) || q.Limit < 1 || q.Limit > maxEventPage {
		return empty, ErrInvalidIssueRead
	}
	if (q.Environment != "" && !validEventText(q.Environment, 128)) || (q.Release != "" && !validEventText(q.Release, 256)) {
		return empty, ErrInvalidIssueRead
	}
	hours := q.SinceHours
	if hours == 0 {
		hours = defaultTagSearchHours
	}
	if hours < 1 || hours > maxTagSearchHours {
		return empty, ErrInvalidIssueRead
	}
	cursor, err := decodeEventCursor(q.After)
	if err != nil {
		return empty, err
	}
	events, err := s.errorEventStore.FindEventEvidenceByTag(ctx, storecontracts.EventTagQuery{
		WorkspaceID: ws, TagKey: q.Key, TagValue: q.Value, Environment: q.Environment, Release: q.Release,
		Since: now.Add(-time.Duration(hours) * time.Hour), After: cursor, Limit: q.Limit + 1,
	})
	if err != nil {
		return empty, err
	}
	// The store applies the workspace predicate; re-check the matched fields so
	// a store defect cannot return events outside the requested lookup.
	return eventPage(events, q.Limit, func(e *domain.ErrorEvent) bool {
		return e.Tags[q.Key] == q.Value && (q.Environment == "" || e.Environment == q.Environment) && (q.Release == "" || e.Release == q.Release)
	})
}
