// Package runner executes external build subprocesses (wails3, go) with a shared
// environment and captured output.
package runner

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

// Run executes name+args in dir with extraEnv prepended to the inherited
// environment, returning combined stdout+stderr. When verbose is non-nil the
// output is also streamed there as it arrives, so long builds are not silent.
func Run(dir string, extraEnv []string, verbose io.Writer, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)

	var buf bytes.Buffer
	out := io.Writer(&buf)
	if verbose != nil {
		out = io.MultiWriter(&buf, verbose)
	}
	cmd.Stdout, cmd.Stderr = out, out

	err := cmd.Run()
	return buf.String(), err
}
