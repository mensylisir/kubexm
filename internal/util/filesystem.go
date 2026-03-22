package util

import (
	"fmt"
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
