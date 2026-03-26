package atsdomain

import (
	"fmt"
	"sort"
	"strings"
)

// VacancyCatalog is the ATS-owned view of the jobs currently managed inside one
// workspace. It lets the extension keep job publishing and application intake in
// ATS language even when the host platform stores the underlying work on shared
// primitives like forms, contacts, cases, and attachments.
type VacancyCatalog struct {
	WorkspaceID string
	vacancies   map[string]*Vacancy
}

func NewVacancyCatalog(workspaceID string, vacancies ...*Vacancy) (*VacancyCatalog, error) {
	catalog := &VacancyCatalog{
		WorkspaceID: strings.TrimSpace(workspaceID),
		vacancies:   make(map[string]*Vacancy, len(vacancies)),
	}
	for _, vacancy := range vacancies {
		if err := catalog.Add(vacancy); err != nil {
			return nil, err
		}
	}
	if catalog.WorkspaceID == "" && len(vacancies) > 0 {
		catalog.WorkspaceID = vacancies[0].WorkspaceID
	}
	if catalog.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	return catalog, nil
}

func (c *VacancyCatalog) Add(vacancy *Vacancy) error {
	if c == nil {
		return fmt.Errorf("vacancy catalog is required")
	}
	if vacancy == nil {
		return fmt.Errorf("vacancy is required")
	}
	if err := vacancy.Validate(); err != nil {
		return err
	}
	if c.WorkspaceID == "" {
		c.WorkspaceID = vacancy.WorkspaceID
	}
	if c.WorkspaceID != vacancy.WorkspaceID {
		return fmt.Errorf("vacancy workspace %q does not match catalog workspace %q", vacancy.WorkspaceID, c.WorkspaceID)
	}
	slug := normalizeSlug(vacancy.Slug)
	if slug == "" {
		return fmt.Errorf("vacancy slug is required")
	}
	if c.vacancies == nil {
		c.vacancies = map[string]*Vacancy{}
	}
	if _, exists := c.vacancies[slug]; exists {
		return fmt.Errorf("vacancy slug %q already exists in catalog", slug)
	}
	c.vacancies[slug] = vacancy
	return nil
}

func (c *VacancyCatalog) FindBySlug(slug string) (*Vacancy, bool) {
	if c == nil || c.vacancies == nil {
		return nil, false
	}
	vacancy, ok := c.vacancies[normalizeSlug(slug)]
	return vacancy, ok
}

func (c *VacancyCatalog) OpenVacancies() []*Vacancy {
	if c == nil || len(c.vacancies) == 0 {
		return nil
	}
	open := make([]*Vacancy, 0, len(c.vacancies))
	for _, vacancy := range c.vacancies {
		if vacancy != nil && vacancy.IsOpen() {
			open = append(open, vacancy)
		}
	}
	sort.Slice(open, func(i, j int) bool {
		left := open[i]
		right := open[j]
		if left.PublishedAt != nil && right.PublishedAt != nil && !left.PublishedAt.Equal(*right.PublishedAt) {
			return left.PublishedAt.Before(*right.PublishedAt)
		}
		if left.Team != right.Team {
			return left.Team < right.Team
		}
		return left.Title < right.Title
	})
	return open
}

func (c *VacancyCatalog) BuildCandidateRecordForVacancy(slug string, submission CandidateSubmission) (*Applicant, *Application, error) {
	if c == nil {
		return nil, nil, fmt.Errorf("vacancy catalog is required")
	}
	vacancy, ok := c.FindBySlug(slug)
	if !ok {
		return nil, nil, fmt.Errorf("vacancy %q not found", normalizeSlug(slug))
	}
	return BuildCandidateRecord(c.WorkspaceID, vacancy, submission)
}

func CandidateSubmissionFromFields(fields map[string]any) (string, CandidateSubmission, error) {
	if len(fields) == 0 {
		return "", CandidateSubmission{}, fmt.Errorf("submission fields are required")
	}

	vacancySlug := stringField(fields, "vacancy_slug")
	if vacancySlug == "" {
		vacancySlug = stringField(fields, "role_slug")
	}
	vacancySlug = normalizeSlug(vacancySlug)
	if vacancySlug == "" {
		return "", CandidateSubmission{}, fmt.Errorf("role_slug or vacancy_slug is required")
	}

	submission := CandidateSubmission{
		FullName:           stringField(fields, "full_name"),
		Email:              stringField(fields, "email"),
		Phone:              stringField(fields, "phone"),
		Location:           stringField(fields, "location"),
		LinkedInURL:        stringField(fields, "linkedin_url"),
		PortfolioURL:       stringField(fields, "portfolio_url"),
		CoverNote:          stringField(fields, "cover_note"),
		ResumeAttachmentID: stringField(fields, "resume_attachment_id"),
		Source:             stringField(fields, "source"),
		FormSubmissionID:   stringField(fields, "form_submission_id"),
	}
	if strings.TrimSpace(submission.FullName) == "" {
		return "", CandidateSubmission{}, fmt.Errorf("full_name is required")
	}
	if strings.TrimSpace(submission.Email) == "" {
		return "", CandidateSubmission{}, fmt.Errorf("email is required")
	}
	return vacancySlug, submission, nil
}

func stringField(fields map[string]any, key string) string {
	value, ok := fields[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}
