package containerd

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

// DefaultContainerdExtractedPathKey is a common default key for this step's output.
const DefaultContainerdExtractedPathKey = "ContainerdExtractedPath"

// ExtractContainerdStepSpec defines parameters for extracting a containerd archive.
type ExtractContainerdStepSpec struct {
	spec.StepMeta `json:",inline"`

	ArchivePathCacheKey         string `json:"archivePathCacheKey,omitempty"` // Required
	TargetDir                   string `json:"targetDir,omitempty"`
	StripComponents             int    `json:"stripComponents"` // Default 0 for containerd archives like cri-containerd-cni
	OutputExtractedPathCacheKey string `json:"outputExtractedPathCacheKey,omitempty"` // Required
	Sudo                        bool   `json:"sudo,omitempty"`
}

// NewExtractContainerdStepSpec creates a new ExtractContainerdStepSpec.
func NewExtractContainerdStepSpec(name, description, archivePathCacheKey, outputExtractedPathCacheKey string) *ExtractContainerdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Extract Containerd Archive"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if archivePathCacheKey == "" || outputExtractedPathCacheKey == "" {
		// Required fields. Consider returning error or panicking.
	}

	return &ExtractContainerdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ArchivePathCacheKey:         archivePathCacheKey,
		OutputExtractedPathCacheKey: outputExtractedPathCacheKey,
		// Defaults for TargetDir, StripComponents, Sudo in populateDefaults
	}
}

// Name returns the step's name.
func (s *ExtractContainerdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ExtractContainerdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ExtractContainerdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ExtractContainerdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ExtractContainerdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ExtractContainerdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ExtractContainerdStepSpec) populateDefaults(logger runtime.Logger, stepID string) {
	if s.TargetDir == "" {
		s.TargetDir = filepath.Join("/tmp", fmt.Sprintf("kubexms_extract_containerd_%s_%d", stepID, time.Now().UnixNano()))
		logger.Debug("TargetDir defaulted.", "dir", s.TargetDir)
	}
	// Containerd archives (like cri-containerd-cni-X.Y.Z-linux-amd64.tar.gz) often extract directly
	// into etc/, opt/, usr/ relative to the extraction directory.
	// So, StripComponents=0 is usually appropriate if TargetDir is meant to be a temporary staging root.
	// If the archive has a single top-level folder (e.g. "containerd-X.Y.Z/"), then StripComponents=1 would be used.
	// The prompt self-corrected to default StripComponents to 0.
	// if s.StripComponents == 0 && !utils.IsFieldExplicitlySet(s, "StripComponents") {
	// Defaulting to 0 as per corrected understanding in prompt. If user wants non-zero, they must set it.
	// }
	// The field is int, so its zero value is 0. No explicit defaulting needed if 0 is the intended default.
	// logger.Debug("StripComponents is 0 (default or explicitly set).")


	if !s.Sudo && utils.PathRequiresSudo(s.TargetDir) {
		s.Sudo = true
		logger.Debug("Sudo defaulted to true due to privileged TargetDir.")
	}

	if s.OutputExtractedPathCacheKey == "" {
	    s.OutputExtractedPathCacheKey = DefaultContainerdExtractedPathKey
	    logger.Debug("OutputExtractedPathCacheKey defaulted.", "key", s.OutputExtractedPathCacheKey)
	}


	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Extracts containerd archive (from cache key '%s') into %s (strip: %d). Output path to cache key '%s'.",
			s.ArchivePathCacheKey, s.TargetDir, s.StripComponents, s.OutputExtractedPathCacheKey)
	}
}

