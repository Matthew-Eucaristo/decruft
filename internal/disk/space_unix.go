//go:build unix

package disk

import "golang.org/x/sys/unix"

// Space is a volume's size and the space available to the user.
func Space(path string) (total, free int64) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return -1, -1
	}
	return int64(st.Blocks) * int64(st.Bsize), int64(st.Bavail) * int64(st.Bsize)
}
