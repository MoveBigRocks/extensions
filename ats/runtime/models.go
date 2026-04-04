package atsruntime

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/lib/pq"

	atsdomain "github.com/movebigrocks/extensions/ats/runtime/domain"
)

const SchemaName = "ext_demandops_ats"

type Vacancy struct {
	ID                      string                   `db:"id" json:"id"`
	WorkspaceID             string                   `db:"workspace_id" json:"workspaceId"`
	Slug                    string                   `db:"slug" json:"slug"`
	Kind                    atsdomain.VacancyKind    `db:"kind" json:"kind"`
	Title                   string                   `db:"title" json:"title"`
	Team                    string                   `db:"team" json:"team"`
	Location                string                   `db:"location" json:"location"`
	WorkMode                atsdomain.WorkMode       `db:"work_mode" json:"workMode"`
	EmploymentType          atsdomain.EmploymentType `db:"employment_type" json:"employmentType"`
	Status                  atsdomain.VacancyStatus  `db:"status" json:"status"`
	Summary                 string                   `db:"summary" json:"summary"`
	Description             string                   `db:"description" json:"description"`
	PublicLanguage          string                   `db:"public_language" json:"language,omitempty"`
	AboutTheJob             string                   `db:"public_about_the_job" json:"aboutTheJob,omitempty"`
	Responsibilities        pq.StringArray           `db:"public_responsibilities" json:"responsibilities,omitempty"`
	ResponsibilitiesHeading string                   `db:"public_responsibilities_heading" json:"responsibilitiesHeading,omitempty"`
	AboutYou                string                   `db:"public_about_you" json:"aboutYou,omitempty"`
	AboutYouHeading         string                   `db:"public_about_you_heading" json:"aboutYouHeading,omitempty"`
	Profile                 pq.StringArray           `db:"public_profile" json:"profile,omitempty"`
	OffersIntro             string                   `db:"public_offers_intro" json:"offersIntro,omitempty"`
	Offers                  pq.StringArray           `db:"public_offers" json:"offers,omitempty"`
	OffersHeading           string                   `db:"public_offers_heading" json:"offersHeading,omitempty"`
	Quote                   string                   `db:"public_quote" json:"quote,omitempty"`
	ApplicationFormSlug     string                   `db:"application_form_slug" json:"applicationFormSlug"`
	CaseQueueID             string                   `db:"case_queue_id" json:"caseQueueId"`
	CaseQueueSlug           string                   `db:"case_queue_slug" json:"caseQueueSlug"`
	CareersPath             string                   `db:"careers_path" json:"careersPath"`
	PublishedAt             *time.Time               `db:"published_at" json:"publishedAt,omitempty"`
	ClosedAt                *time.Time               `db:"closed_at" json:"closedAt,omitempty"`
	CreatedAt               time.Time                `db:"created_at" json:"createdAt"`
	UpdatedAt               time.Time                `db:"updated_at" json:"updatedAt"`
}

func (v Vacancy) IsListedPublicly() bool {
	return strings.TrimSpace(string(v.Kind)) == "" || v.Kind == atsdomain.VacancyKindJob
}

