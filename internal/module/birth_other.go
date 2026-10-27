//go:build !darwin

package module

import "time"

// birth is not recorded portably elsewhere.
func birth(string) time.Time { return time.Time{} }
