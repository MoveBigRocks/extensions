package runtimehost

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const (
	CoreCaseIssueLookupPath = "/__mbr/host/v1/case-issues/lookup"
	CoreEventsPath          = "/__mbr/host/v1/events"
)

type HostWorkspace struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	ShortCode    string `json:"shortCode,omitempty"`
	Description  string `json:"description,omitempty"`
	LogoURL      string `json:"logoUrl,omitempty"`
	PrimaryColor string `json:"primaryColor,omitempty"`
	AccentColor  string `json:"accentColor,omitempty"`
	IsActive     bool   `json:"isActive"`
	IsSuspended  bool   `json:"isSuspended"`
}

type workspaceListResponse struct {
	Workspaces []HostWorkspace `json:"workspaces"`
}

func (c *Client) ListWorkspaces(ctx context.Context) ([]HostWorkspace, error) {
	var out workspaceListResponse
	if err := c.doJSON(ctx, http.MethodGet, CoreWorkspacesPath, nil, &out); err != nil {
		return nil, err
	}
	return out.Workspaces, nil
}

func (c *Client) GetWorkspacesByIDs(ctx context.Context, ids []string) ([]HostWorkspace, error) {
	var out workspaceListResponse
	body := struct {
		IDs []string `json:"ids"`
	}{IDs: ids}
	if err := c.doJSON(ctx, http.MethodPost, CoreWorkspacesPath+"/by-ids", body, &out); err != nil {
		return nil, err
	}
	return out.Workspaces, nil
}

type LinkIssueToCaseInput struct {
	WorkspaceID string `json:"workspaceId"`
	IssueID     string `json:"issueId"`
	ProjectID   string `json:"projectId,omitempty"`
}

func (c *Client) LinkIssueToCase(ctx context.Context, workspaceID, caseID, issueID, projectID string) error {
	return c.doJSON(ctx, http.MethodPost, CoreCasesPath+"/"+url.PathEscape(strings.TrimSpace(caseID))+"/issues", LinkIssueToCaseInput{
		WorkspaceID: workspaceID,
		IssueID:     issueID,
		ProjectID:   projectID,
	}, nil)
}

type UnlinkIssueFromCaseInput struct {
	WorkspaceID string `json:"workspaceId"`
	IssueID     string `json:"issueId"`
}

func (c *Client) UnlinkIssueFromCase(ctx context.Context, workspaceID, caseID, issueID string) error {
	return c.doJSON(ctx, http.MethodDelete, CoreCasesPath+"/"+url.PathEscape(strings.TrimSpace(caseID))+"/issues", UnlinkIssueFromCaseInput{
		WorkspaceID: workspaceID,
		IssueID:     issueID,
	}, nil)
}

func (c *Client) GetCaseByIssueAndContact(ctx context.Context, workspaceID, issueID, contactID string) (*HostCase, bool, error) {
	query := url.Values{}
	query.Set("workspaceId", strings.TrimSpace(workspaceID))
	query.Set("issueId", strings.TrimSpace(issueID))
	query.Set("contactId", strings.TrimSpace(contactID))
	var out HostCase
	if err := c.doJSON(ctx, http.MethodGet, CoreCaseIssueLookupPath+"?"+query.Encode(), nil, &out); err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &out, true, nil
}

type PublishEventInput struct {
	WorkspaceID string         `json:"workspaceId"`
	EventType   string         `json:"eventType"`
	Data        map[string]any `json:"data"`
}

func (c *Client) PublishEvent(ctx context.Context, input PublishEventInput) error {
	return c.doJSON(ctx, http.MethodPost, CoreEventsPath, input, nil)
}
