// Package format holds small human-readable formatting helpers.
package format

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bytes formats n with decimal units like Finder (1 GB = 1000^3 bytes).
func Bytes(n int64) string {
	if n < 0 {
		return "…"
	}
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

// Ago formats how long ago t was, coarsely ("today", "3d", "5w", "4mo", "2y").
func Ago(t, now time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := now.Sub(t)
	days := int(d.Hours() / 24)
	switch {
	case days < 1:
		return "today"
	case days < 14:
		return fmt.Sprintf("%dd ago", days)
	case days < 60:
		return fmt.Sprintf("%dw ago", days/7)
	case days < 365:
		return fmt.Sprintf("%dmo ago", days/30)
	default:
		return fmt.Sprintf("%.1fy ago", float64(days)/365)
	}
}

// ParseAge parses ages like "90d", "6w", "3mo", "1y", or any Go duration.
func ParseAge(s string) (time.Duration, error) {
	units := []struct {
		suffix string
		d      time.Duration
	}{{"mo", 30 * 24 * time.Hour}, {"d", 24 * time.Hour}, {"w", 7 * 24 * time.Hour}, {"y", 365 * 24 * time.Hour}}
	for _, u := range units {
		if v, ok := strings.CutSuffix(s, u.suffix); ok {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return 0, fmt.Errorf("invalid age %q", s)
			}
			return time.Duration(n) * u.d, nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid age %q (try 90d, 6w, 3mo or 1y)", s)
	}
	return d, nil
}

// Tilde shortens home to "~" for display. Tools may report the home folder
// through a symlink or its resolved path (/tmp vs /private/tmp on macOS),
// so both spellings are shortened.
func Tilde(path, home string) string {
	for _, h := range homes(home) {
		if path == h || strings.HasPrefix(path, h+string(filepath.Separator)) {
			return "~" + path[len(h):]
		}
	}
	return path
}

// TildeAll shortens every spelling of home inside a text.
func TildeAll(s, home string) string {
	for _, h := range homes(home) {
		s = strings.ReplaceAll(s, h, "~")
	}
	return s
}

var resolved sync.Map // home → its symlink-free path

func homes(home string) []string {
	if home == "" {
		return nil
	}
	r, ok := resolved.Load(home)
	if !ok {
		real, err := filepath.EvalSymlinks(home)
		if err != nil {
			real = home
		}
		r, _ = resolved.LoadOrStore(home, real)
	}
	if real := r.(string); real != home {
		return []string{home, real}
	}
	return []string{home}
}

// Rel shows path relative to root, or path itself when it is root.
func Rel(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return filepath.Base(path)
}
