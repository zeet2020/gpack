// Package config holds gpack's internal application configuration. An AppConfig
// is the single source of truth produced from either a Pake-compatible JSON file
// or CLI flags, and consumed by the template renderer and build runner.
package config

import (
	"fmt"
	"regexp"
	"strings"
)

// WindowConfig mirrors the per-window settings from a Pake config (windows[0]).
type WindowConfig struct {
	URL                  string
	URLType              string // "web" | "local"
	Title                string
	HideTitleBar         bool
	Fullscreen           bool
	Width                int
	Height               int
	MinWidth             int
	MinHeight            int
	Zoom                 int // percent, 50-200
	Resizable            bool
	AlwaysOnTop          bool
	DarkMode             bool
	ActivationShortcut   string
	DisabledWebShortcuts bool
	HideOnClose          bool
	Incognito            bool
	EnableDragDrop       bool
	Maximize             bool
	StartToTray          bool
	ForceInternalNav     bool
	InternalURLRegex     string
	EnableFind           bool
}

// UserAgent holds the per-platform User-Agent override strings.
type UserAgent struct {
	MacOS   string
	Linux   string
	Windows string
}

// SystemTray records, per platform, whether a system tray icon is shown.
type SystemTray struct {
	MacOS   bool
	Linux   bool
	Windows bool
}

// AppConfig is the resolved configuration after merging JSON, flags and defaults.
type AppConfig struct {
	// Identity
	Name     string
	SafeName string // URL-safe, no spaces; used for binary/module name
	Icon     string // resolved path or URL to icon PNG ("" => default)

	// Window (from windows[0])
	Window WindowConfig

	// Network & UA
	UserAgent UserAgent
	ProxyURL  string

	// Tray
	SystemTray     SystemTray
	SystemTrayPath string

	// Web behaviour
	Inject           []string // JS/CSS file paths to inject on load
	SafeDomain       string
	IgnoreCertErrors bool
	UseLocalFile     bool

	// Instance
	MultiInstance bool

	// Build meta
	AppVersion        string
	Targets           string
	MultiArch         bool
	IterativeBuild    bool
	KeepBinary        bool
	InstallerLanguage string
	Debug             bool
	OutDir            string
	Platform          string // "current" | "darwin" | "windows" | "linux"
	KeepTmp           bool
	InstallDeps       bool // attempt to install missing system webview deps (apt)
}

// ---- Template helpers (consumed by the v3 project templates) ----

// ZoomFloat converts the percent zoom into the float scale Wails v3 expects.
func (c *AppConfig) ZoomFloat() float64 {
	if c.Window.Zoom <= 0 {
		return 1.0
	}
	return float64(c.Window.Zoom) / 100.0
}

// AnyTray reports whether any platform wants a system tray.
func (c *AppConfig) AnyTray() bool {
	return c.SystemTray.MacOS || c.SystemTray.Linux || c.SystemTray.Windows
}

// UserAgentForBuild reports whether any platform UA override is set.
func (c *AppConfig) UserAgentForBuild() bool {
	return c.UserAgent.MacOS != "" || c.UserAgent.Linux != "" || c.UserAgent.Windows != ""
}

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// Sanitise derives SafeName from Name (and fills derived/default fields).
func (c *AppConfig) Sanitise() {
	safe := strings.TrimSpace(c.Name)
	safe = unsafeNameChars.ReplaceAllString(safe, "-")
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "gpack-app"
	}
	c.SafeName = safe
	if c.Window.Title == "" {
		c.Window.Title = c.Name
	}
}

// Summary renders a human-readable view of the resolved config (for dry runs).
func (c *AppConfig) Summary() string {
	var b strings.Builder
	w := c.Window
	fmt.Fprintf(&b, "Resolved gpack config\n")
	fmt.Fprintf(&b, "  Name           %s (safe: %s)\n", c.Name, c.SafeName)
	fmt.Fprintf(&b, "  URL            %s (%s)\n", w.URL, w.URLType)
	fmt.Fprintf(&b, "  Icon           %s\n", orDefault(c.Icon, "<auto/default>"))
	fmt.Fprintf(&b, "  Window         %dx%d  min %dx%d  zoom %d%%\n", w.Width, w.Height, w.MinWidth, w.MinHeight, w.Zoom)
	fmt.Fprintf(&b, "  Flags          frameless=%t fullscreen=%t maximize=%t resizable=%t alwaysOnTop=%t dark=%t\n",
		w.HideTitleBar, w.Fullscreen, w.Maximize, w.Resizable, w.AlwaysOnTop, w.DarkMode)
	fmt.Fprintf(&b, "  Behaviour      hideOnClose=%t startToTray=%t incognito=%t forceInternalNav=%t enableFind=%t\n",
		w.HideOnClose, w.StartToTray, w.Incognito, w.ForceInternalNav, w.EnableFind)
	fmt.Fprintf(&b, "  Tray           macos=%t linux=%t windows=%t  path=%s\n",
		c.SystemTray.MacOS, c.SystemTray.Linux, c.SystemTray.Windows, orDefault(c.SystemTrayPath, "<app icon>"))
	fmt.Fprintf(&b, "  UserAgent      set=%t\n", c.UserAgentForBuild())
	fmt.Fprintf(&b, "  Proxy          %s\n", orDefault(c.ProxyURL, "<none>"))
	fmt.Fprintf(&b, "  Inject         %d snippet(s)\n", len(c.Inject))
	fmt.Fprintf(&b, "  MultiInstance  %t\n", c.MultiInstance)
	fmt.Fprintf(&b, "  Build          version=%s targets=%s multiArch=%t debug=%t out=%s platform=%s\n",
		c.AppVersion, orDefault(c.Targets, "<platform default>"), c.MultiArch, c.Debug, c.OutDir, c.Platform)
	return b.String()
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
