package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyDirRecursive copies a directory tree from src to dest.
func CopyDirRecursive(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}
		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer srcFile.Close()

		destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return nil
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return nil
	})
}

// GetFileSize returns the size of a file.
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// DirSize calculates the total size of all files in a directory.
func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// FormatSize converts bytes to a human-readable string.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
