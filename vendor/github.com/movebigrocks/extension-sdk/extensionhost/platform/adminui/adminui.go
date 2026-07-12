package adminui

import (
	"encoding/json"
	"html/template"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	platformdomain "github.com/movebigrocks/extension-sdk/extensionhost/platform/domain"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
)

type BasePageData struct {
	ActivePage          string
	PageTitle           string
	PageSubtitle        string
	UserName            string
	UserEmail           string
	UserRole            string
	CanManageUsers      bool
	IsWorkspaceScoped   bool
	ShowErrorTracking   bool
	ShowAnalytics       bool
	ExtensionNav        []AdminExtensionNavSection
	ExtensionWidgets    []AdminExtensionWidget
	CurrentWorkspaceID  string
	CurrentWorkspace    string
	CaseCount           int
	IssueCount          int
	RuleCount           int
	FormCount           int
	WorkspaceCount      int
	UserCount           int
	UnreadNotifications int
}

type ErrorPageData struct {
	Error string
}

type WorkspaceOption struct {
	ID   string
	Name string
}

type AdminExtensionNavSection struct {
	Title string
	Items []AdminExtensionNavItem
}

type AdminExtensionNavItem struct {
	Title      string
	Icon       string
	Href       string
	ActivePage string
}

type AdminExtensionWidget struct {
	Title       string
	Description string
	Icon        string
	Href        string
}

type ContextValues struct {
	UserID    string
	UserName  string
	UserEmail string
	Session   *runtimehttp.Session
}

func GetContextValues(c *gin.Context) *ContextValues {
	cv := &ContextValues{}
	if c == nil {
		return cv
	}
	if userID, ok := c.Get("user_id"); ok {
		if id, ok := userID.(string); ok {
			cv.UserID = id
		}
	}
	if userName, ok := c.Get("name"); ok {
		if name, ok := userName.(string); ok {
			cv.UserName = name
		}
	}
	if userEmail, ok := c.Get("email"); ok {
		if email, ok := userEmail.(string); ok {
			cv.UserEmail = email
		}
	}
	if sessionVal, ok := c.Get("session"); ok {
		if session, ok := sessionVal.(*runtimehttp.Session); ok {
			cv.Session = session
		}
	}
	return cv
}

func (cv *ContextValues) InstanceRole() platformdomain.InstanceRole {
	if cv == nil || cv.Session == nil {
		return ""
	}
	return platformdomain.CanonicalizeInstanceRole(platformdomain.InstanceRole(strings.TrimSpace(cv.Session.CurrentContext.Role)))
}

func (cv *ContextValues) CanManageUsers() bool {
	return cv.InstanceRole().IsAdmin()
}

func (cv *ContextValues) UserRole() string {
	if role := cv.InstanceRole(); role != "" {
		return string(role)
	}
	return ""
}

func (cv *ContextValues) IsWorkspaceContext() bool {
	return cv != nil && cv.Session != nil && cv.Session.CurrentContext.WorkspaceID != nil
}

func (cv *ContextValues) WorkspaceContext() (workspaceID, workspaceName, workspaceSlug string, ok bool) {
	if !cv.IsWorkspaceContext() || cv.Session.CurrentContext.WorkspaceID == nil {
		return "", "", "", false
	}
	workspaceID = strings.TrimSpace(*cv.Session.CurrentContext.WorkspaceID)
	if cv.Session.CurrentContext.WorkspaceName != nil {
		workspaceName = strings.TrimSpace(*cv.Session.CurrentContext.WorkspaceName)
	}
	if cv.Session.CurrentContext.WorkspaceSlug != nil {
		workspaceSlug = strings.TrimSpace(*cv.Session.CurrentContext.WorkspaceSlug)
	}
	return workspaceID, workspaceName, workspaceSlug, workspaceID != ""
}

func AdminTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"substr": func(s string, start, end int) string {
			if start < 0 {
				start = 0
			}
			if start >= len(s) {
				return ""
			}
			if end > len(s) {
				end = len(s)
			}
			if end < start {
				return ""
			}
			return s[start:end]
		},
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(b)
		},
		"formatDate": func(v interface{}, layout string) string {
			switch t := v.(type) {
			case time.Time:
				if t.IsZero() {
					return ""
				}
				return t.Format(layout)
			case *time.Time:
				if t == nil || t.IsZero() {
					return ""
				}
				return t.Format(layout)
			case string:
				if t == "" {
					return ""
				}
				parsed, err := time.Parse(time.RFC3339, t)
				if err != nil {
					parsed, err = time.Parse("2006-01-02T15:04:05", t)
					if err != nil {
						return t
					}
				}
				return parsed.Format(layout)
			default:
				return ""
			}
		},
	}
}
