package etcd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/octo-cli/core/pkg/runtime"
	"github.com/octo-cli/core/pkg/step"
	"github.com/octo-cli/core/pkg/step/spec"
)

const (
	// EtcdExtractedPathKey is the key for the extracted etcd directory path in SharedData.
	// This path points to the `etcd-<version>-linux-<arch>` directory.
	EtcdExtractedPathKey = "EtcdExtractedPath"
	// EtcdExtractionDirKey is the key for the parent directory where etcd was extracted.
	EtcdExtractionDirKey = "EtcdExtractionDir"
)

// ExtractEtcdArchiveStepSpec defines the specification for extracting the etcd archive.
type ExtractEtcdArchiveStepSpec struct {
	// ExtractionDirBase is the base directory where a unique extraction folder will be created.
	ExtractionDirBase string `json:"extractionDirBase"`
}

// GetName returns the name of the step.
func (s *ExtractEtcdArchiveStepSpec) GetName() string {
	return "ExtractEtcdArchive"
}

// ApplyDefaults applies default values to the spec.
func (s *ExtractEtcdArchiveStepSpec) ApplyDefaults(ctx *runtime.Context) error {
	if s.ExtractionDirBase == "" {
		s.ExtractionDirBase = filepath.Join(ctx.WorkDir, "etcd-extract")
	}
	return nil
}

// ExtractEtcdArchiveStepExecutor implements the logic for extracting the etcd archive.
type ExtractEtcdArchiveStepExecutor struct{}

// Check checks if the etcd binaries seem to be already extracted.
func (e *ExtractEtcdArchiveStepExecutor) Check(ctx *runtime.Context, s spec.StepSpec) (bool, error) {
	spec, ok := s.(*ExtractEtcdArchiveStepSpec)
	if !ok {
		return false, fmt.Errorf("invalid spec type %T for ExtractEtcdArchiveStepExecutor", s)
	}
	if err := spec.ApplyDefaults(ctx); err != nil {
		// Defaults are needed to construct potential paths, but an error here shouldn't stop check
		ctx.Logger.Debugf("Error applying defaults during check for ExtractEtcdArchive: %v", err)
	}

	// Check if SharedData already contains valid-looking paths
	extractedPath, pathOk := ctx.SharedData.Get(EtcdExtractedPathKey)
	extractionDir, dirOk := ctx.SharedData.Get(EtcdExtractionDirKey)

	if pathOk && dirOk {
		extractedPathStr, ok1 := extractedPath.(string)
		extractionDirStr, ok2 := extractionDir.(string)
		if !ok1 || !ok2 || extractedPathStr == "" || extractionDirStr == "" {
			ctx.Logger.Infof("EtcdExtractedPath or EtcdExtractionDir in SharedData is invalid, proceeding with extraction.")
			return false, nil
		}

		// Check if key files exist
		etcdBinaryPath := filepath.Join(extractedPathStr, "etcd")
		etcdctlBinaryPath := filepath.Join(extractedPathStr, "etcdctl")

		if _, err := ctx.Host.Runner.Stat(etcdBinaryPath); err == nil {
			if _, err := ctx.Host.Runner.Stat(etcdctlBinaryPath); err == nil {
				ctx.Logger.Infof("Etcd binaries already found at %s, skipping extraction.", extractedPathStr)
				return true, nil
			}
		}
		ctx.Logger.Infof("Etcd binaries not found at expected location %s or %s, proceeding with extraction.", etcdBinaryPath, etcdctlBinaryPath)
	} else {
		ctx.Logger.Infof("EtcdExtractedPath or EtcdExtractionDir not found in SharedData, proceeding with extraction.")
	}

	return false, nil
}

// Execute extracts the etcd archive.
func (e *ExtractEtcdArchiveStepExecutor) Execute(ctx *runtime.Context, s spec.StepSpec) error {
	spec, ok := s.(*ExtractEtcdArchiveStepSpec)
	if !ok {
		return fmt.Errorf("invalid spec type %T for ExtractEtcdArchiveStepExecutor", s)
	}
	if err := spec.ApplyDefaults(ctx); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	archivePathVal, ok := ctx.SharedData.Get(EtcdArchivePathKey)
	if !ok {
		return fmt.Errorf("%s not found in SharedData, ensure DownloadEtcdArchiveStep ran successfully", EtcdArchivePathKey)
	}
	archivePath, ok := archivePathVal.(string)
	if !ok || archivePath == "" {
		return fmt.Errorf("%s in SharedData is not a valid path", EtcdArchivePathKey)
	}

	etcdVersionVal, ok := ctx.SharedData.Get(EtcdVersionKey)
	if !ok {
		return fmt.Errorf("%s not found in SharedData, ensure DownloadEtcdArchiveStep ran successfully", EtcdVersionKey)
	}
	etcdVersion, ok := etcdVersionVal.(string)
	if !ok || etcdVersion == "" {
		return fmt.Errorf("%s in SharedData is not a valid version string", EtcdVersionKey)
	}

	// Create a unique extraction directory
	// Using a timestamp is okay for uniqueness, but a more robust approach might involve content hash or a simpler fixed name if cleanup is reliable.
	uniqueDirName := fmt.Sprintf("etcd-extract-%d", time.Now().UnixNano())
	extractionDir := filepath.Join(spec.ExtractionDirBase, uniqueDirName)

	if err := ctx.Host.Runner.Mkdirp(extractionDir); err != nil {
		return fmt.Errorf("failed to create extraction directory %s: %w", extractionDir, err)
	}
	ctx.Logger.Infof("Extracting etcd archive %s to %s", archivePath, extractionDir)

	if err := ctx.Host.Runner.Extract(archivePath, extractionDir); err != nil {
		return fmt.Errorf("failed to extract etcd archive: %w", err)
	}

	// The extracted archive typically creates a directory like "etcd-v3.5.0-linux-amd64"
	// We need to construct this path.
	arch, archOk := ctx.SharedData.Get("Arch") // Assuming Arch was stored by Download step or available
	if !archOk {
		// Fallback to host arch if not in shared data, though it should be.
		// This logic might need refinement if DownloadEtcdArchiveSpec.Arch is not propagated.
		// For now, let's assume EtcdVersionKey and host arch from context are sufficient.
		hostArch := ctx.Host.Arch()
		if hostArch == "x86_64" {
			hostArch = "amd64"
		}
		arch = hostArch
	}

	extractedDirName := fmt.Sprintf("etcd-%s-linux-%s", etcdVersion, arch.(string))
	extractedPath := filepath.Join(extractionDir, extractedDirName)


	// Verify that the main binaries exist after extraction
	etcdBinaryPath := filepath.Join(extractedPath, "etcd")
	etcdctlBinaryPath := filepath.Join(extractedPath, "etcdctl")

	if _, err := ctx.Host.Runner.Stat(etcdBinaryPath); err != nil {
		return fmt.Errorf("etcd binary not found at %s after extraction: %w", etcdBinaryPath, err)
	}
	if _, err := ctx.Host.Runner.Stat(etcdctlBinaryPath); err != nil {
		return fmt.Errorf("etcdctl binary not found at %s after extraction: %w", etcdctlBinaryPath, err)
	}

	ctx.SharedData.Set(EtcdExtractedPathKey, extractedPath)
	ctx.SharedData.Set(EtcdExtractionDirKey, extractionDir) // Store the parent unique dir for cleanup
	ctx.Logger.Infof("Etcd archive extracted, binaries available at %s", extractedPath)

	return nil
}

func init() {
	step.Register(&ExtractEtcdArchiveStepSpec{}, &ExtractEtcdArchiveStepExecutor{})
}
