package etcd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

const (
	// DefaultEtcdExtractedPathKey is a common key that might be used by an etcd extraction step.
	DefaultEtcdExtractedPathKey = "EtcdExtractedPath"
)

// InstallEtcdBinariesStepSpec defines parameters for installing etcd binaries
// from a previously extracted location.
type InstallEtcdBinariesStepSpec struct {
	spec.StepMeta `json:",inline"`

	SourceExtractedPathCacheKey string            `json:"sourceExtractedPathCacheKey,omitempty"` // Required
	BinariesToInstall         map[string]string `json:"binariesToInstall,omitempty"`
	Permissions               string            `json:"permissions,omitempty"`
	Sudo                      bool              `json:"sudo,omitempty"`
}

// NewInstallEtcdBinariesStepSpec creates a new InstallEtcdBinariesStepSpec.
func NewInstallEtcdBinariesStepSpec(name, description, sourceExtractedPathCacheKey string) *InstallEtcdBinariesStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Install Etcd Binaries"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if sourceExtractedPathCacheKey == "" {
		// This is required. Factory user must ensure it's set.
	}

	return &InstallEtcdBinariesStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		SourceExtractedPathCacheKey: sourceExtractedPathCacheKey,
		// Defaults in populateDefaults
	}
}

// Name returns the step's name.
func (s *InstallEtcdBinariesStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *InstallEtcdBinariesStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *InstallEtcdBinariesStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *InstallEtcdBinariesStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *InstallEtcdBinariesStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *InstallEtcdBinariesStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *InstallEtcdBinariesStepSpec) populateDefaults(logger runtime.Logger) {
	if s.SourceExtractedPathCacheKey == "" {
		s.SourceExtractedPathCacheKey = DefaultEtcdExtractedPathKey
		logger.Debug("SourceExtractedPathCacheKey defaulted.", "key", s.SourceExtractedPathCacheKey)
	}

	if len(s.BinariesToInstall) == 0 {
		s.BinariesToInstall = map[string]string{
			"etcd":    "/usr/local/bin/etcd",
			"etcdctl": "/usr/local/bin/etcdctl",
			"etcdutl": "/usr/local/bin/etcdutl",
		}
		logger.Debug("BinariesToInstall defaulted.", "map", s.BinariesToInstall)
	}
	if s.Permissions == "" {
		s.Permissions = "0755" // Typical for executables
		logger.Debug("Permissions defaulted.", "permissions", s.Permissions)
	}
	if !s.Sudo { // Default to true if not explicitly set false
		s.Sudo = true
		logger.Debug("Sudo defaulted to true for binary installation.")
	}

	if s.StepMeta.Description == "" {
		var targets []string
		for _, targetPath := range s.BinariesToInstall {
			targets = append(targets, targetPath)
		}
		s.StepMeta.Description = fmt.Sprintf("Installs etcd binaries (from cache key '%s') to system paths: [%s] with permissions %s.",
			s.SourceExtractedPathCacheKey, strings.Join(targets, ", "), s.Permissions)
	}
}

