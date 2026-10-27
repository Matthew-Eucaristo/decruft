// Package disk explains the disk: free space as the filesystem reports it,
// and on macOS what Settings hides behind "Available" and "System Data"
// (purgeable space, local snapshots, swap, the Trash).
package disk

import (
	"context"
	"os"

	"github.com/Matthew-Eucaristo/decruft/internal/size"
	"github.com/Matthew-Eucaristo/decruft/internal/trash"
)

// Info describes the volume holding the home folder.
type Info struct {
	Name  string `json:"name"`
	Total int64  `json:"total_bytes"`
	// Free is what statfs reports as available to the user: the number every
	// before/after measurement uses.
	Free int64 `json:"free_bytes"`
	// Purgeable is space macOS can reclaim by itself (caches, iCloud
	// copies, snapshots) and counts as "available" in Finder. -1 unknown.
	Purgeable int64 `json:"purgeable_bytes"`
	// Snapshots are APFS local snapshots (Time Machine, updates); info only.
	Snapshots []string `json:"snapshots"`
	// Swap is the virtual-memory swap files, part of System Data. -1 unknown.
	Swap int64 `json:"swap_bytes"`
	// Trash is what the Trash holds: freed only when it is emptied. -1 when
	// it cannot be read (macOS needs Full Disk Access for the terminal).
	Trash     int64  `json:"trash_bytes"`
	TrashNote string `json:"trash_note,omitempty"`
}

// Read gathers Info for the volume holding home. Each part degrades to
// "unknown" on its own.
func Read(ctx context.Context, home string) Info {
	total, free := Space(home)
	in := Info{Name: "Disk", Total: total, Free: free, Purgeable: -1, Swap: -1, Trash: -1}
	platform(ctx, home, &in)
	if dir := trash.Dir(home); dir != "" {
		if _, err := os.ReadDir(dir); err != nil {
			if os.IsNotExist(err) {
				in.Trash = 0
			} else {
				in.TrashNote = "the " + trash.Name + " cannot be read; give your terminal Full Disk Access to measure it"
			}
		} else {
			in.Trash = size.Measure(dir, size.Options{}).Size.Allocated
		}
	}
	return in
}

// Free is the space available on path's volume, -1 when unknown.
func Free(path string) int64 {
	_, f := Space(path)
	return f
}
