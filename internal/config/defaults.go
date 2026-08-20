package config

// Defaults returns an AppConfig pre-populated with gpack's baseline values.
// JSON and CLI flags layer on top of this.
//
// There is deliberately no default User-Agent: the override is a JS-level shim
// (see builder.BuildInject), so it cannot change the HTTP header the way Pake's
// native one does. Unset means the webview's own UA is used, unmodified.
func Defaults() *AppConfig {
	return &AppConfig{
		Window: WindowConfig{
			URLType:   "web",
			Width:     1200,
			Height:    780,
			Zoom:      100,
			Resizable: true,
		},
		AppVersion: "1.0.0",
		OutDir:     ".",
		Platform:   "current",
	}
}
