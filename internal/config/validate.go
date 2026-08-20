package config

import (
	"fmt"
	"strings"
)

// Validate checks the resolved config for hard errors and collects soft
// warnings about partially-supported or no-op options.
func (c *AppConfig) Validate() (warnings []string, err error) {
	if strings.TrimSpace(c.Window.URL) == "" {
		return nil, fmt.Errorf("a target URL is required (pass it as an argument, via --config, or with the windows[0].url field)")
	}

	if !c.IsLocal() {
		if !strings.HasPrefix(c.Window.URL, "http://") && !strings.HasPrefix(c.Window.URL, "https://") {
			warnings = append(warnings, fmt.Sprintf("url %q has no http(s) scheme; assuming https", c.Window.URL))
			c.Window.URL = "https://" + c.Window.URL
		}
	}

	if c.Window.Width <= 0 || c.Window.Height <= 0 {
		return nil, fmt.Errorf("window size must be positive (got %dx%d)", c.Window.Width, c.Window.Height)
	}

	if c.Window.Zoom < 50 || c.Window.Zoom > 200 {
		warnings = append(warnings, fmt.Sprintf("zoom %d%% out of 50-200 range; clamping", c.Window.Zoom))
		if c.Window.Zoom < 50 {
			c.Window.Zoom = 50
		} else {
			c.Window.Zoom = 200
		}
	}

	if c.Window.HideOnClose && !c.AnyTray() {
		warnings = append(warnings, "hide_on_close set but no system tray enabled; closing the window would hide it with no way to restore or quit — enable --show-system-tray")
	}

	if c.Window.StartToTray && !c.AnyTray() {
		warnings = append(warnings, "start_to_tray set but no system tray enabled; window would start hidden with no way to restore — enable --show-system-tray")
	}

	if c.SafeDomain != "" && !c.Window.ForceInternalNav {
		warnings = append(warnings, "--safe-domain has no effect without --force-internal-navigation")
	}

	if c.IgnoreCertErrors {
		warnings = append(warnings, "--ignore-certificate-errors is not supported by the embedded webview; use a properly signed certificate")
	}

	if c.Window.ActivationShortcut != "" {
		warnings = append(warnings, "activation_shortcut is best-effort (global hotkey); behaviour varies by platform")
	}

	if c.ProxyURL != "" {
		warnings = append(warnings, "proxy_url is best-effort; system webviews may use OS proxy settings instead")
	}

	return warnings, nil
}
