// Package builder orchestrates turning an AppConfig into a native binary: it
// scaffolds a Wails v3 project, injects gpack's generated main.go, builds it,
// and copies out the result.
package builder

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

	env := buildEnv()

	tmp, err := os.MkdirTemp("", "gpack-*")
	if err != nil {
		return err
	}
	defer func() {
		if cfg.KeepTmp {
			fmt.Fprintln(os.Stderr, "kept temp project:", tmp)
		} else {
			os.RemoveAll(tmp)
		}
	}()
	proj := filepath.Join(tmp, cfg.SafeName)

	// 1. Scaffold the canonical v3 project. init may exit non-zero because its
	// own `go mod tidy` can fail on a stale toolchain directive; we re-tidy
	// ourselves below, so judge success by whether main.go was written.
	step("Scaffolding Wails v3 project")
	out, err := runner.Run(tmp, env, toolPath("wails3"), "init",
		"-n", cfg.SafeName, "-mod", "gpack/"+cfg.SafeName, "-t", "vanilla", "-d", tmp)
	if _, statErr := os.Stat(filepath.Join(proj, "main.go")); statErr != nil {
		return fmt.Errorf("wails3 init failed: %w\n%s", err, out)
	}

	// 2. Pin the Go directive to 1.25 (template default may request an unavailable toolchain).
	step("Configuring module")
	if err := patchGoMod(filepath.Join(proj, "go.mod")); err != nil {
		return err
	}

	// 3. Replace main.go with gpack's generated entrypoint; drop the demo service;
	// drop a minimal embedded frontend shell (avoids needing npm/Vite).
	// --use-local-file serves the user's directory/file from the asset origin;
	// the window then loads "/" instead of a remote URL.
	localSrc := cfg.Window.URL
	if cfg.UseLocalFile {
		cfg.Window.URL = "/"
	}

	mainSrc, err := renderMainGo(cfg, targetOS)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(proj, "main.go"), []byte(mainSrc), 0644); err != nil {
		return err
	}
	os.Remove(filepath.Join(proj, "greetservice.go"))

	if cfg.UseLocalFile {
		if err := copyLocalFrontend(localSrc, proj); err != nil {
			return err
		}
	} else if err := writeFrontendShell(proj, cfg); err != nil {
		return err
	}

	// 3b. Icons: app icon (always, embedded by main.go) + tray icon when present.
	step("Resolving icons")
	appIcon := ResolveAppIcon(cfg)
	if err := os.WriteFile(filepath.Join(proj, "appicon.png"), appIcon, 0644); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(proj, "build", "appicon.png"), appIcon, 0644) // for `wails3 package`
	if cfg.AnyTray() {
		if err := os.WriteFile(filepath.Join(proj, "trayicon.png"), ResolveTrayIcon(cfg, appIcon), 0644); err != nil {
			return err
		}
	}

	// 4. Resolve dependencies for the rewritten module.
	step("Resolving dependencies (go mod tidy)")
	if out, err := runner.Run(proj, env, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy: %w\n%s", err, out)
	}

	// 4b. Typed JS bindings for the local-file bridge (best-effort).
	if cfg.UseLocalFile {
		step("Generating bindings")
		if out, err := runner.Run(proj, env, toolPath("wails3"), "generate", "bindings"); err != nil {
			fmt.Fprintln(os.Stderr, "warning: wails3 generate bindings failed; the frontend may lack typed bindings\n"+out)
		}
	}

	// 5. Build.
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
	if out, err := runner.Run(proj, env, "go", args...); err != nil {
		return fmt.Errorf("build failed: %w\n%s", err, out)
	}

	// 6. Copy the artifact to the output directory.
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

// buildEnv prepends the Go bin dir (where wails3/task live) to PATH and enables
// automatic toolchain download (Wails v3 needs Go ≥1.25).
func buildEnv() []string {
	gobin := goBinDir()
	env := []string{"GOTOOLCHAIN=auto"}
	if gobin != "" {
		env = append(env, "PATH="+gobin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return env
}

// toolPath resolves a Go-installed tool (wails3, task) to an absolute path, since
// exec.Command resolves names against the parent PATH, not the child's cmd.Env.
func toolPath(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}
	if gobin := goBinDir(); gobin != "" {
		p := filepath.Join(gobin, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return name
}

func goBinDir() string {
	if gp := os.Getenv("GOPATH"); gp != "" {
		return filepath.Join(gp, "bin")
	}
	out, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return ""
	}
	gp := strings.TrimSpace(string(out))
	if gp == "" {
		return ""
	}
	return filepath.Join(gp, "bin")
}

func patchGoMod(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "toolchain "):
			continue // drop; let GOTOOLCHAIN=auto pick
		case strings.HasPrefix(trimmed, "go "):
			lines = append(lines, "go 1.25.0")
		default:
			lines = append(lines, line)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
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
