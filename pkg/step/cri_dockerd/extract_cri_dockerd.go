package cri_dockerd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time" // For unique temp dir names

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// ExtractCriDockerdStepSpec defines parameters for extracting a cri-dockerd archive.
type ExtractCriDockerdStepSpec struct {
	spec.StepMeta `json:",inline"`

	ArchivePathCacheKey             string `json:"archivePathCacheKey,omitempty"` // Required
	TargetDir                       string `json:"targetDir,omitempty"`
	StripComponents                 int    `json:"stripComponents"`
	RelativeBinaryPathInArchive     string `json:"relativeBinaryPathInArchive,omitempty"` // Path of the binary within the archive, relative to the root after stripping
	OutputExtractedBinaryPathCacheKey string `json:"outputExtractedBinaryPathCacheKey,omitempty"` // Required
	Sudo                            bool   `json:"sudo,omitempty"`
}

// NewExtractCriDockerdStepSpec creates a new ExtractCriDockerdStepSpec.
func NewExtractCriDockerdStepSpec(name, description, archivePathCacheKey, outputExtractedBinaryPathCacheKey string) *ExtractCriDockerdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Extract cri-dockerd Archive"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if archivePathCacheKey == "" || outputExtractedBinaryPathCacheKey == "" {
		// Required fields
	}

	return &ExtractCriDockerdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ArchivePathCacheKey:             archivePathCacheKey,
		OutputExtractedBinaryPathCacheKey: outputExtractedBinaryPathCacheKey,
		// Defaults for TargetDir, StripComponents, Sudo, RelativeBinaryPathInArchive in populateDefaults
	}
}

// Name returns the step's name.
func (s *ExtractCriDockerdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ExtractCriDockerdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ExtractCriDockerdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ExtractCriDockerdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ExtractCriDockerdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ExtractCriDockerdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ExtractCriDockerdStepSpec) populateDefaults(logger runtime.Logger, stepID string) {
	if s.TargetDir == "" {
		s.TargetDir = filepath.Join("/tmp", fmt.Sprintf("kubexms_extract_cri_dockerd_%s_%d", stepID, time.Now().UnixNano()))
		logger.Debug("TargetDir defaulted.", "dir", s.TargetDir)
	}
	// cri-dockerd-0.3.10.amd64.tgz contains cri-dockerd/cri-dockerd
	// So stripping 1 component means the binary 'cri-dockerd' is at the root of TargetDir.
	if s.StripComponents == 0 && !utils.IsFieldExplicitlySet(s, "StripComponents") {
		s.StripComponents = 1
		logger.Debug("StripComponents defaulted to 1.")
	}
	if s.RelativeBinaryPathInArchive == "" {
		// After stripping 1 component from "cri-dockerd/cri-dockerd", the binary is just "cri-dockerd"
		s.RelativeBinaryPathInArchive = "cri-dockerd"
		logger.Debug("RelativeBinaryPathInArchive defaulted.", "path", s.RelativeBinaryPathInArchive)
	}

	if !s.Sudo && utils.PathRequiresSudo(s.TargetDir) {
		s.Sudo = true
		logger.Debug("Sudo defaulted to true due to privileged TargetDir.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Extracts cri-dockerd binary from archive (key '%s') into %s. Output binary path to cache key '%s'.",
			s.ArchivePathCacheKey, s.TargetDir, s.OutputExtractedBinaryPathCacheKey)
	}
}

