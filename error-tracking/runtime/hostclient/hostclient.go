package hostclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehost"
)

type Host interface {
	CreateCase(context.Context, runtimehost.CreateCaseInput) (*runtimehost.HostCase, error)
	GetCaseInWorkspace(context.Context, string, string) (*runtimehost.HostCase, bool, error)
	UpdateCase(context.Context, string, runtimehost.CaseUpdateInput) (*runtimehost.HostCase, error)
	MarkCaseResolvedInWorkspace(context.Context, string, string, time.Time) error
	LinkIssueToCase(context.Context, string, string, string, string) error
	UnlinkIssueFromCase(context.Context, string, string, string) error
	GetCaseByIssueAndContact(context.Context, string, string, string) (*runtimehost.HostCase, bool, error)
	ListWorkspaces(context.Context) ([]runtimehost.HostWorkspace, error)
	GetWorkspacesByIDs(context.Context, []string) ([]runtimehost.HostWorkspace, error)
	PublishEvent(context.Context, runtimehost.PublishEventInput) error
}

type Provider func(context.Context) (Host, error)

type contextKey struct{}

func WithClient(ctx context.Context, client Host) context.Context {
	return context.WithValue(ctx, contextKey{}, client)
}

func FromContext(ctx context.Context) (Host, error) {
	client, ok := ctx.Value(contextKey{}).(Host)
	if !ok || client == nil {
		return nil, fmt.Errorf("host API is not available on this request")
	}
	return client, nil
}

// DetachedContext keeps only the request-scoped host client on a fresh
// background context. It is safe to retain after an HTTP request completes and
// deliberately drops request transactions and cancellation.
func DetachedContext(ctx context.Context) context.Context {
	client, err := FromContext(ctx)
	if err != nil {
		return context.Background()
	}
	return WithClient(context.Background(), client)
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if client, err := runtimehost.NewClientFromRequest(c.Request); err == nil {
			c.Request = c.Request.WithContext(WithClient(c.Request.Context(), client))
		}
		c.Next()
	}
}

type Publisher struct{ Provider Provider }

func (p Publisher) Publish(ctx context.Context, workspaceID, eventType string, payload any) error {
	client, err := p.Provider(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode event payload: %w", err)
	}
	data := map[string]any{}
	if err := json.Unmarshal(encoded, &data); err != nil {
		return fmt.Errorf("normalize event payload: %w", err)
	}
	return client.PublishEvent(ctx, runtimehost.PublishEventInput{WorkspaceID: workspaceID, EventType: eventType, Data: data})
}
