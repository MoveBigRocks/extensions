package runtimehost

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// AppendCaseNoteInput records extension-owned evidence without inventing a human author.
// The caller must retain the same key and body across uncertain delivery retries.
type AppendCaseNoteInput struct {
	WorkspaceID    string `json:"workspaceId,omitempty"`
	IdempotencyKey string `json:"idempotencyKey"`
	Body           string `json:"body"`
}

type HostCaseNote struct {
	ID          string `json:"id"`
	CaseID      string `json:"caseId"`
	WorkspaceID string `json:"workspaceId"`
}

func (c *Client) AppendCaseNote(ctx context.Context, caseID string, input AppendCaseNoteInput) (*HostCaseNote, error) {
	var out HostCaseNote
	err := c.doJSON(ctx, http.MethodPost, CoreCasesPath+"/"+url.PathEscape(strings.TrimSpace(caseID))+"/notes", input, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
