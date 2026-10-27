// Package trash moves folders to the operating system's trash, so they can
// be put back until the user empties it.
package trash

// Name is what the OS calls its trash.
var Name = "Trash"

// Move sends path to the trash.
func Move(path string) error { return moveToTrash(path) }
