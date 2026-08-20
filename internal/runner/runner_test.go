package runner

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCapturesAndStreams(t *testing.T) {
	var live bytes.Buffer
	out, err := Run("", nil, &live, "go", "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "go version") {
		t.Errorf("returned output = %q, want the command's output", out)
	}
	// --debug needs the output live, not only in the returned string.
	if live.String() != out {
		t.Errorf("streamed %q, want the same as the returned %q", live.String(), out)
	}
}

func TestRunQuietWhenNoWriter(t *testing.T) {
	out, err := Run("", nil, nil, "go", "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "go version") {
		t.Errorf("output = %q, want it captured even when not streaming", out)
	}
}

func TestRunReturnsOutputOnFailure(t *testing.T) {
	// Failures must still carry their diagnostics back to the caller's error.
	out, err := Run("", nil, nil, "go", "definitely-not-a-subcommand")
	if err == nil {
		t.Fatal("bad subcommand succeeded")
	}
	if out == "" {
		t.Error("no output captured on failure; the build error would be blank")
	}
}
