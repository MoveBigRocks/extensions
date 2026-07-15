package sql

import (
	"bytes"
	"strings"
	"testing"

	errortrackingui "github.com/movebigrocks/extensions/error-tracking/runtimeui"
)

func TestApplicationDetailTemplateRendersInstanceWorkspaceOptions(t *testing.T) {
	tmpl, err := errortrackingui.ParseTemplates()
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	data := ApplicationDetailPageData{
		BasePageData: BasePageData{
			ActivePage:        "applications",
			PageTitle:         "New Application",
			PageSubtitle:      "Create a new monitored application",
			IsWorkspaceScoped: false,
		},
		Workspaces: []WorkspaceOption{
			{ID: "019d21cd-b4f2-712b-ad21-9b521119c4a0", Name: "Support"},
			{ID: "019d21cd-d7e7-77ad-b888-2e1f1029d00e", Name: "Marketing"},
		},
		IsNew:                true,
		ApplicationsBasePath: errorTrackingApplicationsBasePath,
	}

	var rendered bytes.Buffer
	if err := tmpl.ExecuteTemplate(&rendered, "application_detail.html", data); err != nil {
		t.Fatalf("render application detail: %v", err)
	}
	page := rendered.String()
	for _, workspace := range data.Workspaces {
		if !strings.Contains(page, `value="`+workspace.ID+`"`) {
			t.Fatalf("rendered page is missing workspace %s", workspace.ID)
		}
	}
}
