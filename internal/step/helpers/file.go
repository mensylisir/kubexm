package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// CopyFile delegates to common.CopyFile for local file copy.
func CopyFile(src, dst string) error {
	return common.CopyFile(src, dst)
}

// IsDirExist delegates to common.IsDirExist.
func IsDirExist(path string) bool {
	return common.IsDirExist(path)
}

// IsFileExist delegates to common.IsFileExist.
func IsFileExist(path string) bool {
	return common.IsFileExist(path)
}

// FileExists delegates to common.FileExists.
func FileExists(dir, file string) bool {
	return common.FileExists(dir, file)
}

func UploadFile(ctx runtime.ExecutionContext, conn runner.Connector, localFile string, destFile string, permission string, sudo bool) error {
	if _, err := os.Stat(localFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("local source file %s does not exist", localFile)
		}
		return fmt.Errorf("failed to stat local source file %s: %w", localFile, err)
	}

	fileName := filepath.Base(localFile)
	uploadDir := filepath.Join(ctx.GetUploadDir())
	remoteTempFile := filepath.Join(uploadDir, fileName)
	runnerSvc := ctx.GetRunner()

	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, uploadDir, permission, false); err != nil {
		return fmt.Errorf("failed to ensure remote upload directory %s exists: %w", uploadDir, err)
	}

	if err := runnerSvc.Upload(ctx.GoContext(), conn, localFile, remoteTempFile, false); err != nil {
		return fmt.Errorf("failed to upload file from %s to %s: %w", localFile, remoteTempFile, err)
	}
	defer runnerSvc.Remove(ctx.GoContext(), conn, remoteTempFile, false, false)

	remoteFinalDir := filepath.Dir(destFile)
	if remoteFinalDir != "." && remoteFinalDir != "/" {
		if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, remoteFinalDir, "0755", sudo); err != nil {
			return fmt.Errorf("failed to create remote destination directory %s: %w", remoteFinalDir, err)
		}
	}

	if err := runnerSvc.Move(ctx.GoContext(), conn, remoteTempFile, destFile, sudo); err != nil {
		return fmt.Errorf("failed to move remote file from %s to %s: %w", remoteTempFile, destFile, err)
	}

	if permission != "" {
		if err := runnerSvc.Chmod(ctx.GoContext(), conn, destFile, permission, sudo); err != nil {
			return fmt.Errorf("failed to set permissions '%s' on final file %s: %w", permission, destFile, err)
		}
	}

	return nil
}

func WriteContentToRemote(ctx runtime.ExecutionContext, conn runner.Connector, content string, destFile string, permission string, sudo bool) error {
	fileName := filepath.Base(destFile)
	localTempFile := filepath.Join(ctx.GetHostWorkDir(), fileName)
	uploadDir := ctx.GetUploadDir()
	remoteTempFile := filepath.Join(uploadDir, fileName)
	runnerSvc := ctx.GetRunner()

	permVal, err := strconv.ParseUint(permission, 0, 32)
	if err != nil {
		return fmt.Errorf("invalid permission string '%s': %w", permission, err)
	}
	fileMode := os.FileMode(permVal)
	defer os.Remove(localTempFile)
	if err := os.WriteFile(localTempFile, []byte(content), fileMode); err != nil {
		return fmt.Errorf("failed to write content to local temp file %s: %w", localTempFile, err)
	}

	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, uploadDir, permission, false); err != nil {
		return fmt.Errorf("failed to ensure remote upload directory %s exists: %w", uploadDir, err)
	}

	if err := runnerSvc.Upload(ctx.GoContext(), conn, localTempFile, remoteTempFile, false); err != nil {
		return fmt.Errorf("failed to upload file from %s to %s: %w", localTempFile, remoteTempFile, err)
	}
	defer runnerSvc.Remove(ctx.GoContext(), conn, remoteTempFile, false, false)

	if err := runnerSvc.Move(ctx.GoContext(), conn, remoteTempFile, destFile, sudo); err != nil {
		return fmt.Errorf("failed to move remote file from %s to %s: %w", remoteTempFile, destFile, err)
	}

	if err := runnerSvc.Chmod(ctx.GoContext(), conn, destFile, permission, sudo); err != nil {
		return fmt.Errorf("failed to set permissions '%s' on final file %s: %w", permission, destFile, err)
		}

	return nil
}
