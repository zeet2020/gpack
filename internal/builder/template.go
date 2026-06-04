package builder

import (
	"bytes"
	_ "embed"
	"text/template"

	"gpack/internal/config"
)

//go:embed templates/main.go.tmpl
var mainGoTmpl string

// mainView is the data passed to main.go.tmpl.
type mainView struct {
	Cfg           *config.AppConfig
	InjectedJS    string
	InjectedCSS   string
	BgR, BgG, BgB int
	UseEvents     bool // imports the events package (hide-on-close hook)
}

// renderMainGo renders the generated project's main.go for the given target OS.
func renderMainGo(cfg *config.AppConfig, targetOS string) (string, error) {
	js, css := BuildInject(cfg, targetOS)

	r, g, b := 255, 255, 255
	if cfg.Window.DarkMode {
		r, g, b = 26, 26, 26
	}

	view := mainView{
		Cfg:         cfg,
		InjectedJS:  js,
		InjectedCSS: css,
		BgR:         r, BgG: g, BgB: b,
		UseEvents: cfg.Window.HideOnClose,
	}

	t, err := template.New("main").Parse(mainGoTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const indexHTML = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>%s</title>
<style>html,body{margin:0;padding:0;width:100%%;height:100%%;background:%s;}</style></head>
<body><!-- gpack loads the target URL via WebviewWindowOptions.URL --></body></html>
`
