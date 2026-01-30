package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

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

func IsDirExist(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func IsFileExist(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func FileExists(dir, file string) bool {
	if file == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}

func UploadFile(ctx runtime.ExecutionContext, conn connector.Connector, localFile string, destFile string, permission string, sudo bool) error {
	if _, err := os.Stat(localFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("local source file %s does not exist", localFile)
		}
		return fmt.Errorf("failed to stat local source file %s: %w", localFile, err)
	}

	fileName := filepath.Base(localFile)
	uploadDir := filepath.Join(ctx.GetUploadDir())
	remoteTempFile := filepath.Join(uploadDir, fileName)
	runner := ctx.GetRunner()

	if err := runner.Mkdirp(ctx.GoContext(), conn, uploadDir, permission, false); err != nil {
		return fmt.Errorf("failed to ensure remote upload directory %s exists: %w", uploadDir, err)
	}

	if err := runner.Upload(ctx.GoContext(), conn, localFile, remoteTempFile, false); err != nil {
		return fmt.Errorf("failed to upload file from %s to %s: %w", localFile, remoteTempFile, err)
	}
	defer runner.Remove(ctx.GoContext(), conn, remoteTempFile, false, false)

	remoteFinalDir := filepath.Dir(destFile)
	if remoteFinalDir != "." && remoteFinalDir != "/" {
		if err := runner.Mkdirp(ctx.GoContext(), conn, remoteFinalDir, "0755", sudo); err != nil {
			return fmt.Errorf("failed to create remote destination directory %s: %w", remoteFinalDir, err)
		}
	}

	if err := runner.Move(ctx.GoContext(), conn, remoteTempFile, destFile, sudo); err != nil {
		return fmt.Errorf("failed to move remote file from %s to %s: %w", remoteTempFile, destFile, err)
	}

	if permission != "" {
		if err := runner.Chmod(ctx.GoContext(), conn, destFile, permission, sudo); err != nil {
			return fmt.Errorf("failed to set permissions '%s' on final file %s: %w", permission, destFile, err)
		}
	}

	return nil
}

func WriteContentToRemote(ctx runtime.ExecutionContext, conn connector.Connector, content string, destFile string, permission string, sudo bool) error {
	fileName := filepath.Base(destFile)
	localTempFile := filepath.Join(ctx.GetHostWorkDir(), fileName)
	uploadDir := ctx.GetUploadDir()
	remoteTempFile := filepath.Join(uploadDir, fileName)
	runner := ctx.GetRunner()

	permVal, err := strconv.ParseUint(permission, 0, 32)
	if err != nil {
		return fmt.Errorf("invalid permission string '%s': %w", permission, err)
	}
	fileMode := os.FileMode(permVal)
	defer os.Remove(localTempFile)
	if err := os.WriteFile(localTempFile, []byte(content), fileMode); err != nil {
		return fmt.Errorf("failed to write content to local temp file %s: %w", localTempFile, err)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, uploadDir, permission, false); err != nil {
		return fmt.Errorf("failed to ensure remote upload directory %s exists: %w", uploadDir, err)
	}

	if err := runner.Upload(ctx.GoContext(), conn, localTempFile, remoteTempFile, false); err != nil {
		return fmt.Errorf("failed to upload file from %s to %s: %w", localTempFile, remoteTempFile, err)
	}
	defer runner.Remove(ctx.GoContext(), conn, remoteTempFile, false, false)

	if err := runner.Move(ctx.GoContext(), conn, remoteTempFile, destFile, sudo); err != nil {
		return fmt.Errorf("failed to move remote file from %s to %s: %w", remoteTempFile, destFile, err)
	}

	if err := runner.Chmod(ctx.GoContext(), conn, destFile, permission, sudo); err != nil {
		return fmt.Errorf("failed to set permissions '%s' on final file %s: %w", permission, destFile, err)
	}

	return nil
}
