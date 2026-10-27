//go:build !unix && !windows

package module

func running(string) bool { return false }
