package report

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/Matthew-Eucaristo/decruft/internal/act"
	"github.com/Matthew-Eucaristo/decruft/internal/disk"
	"github.com/Matthew-Eucaristo/decruft/internal/item"
	"github.com/Matthew-Eucaristo/decruft/internal/trash"
)

func scan(t *testing.T) Scan {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX home paths")
	}
	home := "/Users/someone"
	return Scan{Home: home, Version: "test",
		Disk:     disk.Info{Name: "Disk", Total: 100e9, Free: 20e9, Purgeable: 5e9, Swap: -1, Trash: -1, TrashNote: "the Trash cannot be read"},
		Warnings: []string{"fell: could not read " + home + "/dev/x"},
		Items: []item.Item{
			{ID: "a", Name: "Go build cache", Category: item.Go, Path: home + "/Library/Caches/go-build", Bytes: 2e9, Frees: 2e9, Level: item.Safe,
				Action: item.Action{Kind: item.Command, Argv: []string{home + "/go/bin/tool", "clean"}}},
			{ID: "b", Name: "Runtime", Category: item.Xcode, Bytes: 8e9, Frees: 8e9, Level: item.Review, SystemData: true,
				Action: item.Action{Kind: item.Command, Argv: []string{"/usr/bin/xcrun", "simctl", "runtime", "delete", "X"}}},
		}}
}

func TestJSONHasNoHomePath(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, scan(t)); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if strings.Contains(out, "/Users/someone") {
		t.Fatalf("home path leaked:\n%s", out)
	}
	for _, want := range []string{`"~/Library/Caches/go-build"`, `"~/go/bin/tool"`, `"schema_version": 1`, `"removable_bytes": 10000000000`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in\n%s", want, out)
		}
	}
}

func TestListExplainsHiddenSpace(t *testing.T) {
	var b bytes.Buffer
	List(&b, scan(t))
	out := b.String()
	for _, want := range []string{"Purgeable 5.0 GB", "simulator runtimes 8.0 GB", "Trash cannot be read", "runs tool clean", "~/dev/x"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "/Users/someone") {
		t.Fatal("home path leaked")
	}
}

func TestReceiptKeepsKindsApart(t *testing.T) {
	r := Receipt{Before: 10e9, After: 13e9, Regrows: true, Outcomes: []act.Outcome{
		{Name: "a", How: act.Trashed, Estimate: 1e9},
		{Name: "b", How: act.Freed, Estimate: 3e9, What: "/opt/homebrew/bin/go clean -cache"},
		{Name: "c", How: act.Delegated, Source: "nodekill", Estimate: 2e9},
		{Name: "d", How: act.Refused, Estimate: 1e9, Error: "changed since the scan"},
		{Name: "e", How: act.Freed, Estimate: 4e9, What: "docker system prune --force", Claimed: true},
	}}
	text := strings.Join(r.Lines(), "\n")
	for _, want := range []string{"Moved to " + trash.Name, "1.0 GB", "Freed now", "go clean -cache", "Delegated", "nodekill ×1",
		"Refused", "changed since the scan", "Claimed, not freed", "10.0 GB → 13.0 GB", "+3.0 GB measured", "1.0 GB more when you empty"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "/opt/homebrew") {
		t.Fatal("commands are shown by name")
	}
}

func TestDryRunReceipt(t *testing.T) {
	r := Receipt{Before: 1, After: 1, Outcomes: []act.Outcome{{How: act.Dry, Estimate: 5e9}}}
	text := strings.Join(r.Lines(), "\n")
	if !strings.Contains(text, "Would free (dry run)") || strings.Contains(text, "Free space") {
		t.Fatal(text)
	}
}
