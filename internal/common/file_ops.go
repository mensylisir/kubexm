package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyFile copies a file from src to dst, preserving permissions.
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file '%s': %w", src, err)
	}
	defer sourceFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file '%s': %w", src, err)
	}

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", dstDir, err)
	}

	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", dst, err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("failed to copy content from '%s' to '%s': %w", src, dst, err)
	}

	err = os.Chmod(dst, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to set permissions on destination file '%s': %w", dst, err)
	}

	return nil
}

// IsDirExist returns true if the given path is an existing directory.
func IsDirExist(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFileExist returns true if the given path is an existing file (not a directory).
func IsFileExist(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// FileExists returns true if the given file exists within the specified directory.
func FileExists(dir, file string) bool {
	if file == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}
