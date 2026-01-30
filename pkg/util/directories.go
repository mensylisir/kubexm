package util

import (
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/pkg/errors"
)

func GenerateWorkDir() (string, error) {
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", errors.Wrap(err, "get current dir failed")
	}

	rootPath := filepath.Join(currentDir, common.KUBEXM)
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
