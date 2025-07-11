package common

import "os"

// Default File Permissions used across the application.
const (
	DefaultDirPermission        = os.FileMode(0755) // Default permission for directories created by Kubexm.
	DefaultFilePermission       = os.FileMode(0644) // Default permission for files created by Kubexm.
	DefaultKubeconfigPermission = os.FileMode(0600) // Restricted permission for Kubeconfig files.
	DefaultPrivateKeyPermission = os.FileMode(0600) // Restricted permission for private key files.
)
