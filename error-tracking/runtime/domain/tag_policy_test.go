package observabilitydomain

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicTagKeysAllowOperationalCorrelationAndDenyPersonalData(t *testing.T) {
	for _, key := range []string{
		"request_id", "route", "method", "status", "service", "queue", "kind", "job_id",
		"stream", "consumer", "event_type", "operation", "task", "component", "alert_type",
		"failure_type", "user_id", "environment", "release", "server_name", "runtime.name",
		"browser", "os.name", "transaction", "level", "handled", "mechanism",
	} {
		require.Truef(t, IsPublicTagKey(key), "%s should be public", key)
	}
	for _, key := range []string{
		"user.email", "email", "ip", "ip_address", "user.ip_address", "username", "user.username",
		"url", "request.url", "client_ip", "origin.ip", "remote_addr", "x_forwarded_for",
		"user.name", "name", "phone", "user.phone", "referer", "http.url", "callback_url",
		"auth_token", "access_token", "session_id", "cookie", "authorization", "password",
		"db.secret", "api_key", "contact_email_hash", "address", "user.geo", "user_agent",
		"Request_ID", "request-id", "request id", "", strings.Repeat("a", 65), "route\n",
	} {
		require.Falsef(t, IsPublicTagKey(key), "%q must stay private", key)
	}
	require.True(t, IsPublicTagKey(strings.Repeat("a", 64)))
}

func TestPublicTagValuesAreBoundedPrintableAndNotEmailShaped(t *testing.T) {
	require.True(t, IsPublicTagValue("019a9a3e-0f6b-7c3e-8f43-6c0b5a1e2d34"))
	require.True(t, IsPublicTagValue("POST /graphql"))
	require.True(t, IsPublicTagValue(strings.Repeat("é", MaxPublicTagValueLength)))
	require.False(t, IsPublicTagValue(strings.Repeat("x", MaxPublicTagValueLength+1)))
	require.False(t, IsPublicTagValue(""))
	require.False(t, IsPublicTagValue("line\nbreak"))
	require.False(t, IsPublicTagValue("someone@example.com"))
	require.False(t, IsPublicTagValue("contact: a.b@c.example"))
	require.False(t, IsPublicTagValue(string([]byte{0xff, 0xfe})))
}

func TestPublicEventTagsFilterAndBoundDeterministically(t *testing.T) {
	tags := map[string]string{
		"request_id": "req-1", "user.email": "private@example.com", "url": "https://x.test/?t=secret",
		"route": "/graphql", "note": "reach me at private@example.com", "oversized": strings.Repeat("x", 201),
	}
	public, truncated := PublicEventTags(tags)
	require.False(t, truncated)
	require.Equal(t, map[string]string{"request_id": "req-1", "route": "/graphql"}, public)

	public, truncated = PublicEventTags(nil)
	require.False(t, truncated)
	require.NotNil(t, public)
	require.Empty(t, public)

	many := map[string]string{}
	for i := 0; i < MaxPublicTags+10; i++ {
		many[fmt.Sprintf("tag_%03d", i)] = "v"
	}
	many["Bad-Key"] = "v"
	public, truncated = PublicEventTags(many)
	require.True(t, truncated)
	require.Len(t, public, MaxPublicTags)
	require.Contains(t, public, "tag_000")
	require.Contains(t, public, fmt.Sprintf("tag_%03d", MaxPublicTags-1))
	require.NotContains(t, public, fmt.Sprintf("tag_%03d", MaxPublicTags))
}
