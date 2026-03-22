package common

import "os"

const (
	DefaultDirPermission        = os.FileMode(0755)
	DefaultFilePermission       = os.FileMode(0644)
	DefaultKubeconfigPermission = os.FileMode(0600)
	DefaultPrivateKeyPermission = os.FileMode(0600)
)
