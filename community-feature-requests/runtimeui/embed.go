package communityrequestsui

import (
	"embed"
	"html/template"
	"strings"
	"time"
	"unicode"
)

//go:embed templates/*.html
var Templates embed.FS

func ParseTemplates() (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"fmtTime": func(value time.Time) string {
			return value.Format("2 Jan 2006")
		},
		"statusLabel": func(value string) string {
			switch strings.TrimSpace(value) {
			case "in-progress":
				return "In Progress"
			default:
				return titleWords(strings.ReplaceAll(value, "-", " "))
			}
		},
	}).ParseFS(Templates, "templates/*.html")
}

func titleWords(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	for idx, part := range parts {
		runes := []rune(strings.ToLower(part))
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		parts[idx] = string(runes)
	}
	return strings.Join(parts, " ")
}
