package kube_components

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/component_downloads"
)

// NewFetchContainerdTask creates a task to download, extract, and install containerd and its components.
func NewFetchContainerdTask(
	cfg *config.Cluster, // For consistency
	version, arch, zone, appFsBaseDir string,
) *spec.TaskSpec {

	componentName := "containerd"
	// Construct structured paths
	downloadDir := filepath.Join(appFsBaseDir, componentName, version, arch)
	extractionDir := filepath.Join(appFsBaseDir, "_extracts", componentName, version, arch)

	// Using a specific SharedData key for containerd's extracted directory,
	// as it contains multiple binaries in a 'bin' subdirectory.
	// This key is defined in the component_downloads.DownloadContainerdStepSpec as ContainerdExtractedDirKey,
	// but for clarity, we use a local const or ensure the ExtractArchiveStep outputs to a known generic key
	// if InstallBinaryStep expects that.
	// The common.ExtractArchiveStepSpec defaults its ExtractedDirSharedDataKey to common.DefaultExtractedPathKey.
	// Let's use that default generic key for broader compatibility.
	extractedDirOutputKey := commonstep.DefaultExtractedPathKey


	downloadStep := &component_downloads.DownloadContainerdStepSpec{
		Version:     version,
		Arch:        arch,
		Zone:        zone,
		DownloadDir: downloadDir, // Use structured path
		// OutputFilePathKey defaults to component_downloads.ContainerdDownloadedPathKey
	}

	extractStep := &commonstep.ExtractArchiveStepSpec{
		ArchivePathSharedDataKey: component_downloads.ContainerdDownloadedPathKey, // From download step
		ExtractionDir:            extractionDir,             // Use structured path
		ExtractedDirSharedDataKey: extractedDirOutputKey,     // Output to this key
	}

	binariesToInstall := []struct {
		SourceFileName string
		TargetFileName string
		TargetDir      string
	}{
		{SourceFileName: "bin/containerd", TargetFileName: "containerd", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/ctr", TargetFileName: "ctr", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/containerd-shim", TargetFileName: "containerd-shim", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/containerd-shim-runc-v1", TargetFileName: "containerd-shim-runc-v1", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/containerd-shim-runc-v2", TargetFileName: "containerd-shim-runc-v2", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/runc", TargetFileName: "runc", TargetDir: "/usr/local/sbin"},
	}

	installSteps := []spec.StepSpec{}
	for _, bin := range binariesToInstall {
		installSteps = append(installSteps, &commonstep.InstallBinaryStepSpec{
			SourcePathSharedDataKey: extractedDirOutputKey, // Use the output key from extractStep
			SourceIsDirectory:       true,
			SourceFileName:          bin.SourceFileName,
			TargetDir:               bin.TargetDir,
			TargetFileName:          bin.TargetFileName,
			Permissions:             "0755",
		})
	}

	taskSteps := []spec.StepSpec{downloadStep, extractStep}
	taskSteps = append(taskSteps, installSteps...)

	return &spec.TaskSpec{
		Name:  fmt.Sprintf("Fetch and Install %s %s (%s)", componentName, version, arch),
		Steps: taskSteps,
	}
}
