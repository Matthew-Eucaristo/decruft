package module

import (
	"time"

	"golang.org/x/sys/unix"
)

// birth is when a directory was created. go clean and friends delete their
// cache folder and the next build recreates it, so a recent birth time says
// the cache came back.
func birth(path string) time.Time {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return time.Time{}
	}
	return time.Unix(st.Btim.Unix())
}
