package salespipelineui

import "testing"

func TestParseTemplates(t *testing.T) {
	if _, err := ParseTemplates(); err != nil {
		t.Fatalf("parse templates: %v", err)
	}
}
