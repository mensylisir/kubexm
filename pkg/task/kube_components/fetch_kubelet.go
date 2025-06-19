package kube_components

import (
	"fmt"
	"path/filepath"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	commonstep "github.com/kubexms/kubexms/pkg/step/common"
	"github.com/kubexms/kubexms/pkg/step/component_downloads"
)

// NewFetchKubeletTask creates a task to download and install kubelet.
func NewFetchKubeletTask(
	cfg *config.Cluster,
	version, arch, zone, appFsBaseDir string,
) *spec.TaskSpec {

	componentName := "kubelet"
	downloadDir := filepath.Join(appFsBaseDir, componentName, version, arch)

	downloadStep := &component_downloads.DownloadKubeletStepSpec{
		Version:     version,
		Arch:        arch,
		Zone:        zone,
		DownloadDir: downloadDir,
		// OutputFilePathKey: component_downloads.KubeletDownloadedPathKey (default)
	}

	installStep := &commonstep.InstallBinaryStepSpec{
		SourcePathSharedDataKey: component_downloads.KubeletDownloadedPathKey,
		SourceIsDirectory:       false,
		TargetDir:               "/usr/local/bin",
		TargetFileName:          componentName,
		Permissions:             "0755",
	}

	return &spec.TaskSpec{
		Name:  fmt.Sprintf("Fetch and Install %s %s (%s)", componentName, version, arch),
		Steps: []spec.StepSpec{downloadStep, installStep},
	}
}
