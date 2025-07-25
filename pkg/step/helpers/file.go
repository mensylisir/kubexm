package helpers

import (
	"os"
	"path/filepath"
)

func FileExists(dir, file string) bool {
	if file == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}
