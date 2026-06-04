package builder

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// wailsVersion is the pinned Wails v3 module/CLI version gpack builds against.
const wailsVersion = "v3.0.0-alpha.98"

// minGoMinor is the lowest Go 1.x that supports automatic toolchain switching
// (GOTOOLCHAIN), which lets an older local Go fetch the 1.25 the project needs.
const minGoMinor = 21

func cacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "gpack")
}

func goExeName() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

// ensureGo returns a usable `go` binary and the build environment, downloading a
// Go toolchain into gpack's cache if none is present. With GOTOOLCHAIN=auto, any
// Go ≥1.21 will fetch the 1.25 the generated module requires at build time.
func ensureGo() (goPath string, env []string, err error) {
	env = []string{"GOTOOLCHAIN=auto"}

	if p, ok := usableGo("go"); ok {
		return p, env, nil
	}
	cached := filepath.Join(cacheDir(), "go", "bin", goExeName())
	if p, ok := usableGo(cached); ok {
		return p, env, nil
	}

	fmt.Fprintln(os.Stderr, "→ No suitable Go found; downloading a Go toolchain (one-time)")
	p, err := downloadGo()
	if err != nil {
		return "", nil, fmt.Errorf("bootstrap Go: %w (install Go ≥1.25 from https://go.dev/dl and retry)", err)
	}
	return p, env, nil
}

// usableGo reports whether the given go binary exists and is new enough to drive
// toolchain switching.
func usableGo(bin string) (string, bool) {
	out, err := exec.Command(bin, "version").Output()
	if err != nil {
		return "", false
	}
	if minor, ok := parseGoMinor(string(out)); ok && minor >= minGoMinor {
		return bin, true
	}
	return "", false
}

// parseGoMinor extracts the minor version from `go version go1.25.4 ...`.
func parseGoMinor(versionOutput string) (int, bool) {
	for _, field := range strings.Fields(versionOutput) {
		if strings.HasPrefix(field, "go1.") {
			parts := strings.Split(strings.TrimPrefix(field, "go"), ".")
			if len(parts) >= 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil {
					return n, true
				}
			}
		}
	}
	return 0, false
}

// downloadGo fetches the latest stable Go tarball for this platform and extracts
// it under the gpack cache, returning the path to the `go` binary.
func downloadGo() (string, error) {
	ver, err := latestGoVersion()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("auto-download supports tar.gz platforms; install %s manually on Windows", ver)
	}

	url := fmt.Sprintf("https://go.dev/dl/%s.%s-%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	dest := cacheDir()
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}
	_ = os.RemoveAll(filepath.Join(dest, "go"))

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	if err := extractTarGz(resp.Body, dest); err != nil {
		return "", err
	}

	goBin := filepath.Join(dest, "go", "bin", goExeName())
	if _, err := os.Stat(goBin); err != nil {
		return "", fmt.Errorf("extracted toolchain missing %s", goBin)
	}
	return goBin, nil
}

func latestGoVersion() (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://go.dev/dl/?mode=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var releases []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", err
	}
	for _, r := range releases {
		if r.Stable {
			return r.Version, nil
		}
	}
	return "", fmt.Errorf("no stable Go release found")
}

func extractTarGz(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name) // archive paths are "go/..."
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

// hasWebview reports whether the Linux webview dev libraries gpack builds against
// (gtk3 + webkit2gtk-4.1) are present. Always true off Linux.
func hasWebview() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	return exec.Command("pkg-config", "--exists", "gtk+-3.0", "webkit2gtk-4.1").Run() == nil
}

const webviewHint = `missing Linux webview libraries (gtk3 + webkit2gtk-4.1).
These are system packages and cannot be auto-built — install them, e.g.:
  Debian/Ubuntu:  sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
  Fedora:         sudo dnf install gtk3-devel webkit2gtk4.1-devel
  Arch:           sudo pacman -S gtk3 webkit2gtk-4.1
…or re-run with --install-deps to attempt the apt command for you.`

// installWebview attempts to install the Linux webview deps via apt (opt-in).
func installWebview() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return fmt.Errorf("--install-deps only supports apt-get; install gtk3 + webkit2gtk-4.1 manually")
	}
	fmt.Fprintln(os.Stderr, "→ Installing webview deps via apt (sudo)")
	cmd := exec.Command("sudo", "apt-get", "install", "-y", "libgtk-3-dev", "libwebkit2gtk-4.1-dev")
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stderr, os.Stderr, os.Stdin
	return cmd.Run()
}
