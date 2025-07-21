//go:build windows

package helpers

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"os"
)

func GetFileMetadata(info os.FileInfo, logger *logger.Logger) *connector.FileTransferOptions {
	opts := &connector.FileTransferOptions{}
	opts.Permissions = fmt.Sprintf("%04o", info.Mode().Perm())
	opts.Owner = ""
	opts.Group = ""

	logger.Info("Info: running on Windows, owner/group information will not be preserved.")

	return opts
}
