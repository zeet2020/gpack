package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"gpack/internal/config"
)

// registerFlags wires the full Pake-compatible flag set plus gpack extras.
func registerFlags(cmd *cobra.Command) {
	f := cmd.Flags()

	// Identity / icons
	f.String("name", "", "Application name (default: derived from URL hostname)")
	f.String("icon", "", "Icon path or URL (default: auto-fetched from site)")
	f.String("system-tray-icon", "", "Tray icon path (requires --show-system-tray)")

	// Window
	f.Int("width", 1200, "Window width in pixels")
	f.Int("height", 780, "Window height in pixels")
	f.Int("min-width", 0, "Minimum window width (0 = no limit)")
	f.Int("min-height", 0, "Minimum window height (0 = no limit)")
	f.Int("zoom", 100, "Initial page zoom 50-200")
	f.Bool("fullscreen", false, "Launch in fullscreen")
	f.Bool("maximize", false, "Launch maximized")
	f.Bool("resizable", true, "Allow window resizing")
	f.Bool("hide-title-bar", false, "Hide native title bar (frameless)")
	f.Bool("always-on-top", false, "Keep window above all others")
	f.Bool("dark-mode", false, "Force dark mode appearance (macOS only)")
	f.String("title", "", "Window title bar text")

	// Web behaviour
	f.String("user-agent", "", "Custom User-Agent (JS-level override, not the HTTP header; unset by default)")
	f.Bool("disabled-web-shortcuts", false, "Disable built-in keyboard shortcuts")
	f.String("activation-shortcut", "", "Global hotkey to bring window to front")
	f.Bool("incognito", false, "Ephemeral session (no persistent cookies/storage)")
	f.Bool("enable-drag-drop", false, "Enable native drag-and-drop")
	f.Bool("force-internal-navigation", false, "Keep all links inside the app window")
	f.Bool("enable-find", false, "Enable in-page find bar (Cmd/Ctrl+F)")
	f.String("safe-domain", "", "Domain regex that opens internally (requires --force-internal-navigation)")
	f.Bool("ignore-certificate-errors", false, "Ignore TLS certificate errors (unsupported — will warn)")

	// System integration
	f.Bool("show-system-tray", false, "Show system tray icon")
	f.Bool("hide-on-close", false, "Hide window on close instead of quitting")
	f.Bool("start-to-tray", false, "Launch hidden in tray (requires --show-system-tray)")
	f.Bool("multi-instance", false, "Allow multiple simultaneous instances")

	// Advanced
	f.StringArray("inject", []string{}, "JS/CSS file(s) to inject at startup")
	f.String("proxy-url", "", "Proxy URL (http://, https://, or socks5://)")
	f.Bool("use-local-file", false, "Treat URL as a local directory path; embed files")

	// Build
	f.String("app-version", "1.0.0", "Application version string")
	f.Bool("debug", false, "Enable DevTools and verbose build output")
	f.String("out", ".", "Output directory for built artifact")
	f.String("platform", "current", "Cross-compile target: current|darwin|windows|linux")

	// gpack-specific
	f.String("config", "", "Path to Pake-compatible JSON config file")
	f.Bool("keep-tmp", false, "Keep generated Wails project directory after build")
	f.Bool("install-deps", false, "Attempt to install missing system webview deps (Linux, apt+sudo)")
	f.Bool("dry-run", false, "Resolve and print the config without building")

	// Accepted for Pake CLI compatibility, but not implemented: registering them
	// keeps existing command lines working, and using one warns.
	for _, u := range unimplemented {
		if u.isBool {
			f.Bool(u.name, false, "(not implemented) "+u.why)
		} else {
			f.String(u.name, "", "(not implemented) "+u.why)
		}
	}
}

// unimplemented lists Pake flags gpack parses but ignores. Slice, not map, so the
// warnings come out in a stable order.
var unimplemented = []struct {
	name   string
	why    string
	isBool bool
}{
	{"wasm", "system webviews always enable WASM", true},
	{"targets", "gpack emits a raw binary, not an installer", false},
	{"multi-arch", "universal binaries are not wired up", true},
	{"iterative-build", "every build is a fresh one", true},
	{"keep-binary", "the raw binary is always the output", true},
	{"installer-language", "no installer is produced", false},
}

// unimplementedWarnings reports which no-op compatibility flags the user asked
// for. Only a flag that requests something warrants a warning: --wasm=false and
// --targets="" ask for nothing, so they get no complaint.
func unimplementedWarnings(f *pflag.FlagSet) []string {
	var out []string
	for _, u := range unimplemented {
		if !f.Changed(u.name) {
			continue
		}
		if u.isBool {
			if on, _ := f.GetBool(u.name); !on {
				continue
			}
		} else if v, _ := f.GetString(u.name); strings.TrimSpace(v) == "" {
			continue
		}
		out = append(out, fmt.Sprintf("--%s is not implemented; %s", u.name, u.why))
	}
	return out
}

