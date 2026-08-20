// Package builder orchestrates turning an AppConfig into a native binary. It
// self-generates a minimal Wails v3 project (no external CLI required),
// bootstraps the Go toolchain if needed, builds it, and copies out the result.
// The only non-bootstrappable requirements are a C compiler and the system
// webview dev libraries (see bootstrap.go).
package builder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"gpack/internal/config"
	"gpack/internal/runner"
)

// Build produces a native binary for cfg and copies it to cfg.OutDir.
func Build(cfg *config.AppConfig) error {
	targetOS := cfg.Platform
	if targetOS == "" || targetOS == "current" {
		targetOS = runtime.GOOS
	}
	if targetOS != runtime.GOOS {
		return fmt.Errorf("cross-compilation to %q is not supported yet (Wails uses cgo); build on the target OS", targetOS)
	}

	// Bootstrap the toolchain and verify the (non-bootstrappable) webview deps.
	goPath, env, err := ensureGo()
	if err != nil {
		return err
	}
	if !hasWebview() {
		if !cfg.InstallDeps {
			return fmt.Errorf("%s", webviewHint)
		}
		if err := installWebview(); err != nil {
			return err
		}
		if !hasWebview() {
			return fmt.Errorf("%s", webviewHint)
		}
	}

	// --debug streams subprocess output live; otherwise it is only shown on failure.
	var verbose io.Writer
	if cfg.Debug {
		verbose = os.Stderr
	}

	proj, err := os.MkdirTemp("", "gpack-*")
	if err != nil {
		return err
	}
	defer func() {
		if cfg.KeepTmp {
			fmt.Fprintln(os.Stderr, "kept temp project:", proj)
		} else {
			os.RemoveAll(proj)
		}
	}()

	// --use-local-file serves the user's directory/file from the asset origin;
	// the window then loads "/" instead of a remote URL.
	localSrc := cfg.Window.URL
	if cfg.IsLocal() {
		cfg.Window.URL = "/"
	}

	// 1. Self-generate the project files.
	step("Generating project")
	if err := writeGoMod(proj, cfg); err != nil {
		return err
	}
	mainSrc, err := renderMainGo(cfg, targetOS)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(proj, "main.go"), []byte(mainSrc), 0644); err != nil {
		return err
	}
	if cfg.IsLocal() {
		if err := copyLocalFrontend(localSrc, proj); err != nil {
			return err
		}
	} else if err := writeFrontendShell(proj, cfg); err != nil {
		return err
	}

	// 2. Icons: app icon (always, embedded by main.go) + tray icon when present.
	step("Resolving icons")
	appIcon := ResolveAppIcon(cfg)
	if err := os.WriteFile(filepath.Join(proj, "appicon.png"), appIcon, 0644); err != nil {
		return err
	}
	if cfg.AnyTray() {
		if err := os.WriteFile(filepath.Join(proj, "trayicon.png"), ResolveTrayIcon(cfg, appIcon), 0644); err != nil {
			return err
		}
	}

	// 3. Resolve dependencies (downloads the Wails v3 module + Go 1.25 toolchain).
	step("Resolving dependencies (go mod tidy)")
	if out, err := runner.Run(proj, env, verbose, goPath, "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy: %w%s", err, unstreamed(out, verbose))
	}

	// 3b. Typed JS bindings for the local-file bridge, via `go run` (no wails3 install).
	if cfg.IsLocal() {
		step("Generating bindings")
		args := []string{"run"}
		if targetOS == "linux" {
			args = append(args, "-tags", "gtk3")
		}
		args = append(args, "github.com/wailsapp/wails/v3/cmd/wails3@"+wailsVersion, "generate", "bindings")
		if out, err := runner.Run(proj, env, verbose, goPath, args...); err != nil {
			fmt.Fprintln(os.Stderr, "warning: generate bindings failed; the frontend may lack typed bindings"+unstreamed(out, verbose))
		}
	}

	// 4. Build.
	step("Building (this can take a minute)")
	binName := cfg.SafeName
	if targetOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(proj, binName)
	args := []string{"build"}
	if targetOS == "linux" {
		args = append(args, "-tags", "gtk3") // webkit2gtk-4.1 path
	}
	args = append(args, "-o", binPath, ".")
	if out, err := runner.Run(proj, env, verbose, goPath, args...); err != nil {
		return fmt.Errorf("build failed: %w%s", err, unstreamed(out, verbose))
	}

	// 5. Copy the artifact to the output directory.
	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return err
	}
	dst := filepath.Join(cfg.OutDir, binName)
	if err := copyFile(binPath, dst); err != nil {
		return err
	}
	fmt.Println("✓ Built:", dst)
	return nil
}

func writeGoMod(proj string, cfg *config.AppConfig) error {
	content := fmt.Sprintf("module gpack/%s\n\ngo 1.25.0\n\nrequire github.com/wailsapp/wails/v3 %s\n",
		cfg.SafeName, wailsVersion)
	return os.WriteFile(filepath.Join(proj, "go.mod"), []byte(content), 0644)
}

func writeFrontendShell(proj string, cfg *config.AppConfig) error {
	dir := filepath.Join(proj, "frontend", "dist")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	bg := "#ffffff"
	if cfg.Window.DarkMode {
		bg = "#1a1a1a"
	}
	html := fmt.Sprintf(indexHTML, cfg.Name, bg)
	return os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0644)
}

// copyLocalFrontend populates frontend/dist from a local file or directory.
func copyLocalFrontend(src, proj string) error {
	dist := filepath.Join(proj, "frontend", "dist")
	if err := os.RemoveAll(dist); err != nil {
		return err
	}
	if err := os.MkdirAll(dist, 0755); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("--use-local-file %q: %w", src, err)
	}
	if info.IsDir() {
		if err := copyTree(src, dist); err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
			return fmt.Errorf("--use-local-file %q: no index.html in directory", src)
		}
		return nil
	}
	return copyFile(src, filepath.Join(dist, "index.html"))
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func step(msg string) { fmt.Fprintln(os.Stderr, "→ "+msg) }

// unstreamed returns captured subprocess output for an error message, or "" when
// verbose already streamed it live — otherwise a failure prints the whole log twice.
func unstreamed(out string, verbose io.Writer) string {
	if verbose != nil {
		return ""
	}
	return "\n" + out
}