// Precheck determines if the etcd binaries are already installed.
func (s *InstallEtcdBinariesStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.SourceExtractedPathCacheKey == "" {
		return false, fmt.Errorf("SourceExtractedPathCacheKey must be specified for %s", s.GetName())
	}
	if len(s.BinariesToInstall) == 0 {
		logger.Info("No binaries specified in BinariesToInstall map. Precheck considers this done.")
		return true, nil
	}

	// Check if source path is in cache. If not, Run would fail.
	// However, if all target binaries already exist, this step might be considered "done"
	// regardless of the cache state for the source (which might be from a different run).
	_, sourcePathFound := ctx.StepCache().Get(s.SourceExtractedPathCacheKey)
	if !sourcePathFound {
		logger.Debug("Source extracted path not found in cache. Will check target binary existence.", "key", s.SourceExtractedPathCacheKey)
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	allExist := true
	for _, targetPath := range s.BinariesToInstall {
		exists, err := conn.Exists(ctx.GoContext(), targetPath)
		if err != nil {
			logger.Warn("Failed to check existence of target binary, assuming not installed.", "path", targetPath, "error", err)
			return false, nil // Let Run attempt.
		}
		if !exists {
			logger.Info("Target binary does not exist. Installation needed.", "path", targetPath)
			allExist = false
			break
		}
	}

	if allExist {
		logger.Info("All target etcd binaries already exist. Assuming installed.")
		// TODO: Optionally verify versions/checksums if that info is available.
		return true, nil
	}

	return false, nil
}

// Run installs etcd binaries from the extracted location to system paths.
func (s *InstallEtcdBinariesStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.SourceExtractedPathCacheKey == "" {
		return fmt.Errorf("SourceExtractedPathCacheKey must be specified for %s", s.GetName())
	}
	if len(s.BinariesToInstall) == 0 {
		logger.Info("No binaries specified in BinariesToInstall map. Nothing to do.")
		return nil
	}

	sourceExtractedPathVal, found := ctx.StepCache().Get(s.SourceExtractedPathCacheKey)
	if !found {
		return fmt.Errorf("source extracted path not found in StepCache using key '%s'", s.SourceExtractedPathCacheKey)
	}
	sourceExtractedPath, ok := sourceExtractedPathVal.(string)
	if !ok || sourceExtractedPath == "" {
		return fmt.Errorf("invalid source extracted path in StepCache (key '%s')", s.SourceExtractedPathCacheKey)
	}
	logger.Info("Using extracted etcd files from.", "path", sourceExtractedPath)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	for srcFilenameInArchive, targetSystemPath := range s.BinariesToInstall {
		fullSourcePath := filepath.Join(sourceExtractedPath, srcFilenameInArchive)

		// Verify source binary exists in the extracted path
		srcExists, errSrcExist := conn.Exists(ctx.GoContext(), fullSourcePath)
		if errSrcExist != nil {
			return fmt.Errorf("failed to check existence of source binary %s: %w", fullSourcePath, errSrcExist)
		}
		if !srcExists {
			return fmt.Errorf("source binary %s not found in extracted location %s", fullSourcePath, sourceExtractedPath)
		}

		targetDir := filepath.Dir(targetSystemPath)
		logger.Debug("Ensuring target directory for binary exists.", "path", targetDir)
		mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
		if errMkdir != nil {
			return fmt.Errorf("failed to create target directory %s for %s (stderr: %s): %w", targetDir, srcFilenameInArchive, string(stderrMkdir), errMkdir)
		}

		logger.Info("Copying etcd binary.", "source", fullSourcePath, "destination", targetSystemPath)
		cpCmd := fmt.Sprintf("cp -f %s %s", fullSourcePath, targetSystemPath)
		_, stderrCp, errCp := conn.Exec(ctx.GoContext(), cpCmd, execOpts)
		if errCp != nil {
			return fmt.Errorf("failed to copy binary from %s to %s (stderr: %s): %w", fullSourcePath, targetSystemPath, string(stderrCp), errCp)
		}

		if s.Permissions != "" {
			chmodCmd := fmt.Sprintf("chmod %s %s", s.Permissions, targetSystemPath)
			logger.Info("Setting permissions on installed binary.", "path", targetSystemPath, "permissions", s.Permissions)
			_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, execOpts)
			if errChmod != nil {
				return fmt.Errorf("failed to set permissions %s on %s (stderr: %s): %w", s.Permissions, targetSystemPath, string(stderrChmod), errChmod)
			}
		}
		logger.Info("Binary installed and permissions set.", "path", targetSystemPath)
	}

	logger.Info("All specified etcd binaries installed successfully.")
	return nil
}

// Rollback removes the installed etcd binaries.
func (s *InstallEtcdBinariesStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger)

	if len(s.BinariesToInstall) == 0 {
		logger.Info("No binaries specified in BinariesToInstall map. Nothing to roll back.")
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	for _, targetPath := range s.BinariesToInstall {
		if targetPath == "" { continue }
		logger.Info("Attempting to remove etcd binary.", "path", targetPath)
		rmCmd := fmt.Sprintf("rm -f %s", targetPath)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)
		if errRm != nil {
			logger.Warn("Failed to remove binary during rollback (best effort).", "path", targetPath, "stderr", string(stderrRm), "error", errRm)
		} else {
			logger.Info("Binary removed successfully.", "path", targetPath)
		}
	}
	logger.Info("Etcd binaries removal attempt finished for rollback.")
	return nil
}

var _ step.Step = (*InstallEtcdBinariesStepSpec)(nil)