// applyFlags overrides config fields for flags the user explicitly set.
func applyFlags(cmd *cobra.Command, cfg *config.AppConfig) error {
	f := cmd.Flags()
	changed := func(name string) bool { return f.Changed(name) }

	for _, w := range unimplementedWarnings(f) {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}

	if changed("name") {
		cfg.Name, _ = f.GetString("name")
	}
	if changed("icon") {
		cfg.Icon, _ = f.GetString("icon")
	}
	if changed("system-tray-icon") {
		cfg.SystemTrayPath, _ = f.GetString("system-tray-icon")
	}

	if changed("width") {
		cfg.Window.Width, _ = f.GetInt("width")
	}
	if changed("height") {
		cfg.Window.Height, _ = f.GetInt("height")
	}
	if changed("min-width") {
		cfg.Window.MinWidth, _ = f.GetInt("min-width")
	}
	if changed("min-height") {
		cfg.Window.MinHeight, _ = f.GetInt("min-height")
	}
	if changed("zoom") {
		cfg.Window.Zoom, _ = f.GetInt("zoom")
	}
	if changed("fullscreen") {
		cfg.Window.Fullscreen, _ = f.GetBool("fullscreen")
	}
	if changed("maximize") {
		cfg.Window.Maximize, _ = f.GetBool("maximize")
	}
	if changed("resizable") {
		cfg.Window.Resizable, _ = f.GetBool("resizable")
	}
	if changed("hide-title-bar") {
		cfg.Window.HideTitleBar, _ = f.GetBool("hide-title-bar")
	}
	if changed("always-on-top") {
		cfg.Window.AlwaysOnTop, _ = f.GetBool("always-on-top")
	}
	if changed("dark-mode") {
		cfg.Window.DarkMode, _ = f.GetBool("dark-mode")
	}
	if changed("title") {
		cfg.Window.Title, _ = f.GetString("title")
	}

	if changed("user-agent") {
		ua, _ := f.GetString("user-agent")
		cfg.UserAgent = config.UserAgent{MacOS: ua, Linux: ua, Windows: ua}
	}
	if changed("disabled-web-shortcuts") {
		cfg.Window.DisabledWebShortcuts, _ = f.GetBool("disabled-web-shortcuts")
	}
	if changed("activation-shortcut") {
		cfg.Window.ActivationShortcut, _ = f.GetString("activation-shortcut")
	}
	if changed("incognito") {
		cfg.Window.Incognito, _ = f.GetBool("incognito")
	}
	if changed("enable-drag-drop") {
		cfg.Window.EnableDragDrop, _ = f.GetBool("enable-drag-drop")
	}
	if changed("force-internal-navigation") {
		cfg.Window.ForceInternalNav, _ = f.GetBool("force-internal-navigation")
	}
	if changed("enable-find") {
		cfg.Window.EnableFind, _ = f.GetBool("enable-find")
	}
	if changed("safe-domain") {
		cfg.SafeDomain, _ = f.GetString("safe-domain")
	}
	if changed("ignore-certificate-errors") {
		cfg.IgnoreCertErrors, _ = f.GetBool("ignore-certificate-errors")
	}

	if changed("show-system-tray") {
		if on, _ := f.GetBool("show-system-tray"); on {
			cfg.SystemTray = config.SystemTray{MacOS: true, Linux: true, Windows: true}
		}
	}
	if changed("hide-on-close") {
		cfg.Window.HideOnClose, _ = f.GetBool("hide-on-close")
	}
	if changed("start-to-tray") {
		cfg.Window.StartToTray, _ = f.GetBool("start-to-tray")
	}
	if changed("multi-instance") {
		cfg.MultiInstance, _ = f.GetBool("multi-instance")
	}

	if changed("inject") {
		paths, _ := f.GetStringArray("inject")
		for _, p := range paths {
			body, err := config.ReadInject(p)
			if err != nil {
				return fmt.Errorf("--inject %s: %w", p, err)
			}
			cfg.Inject = append(cfg.Inject, body)
		}
	}
	if changed("proxy-url") {
		cfg.ProxyURL, _ = f.GetString("proxy-url")
	}
	if changed("use-local-file") {
		// Writes the same field the config and positional argument write, so
		// "local" and "web" can never disagree with the URL they describe.
		if local, _ := f.GetBool("use-local-file"); local {
			cfg.Window.URLType = "local"
		} else {
			cfg.Window.URLType = "web"
		}
	}

	if changed("app-version") {
		cfg.AppVersion, _ = f.GetString("app-version")
	}
	if changed("debug") {
		cfg.Debug, _ = f.GetBool("debug")
	}
	if changed("out") {
		cfg.OutDir, _ = f.GetString("out")
	}
	if changed("platform") {
		cfg.Platform, _ = f.GetString("platform")
	}
	if changed("keep-tmp") {
		cfg.KeepTmp, _ = f.GetBool("keep-tmp")
	}
	if changed("install-deps") {
		cfg.InstallDeps, _ = f.GetBool("install-deps")
	}
	return nil
}
