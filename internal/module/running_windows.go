//go:build windows

package module

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// running asks tasklist for an image name.
func running(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tasklist", "/FI", "IMAGENAME eq "+name+".exe", "/NH").Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), strings.ToLower(name)+".exe")
}
