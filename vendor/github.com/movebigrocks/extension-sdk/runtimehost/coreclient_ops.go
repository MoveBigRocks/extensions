package runtimehost

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// This file extends the core-data host client with the queue, contact, and
// case-mutation operations a first-party extension needs. Every call carries
// the bearer host token and is scoped to the extension's workspace by the host,
// exactly like CreateCase in coreclient.go.

// HostQueue is the host's view of a core case queue.
type HostQueue struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateQueueInput creates a queue in the extension's workspace.
type CreateQueueInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
}

// GetQueue returns a queue by id, or (nil,false,nil) when it does not exist in
// the extension's workspace.
func (c *Client) GetQueue(ctx context.Context, queueID string) (*HostQueue, bool, error) {
	var out HostQueue
	if err := c.doJSON(ctx, http.MethodGet, CoreQueuesPath+"/"+url.PathEscape(strings.TrimSpace(queueID)), nil, &out); err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &out, true, nil
}

// GetQueueBySlug returns a queue by slug in the extension's workspace.
func (c *Client) GetQueueBySlug(ctx context.Context, slug string) (*HostQueue, bool, error) {
	var out HostQueue
	q := CoreQueuesPath + "?slug=" + url.QueryEscape(strings.TrimSpace(slug))
	if err := c.doJSON(ctx, http.MethodGet, q, nil, &out); err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &out, true, nil
}

// CreateQueue creates a queue in the extension's workspace.
func (c *Client) CreateQueue(ctx context.Context, input CreateQueueInput) (*HostQueue, error) {
	var out HostQueue
	if err := c.doJSON(ctx, http.MethodPost, CoreQueuesPath, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HostContact is the host's view of a core contact.
type HostContact struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	Email       string `json:"email"`
	Name        string `json:"name,omitempty"`
	Phone       string `json:"phone,omitempty"`
	Company     string `json:"company,omitempty"`
}

// CreateContactInput creates a contact in the extension's workspace.
type CreateContactInput struct {
	Email    string         `json:"email"`
	Name     string         `json:"name,omitempty"`
	Phone    string         `json:"phone,omitempty"`
	Company  string         `json:"company,omitempty"`
	Source   string         `json:"source,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// CreateContact creates a contact in the extension's workspace.
func (c *Client) CreateContact(ctx context.Context, input CreateContactInput) (*HostContact, error) {
	var out HostContact
	if err := c.doJSON(ctx, http.MethodPost, CoreContactsPath, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CaseUpdateInput is a targeted case patch. Only the non-nil fields are applied,
// so an extension changes a case's tags or custom fields without round-tripping
// and risking clobbering columns it did not intend to touch. Setting a field to
// its pointer sets it; leaving it nil leaves the stored value unchanged.
type CaseUpdateInput struct {
	Status       *string        `json:"status,omitempty"`
	Priority     *string        `json:"priority,omitempty"`
	QueueID      *string        `json:"queueId,omitempty"`
	Category     *string        `json:"category,omitempty"`
	Tags         *[]string      `json:"tags,omitempty"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

// UpdateCase applies a targeted patch to a case in the extension's workspace and
// returns the updated case.
func (c *Client) UpdateCase(ctx context.Context, caseID string, patch CaseUpdateInput) (*HostCase, error) {
	var out HostCase
	if err := c.doJSON(ctx, http.MethodPatch, CoreCasesPath+"/"+url.PathEscape(strings.TrimSpace(caseID)), patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HandoffCaseInput moves a case to another queue, optionally reassigning it.
type HandoffCaseInput struct {
	QueueID          string `json:"queueId"`
	TeamID           string `json:"teamId,omitempty"`
	AssigneeID       string `json:"assigneeId,omitempty"`
	Reason           string `json:"reason,omitempty"`
	PerformedByID    string `json:"performedById,omitempty"`
	PerformedByName  string `json:"performedByName,omitempty"`
	PerformedByType  string `json:"performedByType,omitempty"`
	OnBehalfOfUserID string `json:"onBehalfOfUserId,omitempty"`
}

// HandoffCase moves a case to another queue in the extension's workspace.
func (c *Client) HandoffCase(ctx context.Context, caseID string, input HandoffCaseInput) error {
	return c.doJSON(ctx, http.MethodPost, CoreCasesPath+"/"+url.PathEscape(strings.TrimSpace(caseID))+"/handoff", input, nil)
}

// MarkCaseResolved marks a case resolved at the given time.
func (c *Client) MarkCaseResolved(ctx context.Context, caseID string, resolvedAt time.Time) error {
	body := map[string]any{"resolvedAt": resolvedAt.UTC().Format(time.RFC3339Nano)}
	return c.doJSON(ctx, http.MethodPost, CoreCasesPath+"/"+url.PathEscape(strings.TrimSpace(caseID))+"/resolve", body, nil)
}
