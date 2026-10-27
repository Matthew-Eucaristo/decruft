//go:build !unix && !windows

package disk

func Space(string) (total, free int64) { return -1, -1 }
