package atsruntime

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"

	atsdomain "github.com/movebigrocks/platform/extensions/ats/runtime/domain"
)

const SchemaName = "ext_demandops_ats"

type Vacancy struct {
	ID                  string                   `db:"id" json:"id"`
	WorkspaceID         string                   `db:"workspace_id" json:"workspaceId"`
	Slug                string                   `db:"slug" json:"slug"`
	Title               string                   `db:"title" json:"title"`
	Team                string                   `db:"team" json:"team"`
	Location            string                   `db:"location" json:"location"`
	WorkMode            atsdomain.WorkMode       `db:"work_mode" json:"workMode"`
	EmploymentType      atsdomain.EmploymentType `db:"employment_type" json:"employmentType"`
	Status              atsdomain.VacancyStatus  `db:"status" json:"status"`
	Summary             string                   `db:"summary" json:"summary"`
	Description         string                   `db:"description" json:"description"`
	ApplicationFormSlug string                   `db:"application_form_slug" json:"applicationFormSlug"`
	CaseQueueSlug       string                   `db:"case_queue_slug" json:"caseQueueSlug"`
	CareersPath         string                   `db:"careers_path" json:"careersPath"`
	PublishedAt         *time.Time               `db:"published_at" json:"publishedAt,omitempty"`
	ClosedAt            *time.Time               `db:"closed_at" json:"closedAt,omitempty"`
	CreatedAt           time.Time                `db:"created_at" json:"createdAt"`
	UpdatedAt           time.Time                `db:"updated_at" json:"updatedAt"`
}

type Applicant struct {
	ID                    string    `db:"id" json:"id"`
	WorkspaceID           string    `db:"workspace_id" json:"workspaceId"`
	ContactID             string    `db:"contact_id" json:"contactId"`
	FullName              string    `db:"full_name" json:"fullName"`
	Email                 string    `db:"email" json:"email"`
	Phone                 string    `db:"phone" json:"phone"`
	Location              string    `db:"location" json:"location"`
	LinkedInURL           string    `db:"linkedin_url" json:"linkedinUrl"`
	PortfolioURL          string    `db:"portfolio_url" json:"portfolioUrl"`
	CoverNote             string    `db:"cover_note" json:"coverNote"`
	ResumeAttachmentID    string    `db:"resume_attachment_id" json:"resumeAttachmentId"`
	CoverLetterAttachment string    `db:"cover_letter_attachment" json:"coverLetterAttachment"`
	CreatedAt             time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt             time.Time `db:"updated_at" json:"updatedAt"`
}

type Application struct {
	ID                 string                     `db:"id" json:"id"`
	WorkspaceID        string                     `db:"workspace_id" json:"workspaceId"`
	VacancyID          string                     `db:"vacancy_id" json:"vacancyId"`
	ApplicantID        string                     `db:"applicant_id" json:"applicantId"`
	CaseID             string                     `db:"case_id" json:"caseId"`
	ContactID          string                     `db:"contact_id" json:"contactId"`
	FormSubmissionID   string                     `db:"form_submission_id" json:"formSubmissionId"`
	Source             string                     `db:"source" json:"source"`
	Stage              atsdomain.ApplicationStage `db:"stage" json:"stage"`
	AppliedAt          time.Time                  `db:"applied_at" json:"appliedAt"`
	LastStageChangedAt time.Time                  `db:"last_stage_changed_at" json:"lastStageChangedAt"`
	ReviewedAt         *time.Time                 `db:"reviewed_at" json:"reviewedAt,omitempty"`
	HiredAt            *time.Time                 `db:"hired_at" json:"hiredAt,omitempty"`
	RejectedAt         *time.Time                 `db:"rejected_at" json:"rejectedAt,omitempty"`
	WithdrawnAt        *time.Time                 `db:"withdrawn_at" json:"withdrawnAt,omitempty"`
	RejectionReason    string                     `db:"rejection_reason" json:"rejectionReason"`
	CreatedAt          time.Time                  `db:"created_at" json:"createdAt"`
	UpdatedAt          time.Time                  `db:"updated_at" json:"updatedAt"`
}

type RecruiterNote struct {
	ID            string    `db:"id" json:"id"`
	WorkspaceID   string    `db:"workspace_id" json:"workspaceId"`
	ApplicationID string    `db:"application_id" json:"applicationId"`
	AuthorName    string    `db:"author_name" json:"authorName"`
	AuthorType    string    `db:"author_type" json:"authorType"`
	Body          string    `db:"body" json:"body"`
	CreatedAt     time.Time `db:"created_at" json:"createdAt"`
}

type StagePreset struct {
	ID          string         `db:"id" json:"id"`
	WorkspaceID string         `db:"workspace_id" json:"workspaceId"`
	Slug        string         `db:"slug" json:"slug"`
	Name        string         `db:"name" json:"name"`
	Stages      pq.StringArray `db:"stages" json:"stages"`
	IsDefault   bool           `db:"is_default" json:"isDefault"`
	CreatedAt   time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time      `db:"updated_at" json:"updatedAt"`
}

type SavedFilter struct {
	ID          string          `db:"id" json:"id"`
	WorkspaceID string          `db:"workspace_id" json:"workspaceId"`
	Slug        string          `db:"slug" json:"slug"`
	Name        string          `db:"name" json:"name"`
	Criteria    json.RawMessage `db:"criteria" json:"criteria"`
	CreatedAt   time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time       `db:"updated_at" json:"updatedAt"`
}

type CandidateProfile struct {
	Applicant   Applicant       `json:"applicant"`
	Application Application     `json:"application"`
	Notes       []RecruiterNote `json:"notes"`
}

type WorkspaceDefaults struct {
	StagePresets []StagePreset `json:"stagePresets"`
	SavedFilters []SavedFilter `json:"savedFilters"`
}

type CreateJobInput struct {
	WorkspaceID    string
	Slug           string
	Title          string
	Team           string
	Location       string
	WorkMode       atsdomain.WorkMode
	EmploymentType atsdomain.EmploymentType
	Summary        string
	Description    string
}

type SubmitApplicationInput struct {
	WorkspaceID string
	VacancySlug string
	Submission  atsdomain.CandidateSubmission
}

type StageChangeInput struct {
	Stage      atsdomain.ApplicationStage
	ActorName  string
	ActorType  string
	Reason     string
	Note       string
	OccurredAt time.Time
}

type SubmissionResult struct {
	Vacancy     Vacancy     `json:"vacancy"`
	Applicant   Applicant   `json:"applicant"`
	Application Application `json:"application"`
}
