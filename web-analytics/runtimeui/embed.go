package webanalyticsui

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html templates/partials/*.html
var Templates embed.FS

//go:embed assets/analytics.js
var Assets embed.FS

func ParseTemplates() (*template.Template, error) {
	return template.ParseFS(Templates, "templates/partials/*.html", "templates/*.html")
}
