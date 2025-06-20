package kube_components

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/component_downloads"
)

// NewFetchKubeadmTask creates a task to download and install kubeadm.
func NewFetchKubeadmTask(
	cfg *config.Cluster, // For consistency, not directly used for path construction here
	version, arch, zone, appFsBaseDir string,
) *spec.TaskSpec {

	componentName := "kubeadm" // Component name for directory structure
	// Construct structured path for downloads
	downloadDir := filepath.Join(appFsBaseDir, componentName, version, arch)

	downloadStep := &component_downloads.DownloadKubeadmStepSpec{
		Version:     version,
		Arch:        arch,
		Zone:        zone,
		DownloadDir: downloadDir, // Use structured path
		// OutputFilePathKey defaults to component_downloads.KubeadmDownloadedPathKey
	}

	installStep := &commonstep.InstallBinaryStepSpec{
		// SourcePathSharedDataKey will use its default (common.DefaultExtractedPathKey or similar)
		// if not overridden. Here, we explicitly link it to the output of the download step.
		SourcePathSharedDataKey: component_downloads.KubeadmDownloadedPathKey,
		SourceIsDirectory:       false,
		TargetDir:               "/usr/local/bin",
		TargetFileName:          componentName,    // Install as "kubeadm"
		Permissions:             "0755",
	}

	return &spec.TaskSpec{
		Name:  fmt.Sprintf("Fetch and Install %s %s (%s)", componentName, version, arch),
		Steps: []spec.StepSpec{downloadStep, installStep},
	}
}
