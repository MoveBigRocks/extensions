package runtimeui

import (
	_ "embed"
	"html/template"
)

//go:embed dashboard.html
var page string

//go:embed app.js
var Script []byte

func Template() *template.Template { return template.Must(template.New("dashboard.html").Parse(page)) }
