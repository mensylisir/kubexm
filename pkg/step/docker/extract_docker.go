package docker

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

// ExtractDockerStepSpec defines parameters for extracting a Docker archive.
type ExtractDockerStepSpec struct {
	spec.StepMeta `json:",inline"`

	ArchivePathCacheKey       string `json:"archivePathCacheKey,omitempty"` // Required
	TargetDir                 string `json:"targetDir,omitempty"`
	StripComponents           int    `json:"stripComponents"` // No omitempty, so 0 is a valid value if not set.
	OutputExtractedPathCacheKey string `json:"outputExtractedPathCacheKey,omitempty"`
	Sudo                      bool   `json:"sudo,omitempty"`
}

// NewExtractDockerStepSpec creates a new ExtractDockerStepSpec.
func NewExtractDockerStepSpec(name, description, archivePathCacheKey string) *ExtractDockerStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Extract Docker Archive"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if archivePathCacheKey == "" {
		// This is a required field.
	}

	return &ExtractDockerStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ArchivePathCacheKey: archivePathCacheKey,
		// Defaults for TargetDir, StripComponents, Sudo in populateDefaults
	}
}

// Name returns the step's name.
func (s *ExtractDockerStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ExtractDockerStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ExtractDockerStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ExtractDockerStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ExtractDockerStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ExtractDockerStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ExtractDockerStepSpec) populateDefaults(logger runtime.Logger, stepID string) {
	if s.TargetDir == "" {
		// Use a more unique temp dir if multiple extractions might happen
		s.TargetDir = filepath.Join("/tmp", fmt.Sprintf("kubexms_extract_docker_%s_%d", stepID, time.Now().UnixNano()))
		logger.Debug("TargetDir defaulted.", "dir", s.TargetDir)
	}
	// StripComponents default: Docker archives (docker-XX.YY.ZZ.tgz) usually have a 'docker/' prefix.
	// So, --strip-components=1 is common to get the contents directly.
	if s.StripComponents == 0 && !utils.IsFieldExplicitlySet(s, "StripComponents") {
		// Only default if not explicitly set to 0. If user wants 0, they can set it.
		// This check is tricky without reflection or explicit "isSet" flags.
		// For now, assume if it's 0, it might be unitialized, so default to 1.
		// A better way is to use a pointer or a wrapper type if 0 is a valid user choice different from default.
		// Given the prompt, default to 1.
		s.StripComponents = 1
		logger.Debug("StripComponents defaulted to 1.")
	}

	if !s.Sudo && utils.PathRequiresSudo(s.TargetDir) { // If not set, and path suggests sudo
		s.Sudo = true
		logger.Debug("Sudo defaulted to true due to privileged TargetDir.")
	}


	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Extracts Docker archive (from cache key '%s') to %s, stripping %d component(s).",
			s.ArchivePathCacheKey, s.TargetDir, s.StripComponents)
	}
}

