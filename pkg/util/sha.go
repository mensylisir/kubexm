package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/pkg/errors"
)

func GetLocalFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func GetFileSha256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func CheckRemoteFileIntegrity(ctx runtime.ExecutionContext, localPath, remotePath string, sudo bool) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, errors.Wrap(err, "failed to get connector for file integrity check")
	}
	logger := ctx.GetLogger()

	localHash, err := GetLocalFileSHA256(localPath)
	if err != nil {
		return false, errors.Wrapf(err, "failed to calculate sha256 for local file %s", localPath)
	}

	if !sudo {
		existsCmd := fmt.Sprintf("test -f %s", remotePath)
		if _, err := runner.Run(ctx.GoContext(), conn, existsCmd, false); err != nil {
			logger.Debugf("Remote file %s does not exist. Action is required.", remotePath)
			return false, nil
		}

		remoteHashCmd := fmt.Sprintf("sha256sum %s | cut -d' ' -f1", remotePath)
		remoteHashOutput, err := runner.Run(ctx.GoContext(), conn, remoteHashCmd, false)
		if err != nil {
			logger.Warnf("Failed to get remote hash for %s (no sudo): %v. Assuming re-upload is needed.", remotePath, err)
			return false, nil
		}
		remoteHash := strings.TrimSpace(remoteHashOutput)
		if localHash == remoteHash {
			logger.Debugf("Remote file %s exists and content is up-to-date.", remotePath)
			return true, nil
		}
		logger.Infof("Remote file %s exists but content is outdated. Action is required.", remotePath)
		return false, nil
	}

	uniqueTmpDir := filepath.Join("/tmp", fmt.Sprintf("kubexm-checksum-%d-%s", time.Now().UnixNano(), localHash[:8]))

	defer func() {
		cleanupCmd := fmt.Sprintf("rm -rf %s", uniqueTmpDir)
		if _, cleanupErr := runner.Run(ctx.GoContext(), conn, cleanupCmd, sudo); cleanupErr != nil {
			logger.Warnf("Failed to clean up temporary directory %s on remote host: %v", uniqueTmpDir, cleanupErr)
		}
	}()

	mkdirCmd := fmt.Sprintf("mkdir -p %s", uniqueTmpDir)
	if _, err := runner.Run(ctx.GoContext(), conn, mkdirCmd, sudo); err != nil {
		return false, errors.Wrapf(err, "sudo failed to create temp dir %s", uniqueTmpDir)
	}

	tmpFilePath := filepath.Join(uniqueTmpDir, filepath.Base(remotePath))
	cpCmd := fmt.Sprintf("cp %s %s", remotePath, tmpFilePath)
	if _, err := runner.Run(ctx.GoContext(), conn, cpCmd, sudo); err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			logger.Debugf("Remote file %s does not exist. Action is required.", remotePath)
			return false, nil
		}
		return false, errors.Wrapf(err, "sudo failed to copy file to temp dir")
	}

	currentUser := ctx.GetHost().GetUser()
	chownCmd := fmt.Sprintf("chown %s %s", currentUser, tmpFilePath)
	if _, err := runner.Run(ctx.GoContext(), conn, chownCmd, sudo); err != nil {
		return false, errors.Wrapf(err, "sudo failed to change ownership of temp file")
	}

	remoteHashCmd := fmt.Sprintf("sha256sum %s | cut -d' ' -f1", tmpFilePath)
	remoteHashOutput, err := runner.Run(ctx.GoContext(), conn, remoteHashCmd, false)
	if err != nil {
		return false, errors.Wrapf(err, "failed to calculate hash of temp file")
	}
	remoteHash := strings.TrimSpace(remoteHashOutput)

	if localHash == remoteHash {
		logger.Debugf("Remote file %s exists and content is up-to-date.", remotePath)
		return true, nil
	}

	logger.Infof("Remote file %s exists but content is outdated. Action is required.", remotePath)
	return false, nil
}

func VerifyLocalFileChecksum(filePath string, expectedChecksum string) (bool, error) {
	if expectedChecksum == "" || strings.HasPrefix(expectedChecksum, "dummy-") {
		logger.Get().Debug("Checksum for %s is empty or a dummy value, skipping verification.", filePath)
		return true, nil
	}

	hash, err := GetLocalFileSHA256(filePath)
	if err != nil {
		return false, err
	}

	return hash == expectedChecksum, nil
}