package disk

import "strings"

// parseSnapshots reads `tmutil listlocalsnapshots /`:
//
//	Snapshots for disk /:
//	com.apple.TimeMachine.2026-10-27-101500.local
func parseSnapshots(out string) []string {
	snaps := []string{}
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "com.apple.") {
			snaps = append(snaps, l)
		}
	}
	return snaps
}
