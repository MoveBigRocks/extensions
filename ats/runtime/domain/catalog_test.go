package atsdomain

import (
	"testing"
	"time"
)

func TestCandidateSubmissionFromFields(t *testing.T) {
	t.Parallel()

	vacancySlug, submission, err := CandidateSubmissionFromFields(map[string]any{
		"full_name":            "Margaret Hamilton",
		"email":                "margaret@example.com",
		"role_slug":            "founding-product-engineer",
		"resume_attachment_id": "att_resume_456",
		"cover_note":           "I ship calm systems.",
	})
	if err != nil {
		t.Fatalf("candidate submission from fields: %v", err)
	}

	if vacancySlug != "founding-product-engineer" {
		t.Fatalf("expected normalized vacancy slug, got %q", vacancySlug)
	}
	if submission.ResumeAttachmentID != "att_resume_456" {
		t.Fatalf("expected resume attachment id to round-trip")
	}
	if submission.FullName != "Margaret Hamilton" {
		t.Fatalf("expected full name to round-trip, got %q", submission.FullName)
	}
}

func TestVacancyCatalogBuildCandidateRecordForVacancy(t *testing.T) {
	t.Parallel()

	vacancy, err := NewVacancy("ws_hiring", "founding-product-engineer", "Founding Product Engineer")
	if err != nil {
		t.Fatalf("new vacancy: %v", err)
	}
	vacancy.Team = "Product"
	vacancy.ID = "vac_founding_product_engineer"
	if err := vacancy.Publish(time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("publish vacancy: %v", err)
	}

	catalog, err := NewVacancyCatalog("ws_hiring", vacancy)
	if err != nil {
		t.Fatalf("new vacancy catalog: %v", err)
	}

	applicant, application, err := catalog.BuildCandidateRecordForVacancy("founding-product-engineer", CandidateSubmission{
		FullName:           "Margaret Hamilton",
		Email:              "margaret@example.com",
		ResumeAttachmentID: "att_resume_789",
		Source:             "careers_form",
	})
	if err != nil {
		t.Fatalf("build candidate record for vacancy: %v", err)
	}

	if applicant.WorkspaceID != "ws_hiring" {
		t.Fatalf("expected applicant workspace to match catalog, got %q", applicant.WorkspaceID)
	}
	if application.VacancyID != "vac_founding_product_engineer" {
		t.Fatalf("expected application to point at published vacancy, got %q", application.VacancyID)
	}
}

func TestVacancyCatalogOpenVacancies(t *testing.T) {
	t.Parallel()

	openVacancy, err := NewVacancy("ws_hiring", "backend-engineer", "Backend Engineer")
	if err != nil {
		t.Fatalf("new open vacancy: %v", err)
	}
	openVacancy.Team = "Engineering"
	if err := openVacancy.Publish(time.Date(2026, 3, 26, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("publish open vacancy: %v", err)
	}

	closedVacancy, err := NewVacancy("ws_hiring", "customer-ops", "Customer Operations Lead")
	if err != nil {
		t.Fatalf("new closed vacancy: %v", err)
	}
	closedVacancy.Team = "Operations"
	if err := closedVacancy.Close(time.Date(2026, 3, 26, 9, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("close vacancy: %v", err)
	}

	catalog, err := NewVacancyCatalog("ws_hiring", openVacancy, closedVacancy)
	if err != nil {
		t.Fatalf("new vacancy catalog: %v", err)
	}

	open := catalog.OpenVacancies()
	if len(open) != 1 {
		t.Fatalf("expected exactly one open vacancy, got %d", len(open))
	}
	if open[0].Slug != "backend-engineer" {
		t.Fatalf("expected backend engineer vacancy to remain open, got %q", open[0].Slug)
	}
}
