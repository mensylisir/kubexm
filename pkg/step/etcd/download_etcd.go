package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/spec"
)

const (
	// DefaultEtcdInstallURLBase is the default base URL for etcd downloads.
	DefaultEtcdInstallURLBase = "https://github.com/etcd-io/etcd/releases/download"
	// DefaultEtcdVersion is the version used if not specified.
	DefaultEtcdVersion = "v3.5.9" // Example default
	// EtcdDownloadedArchiveKey is the key used to store the downloaded etcd archive path in shared data.
	EtcdDownloadedArchiveKey = "EtcdDownloadedArchivePath"
)

// DownloadEtcdStepSpec defines the parameters for downloading an etcd release archive.
type DownloadEtcdStepSpec struct {
	spec.StepMeta // Embed common meta fields like Name, Description

	Version        string `json:"version,omitempty"`        // e.g., "v3.5.9"
	Arch           string `json:"arch,omitempty"`           // e.g., "amd64", "arm64"
	InstallURLBase string `json:"installURLBase,omitempty"` // Base URL for downloads
	DownloadDir    string `json:"downloadDir,omitempty"`    // Directory on the host to download the archive to
	Checksum       string `json:"checksum,omitempty"`       // Expected checksum of the archive (e.g., "sha256:...")
	OutputKey      string `json:"outputKey,omitempty"`      // Key to store the downloaded file path in shared data
}

// NewDownloadEtcdStepSpec creates a new DownloadEtcdStepSpec.
func NewDownloadEtcdStepSpec(stepName, version, arch, installURLBase, downloadDir, checksum, outputKey string) *DownloadEtcdStepSpec {
	if stepName == "" {
		stepName = "Download Etcd Archive"
	}
	normalizedVersion := version
	if normalizedVersion == "" {
		normalizedVersion = DefaultEtcdVersion
	}
	if !strings.HasPrefix(normalizedVersion, "v") {
		normalizedVersion = "v" + normalizedVersion
	}

	urlBase := installURLBase
	if urlBase == "" {
		urlBase = DefaultEtcdInstallURLBase
	}

	// Arch default is handled by executor/runner based on host info if empty

	outKey := outputKey
	if outKey == "" {
		outKey = EtcdDownloadedArchiveKey
	}

	desc := fmt.Sprintf("Downloads etcd version %s for %s architecture.", normalizedVersion, arch)
	if checksum != "" {
		desc += fmt.Sprintf(" Verifies checksum %s.", checksum)
	}


	return &DownloadEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: desc,
		},
		Version:        normalizedVersion,
		Arch:           arch, // Let executor determine if empty
		InstallURLBase: urlBase,
		DownloadDir:    downloadDir, // If empty, executor might use a default work dir
		Checksum:       checksum,
		OutputKey:      outKey,
	}
}

// GetName returns the step's name.
func (s *DownloadEtcdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *DownloadEtcdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec validates and returns the spec.
// This is a placeholder for potential future validation logic.
func (s *DownloadEtcdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DownloadEtcdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }
