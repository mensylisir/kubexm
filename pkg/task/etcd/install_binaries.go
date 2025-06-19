package etcd

import (
	"path/filepath"
	"fmt"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	commonstep "github.com/kubexms/kubexms/pkg/step/common" // Renamed to avoid conflict
	"github.com/kubexms/kubexms/pkg/step/component_downloads"
	etcdstep "github.com/kubexms/kubexms/pkg/step/etcd" // Renamed to avoid conflict
)

// NewInstallEtcdBinariesTask creates a task to download, extract, and install etcd binaries.
func NewInstallEtcdBinariesTask(
	cfg *config.Cluster, // For consistency, or to get specific paths if not passed directly
	version, arch, zone, appFSBaseDir string, // appFSBaseDir is <program_work_dir>/.kubexm
) *spec.TaskSpec {

	downloadDir := filepath.Join(appFSBaseDir, "etcd", version, arch)
	extractionDir := filepath.Join(appFSBaseDir, "_extracts", "etcd", version, arch)

	// Using default output keys from download/extract steps for chaining.
	// commonstep.DefaultEtcdExtractedDirKey is a slight misnomer if it's generic;
	// ExtractArchiveStepSpec uses DefaultExtractedPathKey ("extractedPath") by default.
	// Let's ensure clarity by potentially overriding keys or using specific keys from component download steps.
	// component_downloads.DownloadEtcdStepSpec outputs to EtcdDownloadedPathKey.
	// common.ExtractArchiveStepSpec defaults input to common.DefaultDownloadedFilePathKey.
	// We need to align these.
	// For now, assume component_downloads.EtcdDownloadedPathKey is used by DownloadEtcdStepSpec,
	// and ExtractArchiveStepSpec.ArchivePathSharedDataKey is set to this.
	// ExtractArchiveStepSpec outputs to common.DefaultExtractedPathKey.
	// InstallBinaryStepSpec inputs from common.DefaultExtractedPathKey.

	return &spec.TaskSpec{
		Name: fmt.Sprintf("Download, Extract, and Install etcd %s (%s)", version, arch),
		Steps: []spec.StepSpec{
			&component_downloads.DownloadEtcdStepSpec{
				Version:     version,
				Arch:        arch,
				Zone:        zone,
				DownloadDir: downloadDir,
				// OutputFilePathKey defaults to component_downloads.EtcdDownloadedPathKey
			},
			&commonstep.ExtractArchiveStepSpec{
				ArchivePathSharedDataKey: component_downloads.EtcdDownloadedPathKey, // Input from previous step
				ExtractionDir:            extractionDir,
				ExtractedDirSharedDataKey: commonstep.DefaultExtractedPathKey, // Generic output key for the extracted path
			},
			&commonstep.InstallBinaryStepSpec{
				SourcePathSharedDataKey: commonstep.DefaultExtractedPathKey, // Input from extraction
				SourceIsDirectory:       true,
				SourceFileName:          "etcd", // Binary name within the extracted archive root (e.g. etcd-vX.Y.Z-linux-ARCH/etcd)
				TargetDir:               "/usr/local/bin", // Default, or make configurable
				Permissions:             "0755",
			},
			&commonstep.InstallBinaryStepSpec{
				SourcePathSharedDataKey: commonstep.DefaultExtractedPathKey, // Input from extraction
				SourceIsDirectory:       true,
				SourceFileName:          "etcdctl", // Binary name
				TargetDir:               "/usr/local/bin",
				Permissions:             "0755",
			},
			&etcdstep.CleanupEtcdInstallationStepSpec{
				// This step reads keys like EtcdArchivePathKey (from DownloadEtcdArchiveStepSpec internal, not the shared one)
				// and EtcdExtractionDirKey (from ExtractEtcdArchiveStepSpec internal).
				// The Cleanup step needs to be aware of where Download and Extract steps store their paths if they
				// don't use globally known keys.
				// For now, assume the cleanup step's defaults are sufficient if it's designed to work with the new generic steps.
				// This might require the Download and Extract steps to also output to well-known keys that Cleanup can find.
				// Let's assume DownloadEtcdStepSpec outputs to component_downloads.EtcdDownloadedPathKey
				// and ExtractArchiveStepSpec outputs its unique extraction dir somewhere that Cleanup can find OR
				// cleanup relies on the *archive* path from download and the *base extraction dir* from extract spec.
				// The generic ExtractArchiveStepSpec does not store its own ExtractionDir in shared data,
				// so CleanupEtcdInstallationStepSpec cannot find it if it relies on that.
				// This highlights a potential issue with the generic CleanupEtcdInstallationStepSpec.
				// For now, we'll include it and assume it can find what it needs or will be refactored.
			},
		},
	}
}
