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
	"github.com/movebigrocks/extension-sdk/runtimeproto"
)

const envSocketPath = "MBR_EXTENSION_RUNTIME_SOCKET_PATH"

type SessionContext struct {
	Type          string  `json:"type,omitempty"`
	WorkspaceID   *string `json:"workspace_id,omitempty"`
	WorkspaceName *string `json:"workspace_name,omitempty"`
	WorkspaceSlug *string `json:"workspace_slug,omitempty"`
	Role          string  `json:"role,omitempty"`
}

type Session struct {
	CurrentContext SessionContext `json:"current_context"`
}

type ExtensionConfig map[string]any

func DefaultEngine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(ForwardedContextMiddleware())
	return engine
}

func ListenAndServeUnixSocket(handler http.Handler, packageKey string) error {
	socketPath := strings.TrimSpace(os.Getenv(envSocketPath))
	if socketPath == "" {
		socketPath = runtimeproto.SocketPath("", packageKey)
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
	engine.POST(runtimeproto.InternalConsumerPathPrefix+"*target", func(c *gin.Context) {
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
	engine.POST(runtimeproto.InternalJobPathPrefix+"*target", func(c *gin.Context) {
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
		"CanManageUsers":     canManageUsers(userRole),
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

func ExtensionID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString("extension_id"))
}

func ExtensionSlug(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString("extension_slug"))
}

func ExtensionPackageKey(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString("extension_package_key"))
}

func ExtensionConfigMap(c *gin.Context) ExtensionConfig {
	if c == nil {
		return nil
	}
	value, ok := c.Get("extension_config")
	if !ok {
		return nil
	}
	config, ok := value.(ExtensionConfig)
	if !ok {
		return nil
	}
	return config
}

func ExtensionConfigString(c *gin.Context, key string) (string, bool) {
	config := ExtensionConfigMap(c)
	if len(config) == 0 {
		return "", false
	}
	value, ok := config[strings.TrimSpace(key)]
	if !ok {
		return "", false
	}
	parsed, ok := value.(string)
	if !ok {
		return "", false
	}
	parsed = strings.TrimSpace(parsed)
	if parsed == "" {
		return "", false
	}
	return parsed, true
}

func PublicBaseURL(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString("public_base_url"))
}

func AdminBaseURL(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString("admin_base_url"))
}

func APIBaseURL(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString("api_base_url"))
}

func ForwardedContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if extensionID := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderExtensionID)); extensionID != "" {
			c.Set("extension_id", extensionID)
		}
		if extensionSlug := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderExtensionSlug)); extensionSlug != "" {
			c.Set("extension_slug", extensionSlug)
		}
		if packageKey := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderExtensionPackageKey)); packageKey != "" {
			c.Set("extension_package_key", packageKey)
		}
		if publicBaseURL := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderPublicBaseURL)); publicBaseURL != "" {
			c.Set("public_base_url", publicBaseURL)
		}
		if adminBaseURL := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderAdminBaseURL)); adminBaseURL != "" {
			c.Set("admin_base_url", adminBaseURL)
		}
		if apiBaseURL := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderAPIBaseURL)); apiBaseURL != "" {
			c.Set("api_base_url", apiBaseURL)
		}
		if userID := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderUserID)); userID != "" {
			c.Set("user_id", userID)
		}
		if workspaceID := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderWorkspaceID)); workspaceID != "" {
			c.Set("workspace_id", workspaceID)
		}
		if userName := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderUserName)); userName != "" {
			c.Set("name", userName)
		}
		if userEmail := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderUserEmail)); userEmail != "" {
			c.Set("email", userEmail)
		}
		if raw := strings.TrimSpace(c.GetHeader(runtimeproto.HeaderSessionContextJSON)); raw != "" {
			var current SessionContext
			if err := json.Unmarshal([]byte(raw), &current); err == nil {
				c.Set("session", &Session{CurrentContext: current})
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
		decodeExtensionConfigHeader(c, runtimeproto.HeaderExtensionConfigJSON, "extension_config")
		decodeJSONHeader(c, runtimeproto.HeaderAdminExtensionNavJSON, "admin_extension_nav")
		decodeJSONHeader(c, runtimeproto.HeaderAdminWidgetsJSON, "admin_extension_widgets")
		decodeBoolHeader(c, runtimeproto.HeaderShowAnalytics, "admin_feature_analytics")
		decodeBoolHeader(c, runtimeproto.HeaderShowErrorTracking, "admin_feature_error_tracking")
		c.Next()
	}
}

func decodeExtensionConfigHeader(c *gin.Context, headerName, key string) {
	raw := strings.TrimSpace(c.GetHeader(headerName))
	if raw == "" {
		return
	}
	value := make(ExtensionConfig)
	if err := json.Unmarshal([]byte(raw), &value); err == nil {
		c.Set(key, value)
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

func canManageUsers(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "super_admin":
		return true
	default:
		return false
	}
}
