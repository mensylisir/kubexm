package helpers

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

func IsInStringSlice(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func GenerateWorkDir(name string) (string, error) {
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", errors.Wrap(err, "get current dir failed")
	}

	rootPath := filepath.Join(currentDir, common.KUBEXM, name)
	if err := CreateDir(rootPath); err != nil {
		return "", errors.Wrap(err, "create work dir failed")
	}
	return rootPath, nil
}

func GenerateHostWorkDir(clusterName string, workdir string, hostName string) (string, error) {
	hostWorkDir := filepath.Join(workdir, clusterName, hostName)
	if err := CreateDir(hostWorkDir); err != nil {
		return "", errors.Wrap(err, "create hostwork dir failed")
	}

	logDir := filepath.Join(workdir, clusterName, "logs")
	if err := CreateDir(logDir); err != nil {
		return "", errors.Wrap(err, "create log dir failed")
	}

	return hostWorkDir, nil
}

func CreateDir(path string) error {
	fileInfo, err := os.Stat(path)
	if err == nil {
		if fileInfo.IsDir() {
			return nil // Already exists as a directory, success
		}
		return fmt.Errorf("path %s exists but is not a directory", path)
	}

	if os.IsNotExist(err) {
		if mkdirErr := os.MkdirAll(path, 0755); mkdirErr != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, mkdirErr)
		}
		return nil
	}

	return fmt.Errorf("failed to stat path %s: %w", path, err)
}