type CareersSiteProfile struct {
	WorkspaceID        string     `db:"workspace_id" json:"workspaceId"`
	CompanyName        string     `db:"company_name" json:"companyName"`
	SiteTitle          string     `db:"site_title" json:"siteTitle"`
	Tagline            string     `db:"tagline" json:"tagline"`
	MetaDescription    string     `db:"meta_description" json:"metaDescription"`
	HeroEyebrow        string     `db:"hero_eyebrow" json:"heroEyebrow"`
	HeroTitle          string     `db:"hero_title" json:"heroTitle"`
	HeroBody           string     `db:"hero_body" json:"heroBody"`
	HeroPrimaryLabel   string     `db:"hero_primary_label" json:"heroPrimaryLabel"`
	HeroPrimaryHref    string     `db:"hero_primary_href" json:"heroPrimaryHref"`
	HeroSecondaryLabel string     `db:"hero_secondary_label" json:"heroSecondaryLabel"`
	HeroSecondaryHref  string     `db:"hero_secondary_href" json:"heroSecondaryHref"`
	StoryHeading       string     `db:"story_heading" json:"storyHeading"`
	StoryBody          string     `db:"story_body" json:"storyBody"`
	JobsHeading        string     `db:"jobs_heading" json:"jobsHeading"`
	JobsIntro          string     `db:"jobs_intro" json:"jobsIntro"`
	TeamHeading        string     `db:"team_heading" json:"teamHeading"`
	TeamIntro          string     `db:"team_intro" json:"teamIntro"`
	GalleryHeading     string     `db:"gallery_heading" json:"galleryHeading"`
	GalleryIntro       string     `db:"gallery_intro" json:"galleryIntro"`
	ContactEmail       string     `db:"contact_email" json:"contactEmail"`
	WebsiteURL         string     `db:"website_url" json:"websiteUrl"`
	LinkedInURL        string     `db:"linkedin_url" json:"linkedinUrl"`
	InstagramURL       string     `db:"instagram_url" json:"instagramUrl"`
	XURL               string     `db:"x_url" json:"xUrl"`
	LogoURL            string     `db:"logo_url" json:"logoUrl"`
	HeroImageURL       string     `db:"hero_image_url" json:"heroImageUrl"`
	OgImageURL         string     `db:"og_image_url" json:"ogImageUrl"`
	PrimaryColor       string     `db:"primary_color" json:"primaryColor"`
	AccentColor        string     `db:"accent_color" json:"accentColor"`
	SurfaceColor       string     `db:"surface_color" json:"surfaceColor"`
	BackgroundColor    string     `db:"background_color" json:"backgroundColor"`
	TextColor          string     `db:"text_color" json:"textColor"`
	MutedColor         string     `db:"muted_color" json:"mutedColor"`
	StreetAddress      string     `db:"street_address" json:"streetAddress"`
	AddressLocality    string     `db:"address_locality" json:"addressLocality"`
	AddressRegion      string     `db:"address_region" json:"addressRegion"`
	PostalCode         string     `db:"postal_code" json:"postalCode"`
	AddressCountry     string     `db:"address_country" json:"addressCountry"`
	PrivacyPolicyURL   string     `db:"privacy_policy_url" json:"privacyPolicyUrl"`
	CustomCSS          string     `db:"custom_css" json:"customCss"`
	CustomCSSEnabled   bool       `db:"custom_css_enabled" json:"customCssEnabled"`
	PublishedAt        *time.Time `db:"published_at" json:"publishedAt,omitempty"`
	CreatedAt          time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt          time.Time  `db:"updated_at" json:"updatedAt"`
}

type CareersSetupState struct {
	WorkspaceID    string         `db:"workspace_id" json:"workspaceId"`
	CurrentStep    string         `db:"current_step" json:"currentStep"`
	ConfirmedSteps pq.StringArray `db:"confirmed_steps" json:"confirmedSteps"`
	CompletedAt    *time.Time     `db:"completed_at" json:"completedAt,omitempty"`
	CreatedAt      time.Time      `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updatedAt"`
}

