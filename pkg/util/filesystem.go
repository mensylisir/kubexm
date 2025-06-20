package util

import (
	"os"
)

// CreateDir creates a directory if it does not exist.
func CreateDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755) // 0755 gives rwx for owner, rx for group/other
	}
	return nil
}
