// Package family runs the family's own cleaners (fell, nodekill,
// flutterkill) through their JSON contract: `capabilities --json` to find
// them, `scan --json` to see what they see, and `apply --plan` to hand
// removals back, so each tool's own guards always run.
package family

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
)

// Schema is the contract version decruft understands.
const Schema = 1

// Known lists the family's cleaners and the category their items go in.
var Known = []struct {
	Name     string
	Category item.Category
}{
	{"fell", item.Worktrees},
	{"nodekill", item.Node},
	{"flutterkill", item.Flutter},
}

// Runner runs a tool. Tests replace it.
type Runner func(ctx context.Context, stdin []byte, argv ...string) (stdout []byte, exit int, err error)

// Exec is the real runner. A non-zero exit is not an error: the contract
// uses exit codes 3 and 4 for refused and partial runs.
func Exec(ctx context.Context, stdin []byte, argv ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ctx.Err() == nil {
		if code := ee.ExitCode(); code != 0 && len(out) > 0 {
			return out, code, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, ee.ExitCode(), errors.New(firstLine(msg))
	}
	return out, 0, err
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Tool is a family cleaner found on PATH.
type Tool struct {
	Name     string        `json:"name"`
	Path     string        `json:"-"`
	Version  string        `json:"version"`
	Category item.Category `json:"category"`
	// Contract is false for a tool without `capabilities --json` (flutterkill
	// 0.2): decruft can show its items and open it, never act through it.
	Contract bool     `json:"contract"`
	Roots    []string `json:"-"`
}

// Family finds and talks to the tools.
type Family struct {
	Run      Runner
	LookPath func(string) (string, error)
	Home     string
}

// Discover checks one known tool on PATH and its contract. A tool that
// answers with an unknown schema is reported, never guessed at. ok is false
// when the tool cannot be used; warn then says why.
func (f *Family) Discover(ctx context.Context, name string, cat item.Category) (t Tool, warn string, ok bool) {
	p, err := f.LookPath(name)
	if err != nil {
		return t, name + " is not installed, so its area is not scanned (brew install matthew-eucaristo/tap/" + name + ")", false
	}
	t = Tool{Name: name, Path: p, Category: cat}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	out, _, err := f.Run(cctx, nil, p, "capabilities", "--json")
	cancel()
	var caps struct {
		Schema int `json:"schema_version"`
		Tool   struct {
			Version string `json:"version"`
		} `json:"tool"`
	}
	switch {
	case err == nil && json.Unmarshal(out, &caps) == nil && caps.Schema == Schema:
		t.Contract, t.Version = true, caps.Tool.Version
		return t, "", true
	case err == nil && caps.Schema != 0:
		return t, fmt.Sprintf("%s speaks contract version %d; this decruft understands %d, so it is skipped", name, caps.Schema, Schema), false
	case name == "flutterkill":
		cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		v, _, err := f.Run(cctx, nil, p, "--version")
		cancel()
		if err != nil {
			return t, "flutterkill did not answer --version; skipped", false
		}
		t.Version = strings.TrimSpace(strings.TrimPrefix(firstLine(string(v)), "flutterkill"))
		return t, "", true
	}
	return t, name + " did not answer `capabilities --json`; skipped (upgrade it)", false
}

// expand turns the contract's "~/…" paths back into absolute ones.
func (f *Family) expand(p string) string {
	if p == "~" {
		return f.Home
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(f.Home, filepath.FromSlash(rest))
	}
	return p
}
