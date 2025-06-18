package etcd

import (
	"fmt"
	"path/filepath"

	"github.com/octo-cli/core/pkg/runtime"
	"github.com/octo-cli/core/pkg/step"
	"github.com/octo-cli/core/pkg/step/spec"
)

const (
	// EtcdArchivePathKey is the key for the downloaded etcd archive path in SharedData.
	EtcdArchivePathKey = "EtcdArchivePath"
	// EtcdVersionKey is the key for the etcd version in SharedData.
	EtcdVersionKey = "EtcdVersion"
)

// DownloadEtcdArchiveStepSpec defines the specification for downloading the etcd archive.
type DownloadEtcdArchiveStepSpec struct {
	Version        string `json:"version"`
	Arch           string `json:"arch"`
	InstallURLBase string `json:"installURLBase"`
	DownloadDir    string `json:"downloadDir"` // e.g., a temporary work directory
}

// GetName returns the name of the step.
func (s *DownloadEtcdArchiveStepSpec) GetName() string {
	return "DownloadEtcdArchive"
}

// ApplyDefaults applies default values to the spec.
func (s *DownloadEtcdArchiveStepSpec) ApplyDefaults(ctx *runtime.Context) error {
	if s.Version == "" {
		s.Version = "v3.5.0" // Example default, should ideally come from config
	}
	if s.Arch == "" {
		s.Arch = ctx.Host.Arch()
		if s.Arch == "x86_64" { // etcd uses amd64 for x86_64
			s.Arch = "amd64"
		}
	}
	if s.InstallURLBase == "" {
		s.InstallURLBase = "https://github.com/etcd-io/etcd/releases/download"
	}
	if s.DownloadDir == "" {
		s.DownloadDir = filepath.Join(ctx.WorkDir, "etcd-download")
	}
	return nil
}

// DownloadEtcdArchiveStepExecutor implements the logic for downloading the etcd archive.
type DownloadEtcdArchiveStepExecutor struct{}

// Check checks if the etcd archive already exists.
func (e *DownloadEtcdArchiveStepExecutor) Check(ctx *runtime.Context, s spec.StepSpec) (bool, error) {
	spec, ok := s.(*DownloadEtcdArchiveStepSpec)
	if !ok {
		return false, fmt.Errorf("invalid spec type %T for DownloadEtcdArchiveStepExecutor", s)
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", spec.Version, spec.Arch)
	archivePath := filepath.Join(spec.DownloadDir, archiveName)

	if _, err := ctx.Host.Runner.Stat(archivePath); err == nil {
		ctx.Logger.Infof("Etcd archive %s already exists, skipping download.", archivePath)
		// Store path in SharedData for subsequent steps if it exists
		ctx.SharedData.Set(EtcdArchivePathKey, archivePath)
		ctx.SharedData.Set(EtcdVersionKey, spec.Version) // also store version
		return true, nil
	}
	return false, nil
}

// Execute downloads the etcd archive.
func (e *DownloadEtcdArchiveStepExecutor) Execute(ctx *runtime.Context, s spec.StepSpec) error {
	spec, ok := s.(*DownloadEtcdArchiveStepSpec)
	if !ok {
		return fmt.Errorf("invalid spec type %T for DownloadEtcdArchiveStepExecutor", s)
	}
	if err := spec.ApplyDefaults(ctx); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", spec.Version, spec.Arch)
	downloadURL := fmt.Sprintf("%s/%s/%s", spec.InstallURLBase, spec.Version, archiveName)
	archivePath := filepath.Join(spec.DownloadDir, archiveName)

	ctx.Logger.Infof("Downloading etcd archive from %s to %s", downloadURL, archivePath)

	if err := ctx.Host.Runner.Mkdirp(spec.DownloadDir); err != nil {
		return fmt.Errorf("failed to create download directory %s: %w", spec.DownloadDir, err)
	}

	if err := ctx.Host.Runner.Download(downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download etcd archive: %w", err)
	}

	ctx.SharedData.Set(EtcdArchivePathKey, archivePath)
	ctx.SharedData.Set(EtcdVersionKey, spec.Version) // also store version
	ctx.Logger.Infof("Etcd archive downloaded to %s", archivePath)
	return nil
}

func init() {
	step.Register(&DownloadEtcdArchiveStepSpec{}, &DownloadEtcdArchiveStepExecutor{})
}
