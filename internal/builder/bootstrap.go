package builder

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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
const wailsVersion = "v3.0.0-beta.10"

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

// downloadGo fetches the latest stable Go tarball for this platform, verifies it
// against the SHA-256 published in go.dev's release index, and extracts it under
// the gpack cache, returning the path to the `go` binary. gpack executes this
// toolchain, so the archive is never unpacked before the digest matches.
func downloadGo() (string, error) {
	archive, err := latestGoArchive()
	if err != nil {
		return "", err
	}

	dest := cacheDir()
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}

	tarball, err := fetchVerified("https://go.dev/dl/"+archive.Filename, archive.SHA256)
	if err != nil {
		return "", err
	}
	defer os.Remove(tarball.Name())
	defer tarball.Close()

	_ = os.RemoveAll(filepath.Join(dest, "go"))
	if err := extractTarGz(tarball, dest); err != nil {
		return "", err
	}

	goBin := filepath.Join(dest, "go", "bin", goExeName())
	if _, err := os.Stat(goBin); err != nil {
		return "", fmt.Errorf("extracted toolchain missing %s", goBin)
	}
	return goBin, nil
}

// fetchVerified downloads url to a temp file, hashing as it streams, and returns
// the file rewound to the start. A digest mismatch is an error and the partial
// download is discarded — nothing is extracted or run.
func fetchVerified(url, wantSHA256 string) (*os.File, error) {
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	f, err := os.CreateTemp("", "gpack-go-*.tar.gz")
	if err != nil {
		return nil, err
	}
	discard := func() {
		f.Close()
		os.Remove(f.Name())
	}

	sum := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, sum), resp.Body); err != nil {
		discard()
		return nil, err
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != wantSHA256 {
		discard()
		return nil, fmt.Errorf("checksum mismatch for %s:\n  want %s\n  got  %s\nrefusing to install this toolchain", url, wantSHA256, got)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		discard()
		return nil, err
	}
	return f, nil
}

// goRelease is the subset of go.dev/dl's JSON index that gpack reads.
type goRelease struct {
	Version string   `json:"version"`
	Stable  bool     `json:"stable"`
	Files   []goFile `json:"files"`
}

type goFile struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kind     string `json:"kind"`
	SHA256   string `json:"sha256"`
}

func latestGoArchive() (goFile, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://go.dev/dl/?mode=json")
	if err != nil {
		return goFile{}, err
	}
	defer resp.Body.Close()
	var releases []goRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return goFile{}, err
	}
	return pickArchive(releases, runtime.GOOS, runtime.GOARCH)
}

// pickArchive finds the newest stable .tar.gz build for goos/goarch that carries
// a checksum. Releases arrive newest-first.
func pickArchive(releases []goRelease, goos, goarch string) (goFile, error) {
	for _, r := range releases {
		if !r.Stable {
			continue
		}
		for _, f := range r.Files {
			if f.Kind != "archive" || f.OS != goos || f.Arch != goarch {
				continue
			}
			if !strings.HasSuffix(f.Filename, ".tar.gz") {
				continue // Windows ships .zip; extractTarGz cannot read it
			}
			if f.SHA256 == "" {
				return goFile{}, fmt.Errorf("%s: release index has no sha256; refusing to install unverified", f.Filename)
			}
			return f, nil
		}
	}
	return goFile{}, fmt.Errorf("no stable Go tar.gz build for %s/%s; install Go from https://go.dev/dl manually", goos, goarch)
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