type CareersTeamMember struct {
	ID           string    `db:"id" json:"id"`
	WorkspaceID  string    `db:"workspace_id" json:"workspaceId"`
	DisplayOrder int       `db:"display_order" json:"displayOrder"`
	Name         string    `db:"name" json:"name"`
	Role         string    `db:"role" json:"role"`
	Bio          string    `db:"bio" json:"bio"`
	ImageURL     string    `db:"image_url" json:"imageUrl"`
	LinkedInURL  string    `db:"linkedin_url" json:"linkedinUrl"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

type CareersGalleryItem struct {
	ID           string    `db:"id" json:"id"`
	WorkspaceID  string    `db:"workspace_id" json:"workspaceId"`
	DisplayOrder int       `db:"display_order" json:"displayOrder"`
	Section      string    `db:"section" json:"section"`
	AltText      string    `db:"alt_text" json:"altText"`
	Caption      string    `db:"caption" json:"caption"`
	ImageURL     string    `db:"image_url" json:"imageUrl"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

type CareersMediaAsset struct {
	ID           string    `db:"id" json:"id"`
	WorkspaceID  string    `db:"workspace_id" json:"workspaceId"`
	Purpose      string    `db:"purpose" json:"purpose"`
	Filename     string    `db:"filename" json:"filename"`
	ContentType  string    `db:"content_type" json:"contentType"`
	SizeBytes    int64     `db:"size_bytes" json:"sizeBytes"`
	ArtifactPath string    `db:"artifact_path" json:"artifactPath"`
	PublicURL    string    `db:"public_url" json:"publicUrl"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
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
	ID                              string                          `db:"id" json:"id"`
	WorkspaceID                     string                          `db:"workspace_id" json:"workspaceId"`
	VacancyID                       string                          `db:"vacancy_id" json:"vacancyId"`
	ApplicantID                     string                          `db:"applicant_id" json:"applicantId"`
	CaseID                          string                          `db:"case_id" json:"caseId"`
	ContactID                       string                          `db:"contact_id" json:"contactId"`
	FormSubmissionID                string                          `db:"form_submission_id" json:"formSubmissionId"`
	SourceKind                      atsdomain.ApplicationSourceKind `db:"source_kind" json:"sourceKind"`
	SourceRefID                     string                          `db:"source_ref_id" json:"sourceRefId"`
	Source                          string                          `db:"source" json:"source"`
	SubmissionFullName              string                          `db:"submission_full_name" json:"submissionFullName"`
	SubmissionEmail                 string                          `db:"submission_email" json:"submissionEmail"`
	SubmissionPhone                 string                          `db:"submission_phone" json:"submissionPhone"`
	SubmissionLocation              string                          `db:"submission_location" json:"submissionLocation"`
	SubmissionLinkedInURL           string                          `db:"submission_linkedin_url" json:"submissionLinkedInUrl"`
	SubmissionPortfolioURL          string                          `db:"submission_portfolio_url" json:"submissionPortfolioUrl"`
	SubmissionCoverNote             string                          `db:"submission_cover_note" json:"submissionCoverNote"`
	SubmissionResumeAttachmentID    string                          `db:"submission_resume_attachment_id" json:"submissionResumeAttachmentId"`
	SubmissionCoverLetterAttachment string                          `db:"submission_cover_letter_attachment" json:"submissionCoverLetterAttachment"`
	Stage                           atsdomain.ApplicationStage      `db:"stage" json:"stage"`
	AppliedAt                       time.Time                       `db:"applied_at" json:"appliedAt"`
	LastStageChangedAt              time.Time                       `db:"last_stage_changed_at" json:"lastStageChangedAt"`
	ReviewedAt                      *time.Time                      `db:"reviewed_at" json:"reviewedAt,omitempty"`
	HiredAt                         *time.Time                      `db:"hired_at" json:"hiredAt,omitempty"`
	RejectedAt                      *time.Time                      `db:"rejected_at" json:"rejectedAt,omitempty"`
	WithdrawnAt                     *time.Time                      `db:"withdrawn_at" json:"withdrawnAt,omitempty"`
	RejectionReason                 string                          `db:"rejection_reason" json:"rejectionReason"`
	CreatedAt                       time.Time                       `db:"created_at" json:"createdAt"`
	UpdatedAt                       time.Time                       `db:"updated_at" json:"updatedAt"`
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
	Applicant     Applicant       `json:"applicant"`
	Application   Application     `json:"application"`
	Notes         []RecruiterNote `json:"notes"`
	CaseQueueID   string          `json:"caseQueueId,omitempty"`
	CaseQueueSlug string          `json:"caseQueueSlug,omitempty"`
	CaseQueueName string          `json:"caseQueueName,omitempty"`
	IsTalentPool  bool            `json:"isTalentPool"`
}

type WorkspaceDefaults struct {
	StagePresets []StagePreset `json:"stagePresets"`
	SavedFilters []SavedFilter `json:"savedFilters"`
}

type CreateJobInput struct {
	WorkspaceID             string
	Slug                    string
	Title                   string
	Team                    string
	Location                string
	WorkMode                atsdomain.WorkMode
	EmploymentType          atsdomain.EmploymentType
	Summary                 string
	Description             string
	Language                string
	AboutTheJob             string
	Responsibilities        []string
	ResponsibilitiesHeading string
	AboutYou                string
	AboutYouHeading         string
	Profile                 []string
	OffersIntro             string
	Offers                  []string
	OffersHeading           string
	Quote                   string
}

type UpdateJobInput struct {
	Title                   string
	Team                    string
	Location                string
	WorkMode                atsdomain.WorkMode
	EmploymentType          atsdomain.EmploymentType
	Summary                 string
	Description             string
	Language                string
	AboutTheJob             string
	Responsibilities        []string
	ResponsibilitiesHeading string
	AboutYou                string
	AboutYouHeading         string
	Profile                 []string
	OffersIntro             string
	Offers                  []string
	OffersHeading           string
	Quote                   string
}

type UpsertCareersSiteInput struct {
	WorkspaceID        string
	CompanyName        string
	SiteTitle          string
	Tagline            string
	MetaDescription    string
	HeroEyebrow        string
	HeroTitle          string
	HeroBody           string
	HeroPrimaryLabel   string
	HeroPrimaryHref    string
	HeroSecondaryLabel string
	HeroSecondaryHref  string
	StoryHeading       string
	StoryBody          string
	JobsHeading        string
	JobsIntro          string
	TeamHeading        string
	TeamIntro          string
	GalleryHeading     string
	GalleryIntro       string
	ContactEmail       string
	WebsiteURL         string
	LinkedInURL        string
	InstagramURL       string
	XURL               string
	LogoURL            string
	HeroImageURL       string
	OgImageURL         string
	PrimaryColor       string
	AccentColor        string
	SurfaceColor       string
	BackgroundColor    string
	TextColor          string
	MutedColor         string
	StreetAddress      string
	AddressLocality    string
	AddressRegion      string
	PostalCode         string
	AddressCountry     string
	PrivacyPolicyURL   string
	CustomCSS          string
	CustomCSSEnabled   bool
}

type SubmitApplicationInput struct {
	WorkspaceID string
	VacancySlug string
	Submission  atsdomain.CandidateSubmission
}

type CandidateListScope string

const (
	CandidateListScopeAll        CandidateListScope = "all"
	CandidateListScopeJob        CandidateListScope = "job"
	CandidateListScopeGeneral    CandidateListScope = "general"
	CandidateListScopeTalentPool CandidateListScope = "talent_pool"
)

type CandidateListOptions struct {
	VacancyID       string
	Scope           CandidateListScope
	ViewSlug        string
	StagePresetSlug string
}

type SavedViewCriteria struct {
	Stages          []string `json:"stages,omitempty"`
	SourceKinds     []string `json:"sourceKinds,omitempty"`
	QueueSlugs      []string `json:"queueSlugs,omitempty"`
	VacancyStatuses []string `json:"vacancyStatuses,omitempty"`
	VacancyKinds    []string `json:"vacancyKinds,omitempty"`
	TalentPoolOnly  bool     `json:"talentPoolOnly,omitempty"`
}

type StageChangeInput struct {
	Stage      atsdomain.ApplicationStage
	ActorName  string
	ActorType  string
	Reason     string
	Note       string
	OccurredAt time.Time
}

type CandidateRouteInput struct {
	Destination string
	ActorName   string
	ActorType   string
	Reason      string
	Note        string
}

type SubmissionResult struct {
	Vacancy     Vacancy     `json:"vacancy"`
	Applicant   Applicant   `json:"applicant"`
	Application Application `json:"application"`
}

type SetupChecklistStep struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ActionLabel string `json:"actionLabel"`
	Completed   bool   `json:"completed"`
}

type SetupStatus struct {
	WorkspaceID string               `json:"workspaceId"`
	CurrentStep string               `json:"currentStep"`
	IsCompleted bool                 `json:"isCompleted"`
	CompletedAt *time.Time           `json:"completedAt,omitempty"`
	PublishedAt *time.Time           `json:"publishedAt,omitempty"`
	Steps       []SetupChecklistStep `json:"steps"`
}

type CareersSiteBundle struct {
	Site                 CareersSiteProfile   `json:"site"`
	Team                 []CareersTeamMember  `json:"team"`
	Gallery              []CareersGalleryItem `json:"gallery"`
	Assets               []CareersMediaAsset  `json:"assets"`
	Jobs                 []Vacancy            `json:"jobs"`
	Setup                SetupStatus          `json:"setup"`
	PreviewURL           string               `json:"previewUrl"`
	ResumeUploadsEnabled bool                 `json:"resumeUploadsEnabled"`
}
