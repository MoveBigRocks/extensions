package errortrackingui

import (
	"embed"
	"html/template"

	platformhandlers "github.com/movebigrocks/platform/pkg/extensionhost/platform/handlers"
)

//go:embed templates/*.html templates/partials/*.html
var Templates embed.FS

func ParseTemplates() (*template.Template, error) {
	return template.New("").Funcs(platformhandlers.AdminTemplateFuncMap()).ParseFS(Templates, "templates/partials/*.html", "templates/*.html")
}
