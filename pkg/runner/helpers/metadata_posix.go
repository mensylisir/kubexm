//go:build !windows

package helpers

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func GetFileMetadata(info os.FileInfo, logger *logger.Logger) *connector.FileTransferOptions {
	opts := &connector.FileTransferOptions{}
	opts.Permissions = fmt.Sprintf("%04o", info.Mode().Perm())
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uidStr := strconv.Itoa(int(stat.Uid))
		u, err := user.LookupId(uidStr)
		if err == nil {
			opts.Owner = u.Username
		} else {
			logger.Infof("Warning: could not look up user for uid %s on source system, defaulting to 'root'. Error: %v", uidStr, err)
			opts.Owner = "root"
		}

		gidStr := strconv.Itoa(int(stat.Gid))
		g, err := user.LookupGroupId(gidStr)
		if err == nil {
			opts.Group = g.Name
		} else {
			logger.Infof("Warning: could not look up group for gid %s on source system, defaulting to 'root'. Error: %v", gidStr, err)
			opts.Group = "root"
		}
	} else {
		logger.Info("Warning: could not determine owner/group for file; info.Sys() was not a syscall.Stat_t.")
	}

	return opts
}
