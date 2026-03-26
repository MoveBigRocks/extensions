package atsdomain

import (
	"testing"
	"time"
)

func TestVacancyLifecycle(t *testing.T) {
	t.Parallel()

	vacancy, err := NewVacancy("ws_hiring", "founding-engineer", "Founding Engineer")
	if err != nil {
		t.Fatalf("new vacancy: %v", err)
	}
	if vacancy.Status != VacancyStatusDraft {
		t.Fatalf("expected draft vacancy, got %q", vacancy.Status)
	}

	publishedAt := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	if err := vacancy.Publish(publishedAt); err != nil {
		t.Fatalf("publish vacancy: %v", err)
	}
	if !vacancy.IsOpen() {
		t.Fatalf("expected vacancy to be open after publish")
	}
	if vacancy.PublishedAt == nil || !vacancy.PublishedAt.Equal(publishedAt) {
		t.Fatalf("expected published_at to be recorded")
	}

	closedAt := publishedAt.Add(24 * time.Hour)
	if err := vacancy.Close(closedAt); err != nil {
		t.Fatalf("close vacancy: %v", err)
	}
	if vacancy.Status != VacancyStatusClosed {
		t.Fatalf("expected closed vacancy, got %q", vacancy.Status)
	}
	if vacancy.ClosedAt == nil || !vacancy.ClosedAt.Equal(closedAt) {
		t.Fatalf("expected closed_at to be recorded")
	}

	if err := vacancy.Reopen(closedAt.Add(time.Hour)); err != nil {
		t.Fatalf("reopen vacancy: %v", err)
	}
	if vacancy.Status != VacancyStatusOpen {
		t.Fatalf("expected vacancy to reopen, got %q", vacancy.Status)
	}
}

func TestBuildCandidateRecord(t *testing.T) {
	t.Parallel()

	vacancy, err := NewVacancy("ws_hiring", "product-designer", "Product Designer")
	if err != nil {
		t.Fatalf("new vacancy: %v", err)
	}
	vacancy.ID = "vac_123"

	applicant, application, err := BuildCandidateRecord("", vacancy, CandidateSubmission{
		FullName:           "Ada Lovelace",
		Email:              "Ada@Example.com",
		Phone:              "+31 6 1234 5678",
		Location:           "Amsterdam",
		LinkedInURL:        "https://www.linkedin.com/in/ada",
		PortfolioURL:       "https://ada.example.com",
		CoverNote:          "I care about product craft.",
		ResumeAttachmentID: "att_resume_123",
		FormSubmissionID:   "sub_123",
	})
	if err != nil {
		t.Fatalf("build candidate record: %v", err)
	}

	if applicant.WorkspaceID != "ws_hiring" {
		t.Fatalf("expected applicant workspace to default from vacancy, got %q", applicant.WorkspaceID)
	}
	if applicant.Email != "ada@example.com" {
		t.Fatalf("expected normalized applicant email, got %q", applicant.Email)
	}
	if applicant.ResumeAttachmentID != "att_resume_123" {
		t.Fatalf("expected resume attachment to be preserved")
	}

	if application.VacancyID != "vac_123" {
		t.Fatalf("expected application vacancy id, got %q", application.VacancyID)
	}
	if application.Stage != ApplicationStageReceived {
		t.Fatalf("expected new application stage received, got %q", application.Stage)
	}
	if application.FormSubmissionID != "sub_123" {
		t.Fatalf("expected form submission id, got %q", application.FormSubmissionID)
	}
}

func TestApplicationStageTransitions(t *testing.T) {
	t.Parallel()

	application, err := NewApplication("ws_hiring", "vac_123", "applicant_123", "careers_form")
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	if err := application.AdvanceTo(ApplicationStageScreening, time.Date(2026, 3, 26, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("advance to screening: %v", err)
	}
	if err := application.AdvanceTo(ApplicationStageInterview, time.Date(2026, 3, 27, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("advance to interview: %v", err)
	}
	if application.ReviewedAt == nil {
		t.Fatalf("expected reviewed_at to be set after interview stage")
	}

	if err := application.Reject("not enough backend depth", time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("reject application: %v", err)
	}
	if !application.IsTerminal() {
		t.Fatalf("expected rejected application to be terminal")
	}
	if err := application.AdvanceTo(ApplicationStageOffer, time.Date(2026, 3, 29, 9, 0, 0, 0, time.UTC)); err == nil {
		t.Fatalf("expected terminal application stage change to fail")
	}
}
