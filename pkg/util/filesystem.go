package util

import (
	"fmt"
	"os"
)

// CreateDir creates a directory if it does not exist.
// If the path already exists and is a directory, it does nothing and returns nil.
// If the path already exists and is not a directory, it returns an error.
func CreateDir(path string) error {
	fileInfo, err := os.Stat(path)
	if err == nil { // Path exists
		if fileInfo.IsDir() {
			return nil // Already exists as a directory, success
		}
		return fmt.Errorf("path %s exists but is not a directory", path) // Exists but is a file
	}

	// Path does not exist, or Stat failed for other reasons (e.g. permission on parent)
	if os.IsNotExist(err) {
		// Create the directory
		if mkdirErr := os.MkdirAll(path, 0755); mkdirErr != nil { // 0755 gives rwx for owner, rx for group/other
			return fmt.Errorf("failed to create directory %s: %w", path, mkdirErr)
		}
		return nil // Successfully created
	}

	// os.Stat failed for a reason other than NotExist (e.g. permission error on parent path)
	return fmt.Errorf("failed to stat path %s: %w", path, err)
}

// FileExists checks if a file or directory exists at the given path.
// It returns true if the path exists, and false if it does not exist.
// An error is returned if os.Stat fails for reasons other than the path not existing
// (e.g., permission issues on a parent directory).
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil // Path exists
	}
	if os.IsNotExist(err) {
		return false, nil // Path does not exist
	}
	// Another error occurred (e.g., permission denied to access parent dir)
	return false, fmt.Errorf("error checking if path '%s' exists: %w", path, err)
}
