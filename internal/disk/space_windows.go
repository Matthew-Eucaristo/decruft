//go:build windows

package disk

import "golang.org/x/sys/windows"

// Space is a volume's size and the space available to the user.
func Space(path string) (total, free int64) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return -1, -1
	}
	var avail, tot, f uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &tot, &f); err != nil {
		return -1, -1
	}
	return int64(tot), int64(avail)
}
