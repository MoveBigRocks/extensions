package errortrackingui

import (
	"embed"
	"encoding/json"
	"html/template"
	"time"
)

//go:embed templates/*.html templates/partials/*.html
var Templates embed.FS

func ParseTemplates() (*template.Template, error) {
	return template.New("").Funcs(templateFunctions()).ParseFS(Templates, "templates/partials/*.html", "templates/*.html")
}

func templateFunctions() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"substr": func(value string, start, end int) string {
			if start < 0 {
				start = 0
			}
			if start >= len(value) {
				return ""
			}
			if end > len(value) {
				end = len(value)
			}
			if end < start {
				return ""
			}
			return value[start:end]
		},
		"json": func(value any) template.JS {
			encoded, err := json.Marshal(value)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(encoded)
		},
		"formatDate": func(value any, layout string) string {
			switch parsed := value.(type) {
			case time.Time:
				return parsed.Format(layout)
			case *time.Time:
				if parsed != nil {
					return parsed.Format(layout)
				}
			}
			return ""
		},
	}
}
