package kube_components

import (
	"fmt"
	"path/filepath"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	commonstep "github.com/kubexms/kubexms/pkg/step/common"
	"github.com/kubexms/kubexms/pkg/step/component_downloads"
)

// NewFetchContainerdTask creates a task to download, extract, and install containerd and its components.
func NewFetchContainerdTask(
	cfg *config.Cluster, // For consistency
	version, arch, zone, appFsBaseDir string,
) *spec.TaskSpec {

	componentName := "containerd"
	downloadDir := filepath.Join(appFsBaseDir, componentName, version, arch)
	extractionDir := filepath.Join(appFsBaseDir, "_extracts", componentName, version, arch)

	// Using a specific SharedData key for containerd's extracted directory,
	// as it contains multiple binaries in a 'bin' subdirectory.
	containerdExtractedDirOutputKey := "ContainerdExtractedArchiveDir" // Custom key for this task's context

	downloadStep := &component_downloads.DownloadContainerdStepSpec{
		Version:     version,
		Arch:        arch,
		Zone:        zone,
		DownloadDir: downloadDir,
		// OutputFilePathKey defaults to component_downloads.ContainerdDownloadedPathKey
	}

	extractStep := &commonstep.ExtractArchiveStepSpec{
		ArchivePathSharedDataKey: component_downloads.ContainerdDownloadedPathKey, // From download step
		ExtractionDir:            extractionDir,
		ExtractedDirSharedDataKey: containerdExtractedDirOutputKey, // Specific key for containerd's extracted root
	}

	// Binaries to install from the 'bin' subdirectory of the extracted archive
	binariesToInstall := []struct {
		SourceFileName string // Relative to the 'bin' directory is implied by InstallBinaryStep logic if SourcePath points to root of extracted archive
		TargetFileName string // Name in /usr/local/bin or /usr/local/sbin
		TargetDir      string
	}{
		{SourceFileName: "bin/containerd", TargetFileName: "containerd", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/ctr", TargetFileName: "ctr", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/containerd-shim", TargetFileName: "containerd-shim", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/containerd-shim-runc-v1", TargetFileName: "containerd-shim-runc-v1", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/containerd-shim-runc-v2", TargetFileName: "containerd-shim-runc-v2", TargetDir: "/usr/local/bin"},
		{SourceFileName: "bin/runc", TargetFileName: "runc", TargetDir: "/usr/local/sbin"}, // runc often goes to sbin
	}

	installSteps := []spec.StepSpec{}
	for _, bin := range binariesToInstall {
		installSteps = append(installSteps, &commonstep.InstallBinaryStepSpec{
			SourcePathSharedDataKey: containerdExtractedDirOutputKey, // Root of extracted archive
			SourceIsDirectory:       true,
			SourceFileName:          bin.SourceFileName, // e.g., "bin/containerd"
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
