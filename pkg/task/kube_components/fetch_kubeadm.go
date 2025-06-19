package kube_components

import (
	"fmt"
	"path/filepath"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	commonstep "github.com/kubexms/kubexms/pkg/step/common"
	"github.com/kubexms/kubexms/pkg/step/component_downloads"
)

// NewFetchKubeadmTask creates a task to download and install kubeadm.
func NewFetchKubeadmTask(
	cfg *config.Cluster, // For consistency, though specific params are passed
	version, arch, zone, appFsBaseDir string,
) *spec.TaskSpec {

	componentName := "kubeadm"
	downloadDir := filepath.Join(appFsBaseDir, componentName, version, arch)

	// The DownloadKubeadmStepSpec will use its default OutputFilePathKey,
	// which is component_downloads.KubeadmDownloadedPathKey.
	// InstallBinaryStepSpec needs this key for its SourcePathSharedDataKey.
	downloadStep := &component_downloads.DownloadKubeadmStepSpec{
		Version:     version,
		Arch:        arch,
		Zone:        zone,
		DownloadDir: downloadDir,
		// OutputFilePathKey: component_downloads.KubeadmDownloadedPathKey (default)
	}

	installStep := &commonstep.InstallBinaryStepSpec{
		SourcePathSharedDataKey: component_downloads.KubeadmDownloadedPathKey, // Matches output of downloadStep
		SourceIsDirectory:       false, // Kubeadm is downloaded as a direct binary
		// SourceFileName is not needed if SourceIsDirectory is false and SourcePathSharedDataKey points to the file.
		TargetDir:      "/usr/local/bin", // Common installation directory for binaries
		TargetFileName: componentName,    // Ensure it's named "kubeadm"
		Permissions:    "0755",           // Standard executable permissions
	}

	return &spec.TaskSpec{
		Name:  fmt.Sprintf("Fetch and Install %s %s (%s)", componentName, version, arch),
		Steps: []spec.StepSpec{downloadStep, installStep},
	}
}
