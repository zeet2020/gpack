package builder

import (
	"bytes"
	"strings"
	"testing"
)

// With --debug the subprocess output has already gone to stderr live, so folding
// it into the error too would print the whole build log twice.
func TestUnstreamed(t *testing.T) {
	const log = "compile error: everything is broken"

	if got := unstreamed(log, &bytes.Buffer{}); got != "" {
		t.Errorf("verbose: got %q, want empty (already streamed)", got)
	}
	got := unstreamed(log, nil)
	if !strings.Contains(got, log) {
		t.Errorf("quiet: got %q, want the captured output", got)
	}
}
