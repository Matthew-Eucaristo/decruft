//go:build unix

package module

import (
	"context"
	"os/exec"
	"time"
)

// running asks pgrep for an exact process name. Exit status 1 means none.
func running(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "pgrep", "-x", name).Run() == nil
}