// Precheck determines if the cri-dockerd binary seems already extracted.
func (s *ExtractCriDockerdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger, ctx.GetStepID())

	if s.ArchivePathCacheKey == "" || s.OutputExtractedBinaryPathCacheKey == "" {
		return false, fmt.Errorf("ArchivePathCacheKey and OutputExtractedBinaryPathCacheKey must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check if archive path is in cache (it must be for Run to succeed)
	if _, archivePathFound := ctx.StepCache().Get(s.ArchivePathCacheKey); !archivePathFound {
		logger.Info("Archive path not found in cache. Extraction cannot proceed with precheck validation of output.", "key", s.ArchivePathCacheKey)
		// Don't error, let Run fail if archive truly missing. Precheck is about target state.
		// If target already exists and is correct (checked next), then this step is done.
	}

	expectedExtractedBinaryPath := filepath.Join(s.TargetDir, s.RelativeBinaryPathInArchive)

	// Check if OutputExtractedBinaryPathCacheKey already holds the correct, existing path
	if cachedPathVal, found := ctx.StepCache().Get(s.OutputExtractedBinaryPathCacheKey); found {
		if cachedPath, ok := cachedPathVal.(string); ok && cachedPath == expectedExtractedBinaryPath {
			exists, _ := conn.Exists(ctx.GoContext(), cachedPath)
			if exists {
				logger.Info("Cached extracted binary path exists. Assuming already extracted correctly.", "key", s.OutputExtractedBinaryPathCacheKey, "path", cachedPath)
				return true, nil
			}
			logger.Info("Cached extracted binary path points to a non-existent file. Re-extraction needed.", "path", cachedPath)
		}
	}

	// Fallback: Directly check if the expected binary path exists
	exists, err := conn.Exists(ctx.GoContext(), expectedExtractedBinaryPath)
	if err != nil {
		logger.Warn("Failed to check existence of expected extracted binary. Assuming extraction needed.", "path", expectedExtractedBinaryPath, "error", err)
		return false, nil
	}
	if exists {
		logger.Info("Expected extracted binary path exists. Assuming already extracted.", "path", expectedExtractedBinaryPath)
		// Populate cache if it wasn't already set correctly
		ctx.StepCache().Set(s.OutputExtractedBinaryPathCacheKey, expectedExtractedBinaryPath)
		return true, nil
	}

	logger.Info("cri-dockerd binary does not appear to be extracted to the target location. Extraction needed.", "expectedPath", expectedExtractedBinaryPath)
	return false, nil
}

// Run performs the archive extraction.
func (s *ExtractCriDockerdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger, ctx.GetStepID())

	if s.ArchivePathCacheKey == "" || s.OutputExtractedBinaryPathCacheKey == "" {
		return fmt.Errorf("ArchivePathCacheKey and OutputExtractedBinaryPathCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetDir == "" || s.RelativeBinaryPathInArchive == "" {
	    return fmt.Errorf("TargetDir and RelativeBinaryPathInArchive must be set (via defaults or explicitly) for %s", s.GetName())
	}


	archivePathVal, found := ctx.StepCache().Get(s.ArchivePathCacheKey)
	if !found {
		return fmt.Errorf("cri-dockerd archive path not found in StepCache using key '%s'", s.ArchivePathCacheKey)
	}
	archivePath, ok := archivePathVal.(string)
	if !ok || archivePath == "" {
		return fmt.Errorf("invalid or empty cri-dockerd archive path in StepCache (key '%s')", s.ArchivePathCacheKey)
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

	logger.Info("Extracting cri-dockerd archive.", "archive", archivePath, "destination", s.TargetDir, "strip", s.StripComponents)

	// The cri-dockerd archive (e.g., cri-dockerd-0.3.10.amd64.tgz) contains a "cri-dockerd/cri-dockerd" binary.
	// If StripComponents = 1, we are stripping "cri-dockerd/" prefix.
	// So, the member to extract from tar becomes just "cri-dockerd" (which is s.RelativeBinaryPathInArchive after defaults).
	// If StripComponents = 0, member would be "cri-dockerd/cri-dockerd".
	// The command needs to handle the path *within* the archive correctly.
	// Default: StripComponents = 1, RelativeBinaryPathInArchive = "cri-dockerd".
	// Path in archive before stripping: "cri-dockerd/" + "cri-dockerd" = "cri-dockerd/cri-dockerd".
	// This means we need to tell tar to extract "cri-dockerd/cri-dockerd" and strip 1 component.

	pathInArchiveToExtract := s.RelativeBinaryPathInArchive
	if s.StripComponents == 1 { // This is the typical case for cri-dockerd
		// If we strip 1, and the binary inside is `cri-dockerd/cri-dockerd`, we need to specify that full path to tar.
		pathInArchiveToExtract = filepath.Join("cri-dockerd", s.RelativeBinaryPathInArchive)
	}
	// If StripComponents = 0, and RelativeBinaryPathInArchive = "cri-dockerd/cri-dockerd", then pathInArchiveToExtract is correct.

	extractCmd := fmt.Sprintf("tar -xzf %s -C %s --strip-components=%d %s",
		archivePath, s.TargetDir, s.StripComponents, pathInArchiveToExtract)

	_, stderrExtract, errExtract := conn.Exec(ctx.GoContext(), extractCmd, execOpts)
	if errExtract != nil {
		return fmt.Errorf("failed to extract cri-dockerd binary from %s to %s (stderr: %s): %w", archivePath, s.TargetDir, string(stderrExtract), errExtract)
	}

	fullExtractedBinaryPath := filepath.Join(s.TargetDir, s.RelativeBinaryPathInArchive)

	// Verify the binary exists at the final location
	binaryExists, verifyErr := conn.Exists(ctx.GoContext(), fullExtractedBinaryPath)
	if verifyErr != nil {
		return fmt.Errorf("failed to verify existence of extracted binary %s: %w", fullExtractedBinaryPath, verifyErr)
	}
	if !binaryExists {
		return fmt.Errorf("extracted binary %s not found after tar command", fullExtractedBinaryPath)
	}

	logger.Info("cri-dockerd binary extracted successfully.", "path", fullExtractedBinaryPath)

	ctx.StepCache().Set(s.OutputExtractedBinaryPathCacheKey, fullExtractedBinaryPath)
	logger.Debug("Stored extracted binary path in StepCache.", "key", s.OutputExtractedBinaryPathCacheKey, "path", fullExtractedBinaryPath)

	return nil
}

// Rollback removes the target extraction directory.
func (s *ExtractCriDockerdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
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

	if s.OutputExtractedBinaryPathCacheKey != "" {
		ctx.StepCache().Delete(s.OutputExtractedBinaryPathCacheKey)
		logger.Debug("Removed extracted binary path from cache.", "key", s.OutputExtractedBinaryPathCacheKey)
	}
	return nil
}

var _ step.Step = (*ExtractCriDockerdStepSpec)(nil)
