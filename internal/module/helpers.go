package module

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/size"
)

// spec describes one item a module offers.
type spec struct {
	source   string
	category item.Category
	kind     string
	name     string
	path     string   // measured when set
	argv     []string // the official command
	env      []string
	level    item.Level
	explain  string
	regen    string
	regrows  bool
	upTo     bool
	priority int
}

// build turns a spec into a sized, sealed item. An empty or missing
// directory yields ok == false: nothing to offer.
func build(env *Env, s spec, evidence ...string) (item.Item, bool) {
	it := item.Item{
		ID:         item.ID(s.source, s.kind+"\x00"+s.path+"\x00"+strings.Join(s.argv, " ")),
		Source:     s.source,
		Category:   s.category,
		Kind:       s.kind,
		Name:       s.name,
		Path:       s.path,
		Bytes:      -1,
		Frees:      -1,
		UpTo:       s.upTo,
		Level:      s.level,
		Action:     item.Action{Kind: item.Command, Argv: s.argv, Env: s.env},
		Explain:    s.explain,
		Regenerate: s.regen,
		Regrows:    s.regrows,
		Priority:   s.priority,
	}
	if it.Level == "" {
		it.Level = item.Safe
	}
	if len(s.argv) == 0 {
		it.Action.Kind = item.Trash
	}
	if s.path != "" {
		m := env.Measure(s.path)
		if m.Missing || m.Bytes == 0 {
			return it, false
		}
		it.Bytes, it.Frees = m.Bytes, m.Frees
		if m.Unreadable {
			it.Note = joinNote(it.Note, "part of it could not be read, so the size is a lower bound")
		}
		if s.regrows {
			it.Rebuilt = birth(s.path)
		}
		evidence = append(evidence, identity(s.path))
	}
	it.Evidence = evidence
	return it, true
}

// identity is a directory's device and inode, so a folder replaced between
// scan and apply is not mistaken for the one the user saw.
func identity(path string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		return "missing"
	}
	dev, ino := size.Identity(fi)
	return fmt.Sprintf("%d:%d:%v", dev, ino, fi.Mode().Type())
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

// parseSize reads sizes such as "373.3MB", "1.2GB (50%)", "512kB" or
// "4.0 KiB". It returns -1 when it cannot.
func parseSize(s string) int64 {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "("); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.ReplaceAll(s, ",", "")
	i := 0
	for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	n, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return -1
	}
	unit := strings.ToUpper(strings.TrimSpace(s[i:]))
	mult := map[string]float64{
		"": 1, "B": 1,
		"KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12,
		"K": 1e3, "M": 1e6, "G": 1e9, "T": 1e12,
		"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "TIB": 1 << 40,
	}
	m, ok := mult[unit]
	if !ok {
		return -1
	}
	return int64(n * m)
}
