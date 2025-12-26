package util

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/runtime"
)

// GetRemoteFileChecksum calculates the SHA256 checksum of a file on a remote host.
// It returns the checksum as a hex-encoded string. If the file does not exist or another
// error occurs, it returns an error.
func GetRemoteFileChecksum(ctx runtime.ExecutionContext, remotePath string, sudo bool) (string, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	checksumCmd := fmt.Sprintf("sha256sum %s | awk '{print $1}'", remotePath)
	output, runErr := runner.Run(ctx.GoContext(), conn, checksumCmd, sudo)
	if runErr != nil {
		return "", fmt.Errorf("failed to get remote file checksum for %s: %w", remotePath, runErr)
	}

	checksum := strings.TrimSpace(string(output))
	if checksum == "" {
		return "", fmt.Errorf("got empty checksum for remote file %s", remotePath)
	}

	return checksum, nil
}