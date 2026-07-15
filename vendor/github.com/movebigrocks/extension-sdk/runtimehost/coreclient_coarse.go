package runtimehost

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Coarse host operations. A coarse operation performs several core writes that
// must commit together (which an extension cannot do across the API boundary as
// separate calls) in one core transaction, keyed by an idempotency key so a
// retry returns the same result instead of duplicating the core rows. The
// extension writes its own tables in its own transaction and calls the coarse
// operation for the core side.
const CoreIngestApplicationPath = "/__mbr/host/v1/ingest/application"

// IngestCaseInput is the case portion of an application ingest. The contact is
// created from the Contact field and linked to this case by the host, so this
// carries no contact identifiers.
type IngestCaseInput struct {
	Subject      string         `json:"subject"`
	Description  string         `json:"description,omitempty"`
	Priority     string         `json:"priority,omitempty"`
	Channel      string         `json:"channel,omitempty"`
	Category     string         `json:"category,omitempty"`
	QueueID      string         `json:"queueId,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

// IngestApplicationInput creates a core contact and case and links attachments
// in one transaction. IdempotencyKey (for example the extension's application
// id) makes the call safe to retry.
type IngestApplicationInput struct {
	IdempotencyKey string             `json:"idempotencyKey"`
	Contact        CreateContactInput `json:"contact"`
	Case           IngestCaseInput    `json:"case"`
	AttachmentIDs  []string           `json:"attachmentIds,omitempty"`
}

// IngestApplicationResult is the identifiers of the created (or previously
// created) core contact and case.
type IngestApplicationResult struct {
	ContactID string `json:"contactId"`
	CaseID    string `json:"caseId"`
}

// IngestApplication atomically creates the core contact and case for an
// application and links its attachments, returning their ids. Calling it again
// with the same idempotency key returns the same ids without creating new rows.
func (c *Client) IngestApplication(ctx context.Context, input IngestApplicationInput) (*IngestApplicationResult, error) {
	var out IngestApplicationResult
	if err := c.doJSON(ctx, http.MethodPost, CoreIngestApplicationPath, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ApplyCaseChangeInput updates a case and evaluates the workspace's automation
// rules for it in one core transaction. IdempotencyKey (for example the
// stage-change event id) makes the call safe to retry without re-firing rules.
type ApplyCaseChangeInput struct {
	IdempotencyKey string          `json:"idempotencyKey"`
	Patch          CaseUpdateInput `json:"patch"`
	Event          string          `json:"event,omitempty"`
	Changes        map[string]any  `json:"changes,omitempty"`
}

// ApplyCaseChange applies a case patch and fires automation rules for the case
// in one core transaction, and returns the case. A repeat call with the same
// idempotency key returns the case without re-applying the patch or re-firing
// the rules.
func (c *Client) ApplyCaseChange(ctx context.Context, caseID string, input ApplyCaseChangeInput) (*HostCase, error) {
	var out HostCase
	path := CoreCasesPath + "/" + url.PathEscape(strings.TrimSpace(caseID)) + "/apply-change"
	if err := c.doJSON(ctx, http.MethodPost, path, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
