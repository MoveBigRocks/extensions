package atsruntime

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/runtimehost"
)

// coreHost is the slice of the platform host API the ATS runtime depends on.
// The live implementation is *runtimehost.Client, built per request from the
// host token and base URL the platform forwards; tests supply a fake. Keeping
// it an interface is what lets the ATS runtime speak the language-neutral host
// API instead of importing platform internals.
type coreHost interface {
	GetQueue(ctx context.Context, queueID string) (*runtimehost.HostQueue, bool, error)
	GetQueueBySlug(ctx context.Context, slug string) (*runtimehost.HostQueue, bool, error)
	CreateQueue(ctx context.Context, input runtimehost.CreateQueueInput) (*runtimehost.HostQueue, error)
	GetCase(ctx context.Context, caseID string) (*runtimehost.HostCase, bool, error)
	UpdateCase(ctx context.Context, caseID string, patch runtimehost.CaseUpdateInput) (*runtimehost.HostCase, error)
	HandoffCase(ctx context.Context, caseID string, input runtimehost.HandoffCaseInput) error
	UploadAttachment(ctx context.Context, input runtimehost.UploadAttachmentInput) (*runtimehost.HostAttachment, error)
	GetAttachment(ctx context.Context, attachmentID string) (*runtimehost.HostAttachment, bool, error)
	LinkAttachmentsToCase(ctx context.Context, caseID string, attachmentIDs []string) error
	PublishArtifact(ctx context.Context, input runtimehost.PublishArtifactInput) error
	IngestApplication(ctx context.Context, input runtimehost.IngestApplicationInput) (*runtimehost.IngestApplicationResult, error)
	ApplyCaseChange(ctx context.Context, caseID string, input runtimehost.ApplyCaseChangeInput) (*runtimehost.HostCase, error)
}

// *runtimehost.Client is the production coreHost.
var _ coreHost = (*runtimehost.Client)(nil)

// hostProvider yields the core host bound to a request's context. Because the
// host token is per request, the runtime resolves the host client from the
// context rather than holding a long-lived one.
type hostProvider func(ctx context.Context) (coreHost, error)

type hostClientContextKey struct{}

// withHostClient stores a request-scoped host client on the context.
func withHostClient(ctx context.Context, client *runtimehost.Client) context.Context {
	return context.WithValue(ctx, hostClientContextKey{}, client)
}

// hostFromContext returns the host client the middleware placed on the context.
func hostFromContext(ctx context.Context) (coreHost, error) {
	client, ok := ctx.Value(hostClientContextKey{}).(*runtimehost.Client)
	if !ok || client == nil {
		return nil, fmt.Errorf("host API is not available on this request")
	}
	return client, nil
}

// HostClientMiddleware builds a host client from the token and base URL the
// platform forwards on each proxied request and puts it on the request context,
// so ATS handlers can call the platform host API. Requests with no host context
// (health checks, asset fetches) pass through unwrapped and only fail if they
// try to reach the host API.
func HostClientMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if client, err := runtimehost.NewClientFromRequest(c.Request); err == nil {
			c.Request = c.Request.WithContext(withHostClient(c.Request.Context(), client))
		}
		c.Next()
	}
}
