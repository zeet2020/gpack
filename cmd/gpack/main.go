// Command gpack turns any URL into a native desktop app, built with Go + Wails v3.
//
// Usage:
//
//	gpack <url> [flags]
//	gpack --config weekly.json --name Weekly --icon ./icon.png
package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"gpack/internal/builder"
	"gpack/internal/config"
)

// version is the gpack tool version (distinct from --app-version, the built app's version).
const version = "0.2.0"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gpack [url] [flags]",
		Version: version,
		Short:   "Turn any URL into a native desktop app (Go + Wails v3)",
		Long: "gpack builds a native desktop application from a web URL. It accepts a\n" +
			"Pake-compatible JSON config (--config) or CLI flags, generates a throwaway\n" +
			"Wails v3 project, builds it, and copies out the binary.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          run,
	}
	registerFlags(cmd)
	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := resolveConfig(cmd, args)
	if err != nil {
		return err
	}

	// --dry-run inspects the resolved config without building.
	if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
		fmt.Print(cfg.Summary())
		return nil
	}

	fmt.Fprint(os.Stderr, cfg.Summary())
	return builder.Build(cfg)
}

// resolveConfig layers the Pake JSON config, the positional URL and the CLI flags
// over the defaults, in that precedence order, then validates the result.
func resolveConfig(cmd *cobra.Command, args []string) (*config.AppConfig, error) {
	cfg := config.Defaults()

	// 1. Pake JSON config (lowest precedence above defaults).
	if path, _ := cmd.Flags().GetString("config"); path != "" {
		pj, err := config.LoadPakeJSON(path)
		if err != nil {
			return nil, err
		}
		warnings, err := cfg.Apply(pj)
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
		if err != nil {
			return nil, err
		}
	}

	// 2. A positional argument replaces the config's URL — and with it the config's
	// url_type, which described the URL being replaced. --use-local-file runs after
	// this and can still mark the new one local.
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		cfg.Window.URL = args[0]
		cfg.Window.URLType = "web"
	}

	// 3. Explicit CLI flags override individual fields.
	if err := applyFlags(cmd, cfg); err != nil {
		return nil, err
	}

	// 4. Derive Name (from URL host if absent) and SafeName.
	if cfg.Name == "" {
		cfg.Name = deriveName(cfg.Window.URL)
	}
	cfg.Sanitise()

	// 5. Validate.
	warnings, err := cfg.Validate()
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	return cfg, nil
}

// deriveName produces a friendly app name from a URL hostname, e.g.
// https://weekly.tw93.fun/en -> "Weekly".
func deriveName(raw string) string {
	if raw == "" {
		return "App"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		// Not a remote URL — it's a local path; name the app after the file.
		base := strings.TrimSuffix(filepath.Base(raw), filepath.Ext(raw))
		if base == "." || base == string(filepath.Separator) {
			return "App"
		}
		return title(base)
	}
	host := u.Hostname()
	host = strings.TrimPrefix(host, "www.")
	label := host
	if i := strings.IndexByte(host, '.'); i > 0 {
		label = host[:i]
	}
	return title(label)
}

// title upper-cases the first rune, leaving the rest alone.
func title(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "App"
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}
