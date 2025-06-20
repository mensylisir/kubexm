package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/spec"
)

// ExtractEtcdStepSpec defines the parameters for extracting an etcd archive
// and installing the binaries.
type ExtractEtcdStepSpec struct {
	spec.StepMeta // Embed common meta fields

	ArchivePathKey   string `json:"archivePathKey,omitempty"`   // Key to retrieve the downloaded archive path from shared data (e.g., EtcdDownloadedArchiveKey)
	Version          string `json:"version,omitempty"`          // Etcd version, e.g., "v3.5.9". Used for determining path inside tarball.
	Arch             string `json:"arch,omitempty"`             // Architecture, e.g., "amd64". Used for determining path inside tarball.
	TargetInstallDir string `json:"targetInstallDir,omitempty"` // Directory to install etcd/etcdctl binaries, e.g., "/usr/local/bin"
	// ExtractedBinDirKey string `json:"extractedBinDirKey,omitempty"` // Optional: Key to store the path of the temp extraction dir that contains the binaries
}

// NewExtractEtcdStepSpec creates a new ExtractEtcdStepSpec.
func NewExtractEtcdStepSpec(stepName, archivePathKey, version, arch, targetInstallDir string) *ExtractEtcdStepSpec {
	if stepName == "" {
		stepName = "Extract and Install Etcd Binaries"
	}

	inKey := archivePathKey
	if inKey == "" {
		inKey = EtcdDownloadedArchiveKey // Default key from download step
	}

	normalizedVersion := version
	if !strings.HasPrefix(normalizedVersion, "v") && normalizedVersion != "" {
		normalizedVersion = "v" + normalizedVersion
	}
	// Version and Arch are important for constructing the path within the tarball,
	// e.g., etcd-v3.5.9-linux-amd64/etcd
	// If version or arch is empty, the executor might need to determine them or raise an error.

	installDir := targetInstallDir
	if installDir == "" {
		installDir = "/usr/local/bin" // Default install directory
	}

	return &ExtractEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Extracts etcd binaries from archive (input key: %s, version: %s, arch: %s) and installs them to %s.", inKey, normalizedVersion, arch, installDir),
		},
		ArchivePathKey:   inKey,
		Version:          normalizedVersion,
		Arch:             arch,
		TargetInstallDir: installDir,
	}
}

// GetName returns the step's name.
func (s *ExtractEtcdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *ExtractEtcdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec validates and returns the spec.
func (s *ExtractEtcdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ExtractEtcdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }
