package atsdomain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type VacancyStatus string

const (
	VacancyStatusDraft    VacancyStatus = "draft"
	VacancyStatusOpen     VacancyStatus = "open"
	VacancyStatusPaused   VacancyStatus = "paused"
	VacancyStatusClosed   VacancyStatus = "closed"
	VacancyStatusArchived VacancyStatus = "archived"
)

type EmploymentType string

const (
	EmploymentTypeFullTime   EmploymentType = "full_time"
	EmploymentTypePartTime   EmploymentType = "part_time"
	EmploymentTypeContract   EmploymentType = "contract"
	EmploymentTypeInternship EmploymentType = "internship"
)

type WorkMode string

const (
	WorkModeRemote WorkMode = "remote"
	WorkModeHybrid WorkMode = "hybrid"
	WorkModeOnsite WorkMode = "onsite"
)

type ApplicationStage string

const (
	ApplicationStageReceived  ApplicationStage = "received"
	ApplicationStageScreening ApplicationStage = "screening"
	ApplicationStageInterview ApplicationStage = "interview"
	ApplicationStageOffer     ApplicationStage = "offer"
	ApplicationStageHired     ApplicationStage = "hired"
	ApplicationStageRejected  ApplicationStage = "rejected"
	ApplicationStageWithdrawn ApplicationStage = "withdrawn"
)

var applicationStageOrder = map[ApplicationStage]int{
	ApplicationStageReceived:  1,
	ApplicationStageScreening: 2,
	ApplicationStageInterview: 3,
	ApplicationStageOffer:     4,
	ApplicationStageHired:     5,
	ApplicationStageRejected:  5,
	ApplicationStageWithdrawn: 5,
}

type Vacancy struct {
	ID                  string
	WorkspaceID         string
	Slug                string
	Title               string
	Team                string
	Location            string
	WorkMode            WorkMode
	EmploymentType      EmploymentType
	Status              VacancyStatus
	Summary             string
	Description         string
	ApplicationFormSlug string
	CaseQueueSlug       string
	CareersPath         string
	PublishedAt         *time.Time
	ClosedAt            *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func NewVacancy(workspaceID, slug, title string) (*Vacancy, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	slug = normalizeSlug(slug)
	title = strings.TrimSpace(title)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	now := time.Now().UTC()
	vacancy := &Vacancy{
		ID:                  newATSID("vac"),
		WorkspaceID:         workspaceID,
		Slug:                slug,
		Title:               title,
		Status:              VacancyStatusDraft,
		WorkMode:            WorkModeRemote,
		EmploymentType:      EmploymentTypeFullTime,
		ApplicationFormSlug: "job-application",
		CareersPath:         "/careers/apply?job=" + slug,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return vacancy, vacancy.Validate()
}

func (v *Vacancy) Validate() error {
	if v == nil {
		return fmt.Errorf("vacancy is required")
	}
	v.WorkspaceID = strings.TrimSpace(v.WorkspaceID)
	v.Slug = normalizeSlug(v.Slug)
	v.Title = strings.TrimSpace(v.Title)
	v.Team = strings.TrimSpace(v.Team)
	v.Location = strings.TrimSpace(v.Location)
	v.Summary = strings.TrimSpace(v.Summary)
	v.Description = strings.TrimSpace(v.Description)
	v.ApplicationFormSlug = normalizeSlug(v.ApplicationFormSlug)
	v.CaseQueueSlug = normalizeSlug(v.CaseQueueSlug)
	v.CareersPath = strings.TrimSpace(v.CareersPath)
	if v.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if v.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if v.Title == "" {
		return fmt.Errorf("title is required")
	}
	switch v.Status {
	case VacancyStatusDraft, VacancyStatusOpen, VacancyStatusPaused, VacancyStatusClosed, VacancyStatusArchived:
	default:
		return fmt.Errorf("invalid vacancy status %q", v.Status)
	}
	switch v.WorkMode {
	case "", WorkModeRemote, WorkModeHybrid, WorkModeOnsite:
	default:
		return fmt.Errorf("invalid work mode %q", v.WorkMode)
	}
	switch v.EmploymentType {
	case "", EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeContract, EmploymentTypeInternship:
	default:
		return fmt.Errorf("invalid employment type %q", v.EmploymentType)
	}
	return nil
}

func (v *Vacancy) Publish(at time.Time) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Status == VacancyStatusArchived {
		return fmt.Errorf("archived vacancies cannot be published")
	}
	ts := normalizedTime(at)
	v.Status = VacancyStatusOpen
	v.ClosedAt = nil
	if v.PublishedAt == nil {
		v.PublishedAt = &ts
	}
	v.UpdatedAt = ts
	return nil
}

func (v *Vacancy) Pause(at time.Time) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Status != VacancyStatusOpen {
		return fmt.Errorf("only open vacancies can be paused")
	}
	v.Status = VacancyStatusPaused
	v.UpdatedAt = normalizedTime(at)
	return nil
}

func (v *Vacancy) Close(at time.Time) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Status == VacancyStatusArchived {
		return fmt.Errorf("archived vacancies cannot be closed")
	}
	ts := normalizedTime(at)
	v.Status = VacancyStatusClosed
	v.ClosedAt = &ts
	v.UpdatedAt = ts
	return nil
}

func (v *Vacancy) Reopen(at time.Time) error {
	if err := v.Validate(); err != nil {
		return err
	}
	if v.Status != VacancyStatusClosed && v.Status != VacancyStatusPaused {
		return fmt.Errorf("only closed or paused vacancies can be reopened")
	}
	ts := normalizedTime(at)
	v.Status = VacancyStatusOpen
	v.ClosedAt = nil
	if v.PublishedAt == nil {
		v.PublishedAt = &ts
	}
	v.UpdatedAt = ts
	return nil
}

func (v Vacancy) IsOpen() bool {
	return v.Status == VacancyStatusOpen
}

func (v Vacancy) CaseCustomFields() map[string]any {
	return map[string]any{
		"ats_vacancy_id":              strings.TrimSpace(v.ID),
		"ats_vacancy_slug":            v.Slug,
		"ats_vacancy_title":           v.Title,
		"ats_vacancy_status":          string(v.Status),
		"ats_vacancy_team":            v.Team,
		"ats_vacancy_location":        v.Location,
		"ats_vacancy_work_mode":       string(v.WorkMode),
		"ats_vacancy_employment_type": string(v.EmploymentType),
		"ats_vacancy_careers_path":    v.CareersPath,
	}
}

type Applicant struct {
	ID                    string
	WorkspaceID           string
	FullName              string
	Email                 string
	Phone                 string
	Location              string
	LinkedInURL           string
	PortfolioURL          string
	CoverNote             string
	ResumeAttachmentID    string
	CoverLetterAttachment string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func NewApplicant(workspaceID, fullName, email string) (*Applicant, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	fullName = strings.TrimSpace(fullName)
	email = normalizeEmail(email)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if fullName == "" {
		return nil, fmt.Errorf("full_name is required")
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("invalid applicant email: %w", err)
	}
	now := time.Now().UTC()
	applicant := &Applicant{
		ID:          newATSID("applicant"),
		WorkspaceID: workspaceID,
		FullName:    fullName,
		Email:       email,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return applicant, nil
}

func (a *Applicant) AttachResume(attachmentID string) error {
	if a == nil {
		return fmt.Errorf("applicant is required")
	}
	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return fmt.Errorf("resume attachment id is required")
	}
	a.ResumeAttachmentID = attachmentID
	a.UpdatedAt = time.Now().UTC()
	return nil
}

func (a Applicant) CaseCustomFields() map[string]any {
	return map[string]any{
		"ats_applicant_id":                   strings.TrimSpace(a.ID),
		"ats_applicant_full_name":            a.FullName,
		"ats_applicant_email":                a.Email,
		"ats_applicant_phone":                a.Phone,
		"ats_applicant_location":             a.Location,
		"ats_applicant_linkedin_url":         a.LinkedInURL,
		"ats_applicant_portfolio_url":        a.PortfolioURL,
		"ats_applicant_resume_attachment_id": a.ResumeAttachmentID,
	}
}

type Application struct {
	ID                string
	WorkspaceID       string
	VacancyID         string
	ApplicantID       string
	CaseID            string
	ContactID         string
	FormSubmissionID  string
	Source            string
	Stage             ApplicationStage
	AppliedAt         time.Time
	LastStageChangedAt time.Time
	ReviewedAt        *time.Time
	HiredAt           *time.Time
	RejectedAt        *time.Time
	WithdrawnAt       *time.Time
	RejectionReason   string
}

func NewApplication(workspaceID, vacancyID, applicantID, source string) (*Application, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	vacancyID = strings.TrimSpace(vacancyID)
	applicantID = strings.TrimSpace(applicantID)
	source = strings.TrimSpace(source)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if vacancyID == "" {
		return nil, fmt.Errorf("vacancy_id is required")
	}
	if applicantID == "" {
		return nil, fmt.Errorf("applicant_id is required")
	}
	if source == "" {
		source = "careers_form"
	}
	now := time.Now().UTC()
	return &Application{
		ID:                 newATSID("application"),
		WorkspaceID:        workspaceID,
		VacancyID:          vacancyID,
		ApplicantID:        applicantID,
		Source:             source,
		Stage:              ApplicationStageReceived,
		AppliedAt:          now,
		LastStageChangedAt: now,
	}, nil
}

func (a *Application) AdvanceTo(stage ApplicationStage, at time.Time) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.IsTerminal() {
		return fmt.Errorf("terminal applications cannot change stage")
	}
	if !validApplicationStage(stage) {
		return fmt.Errorf("invalid application stage %q", stage)
	}
	if applicationStageOrder[stage] < applicationStageOrder[a.Stage] {
		return fmt.Errorf("application stage cannot move backwards from %q to %q", a.Stage, stage)
	}
	ts := normalizedTime(at)
	a.Stage = stage
	a.LastStageChangedAt = ts
	if stage == ApplicationStageInterview || stage == ApplicationStageOffer {
		a.ReviewedAt = &ts
	}
	return nil
}

func (a *Application) Reject(reason string, at time.Time) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.IsTerminal() {
		return fmt.Errorf("terminal applications cannot be rejected")
	}
	ts := normalizedTime(at)
	a.Stage = ApplicationStageRejected
	a.RejectionReason = strings.TrimSpace(reason)
	a.RejectedAt = &ts
	a.LastStageChangedAt = ts
	return nil
}

func (a *Application) Hire(at time.Time) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.IsTerminal() {
		return fmt.Errorf("terminal applications cannot be hired")
	}
	ts := normalizedTime(at)
	a.Stage = ApplicationStageHired
	a.HiredAt = &ts
	a.LastStageChangedAt = ts
	return nil
}

func (a *Application) Withdraw(at time.Time) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.IsTerminal() {
		return fmt.Errorf("terminal applications cannot be withdrawn")
	}
	ts := normalizedTime(at)
	a.Stage = ApplicationStageWithdrawn
	a.WithdrawnAt = &ts
	a.LastStageChangedAt = ts
	return nil
}

func (a *Application) Validate() error {
	if a == nil {
		return fmt.Errorf("application is required")
	}
	a.WorkspaceID = strings.TrimSpace(a.WorkspaceID)
	a.VacancyID = strings.TrimSpace(a.VacancyID)
	a.ApplicantID = strings.TrimSpace(a.ApplicantID)
	a.CaseID = strings.TrimSpace(a.CaseID)
	a.ContactID = strings.TrimSpace(a.ContactID)
	a.FormSubmissionID = strings.TrimSpace(a.FormSubmissionID)
	a.Source = strings.TrimSpace(a.Source)
	a.RejectionReason = strings.TrimSpace(a.RejectionReason)
	if a.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if a.VacancyID == "" {
		return fmt.Errorf("vacancy_id is required")
	}
	if a.ApplicantID == "" {
		return fmt.Errorf("applicant_id is required")
	}
	if !validApplicationStage(a.Stage) {
		return fmt.Errorf("invalid application stage %q", a.Stage)
	}
	return nil
}

func (a Application) IsTerminal() bool {
	switch a.Stage {
	case ApplicationStageHired, ApplicationStageRejected, ApplicationStageWithdrawn:
		return true
	default:
		return false
	}
}

func (a Application) CaseCustomFields() map[string]any {
	fields := map[string]any{
		"ats_application_id":               strings.TrimSpace(a.ID),
		"ats_application_vacancy_id":       a.VacancyID,
		"ats_application_applicant_id":     a.ApplicantID,
		"ats_application_stage":            string(a.Stage),
		"ats_application_source":           a.Source,
		"ats_application_form_submission":  a.FormSubmissionID,
		"ats_application_rejection_reason": a.RejectionReason,
	}
	if !a.AppliedAt.IsZero() {
		fields["ats_application_applied_at"] = a.AppliedAt.UTC().Format(time.RFC3339)
	}
	return fields
}

type CandidateSubmission struct {
	FullName           string
	Email              string
	Phone              string
	Location           string
	LinkedInURL        string
	PortfolioURL       string
	CoverNote          string
	ResumeAttachmentID string
	Source             string
	FormSubmissionID   string
}

func BuildCandidateRecord(workspaceID string, vacancy *Vacancy, submission CandidateSubmission) (*Applicant, *Application, error) {
	if vacancy == nil {
		return nil, nil, fmt.Errorf("vacancy is required")
	}
	if err := vacancy.Validate(); err != nil {
		return nil, nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		workspaceID = vacancy.WorkspaceID
	}
	applicant, err := NewApplicant(workspaceID, submission.FullName, submission.Email)
	if err != nil {
		return nil, nil, err
	}
	applicant.Phone = strings.TrimSpace(submission.Phone)
	applicant.Location = strings.TrimSpace(submission.Location)
	applicant.LinkedInURL = strings.TrimSpace(submission.LinkedInURL)
	applicant.PortfolioURL = strings.TrimSpace(submission.PortfolioURL)
	applicant.CoverNote = strings.TrimSpace(submission.CoverNote)
	if strings.TrimSpace(submission.ResumeAttachmentID) != "" {
		if err := applicant.AttachResume(submission.ResumeAttachmentID); err != nil {
			return nil, nil, err
		}
	}

	application, err := NewApplication(workspaceID, vacancy.ID, applicant.ID, submission.Source)
	if err != nil {
		return nil, nil, err
	}
	application.FormSubmissionID = strings.TrimSpace(submission.FormSubmissionID)
	return applicant, application, nil
}

func validApplicationStage(stage ApplicationStage) bool {
	_, ok := applicationStageOrder[stage]
	return ok
}

func normalizeSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func normalizeEmail(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizedTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func newATSID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
