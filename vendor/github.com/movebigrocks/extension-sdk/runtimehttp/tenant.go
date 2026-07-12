package runtimehttp

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantScoper is the subset of an extension store the tenant middleware needs
// to confine a request's database access to its workspace under row-level
// security.
type TenantScoper interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	SetTenantContext(ctx context.Context, workspaceID string) error
}

// AdminScoper is the subset of an extension store the admin middleware needs to
// run a request across workspaces under row-level security.
type AdminScoper interface {
	WithAdminContext(ctx context.Context, fn func(ctx context.Context) error) error
}

// TenantContext wraps each workspace-scoped request in a database transaction
// that sets the workspace context, so a runtime that reads or writes core tables
// is confined to the request's workspace under enforced row-level security. Use
// it for extensions whose requests each belong to a single workspace. Requests
// with no workspace (health checks, public assets) pass through unwrapped.
// ForwardedContextMiddleware, applied by DefaultEngine, sets workspace_id, so
// register this after it.
func TenantContext(scoper TenantScoper) gin.HandlerFunc {
	return func(c *gin.Context) {
		workspaceID := c.GetString("workspace_id")
		if scoper == nil || workspaceID == "" {
			c.Next()
			return
		}
		runScoped(c, func(txCtx context.Context) error {
			return scoper.SetTenantContext(txCtx, workspaceID)
		}, scoper.WithTransaction)
	}
}

// AdminContext wraps each request in a transaction that switches to the
// cross-workspace admin role, for extensions that legitimately operate across
// workspaces (for example an instance-admin observability dashboard). It runs
// every request through the admin role rather than confining it to one
// workspace.
func AdminContext(scoper AdminScoper) gin.HandlerFunc {
	return func(c *gin.Context) {
		if scoper == nil {
			c.Next()
			return
		}
		runScoped(c, nil, scoper.WithAdminContext)
	}
}

// runScoped runs the request inside the given transaction wrapper, optionally
// applying setup (setting the workspace context) first, and propagates the
// transaction to downstream handlers via the request context.
func runScoped(
	c *gin.Context,
	setup func(ctx context.Context) error,
	wrap func(ctx context.Context, fn func(ctx context.Context) error) error,
) {
	err := wrap(c.Request.Context(), func(txCtx context.Context) error {
		if setup != nil {
			if err := setup(txCtx); err != nil {
				return err
			}
		}
		c.Request = c.Request.WithContext(txCtx)
		c.Next()
		if len(c.Errors) > 0 {
			return c.Errors.Last()
		}
		return nil
	})
	if err != nil && !c.IsAborted() {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "request scope failed"})
	}
}
