//go:build !darwin && !linux

package module

// Windows and others: only the cross-platform modules so far.
func platform() []Module { return nil }
