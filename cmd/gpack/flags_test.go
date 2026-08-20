package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pake command lines must keep parsing (no "unknown flag") while saying plainly
// that the flag does nothing.
func TestUnimplementedFlagsParseAndWarn(t *testing.T) {
	cmd := newRootCmd()
	args := []string{"https://example.com", "--targets", "deb", "--multi-arch", "--wasm"}
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatalf("Pake flags no longer parse: %v", err)
	}

	warnings := unimplementedWarnings(cmd.Flags())
	if len(warnings) != 3 {
		t.Fatalf("warnings = %v, want one per flag set", warnings)
	}
	for _, name := range []string{"--targets", "--multi-arch", "--wasm"} {
		if !strings.Contains(strings.Join(warnings, "\n"), name) {
			t.Errorf("no warning mentions %s: %v", name, warnings)
		}
	}
}

func TestUnimplementedFlagsSilentWhenUnset(t *testing.T) {
	cmd := newRootCmd()
	if err := cmd.Flags().Parse([]string{"https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if w := unimplementedWarnings(cmd.Flags()); len(w) != 0 {
		t.Errorf("warnings = %v, want none", w)
	}
}

// --wasm=false asks for nothing, so it has nothing to warn about.
func TestUnimplementedBoolFlagsWarnOnlyWhenOn(t *testing.T) {
	cmd := newRootCmd()
	if err := cmd.Flags().Parse([]string{"--wasm=false", "--multi-arch=false", "--targets="}); err != nil {
		t.Fatal(err)
	}
	if w := unimplementedWarnings(cmd.Flags()); len(w) != 0 {
		t.Errorf("warnings = %v, want none for flags explicitly turned off", w)
	}
}

// resolveConfig owns the JSON / positional-arg / flag precedence. Local-ness is
// one field, so the URL and its type can never end up describing different things.
func TestResolveConfigLocalness(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	if err := os.MkdirAll(site, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "pake.json")
	if err := os.WriteFile(cfgPath, []byte(`{"windows":[{"url":"site","url_type":"local"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		args      []string
		wantLocal bool
		wantURL   string
	}{
		{"config alone stays local", []string{"--config", cfgPath}, true, site},
		// A positional URL replaces the config's url, so it replaces its url_type too.
		{"positional url overrides a local config", []string{"--config", cfgPath, "https://example.com"}, false, "https://example.com"},
		// ...unless the flag marks the new url local as well.
		{"positional plus flag stays local", []string{"--config", cfgPath, site, "--use-local-file"}, true, site},
		// An explicit =false must not leave a local url typed as remote.
		{"explicit false turns it remote", []string{"--config", cfgPath, "--use-local-file=false"}, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd()
			if err := cmd.Flags().Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			cfg, err := resolveConfig(cmd, cmd.Flags().Args())
			if err != nil {
				t.Fatal(err)
			}
			if cfg.IsLocal() != tc.wantLocal {
				t.Errorf("IsLocal() = %v, want %v", cfg.IsLocal(), tc.wantLocal)
			}
			if tc.wantURL != "" && cfg.Window.URL != tc.wantURL {
				t.Errorf("URL = %q, want %q", cfg.Window.URL, tc.wantURL)
			}
			// A remote build hands Window.URL to the webview, so it must be a real
			// URL by then — never a bare filesystem path.
			if !cfg.IsLocal() && !strings.HasPrefix(cfg.Window.URL, "http") {
				t.Errorf("non-local URL = %q, want an http(s) URL", cfg.Window.URL)
			}
		})
	}
}

func TestDeriveName(t *testing.T) {
	cases := map[string]string{
		"https://weekly.tw93.fun/en": "Weekly",
		"https://www.example.com":    "Example",
		"http://localhost:3000":      "Localhost",
		"/home/me/projects/mysite":   "Mysite",
		"./site/index.html":          "Index",
		"":                           "App",
	}
	for in, want := range cases {
		if got := deriveName(in); got != want {
			t.Errorf("deriveName(%q) = %q, want %q", in, got, want)
		}
	}
}