// Precheck determines if the extraction seems already done.
func (s *ExtractDockerStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger, ctx.GetStepID()) // Pass unique ID for temp dir generation

	if s.ArchivePathCacheKey == "" {
		return false, fmt.Errorf("ArchivePathCacheKey must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check if archive path is even in cache. If not, Run will fail, so Precheck should indicate not done.
	_, archivePathFound := ctx.StepCache().Get(s.ArchivePathCacheKey)
	if !archivePathFound {
		logger.Info("Archive path not found in cache. Extraction cannot proceed.", "key", s.ArchivePathCacheKey)
		return false, fmt.Errorf("archive path key '%s' not found in StepCache", s.ArchivePathCacheKey)
	}


	// If OutputExtractedPathCacheKey is set and valid, it's a strong indicator.
	if s.OutputExtractedPathCacheKey != "" {
		cachedExtractedPath, found := ctx.StepCache().Get(s.OutputExtractedPathCacheKey)
		if found {
			pathStr, ok := cachedExtractedPath.(string)
			if ok && pathStr != "" {
				exists, _ := conn.Exists(ctx.GoContext(), pathStr)
				if exists {
					// Further check: does pathStr contain expected files like 'docker/dockerd'?
					// For Docker, after stripping 'docker/', the binaries are at the root of TargetDir.
					// So, pathStr would be s.TargetDir + "/docker" (if not stripped) or s.TargetDir itself.
					// A simple check for a key binary:
					expectedBin := filepath.Join(pathStr, "dockerd") // Assuming pathStr IS the 'docker' subdir
					if s.StripComponents == 1 { // If 'docker/' was stripped, binaries are directly in pathStr (which is s.TargetDir)
					    expectedBin = filepath.Join(pathStr, "dockerd")
					}

					binExists, _ := conn.Exists(ctx.GoContext(),expectedBin)
					if binExists {
						logger.Info("OutputExtractedPathCacheKey found, path exists, and key binary found. Assuming already extracted.", "key", s.OutputExtractedPathCacheKey, "path", pathStr, "checkBin", expectedBin)
						return true, nil
					}
					logger.Info("OutputExtractedPathCacheKey found and path exists, but key binary not found. Re-extraction might be needed.", "path", pathStr, "checkBin", expectedBin)
				}
			}
		}
	}

	// Fallback: Check if TargetDir exists and contains expected content (e.g., "docker/dockerd" or just "dockerd" if stripped).
	if s.TargetDir != "" {
		targetExists, _ := conn.Exists(ctx.GoContext(), s.TargetDir)
		if targetExists {
			expectedContentPath := filepath.Join(s.TargetDir, "dockerd") // Assumes strip component 1 from `docker/dockerd`
			if s.StripComponents == 0 { // If no stripping, expect 'docker' subdir
				expectedContentPath = filepath.Join(s.TargetDir, "docker", "dockerd")
			}

			contentExists, _ := conn.Exists(ctx.GoContext(), expectedContentPath)
			if contentExists {
				logger.Info("TargetDir exists and contains expected content (dockerd). Assuming already extracted.", "targetDir", s.TargetDir, "contentPath", expectedContentPath)
				// If we reach here, also populate the cache key if it's not set, similar to Download step.
				if s.OutputExtractedPathCacheKey != "" {
				    determinedOutputPath := s.TargetDir
				    if s.StripComponents == 0 { determinedOutputPath = filepath.Join(s.TargetDir, "docker")}
				    ctx.StepCache().Set(s.OutputExtractedPathCacheKey, determinedOutputPath)
				}
				return true, nil
			}
			logger.Info("TargetDir exists but doesn't contain expected 'dockerd'. Extraction needed.", "targetDir", s.TargetDir)
		} else {
			logger.Info("TargetDir does not exist. Extraction needed.", "targetDir", s.TargetDir)
		}
	}
	return false, nil
}

// Run performs the archive extraction.
func (s *ExtractDockerStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger, ctx.GetStepID())

	if s.ArchivePathCacheKey == "" {
		return fmt.Errorf("ArchivePathCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetDir == "" {
		return fmt.Errorf("TargetDir could not be defaulted (e.g. missing workdir context) and was not specified for %s", s.GetName())
	}

	archivePathVal, found := ctx.StepCache().Get(s.ArchivePathCacheKey)
	if !found {
		return fmt.Errorf("archive path not found in StepCache using key '%s' for %s", s.ArchivePathCacheKey, s.GetName())
	}
	archivePath, ok := archivePathVal.(string)
	if !ok || archivePath == "" {
		return fmt.Errorf("invalid or empty archive path in StepCache (key '%s') for %s", s.ArchivePathCacheKey, s.GetName())
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

	logger.Info("Extracting Docker archive.", "archive", archivePath, "destination", s.TargetDir, "strip", s.StripComponents)
	// tar options: x - extract, z - gzip, f - file, C - change to directory, --strip-components
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s --strip-components=%d", archivePath, s.TargetDir, s.StripComponents)

	_, stderrExtract, errExtract := conn.Exec(ctx.GoContext(), extractCmd, execOpts)
	if errExtract != nil {
		return fmt.Errorf("failed to extract Docker archive %s to %s (stderr: %s): %w", archivePath, s.TargetDir, string(stderrExtract), errExtract)
	}

	if s.OutputExtractedPathCacheKey != "" {
		// After stripping, the contents are directly in TargetDir.
		// If Docker archive was `docker/binary` and strip=1, then `binary` is in TargetDir.
		// So, the "root" of useful extracted content is TargetDir itself.
		ctx.StepCache().Set(s.OutputExtractedPathCacheKey, s.TargetDir)
		logger.Debug("Stored extracted path in StepCache.", "key", s.OutputExtractedPathCacheKey, "path", s.TargetDir)
	}

	logger.Info("Docker archive extracted successfully.", "destination", s.TargetDir)
	return nil
}

// Rollback removes the target extraction directory.
func (s *ExtractDockerStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
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
		// Do not return error for rollback as it's best-effort.
	} else {
		logger.Info("Extraction directory removed successfully.", "dir", s.TargetDir)
	}

	if s.OutputExtractedPathCacheKey != "" {
		ctx.StepCache().Delete(s.OutputExtractedPathCacheKey)
		logger.Debug("Removed extracted path from cache.", "key", s.OutputExtractedPathCacheKey)
	}
	return nil
}

var _ step.Step = (*ExtractDockerStepSpec)(nil)
