package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig drops a config file (plus any sibling files) in a temp dir and
// returns the config's path, mimicking a user running gpack from elsewhere.
func writeConfig(t *testing.T, body string, siblings map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pake.json")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	for name, content := range siblings {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func loadApply(t *testing.T, body string, siblings map[string]string) *AppConfig {
	t.Helper()
	pj, err := LoadPakeJSON(writeConfig(t, body, siblings))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	if _, err := cfg.Apply(pj); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestApplyResizableDefault(t *testing.T) {
	// An absent "resizable" must not clobber the true default; an explicit
	// false must win.
	cases := []struct {
		name string
		json string
		want bool
	}{
		{"absent", `{"windows":[{"url":"https://example.com"}]}`, true},
		{"false", `{"windows":[{"url":"https://example.com","resizable":false}]}`, false},
		{"true", `{"windows":[{"url":"https://example.com","resizable":true}]}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := loadApply(t, tc.json, nil).Window.Resizable; got != tc.want {
				t.Errorf("Resizable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyLocalURLType(t *testing.T) {
	cfg := loadApply(t, `{"windows":[{"url":"site","url_type":"local"}]}`, nil)
	if !cfg.IsLocal() {
		t.Error("url_type local did not make the config local")
	}
	if !filepath.IsAbs(cfg.Window.URL) || filepath.Base(cfg.Window.URL) != "site" {
		t.Errorf("URL = %q, want an absolute path to the config-relative dir", cfg.Window.URL)
	}
}

func TestApplyInjectReadsFiles(t *testing.T) {
	cfg := loadApply(t,
		`{"windows":[{"url":"https://example.com"}],"inject":["theme.css","boot.js"]}`,
		map[string]string{"theme.css": "body{color:red}", "boot.js": "console.log(1)"})

	if len(cfg.Inject) != 2 {
		t.Fatalf("Inject = %#v, want 2 snippets", cfg.Inject)
	}
	// Contents, not paths — and .css tagged so BuildInject routes it to CSS.
	if !strings.HasPrefix(cfg.Inject[0], "/*css*/") || !strings.Contains(cfg.Inject[0], "color:red") {
		t.Errorf("css inject = %q, want tagged file contents", cfg.Inject[0])
	}
	if cfg.Inject[1] != "console.log(1)" {
		t.Errorf("js inject = %q, want file contents", cfg.Inject[1])
	}
}

func TestApplyMissingInjectFails(t *testing.T) {
	pj, err := LoadPakeJSON(writeConfig(t,
		`{"windows":[{"url":"https://example.com"}],"inject":["nope.js"]}`, nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Defaults().Apply(pj); err == nil {
		t.Error("Apply succeeded with a missing inject file, want error")
	}
}

func TestApplyResolvesTrayPath(t *testing.T) {
	cfg := loadApply(t,
		`{"windows":[{"url":"https://example.com"}],"system_tray_path":"icons/tray.png"}`, nil)
	if !filepath.IsAbs(cfg.SystemTrayPath) {
		t.Errorf("SystemTrayPath = %q, want resolved against the config dir", cfg.SystemTrayPath)
	}
}

func TestValidateHideOnCloseNeedsTray(t *testing.T) {
	cfg := Defaults()
	cfg.Window.URL = "https://example.com"
	cfg.Window.HideOnClose = true

	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(warnings, "hide_on_close") {
		t.Errorf("warnings = %v, want one about hide_on_close with no tray", warnings)
	}

	cfg.SystemTray.Linux = true
	warnings, err = cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if hasWarning(warnings, "hide_on_close") {
		t.Errorf("warnings = %v, want none once a tray is enabled", warnings)
	}
}

func hasWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
