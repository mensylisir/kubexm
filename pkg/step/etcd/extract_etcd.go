package etcd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils"
)

// ExtractEtcdStepSpec defines the parameters for extracting an etcd archive
// and placing them in a target directory.
type ExtractEtcdStepSpec struct {
	spec.StepMeta `json:",inline"`

	ArchivePathCacheKey       string `json:"archivePathCacheKey,omitempty"`       // Required
	TargetDir                 string `json:"targetDir,omitempty"`                 // Staging directory for extracted contents
	StripComponents           int    `json:"stripComponents"`                     // For tar --strip-components
	OutputExtractedPathCacheKey string `json:"outputExtractedPathCacheKey,omitempty"` // Required: Key to store the path to the extracted root (e.g., TargetDir or TargetDir/etcd-vX.Y.Z-linux-ARCH)
	Sudo                      bool   `json:"sudo,omitempty"`                      // For mkdir if TargetDir needs sudo
	// Version and Arch are implicitly part of the archive, not direct fields for this step's core logic,
	// but might be useful if a deeper inspection of archive contents were needed by the step itself.
	// For now, this step just extracts based on strip_components.
}

// NewExtractEtcdStepSpec creates a new ExtractEtcdStepSpec.
func NewExtractEtcdStepSpec(name, description, archivePathCacheKey, outputExtractedPathCacheKey string) *ExtractEtcdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Extract Etcd Archive"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if archivePathCacheKey == "" || outputExtractedPathCacheKey == "" {
		// Required fields
	}

	return &ExtractEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ArchivePathCacheKey:       archivePathCacheKey,
		OutputExtractedPathCacheKey: outputExtractedPathCacheKey,
		// Defaults for TargetDir, StripComponents, Sudo in populateDefaults
	}
}

