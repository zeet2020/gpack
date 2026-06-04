package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// PakeJSON is the on-disk Pake configuration format. gpack accepts these files
// directly so existing Pake users can reuse their configs.
type PakeJSON struct {
	Windows   []PakeWindow `json:"windows"`
	UserAgent struct {
		MacOS   string `json:"macos"`
		Linux   string `json:"linux"`
		Windows string `json:"windows"`
	} `json:"user_agent"`
	SystemTray struct {
		MacOS   bool `json:"macos"`
		Linux   bool `json:"linux"`
		Windows bool `json:"windows"`
	} `json:"system_tray"`
	SystemTrayPath string   `json:"system_tray_path"`
	Inject         []string `json:"inject"`
	ProxyURL       string   `json:"proxy_url"`
	MultiInstance  bool     `json:"multi_instance"`
	MultiWindow    bool     `json:"multi_window"`
}

// PakeWindow mirrors a single entry of the Pake "windows" array.
type PakeWindow struct {
	URL                  string `json:"url"`
	URLType              string `json:"url_type"`
	HideTitleBar         bool   `json:"hide_title_bar"`
	Fullscreen           bool   `json:"fullscreen"`
	Width                int    `json:"width"`
	Height               int    `json:"height"`
	Resizable            bool   `json:"resizable"`
	AlwaysOnTop          bool   `json:"always_on_top"`
	DarkMode             bool   `json:"dark_mode"`
	ActivationShortcut   string `json:"activation_shortcut"`
	DisabledWebShortcuts bool   `json:"disabled_web_shortcuts"`
	HideOnClose          bool   `json:"hide_on_close"`
	Incognito            bool   `json:"incognito"`
	EnableWasm           bool   `json:"enable_wasm"`
	EnableDragDrop       bool   `json:"enable_drag_drop"`
	Maximize             bool   `json:"maximize"`
	StartToTray          bool   `json:"start_to_tray"`
	ForceInternalNav     bool   `json:"force_internal_navigation"`
	InternalURLRegex     string `json:"internal_url_regex"`
	EnableFind           bool   `json:"enable_find"`
	NewWindow            bool   `json:"new_window"`
}

// LoadPakeJSON reads and parses a Pake config file.
func LoadPakeJSON(path string) (*PakeJSON, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var pj PakeJSON
	if err := json.Unmarshal(raw, &pj); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if len(pj.Windows) == 0 {
		return nil, fmt.Errorf("config %s: \"windows\" array is empty", path)
	}
	return &pj, nil
}

// Apply merges a parsed Pake config onto an AppConfig (over defaults). Only
// windows[0] is used in v1 (single primary window). Unsupported fields produce
// warnings via the returned slice rather than errors.
func (c *AppConfig) Apply(pj *PakeJSON) (warnings []string) {
	w := pj.Windows[0]

	c.Window.URL = w.URL
	if w.URLType != "" {
		c.Window.URLType = w.URLType
	}
	c.Window.HideTitleBar = w.HideTitleBar
	c.Window.Fullscreen = w.Fullscreen
	if w.Width > 0 {
		c.Window.Width = w.Width
	}
	if w.Height > 0 {
		c.Window.Height = w.Height
	}
	c.Window.Resizable = w.Resizable
	c.Window.AlwaysOnTop = w.AlwaysOnTop
	c.Window.DarkMode = w.DarkMode
	c.Window.ActivationShortcut = w.ActivationShortcut
	c.Window.DisabledWebShortcuts = w.DisabledWebShortcuts
	c.Window.HideOnClose = w.HideOnClose
	c.Window.Incognito = w.Incognito
	c.Window.EnableDragDrop = w.EnableDragDrop
	c.Window.Maximize = w.Maximize
	c.Window.StartToTray = w.StartToTray
	c.Window.ForceInternalNav = w.ForceInternalNav
	c.Window.InternalURLRegex = w.InternalURLRegex
	c.Window.EnableFind = w.EnableFind

	c.UserAgent.MacOS = pj.UserAgent.MacOS
	c.UserAgent.Linux = pj.UserAgent.Linux
	c.UserAgent.Windows = pj.UserAgent.Windows

	c.SystemTray.MacOS = pj.SystemTray.MacOS
	c.SystemTray.Linux = pj.SystemTray.Linux
	c.SystemTray.Windows = pj.SystemTray.Windows
	c.SystemTrayPath = pj.SystemTrayPath

	c.Inject = append(c.Inject, pj.Inject...)
	c.ProxyURL = pj.ProxyURL
	c.MultiInstance = pj.MultiInstance

	// Warn on fields gpack cannot honour.
	if pj.MultiWindow {
		warnings = append(warnings, "multi_window: v1 builds a single primary window; ignoring")
	}
	if w.NewWindow {
		warnings = append(warnings, "new_window: opening extra windows is not wired in v1; ignoring")
	}
	if w.EnableWasm {
		warnings = append(warnings, "enable_wasm: system webviews always support WASM; no toggle needed")
	}
	return warnings
}
