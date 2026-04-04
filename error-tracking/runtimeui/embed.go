package errortrackingui

import (
	"embed"
	"html/template"

	platformadmin "github.com/movebigrocks/extension-sdk/extensionhost/platform/adminui"
)

//go:embed templates/*.html templates/partials/*.html
var Templates embed.FS

func ParseTemplates() (*template.Template, error) {
	return template.New("").Funcs(platformadmin.AdminTemplateFuncMap()).ParseFS(Templates, "templates/partials/*.html", "templates/*.html")
}