func (s *ExtractEtcdStepSpec) populateDefaults(logger runtime.Logger, stepID string) {
	if s.TargetDir == "" {
		s.TargetDir = filepath.Join("/tmp", fmt.Sprintf("kubexms_extract_etcd_%s_%d", stepID, time.Now().UnixNano()))
		logger.Debug("TargetDir defaulted.", "dir", s.TargetDir)
	}
	// etcd archives (e.g. etcd-v3.5.9-linux-amd64.tar.gz) typically contain a single top-level directory
	// (e.g. etcd-v3.5.9-linux-amd64/) inside which the binaries (etcd, etcdctl) reside.
	// Stripping 1 component will place these binaries directly into TargetDir.
	if s.StripComponents == 0 && !utils.IsFieldExplicitlySet(s, "StripComponents") { // Check hypothetical IsFieldExplicitlySet
		s.StripComponents = 1
		logger.Debug("StripComponents defaulted to 1.")
	}

	if !s.Sudo && utils.PathRequiresSudo(s.TargetDir) {
		s.Sudo = true
		logger.Debug("Sudo defaulted to true due to privileged TargetDir.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Extracts etcd archive (from cache key '%s') into %s, stripping %d component(s). Output path to cache key '%s'.",
			s.ArchivePathCacheKey, s.TargetDir, s.StripComponents, s.OutputExtractedPathCacheKey)
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

// Name returns the step's name (implementing step.Step).
func (s *ExtractEtcdStepSpec) Name() string { return s.GetName() }

// Description returns the step's description (implementing step.Step).
func (s *ExtractEtcdStepSpec) Description() string { return s.GetDescription() }

func (s *ExtractEtcdStepSpec) getEffectiveArchivePath(ctx runtime.StepContext) (string, error) {
	if s.ArchivePathKey == "" {
		return "", fmt.Errorf("ArchivePathKey must be specified for ExtractEtcdStepSpec: %s", s.GetName())
	}
	// Try StepCache, then TaskCache, then ModuleCache
	val, found := ctx.StepCache().Get(s.ArchivePathKey)
	if found {
		if pathStr, ok := val.(string); ok { return pathStr, nil }
		return "", fmt.Errorf("cached archive path (StepCache key %s) is not a string", s.ArchivePathKey)
	}
	val, found = ctx.TaskCache().Get(s.ArchivePathKey)
	if found {
		if pathStr, ok := val.(string); ok { return pathStr, nil }
		return "", fmt.Errorf("cached archive path (TaskCache key %s) is not a string", s.ArchivePathKey)
	}
	val, found = ctx.ModuleCache().Get(s.ArchivePathKey)
	if found {
		if pathStr, ok := val.(string); ok { return pathStr, nil }
		return "", fmt.Errorf("cached archive path (ModuleCache key %s) is not a string", s.ArchivePathKey)
	}
	return "", fmt.Errorf("archive path cache key %s not found in any cache for step %s", s.ArchivePathKey, s.GetName())
}


// Precheck determines if the extraction seems already done.
func (s *ExtractEtcdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger, ctx.GetStepID())

	if s.ArchivePathCacheKey == "" || s.OutputExtractedPathCacheKey == "" {
		return false, fmt.Errorf("ArchivePathCacheKey and OutputExtractedPathCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetDir == "" { // Should be set by populateDefaults
	    return false, fmt.Errorf("TargetDir is empty after populateDefaults for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check if archive path is even in cache.
	if _, archivePathFound := ctx.StepCache().Get(s.ArchivePathCacheKey); !archivePathFound {
		logger.Info("Archive path not found in cache. Extraction cannot proceed with precheck validation of output.", "key", s.ArchivePathCacheKey)
		// If target already exists and is correct (checked next), then this step is done.
	}

	// Check if OutputExtractedPathCacheKey already holds a valid, existing path with expected content.
	expectedExtractedPath := s.TargetDir // After stripping, TargetDir is the root of extracted content.
	if cachedPathVal, found := ctx.StepCache().Get(s.OutputExtractedPathCacheKey); found {
		if cachedPath, ok := cachedPathVal.(string); ok && cachedPath == expectedExtractedPath {
			// Check for a marker file, e.g., "etcd" binary within this path.
			// The exact relative path of "etcd" depends on how much is stripped and original archive structure.
			// If strip N, and original was dir1/dir2/.../dirN/etcd, then "etcd" is at root of TargetDir.
			markerFile := filepath.Join(cachedPath, "etcd")
			exists, _ := conn.Exists(ctx.GoContext(), markerFile)
			if exists {
				logger.Info("Cached extracted path exists and contains marker file 'etcd'. Assuming already extracted correctly.", "key", s.OutputExtractedPathCacheKey, "path", cachedPath)
				return true, nil
			}
			logger.Info("Cached extracted path exists, but marker file 'etcd' not found. Re-extraction might be needed.", "path", cachedPath)
		}
	}

	// Fallback: Directly check if the TargetDir exists and contains the marker.
	targetExists, _ := conn.Exists(ctx.GoContext(), s.TargetDir)
	if targetExists {
		markerFile := filepath.Join(s.TargetDir, "etcd")
		contentExists, _ := conn.Exists(ctx.GoContext(), markerFile)
		if contentExists {
			logger.Info("TargetDir exists and contains marker file 'etcd'. Assuming already extracted.", "targetDir", s.TargetDir)
			ctx.StepCache().Set(s.OutputExtractedPathCacheKey, s.TargetDir) // Populate cache
			return true, nil
		}
		logger.Info("TargetDir exists but does not contain marker file 'etcd'. Extraction needed.", "targetDir", s.TargetDir)
	} else {
		logger.Info("TargetDir does not exist. Extraction needed.", "targetDir", s.TargetDir)
	}
	return false, nil
}

// Run performs the archive extraction into TargetDir.
func (s *ExtractEtcdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger, ctx.GetStepID())

	if s.ArchivePathCacheKey == "" || s.OutputExtractedPathCacheKey == "" {
		return fmt.Errorf("ArchivePathCacheKey and OutputExtractedPathCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetDir == "" {
	    return fmt.Errorf("TargetDir is empty after populateDefaults for %s", s.GetName())
	}

	archivePathVal, found := ctx.StepCache().Get(s.ArchivePathCacheKey)
	if !found {
		return fmt.Errorf("etcd archive path not found in StepCache using key '%s'", s.ArchivePathCacheKey)
	}
	archivePath, ok := archivePathVal.(string)
	if !ok || archivePath == "" {
		return fmt.Errorf("invalid or empty etcd archive path in StepCache (key '%s')", s.ArchivePathCacheKey)
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	logger.Info("Ensuring target extraction directory exists.", "path", s.TargetDir)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.TargetDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
	if errMkdir != nil {
		return fmt.Errorf("failed to create target extraction directory %s (stderr: %s): %w", s.TargetDir, string(stderrMkdir), errMkdir)
	}

	logger.Info("Extracting etcd archive.", "archive", archivePath, "destination", s.TargetDir, "strip", s.StripComponents)
	// Standard etcd archive (e.g., etcd-v3.5.9-linux-amd64.tar.gz) has a top-level directory like "etcd-v3.5.9-linux-amd64/".
	// --strip-components=1 will remove this top-level directory, placing contents (etcd, etcdctl, Documentation, etc.) into TargetDir.
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s --strip-components=%d",
		archivePath, s.TargetDir, s.StripComponents)

	_, stderrExtract, errExtract := conn.Exec(ctx.GoContext(), extractCmd, execOpts)
	if errExtract != nil {
		return fmt.Errorf("failed to extract etcd archive %s to %s (stderr: %s): %w", archivePath, s.TargetDir, string(stderrExtract), errExtract)
	}

	// The actual root of extracted content is now s.TargetDir itself due to stripping.
	ctx.StepCache().Set(s.OutputExtractedPathCacheKey, s.TargetDir)
	logger.Info("Etcd archive extracted successfully, output path cached.", "key", s.OutputExtractedPathCacheKey, "path", s.TargetDir)
	return nil
}

// Rollback removes the target extraction directory.
func (s *ExtractEtcdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger, ctx.GetStepID()) // Ensure TargetDir is populated

	if s.TargetDir == "" {
		logger.Info("TargetDir is empty, cannot perform rollback.")
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove extraction directory for rollback (best effort).", "dir", s.TargetDir)
	rmCmd := fmt.Sprintf("rm -rf %s", s.TargetDir)
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)

	if errRm != nil {
		logger.Error("Failed to remove extraction directory during rollback.", "dir", s.TargetDir, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Extraction directory removed successfully.", "dir", s.TargetDir)
	}

	if s.OutputExtractedPathCacheKey != "" {
		ctx.StepCache().Delete(s.OutputExtractedPathCacheKey)
		logger.Debug("Removed extracted path from cache.", "key", s.OutputExtractedPathCacheKey)
	}
	return nil
}

var _ step.Step = (*ExtractEtcdStepSpec)(nil)
