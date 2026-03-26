package runtimehttp

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	platformdomain "github.com/movebigrocks/platform/internal/platform/domain"
	publicruntime "github.com/movebigrocks/platform/pkg/extensionsruntime"
)

const envSocketPath = "MBR_EXTENSION_RUNTIME_SOCKET_PATH"

func DefaultEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(forwardedContextMiddleware())
	return engine
}

func ListenAndServeUnixSocket(handler http.Handler, packageKey string) error {
	socketPath := strings.TrimSpace(os.Getenv(envSocketPath))
	if socketPath == "" {
		socketPath = publicruntime.SocketPath("", packageKey)
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return err
	}
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()
	if err := os.Chmod(socketPath, 0o666); err != nil {
		return err
	}
	server := &http.Server{Handler: handler}
	return server.Serve(listener)
}

func RegisterInternalRoutes(
	engine *gin.Engine,
	eventConsumers map[string]func(context.Context, []byte) error,
	jobs map[string]func(context.Context) error,
) {
	if engine == nil {
		return
	}
	engine.POST(publicruntime.InternalConsumerPathPrefix+"*target", func(c *gin.Context) {
		target := strings.TrimPrefix(c.Param("target"), "/")
		handler, ok := eventConsumers[target]
		if !ok || handler == nil {
			c.String(http.StatusNotFound, "unknown consumer")
			return
		}
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "read consumer payload: %v", err)
			return
		}
		if err := handler(c.Request.Context(), payload); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	})
	engine.POST(publicruntime.InternalJobPathPrefix+"*target", func(c *gin.Context) {
		target := strings.TrimPrefix(c.Param("target"), "/")
		handler, ok := jobs[target]
		if !ok || handler == nil {
			c.String(http.StatusNotFound, "unknown job")
			return
		}
		if err := handler(c.Request.Context()); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func BuildBasePageData(c *gin.Context, activePage, title, subtitle string) gin.H {
	userName := c.GetString("name")
	userEmail := c.GetString("email")
	userRole := c.GetString("user_role")
	workspaceID := c.GetString("workspace_id")
	workspaceName := c.GetString("workspace_name")
	data := gin.H{
		"ActivePage":         activePage,
		"PageTitle":          title,
		"PageSubtitle":       subtitle,
		"UserName":           userName,
		"UserEmail":          userEmail,
		"UserRole":           userRole,
		"CanManageUsers":     strings.EqualFold(userRole, string(platformdomain.InstanceRoleAdmin)),
		"IsWorkspaceScoped":  workspaceID != "",
		"CurrentWorkspaceID": workspaceID,
		"CurrentWorkspace":   workspaceName,
	}
	if nav, ok := c.Get("admin_extension_nav"); ok {
		data["ExtensionNav"] = nav
	}
	if widgets, ok := c.Get("admin_extension_widgets"); ok {
		data["ExtensionWidgets"] = widgets
	}
	if show, ok := c.Get("admin_feature_error_tracking"); ok {
		data["ShowErrorTracking"] = show
	}
	if show, ok := c.Get("admin_feature_analytics"); ok {
		data["ShowAnalytics"] = show
	}
	return data
}

func forwardedContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if userID := strings.TrimSpace(c.GetHeader(publicruntime.HeaderUserID)); userID != "" {
			c.Set("user_id", userID)
		}
		if workspaceID := strings.TrimSpace(c.GetHeader(publicruntime.HeaderWorkspaceID)); workspaceID != "" {
			c.Set("workspace_id", workspaceID)
		}
		if userName := strings.TrimSpace(c.GetHeader(publicruntime.HeaderUserName)); userName != "" {
			c.Set("name", userName)
		}
		if userEmail := strings.TrimSpace(c.GetHeader(publicruntime.HeaderUserEmail)); userEmail != "" {
			c.Set("email", userEmail)
		}
		if raw := strings.TrimSpace(c.GetHeader(publicruntime.HeaderSessionContextJSON)); raw != "" {
			var current platformdomain.Context
			if err := json.Unmarshal([]byte(raw), &current); err == nil {
				session := &platformdomain.Session{CurrentContext: current}
				c.Set("session", session)
				c.Set("user_role", strings.TrimSpace(current.Role))
				if current.WorkspaceName != nil && strings.TrimSpace(*current.WorkspaceName) != "" {
					c.Set("workspace_name", strings.TrimSpace(*current.WorkspaceName))
				}
				if current.WorkspaceSlug != nil && strings.TrimSpace(*current.WorkspaceSlug) != "" {
					c.Set("workspace_slug", strings.TrimSpace(*current.WorkspaceSlug))
				}
				if current.WorkspaceID != nil && strings.TrimSpace(*current.WorkspaceID) != "" {
					c.Set("workspace_id", strings.TrimSpace(*current.WorkspaceID))
				}
			}
		}
		decodeJSONHeader(c, publicruntime.HeaderAdminExtensionNavJSON, "admin_extension_nav")
		decodeJSONHeader(c, publicruntime.HeaderAdminWidgetsJSON, "admin_extension_widgets")
		decodeBoolHeader(c, publicruntime.HeaderShowAnalytics, "admin_feature_analytics")
		decodeBoolHeader(c, publicruntime.HeaderShowErrorTracking, "admin_feature_error_tracking")
		c.Next()
	}
}

func decodeJSONHeader(c *gin.Context, headerName, key string) {
	raw := strings.TrimSpace(c.GetHeader(headerName))
	if raw == "" {
		return
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err == nil {
		c.Set(key, value)
	}
}

func decodeBoolHeader(c *gin.Context, headerName, key string) {
	raw := strings.TrimSpace(c.GetHeader(headerName))
	if raw == "" {
		return
	}
	if parsed, err := strconv.ParseBool(raw); err == nil {
		c.Set(key, parsed)
	}
}
