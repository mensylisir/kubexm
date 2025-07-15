package helpers

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util"
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

func GenerateWorkDir() (string, error) {
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", errors.Wrap(err, "get current dir failed")
	}

	rootPath := filepath.Join(currentDir, common.KUBEXM)
	if err := util.CreateDir(rootPath); err != nil {
		return "", errors.Wrap(err, "create work dir failed")
	}
	return rootPath, nil
}

func GenerateHostWorkDir(clusterName string, workdir string, hostName string) (string, error) {
	hostWorkDir := filepath.Join(workdir, clusterName, hostName)
	if err := util.CreateDir(hostWorkDir); err != nil {
		return "", errors.Wrap(err, "create hostwork dir failed")
	}

	logDir := filepath.Join(workdir, clusterName, "logs")
	if err := util.CreateDir(logDir); err != nil {
		return "", errors.Wrap(err, "create log dir failed")
	}

	return hostWorkDir, nil
}
