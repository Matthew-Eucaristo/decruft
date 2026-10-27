//go:build !darwin && !linux && !(windows && (amd64 || arm64))

package trash

import "errors"

func moveToTrash(string) error {
	return errors.New("moving to the trash is not supported on this OS; use --permanent")
}

// Dir is not a single folder here, so the trash is not measured.
func Dir(string) string { return "" }
