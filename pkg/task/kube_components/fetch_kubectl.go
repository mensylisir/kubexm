package kube_components

import (
	"fmt"
	"path/filepath"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	commonstep "github.com/kubexms/kubexms/pkg/step/common"
	"github.com/kubexms/kubexms/pkg/step/component_downloads"
)

// NewFetchKubectlTask creates a task to download and install kubectl.
func NewFetchKubectlTask(
	cfg *config.Cluster,
	version, arch, zone, appFsBaseDir string,
) *spec.TaskSpec {

	componentName := "kubectl"
	downloadDir := filepath.Join(appFsBaseDir, componentName, version, arch)

	downloadStep := &component_downloads.DownloadKubectlStepSpec{
		Version:     version,
		Arch:        arch,
		Zone:        zone,
		DownloadDir: downloadDir,
		// OutputFilePathKey: component_downloads.KubectlDownloadedPathKey (default)
	}

	installStep := &commonstep.InstallBinaryStepSpec{
		SourcePathSharedDataKey: component_downloads.KubectlDownloadedPathKey,
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