// Precheck determines if the extraction seems already done.
func (s *ExtractContainerdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger, ctx.GetStepID())

	if s.ArchivePathCacheKey == "" || s.OutputExtractedPathCacheKey == "" {
		return false, fmt.Errorf("ArchivePathCacheKey and OutputExtractedPathCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetDir == "" {
	    return false, fmt.Errorf("TargetDir is empty after populateDefaults for %s", s.GetName())
	}


	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, archivePathFound := ctx.StepCache().Get(s.ArchivePathCacheKey); !archivePathFound {
		logger.Info("Archive path not found in cache. Precheck cannot confirm if extraction is valid without source info.", "key", s.ArchivePathCacheKey)
	}

	expectedExtractedPath := s.TargetDir
	// With StripComponents=0, the actual content might be in subdirs like TargetDir/usr/local/bin
	// So, check for a known binary within the expected structure.
	// e.g. cri-containerd-cni...tar.gz extracts to usr/local/bin/containerd, etc/, opt/
	// So, if TargetDir is /tmp/extract, we'd look for /tmp/extract/usr/local/bin/containerd or /tmp/extract/bin/ctr (if containerd main tgz)
	// Let's use "ctr" as a common marker for containerd binaries.
	// Path depends on archive structure and strip components.
	// If stripping 0 from cri-containerd-cni... : TargetDir/usr/local/bin/ctr
	// If stripping 1 from containerd-1.x.y... : TargetDir/bin/ctr

	// For this generic step, let's assume if OutputExtractedPathCacheKey is set, its value is the path to check.
	// And if that path exists and contains a marker (e.g., "bin/ctr" or "usr/local/bin/containerd"), it's done.
	// This is simpler than trying to guess the internal structure based on StripComponents here.
	// The Run method will store s.TargetDir as the OutputExtractedPathCacheKey.

	markerRelPath := "bin/ctr" // A common binary in containerd bundles.
	// If containerd-cni bundle, it might be "usr/local/bin/containerd"
	// For flexibility, this marker could be a parameter. For now, pick one.
	// If StripComponents=0 and archive is cri-containerd-cni... then path is TargetDir/usr/local/bin/containerd
	// If StripComponents=1 and archive is containerd-X.Y.Z.tar.gz (which has containerd/bin/...), then path is TargetDir/bin/containerd
	// Given default StripComponents is 0 for this step, let's check for a common path from cri-containerd-cni bundle.
	if s.StripComponents == 0 {
	    markerRelPath = "usr/local/bin/containerd" // From cri-containerd-cni bundle
	} // else if s.StripComponents == 1, "bin/ctr" is more likely from main containerd tgz.

	if cachedPathVal, found := ctx.StepCache().Get(s.OutputExtractedPathCacheKey); found {
		if cachedPath, ok := cachedPathVal.(string); ok && cachedPath == expectedExtractedPath {
			markerFile := filepath.Join(cachedPath, markerRelPath)
			exists, _ := conn.Exists(ctx.GoContext(), markerFile)
			if exists {
				logger.Info("Cached extracted path exists and contains marker file. Assuming already extracted.", "key", s.OutputExtractedPathCacheKey, "path", cachedPath, "marker", markerFile)
				return true, nil
			}
		}
	}

	targetExists, _ := conn.Exists(ctx.GoContext(), s.TargetDir)
	if targetExists {
		markerFile := filepath.Join(s.TargetDir, markerRelPath)
		contentExists, _ := conn.Exists(ctx.GoContext(), markerFile)
		if contentExists {
			logger.Info("TargetDir exists and contains marker file. Assuming already extracted.", "targetDir", s.TargetDir, "marker", markerFile)
			ctx.StepCache().Set(s.OutputExtractedPathCacheKey, s.TargetDir)
			return true, nil
		}
	}

	logger.Info("Containerd archive does not appear to be extracted to the target location or marker file not found.")
	return false, nil
}

// Run performs the archive extraction.
func (s *ExtractContainerdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
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
		return fmt.Errorf("containerd archive path not found in StepCache using key '%s'", s.ArchivePathCacheKey)
	}
	archivePath, ok := archivePathVal.(string)
	if !ok || archivePath == "" {
		return fmt.Errorf("invalid or empty containerd archive path in StepCache (key '%s')", s.ArchivePathCacheKey)
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

	logger.Info("Extracting containerd archive.", "archive", archivePath, "destination", s.TargetDir, "strip", s.StripComponents)
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s --strip-components=%d",
		archivePath, s.TargetDir, s.StripComponents)

	_, stderrExtract, errExtract := conn.Exec(ctx.GoContext(), extractCmd, execOpts)
	if errExtract != nil {
		return fmt.Errorf("failed to extract containerd archive %s to %s (stderr: %s): %w", archivePath, s.TargetDir, string(stderrExtract), errExtract)
	}

	// The root of extracted content is s.TargetDir itself.
	ctx.StepCache().Set(s.OutputExtractedPathCacheKey, s.TargetDir)
	logger.Info("Containerd archive extracted successfully, output path cached.", "key", s.OutputExtractedPathCacheKey, "path", s.TargetDir)
	return nil
}

// Rollback removes the target extraction directory.
func (s *ExtractContainerdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger, ctx.GetStepID())

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

var _ step.Step = (*ExtractContainerdStepSpec)(nil)
