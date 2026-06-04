package config

// Default platform User-Agent strings, matching Pake's defaults closely enough
// that UA-detecting sites treat the app like a normal browser.
const (
	defaultUAMacOS = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
	defaultUALinux = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	defaultUAWin   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// Defaults returns an AppConfig pre-populated with gpack's baseline values.
// JSON and CLI flags layer on top of this.
func Defaults() *AppConfig {
	return &AppConfig{
		Window: WindowConfig{
			URLType:   "web",
			Width:     1200,
			Height:    780,
			Zoom:      100,
			Resizable: true,
		},
		AppVersion:        "1.0.0",
		InstallerLanguage: "en-US",
		OutDir:            ".",
		Platform:          "current",
	}
}
