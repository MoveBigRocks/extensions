package salespipelineui

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var Templates embed.FS

func ParseTemplates() (*template.Template, error) {
	return template.ParseFS(Templates, "templates/*.html")
}
