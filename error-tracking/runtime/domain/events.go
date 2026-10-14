package observabilitydomain

import "time"

type IssueCreatedEvent struct {
	IssueID      string
	ProjectID    string
	WorkspaceID  string
	Title        string
	Level        string
	Fingerprint  string
	FirstEventID string
	Platform     string
	Culprit      string
	CreatedAt    time.Time
}

type IssueUpdatedEvent struct {
	IssueID     string
	ProjectID   string
	WorkspaceID string
	NewEventID  string
	HasNewUser  bool
	LastSeen    time.Time
	UpdatedAt   time.Time
}

type IssueResolvedEvent struct {
	IssueID              string
	ProjectID            string
	WorkspaceID          string
	Resolution           string
	ResolutionNotes      string
	ResolvedInVersion    string
	ResolvedInCommit     string
	ResolvedBy           string
	ResolvedAt           time.Time
	AffectedCaseCount    int
	AutoCloseSystemCases bool
	NotifyContacts       bool
	SystemCaseIDs        []string
}

type CasesBulkResolvedEvent struct {
	IssueID         string
	ProjectID       string
	WorkspaceID     string
	CaseIDs         []string
	SystemCaseIDs   []string
	CustomerCaseIDs []string
	Resolution      string
	ResolvedAt      time.Time
}

type IssueCaseLinkedEvent struct {
	IssueID     string
	CaseID      string
	ProjectID   string
	WorkspaceID string
	ContactID   string
	LinkedBy    string
	LinkReason  string
	LinkedAt    time.Time
}

type IssueCaseUnlinkedEvent struct {
	IssueID     string
	CaseID      string
	ProjectID   string
	WorkspaceID string
	UnlinkedBy  string
	UnlinkedAt  time.Time
}

type CaseCreatedForContactEvent struct {
	ContactID    string
	ContactEmail string
	IssueID      string
	ProjectID    string
	WorkspaceID  string
	IssueTitle   string
	IssueLevel   string
	Priority     string
	CreatedAt    time.Time
}

// IssueForSource derives the initial issue contribution from the accepted source
// event. Redelivered transport timestamps and user-count hints are not authority
// for the event's chronology or contribution.
func (e IssueCreatedEvent) IssueForSource(source *ErrorEvent) *Issue {
	issue := &Issue{
		WorkspaceID: e.WorkspaceID, ProjectID: e.ProjectID,
		Title: e.Title, Level: e.Level, Fingerprint: e.Fingerprint,
		Platform: e.Platform, Culprit: e.Culprit, Status: IssueStatusUnresolved,
		FirstSeen: source.Timestamp, LastSeen: source.Timestamp, EventCount: 1,
		LastEventID: source.EventID, Tags: make(map[string]string),
	}
	if source.HasIdentifiedUser() {
		issue.UserCount = 1
	}
	return issue
}

func (e *ErrorEvent) HasIdentifiedUser() bool {
	return e.User != nil && e.User.ID != ""
}
