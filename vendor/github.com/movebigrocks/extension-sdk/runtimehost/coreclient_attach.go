package runtimehost

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Attachment, rule-evaluation, and artifact-publish host operations. These
// complete the surface a first-party extension needs from core, on the same
// bearer-token transport as the case and queue operations. Binary payloads
// travel as base64 (Go encodes []byte fields that way), which suits the small
// files these paths carry (resumes, generated site assets).
const (
	CoreAttachmentsPath   = "/__mbr/host/v1/attachments"
	CoreRulesEvaluatePath = "/__mbr/host/v1/rules/evaluate"
	CoreArtifactsPath     = "/__mbr/host/v1/artifacts"
)

// HostAttachment is the host's view of a stored, scanned attachment.
type HostAttachment struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Status      string `json:"status,omitempty"`
	SHA256Hash  string `json:"sha256Hash,omitempty"`
	CaseID      string `json:"caseId,omitempty"`
	ContactID   string `json:"contactId,omitempty"`
	Source      string `json:"source,omitempty"`
}

// UploadAttachmentInput uploads a file to the extension's workspace. The host
// stores and virus-scans it and returns the stored record.
type UploadAttachmentInput struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType,omitempty"`
	Description string `json:"description,omitempty"`
	ContactID   string `json:"contactId,omitempty"`
	Content     []byte `json:"content"`
}

// UploadAttachment stores and scans a file and returns the attachment record.
func (c *Client) UploadAttachment(ctx context.Context, input UploadAttachmentInput) (*HostAttachment, error) {
	var out HostAttachment
	if err := c.doJSON(ctx, http.MethodPost, CoreAttachmentsPath, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAttachment returns an attachment in the extension's workspace, or
// (nil,false,nil) when it does not exist.
func (c *Client) GetAttachment(ctx context.Context, attachmentID string) (*HostAttachment, bool, error) {
	var out HostAttachment
	err := c.doJSON(ctx, http.MethodGet, CoreAttachmentsPath+"/"+url.PathEscape(strings.TrimSpace(attachmentID)), nil, &out)
	if err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &out, true, nil
}

// LinkAttachmentsToCase links stored attachments to a case in the extension's
// workspace.
func (c *Client) LinkAttachmentsToCase(ctx context.Context, caseID string, attachmentIDs []string) error {
	body := map[string]any{"attachmentIds": attachmentIDs}
	return c.doJSON(ctx, http.MethodPost, CoreCasesPath+"/"+url.PathEscape(strings.TrimSpace(caseID))+"/attachments", body, nil)
}

// EvaluateRulesInput fires the workspace's automation rules for a case on an
// event, with the field changes that triggered it.
type EvaluateRulesInput struct {
	CaseID  string         `json:"caseId"`
	Event   string         `json:"event"`
	Changes map[string]any `json:"changes,omitempty"`
}

// EvaluateRulesForCase runs automation rules for a case in the extension's
// workspace.
func (c *Client) EvaluateRulesForCase(ctx context.Context, input EvaluateRulesInput) error {
	return c.doJSON(ctx, http.MethodPost, CoreRulesEvaluatePath, input, nil)
}

// PublishArtifactInput publishes generated content to one of the extension's
// declared artifact surfaces (for example a workspace website).
type PublishArtifactInput struct {
	Surface      string `json:"surface"`
	RelativePath string `json:"relativePath"`
	ActorID      string `json:"actorId,omitempty"`
	Content      []byte `json:"content"`
}

// PublishArtifact publishes content to one of the extension's artifact surfaces.
func (c *Client) PublishArtifact(ctx context.Context, input PublishArtifactInput) error {
	return c.doJSON(ctx, http.MethodPost, CoreArtifactsPath, input, nil)
}
