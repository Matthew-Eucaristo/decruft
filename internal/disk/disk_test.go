package disk

import (
	"context"
	"testing"
)

func TestParseSnapshots(t *testing.T) {
	got := parseSnapshots("Snapshots for disk /:\ncom.apple.TimeMachine.2026-10-27-101500.local\ncom.apple.os.update-ABC\n")
	if len(got) != 2 {
		t.Fatal(got)
	}
	if got := parseSnapshots("Snapshots for disk /:\n"); got == nil || len(got) != 0 {
		t.Fatal("no snapshots is an empty list, not unknown")
	}
}

func TestReadThisVolume(t *testing.T) {
	in := Read(context.Background(), t.TempDir())
	if in.Total <= 0 || in.Free < 0 || in.Free > in.Total {
		t.Fatalf("%+v", in)
	}
}
