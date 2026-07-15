package runtimehost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimeproto"
)

const IdentitySessionPath = "/__mbr/host/v1/identity/session"

type IdentitySessionRequest struct {
	Email                string `json:"email"`
	Name                 string `json:"name"`
	InstanceRole         string `json:"instanceRole,omitempty"`
	AllowJITProvisioning bool   `json:"allowJitProvisioning"`
	IPAddress            string `json:"ipAddress,omitempty"`
	UserAgent            string `json:"userAgent,omitempty"`
}

type IdentitySessionResponse struct {
	UserID           string    `json:"userId"`
	UserEmail        string    `json:"userEmail"`
	UserName         string    `json:"userName"`
	InstanceRole     string    `json:"instanceRole,omitempty"`
	SessionToken     string    `json:"sessionToken"`
	SessionCreatedAt time.Time `json:"sessionCreatedAt"`
	SessionExpiresAt time.Time `json:"sessionExpiresAt"`
	CookieName       string    `json:"cookieName,omitempty"`
	CookieDomain     string    `json:"cookieDomain,omitempty"`
	CookiePath       string    `json:"cookiePath,omitempty"`
	CookieSecure     bool      `json:"cookieSecure,omitempty"`
	Message          string    `json:"message,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewClientFromRequest(req *http.Request) (*Client, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	token := strings.TrimSpace(req.Header.Get(runtimeproto.HeaderHostToken))
	if token == "" {
		return nil, fmt.Errorf("host token is missing")
	}
	// Prefer the host-API base URL the host supplies explicitly. It always
	// points at a surface that serves /__mbr/host/v1, independent of which
	// public, admin, or workspace domain the inbound request arrived on. Fall
	// back to the forwarded host only when the host has not supplied it, so an
	// older host that injects just the forwarded headers keeps working.
	baseURL := strings.TrimSpace(req.Header.Get(runtimeproto.HeaderAPIBaseURL))
	if baseURL == "" {
		baseURL = forwardedBaseURL(req)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("forwarded host context is missing")
	}
	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *Client) IssueIdentitySession(ctx context.Context, input IdentitySessionRequest) (*IdentitySessionResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("host client is not configured")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("host client base URL is not configured")
	}
	if strings.TrimSpace(c.Token) == "" {
		return nil, fmt.Errorf("host client token is not configured")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode identity session request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+IdentitySessionPath, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build identity session request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("issue identity session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var failure ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&failure); err == nil && strings.TrimSpace(failure.Message) != "" {
			return nil, fmt.Errorf("issue identity session: %s", strings.TrimSpace(failure.Message))
		}
		return nil, fmt.Errorf("issue identity session: host returned %s", resp.Status)
	}

	var output IdentitySessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return nil, fmt.Errorf("decode identity session response: %w", err)
	}
	return &output, nil
}

func forwardedBaseURL(req *http.Request) string {
	if req == nil {
		return ""
	}
	scheme := firstHeaderValue(req, "X-Forwarded-Proto")
	if scheme == "" {
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := firstHeaderValue(req, "X-Forwarded-Host")
	if host == "" {
		host = strings.TrimSpace(req.Host)
	}
	if host == "" || host == "unix" {
		return ""
	}
	return scheme + "://" + host
}

func firstHeaderValue(req *http.Request, key string) string {
	if req == nil {
		return ""
	}
	raw := strings.TrimSpace(req.Header.Get(key))
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	return strings.TrimSpace(parts[0])
}
