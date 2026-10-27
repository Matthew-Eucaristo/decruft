// Package module holds decruft's built-in modules: areas of a developer's
// machine that have one official cleanup command (go clean, brew cleanup,
// xcrun simctl, pod cache clean, uv cache prune, docker system prune …).
// Modules only read during a scan; the act package runs what they return.
package module

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/size"
)

// Env is everything a module may touch, so tests can replace the machine.
type Env struct {
	Home   string
	OS     string
	Getenv func(string) string
	// LookPath finds an executable on PATH.
	LookPath func(string) (string, error)
	// Output runs a read-only command and returns its stdout.
	Output func(ctx context.Context, extraEnv []string, argv ...string) ([]byte, error)
	// Running reports whether a process with this exact name runs.
	Running func(name string) bool
	// Measure sizes a tree: on disk, and what removing it frees now.
	Measure func(path string) Measured
}

// Measured is a sized tree.
type Measured struct {
	Bytes, Frees int64
	Unreadable   bool // part of it could not be read; sizes are a lower bound
	Missing      bool
}

// System is the real machine.
func System() *Env {
	home, _ := os.UserHomeDir()
	return &Env{
		Home:     home,
		OS:       runtime.GOOS,
		Getenv:   os.Getenv,
		LookPath: exec.LookPath,
		Output:   output,
		Running:  running,
		Measure:  measure,
	}
}

// Quiet is added to every command decruft runs or reads, so tools do not
// update themselves, phone home or prompt.
var Quiet = []string{
	"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1", "HOMEBREW_NO_INSTALL_CLEANUP=1",
	"HOMEBREW_NO_ANALYTICS=1", "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8", "NO_COLOR=1",
	"DOCKER_CLI_HINTS=false", "PIP_DISABLE_PIP_VERSION_CHECK=1", "PIP_NO_INPUT=1",
}

// readTimeout bounds every read-only command, so one stuck tool (a Docker
// daemon that does not answer) cannot hold the scan.
const readTimeout = 30 * time.Second

func output(ctx context.Context, extra []string, argv ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(append(os.Environ(), Quiet...), extra...)
	cmd.Stdin = nil
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return out, fmt.Errorf("%s: %w", argv[0], ctx.Err())
		}
		msg := strings.TrimSpace(stderr.String())
		if i := strings.IndexByte(msg, '\n'); i > 0 {
			msg = msg[:i]
		}
		if msg != "" {
			return out, fmt.Errorf("%s: %s", filepath.Base(argv[0]), msg)
		}
		return out, fmt.Errorf("%s: %w", filepath.Base(argv[0]), err)
	}
	return out, nil
}

func measure(path string) Measured {
	if _, err := os.Lstat(path); err != nil {
		return Measured{Bytes: 0, Frees: 0, Missing: errors.Is(err, os.ErrNotExist)}
	}
	r := size.Measure(path, size.Options{})
	return Measured{Bytes: r.Size.Allocated, Frees: r.Size.Exclusive(), Unreadable: r.Unreadable > 0}
}

// Module is one built-in area.
type Module struct {
	ID   string
	Name string
	Scan func(ctx context.Context, env *Env) Result
}

// Result is what a module found.
type Result struct {
	Items    []item.Item
	Warnings []string
}

// isDir reports whether path is a directory (not a symlink to one).
func isDir(path string) bool {
	fi, err := os.Lstat(path)
	return err == nil && fi.IsDir()
}

// subdirs lists the directories directly inside dir, skipping symlinks.
func subdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// firstLine is the first non-empty line of b, trimmed.
func firstLine(b []byte) string {
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}
