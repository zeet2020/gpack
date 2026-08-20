package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// A trimmed copy of go.dev/dl/?mode=json: newest release first, several files per
// release, and an unstable entry that must be skipped.
const releaseIndex = `[
 {"version":"go1.99rc1","stable":false,"files":[
   {"filename":"go1.99rc1.linux-amd64.tar.gz","os":"linux","arch":"amd64","kind":"archive","sha256":"rc"}]},
 {"version":"go1.25.4","stable":true,"files":[
   {"filename":"go1.25.4.src.tar.gz","os":"","arch":"","kind":"source","sha256":"src"},
   {"filename":"go1.25.4.linux-amd64.tar.gz","os":"linux","arch":"amd64","kind":"archive","sha256":"aaa"},
   {"filename":"go1.25.4.darwin-arm64.pkg","os":"darwin","arch":"arm64","kind":"installer","sha256":"pkg"},
   {"filename":"go1.25.4.darwin-arm64.tar.gz","os":"darwin","arch":"arm64","kind":"archive","sha256":"bbb"},
   {"filename":"go1.25.4.windows-amd64.zip","os":"windows","arch":"amd64","kind":"archive","sha256":"zip"}]},
 {"version":"go1.25.3","stable":true,"files":[
   {"filename":"go1.25.3.linux-amd64.tar.gz","os":"linux","arch":"amd64","kind":"archive","sha256":"old"}]}
]`

func decodeIndex(t *testing.T, body string) []goRelease {
	t.Helper()
	var releases []goRelease
	if err := json.Unmarshal([]byte(body), &releases); err != nil {
		t.Fatal(err)
	}
	return releases
}

func TestPickArchive(t *testing.T) {
	releases := decodeIndex(t, releaseIndex)

	got, err := pickArchive(releases, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "go1.25.4.linux-amd64.tar.gz" || got.SHA256 != "aaa" {
		t.Errorf("linux/amd64 = %+v, want the newest stable tar.gz with its checksum", got)
	}

	got, err = pickArchive(releases, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if got.Filename != "go1.25.4.darwin-arm64.tar.gz" {
		t.Errorf("darwin/arm64 = %q, want the archive, not the .pkg installer", got.Filename)
	}

	// Windows ships a .zip, which extractTarGz cannot read: no match, not a
	// silently wrong one.
	if _, err := pickArchive(releases, "windows", "amd64"); err == nil {
		t.Error("windows/amd64 returned a build, want an error (zip is unsupported)")
	}
	if _, err := pickArchive(releases, "plan9", "386"); err == nil {
		t.Error("plan9/386 returned a build, want an error")
	}
}

func TestPickArchiveRejectsMissingChecksum(t *testing.T) {
	releases := decodeIndex(t, `[{"version":"go1.25.4","stable":true,"files":[
	  {"filename":"go1.25.4.linux-amd64.tar.gz","os":"linux","arch":"amd64","kind":"archive","sha256":""}]}]`)

	if _, err := pickArchive(releases, "linux", "amd64"); err == nil {
		t.Error("accepted a release with no sha256, want refusal")
	}
}

func TestFetchVerified(t *testing.T) {
	payload := []byte("pretend this is a Go toolchain")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	f, err := fetchVerified(srv.URL, digest)
	if err != nil {
		t.Fatalf("matching digest rejected: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	got, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("downloaded %q, want %q", got, payload)
	}
}

func TestFetchVerifiedRejectsTamperedDownload(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp) // so the leftover check below sees only this test's files

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("malicious toolchain"))
	}))
	defer srv.Close()

	want := strings.Repeat("0", 64)
	f, err := fetchVerified(srv.URL, want)
	if err == nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatal("mismatched digest accepted; the toolchain would have been extracted and run")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("err = %v, want a checksum mismatch", err)
	}
	leftovers, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range leftovers {
		t.Errorf("left a partial download behind: %s", e.Name())
	}
}
