package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func CreateDir(path string) error {
	fileInfo, err := os.Stat(path)
	if err == nil {
		if fileInfo.IsDir() {
			return nil // Already exists as a directory, success
		}
		return fmt.Errorf("path %s exists but is not a directory", path) // Exists but is a file
	}

	if os.IsNotExist(err) {
		if mkdirErr := os.MkdirAll(path, 0755); mkdirErr != nil { // 0755 gives rwx for owner, rx for group/other
			return fmt.Errorf("failed to create directory %s: %w", path, mkdirErr)
		}
		return nil
	}

	return fmt.Errorf("failed to stat path %s: %w", path, err)
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil // Path does not exist
	}
	return false, fmt.Errorf("error checking if path '%s' exists: %w", path, err)
}

func IsFile(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err == nil {
		return !fileInfo.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("error checking file status for '%s': %w", path, err)
}

func ReadFileToString(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return string(content), nil
}

func WriteStringToFile(filePath string, content string) error {
	return WriteBytesToFile(filePath, []byte(content))
}

func WriteBytesToFile(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := CreateDir(dir); err != nil {
		return fmt.Errorf("failed to ensure directory exists for file %s: %w", filePath, err)
	}

	err := os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filePath, err)
	}
	return nil
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer sourceFile.Close()

	if err := CreateDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create destination directory for %s: %w", dst, err)
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy content from %s to %s: %w", src, dst, err)
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file %s for permissions: %w", src, err)
	}
	return os.Chmod(dst, sourceInfo.Mode())
}

func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %s is not a directory", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dst, err)

	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func Move(src, dst string) error {
	if err := CreateDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create destination directory for %s: %w", dst, err)
	}

	err := os.Rename(src, dst)
	if err != nil {
		return fmt.Errorf("failed to move %s to %s: %w", src, dst, err)
	}
	return nil
}

func IsDir(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("error checking directory status for '%s': %w", path, err)
	}
	return fileInfo.IsDir(), nil
}

func GetFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get info for file %s: %w", filePath, err)
	}
	if fileInfo.IsDir() {
		return 0, fmt.Errorf("path %s is a directory, not a file", filePath)
	}
	return fileInfo.Size(), nil
}

func ListFiles(dirPath string, pattern string) ([]string, error) {
	isDir, err := IsDir(dirPath)
	if err != nil {
		return nil, err
	}
	if !isDir {
		return nil, fmt.Errorf("path %s is not a directory", dirPath)
	}

	globPattern := filepath.Join(dirPath, pattern)
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob pattern %s: %w", globPattern, err)
	}
	return matches, nil
}
