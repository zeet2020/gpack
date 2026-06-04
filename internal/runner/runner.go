// Package runner executes external build subprocesses (wails3, go) with a shared
// environment and captured output.
package runner

import (
	"os"
	"os/exec"
)

// Run executes name+args in dir with extraEnv prepended to the inherited
// environment, returning combined stdout+stderr.
func Run(dir string, extraEnv []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
