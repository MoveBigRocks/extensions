package runtimehost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func defaultHostHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// Core-data host API. These let a first-party extension read and write core
// entities (cases, contacts, queues, workspaces) through the host instead of
// embedding a copy of the core stores. The extension holds no database handle
// to core tables; the host applies the extension's workspace scope and
// row-level security. Each path lives under the same /__mbr/host/v1 prefix and
// bearer-token auth as IssueIdentitySession.
const (
	CoreCasesPath      = "/__mbr/host/v1/cases"
	CoreContactsPath   = "/__mbr/host/v1/contacts"
	CoreQueuesPath     = "/__mbr/host/v1/queues"
	CoreWorkspacesPath = "/__mbr/host/v1/workspaces"
)

// HostCase is the host's view of a core case. It carries the fields a
// first-party extension reads or round-trips; targeted update calls (custom
// fields, tags, status) avoid whole-case read-modify-write, so this type does
// not need to reproduce every core column.
type HostCase struct {
	ID           string         `json:"id"`
	HumanID      string         `json:"humanId,omitempty"`
	WorkspaceID  string         `json:"workspaceId"`
	Subject      string         `json:"subject"`
	Description  string         `json:"description,omitempty"`
	Status       string         `json:"status,omitempty"`
	Priority     string         `json:"priority,omitempty"`
	Channel      string         `json:"channel,omitempty"`
	Category     string         `json:"category,omitempty"`
	QueueID      string         `json:"queueId,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	ContactID    string         `json:"contactId,omitempty"`
	ContactEmail string         `json:"contactEmail,omitempty"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

// CreateCaseInput mirrors the core CreateCaseParams the extensions build, with
// the workspace supplied by the host from the extension's scope rather than the
// caller. Priority and Channel are the string forms of the core enums.
type CreateCaseInput struct {
	WorkspaceID  string         `json:"workspaceId,omitempty"`
	Subject      string         `json:"subject"`
	Description  string         `json:"description,omitempty"`
	Priority     string         `json:"priority,omitempty"`
	Channel      string         `json:"channel,omitempty"`
	Category     string         `json:"category,omitempty"`
	QueueID      string         `json:"queueId,omitempty"`
	ContactID    string         `json:"contactId,omitempty"`
	ContactName  string         `json:"contactName,omitempty"`
	ContactEmail string         `json:"contactEmail,omitempty"`
	TeamID       string         `json:"teamId,omitempty"`
	AssignedToID string         `json:"assignedToId,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

// CreateCase creates a core case in the extension's workspace and returns it.
func (c *Client) CreateCase(ctx context.Context, input CreateCaseInput) (*HostCase, error) {
	var out HostCase
	if err := c.doJSON(ctx, http.MethodPost, CoreCasesPath, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCase returns a core case the extension is allowed to see (in its
// workspace). It returns a nil case and false when the case does not exist.
func (c *Client) GetCase(ctx context.Context, caseID string) (*HostCase, bool, error) {
	return c.GetCaseInWorkspace(ctx, "", caseID)
}

// GetCaseInWorkspace returns a case in the named workspace. Instance-scoped
// extensions must supply workspaceID; workspace-scoped extensions may leave it
// empty or pass their installed workspace.
func (c *Client) GetCaseInWorkspace(ctx context.Context, workspaceID, caseID string) (*HostCase, bool, error) {
	var out HostCase
	path := CoreCasesPath + "/" + url.PathEscape(strings.TrimSpace(caseID))
	if workspaceID = strings.TrimSpace(workspaceID); workspaceID != "" {
		path += "?workspaceId=" + url.QueryEscape(workspaceID)
	}
	err := c.doJSON(ctx, http.MethodGet, path, nil, &out)
	if err != nil {
		if isNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &out, true, nil
}

// doJSON performs a host-API request with bearer auth, encoding body (when
// non-nil) and decoding a JSON response into out (when non-nil). It is the
// shared transport for every core-data call, matching IssueIdentitySession.
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	if c == nil {
		return fmt.Errorf("host client is not configured")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("host client base URL is not configured")
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("host client token is not configured")
	}

	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s request: %w", path, err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, reader)
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := c.HTTPClient
	if client == nil {
		client = defaultHostHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var failure ErrorResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&failure); decErr == nil && strings.TrimSpace(failure.Message) != "" {
			return &hostError{status: resp.StatusCode, message: strings.TrimSpace(failure.Message)}
		}
		return &hostError{status: resp.StatusCode, message: resp.Status}
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode %s response: %w", path, err)
		}
	}
	return nil
}

type hostError struct {
	status  int
	message string
}

func (e *hostError) Error() string { return e.message }

func isNotFound(err error) bool {
	var he *hostError
	if errors.As(err, &he) {
		return he.status == http.StatusNotFound
	}
	return false
}
