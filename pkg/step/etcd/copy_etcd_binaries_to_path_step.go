package etcd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	ExtractedEtcdDirCacheKey = "ExtractedEtcdDir" // Cache key for the directory where etcd/etcdctl are after extraction
)

// CopyEtcdBinariesToPathStep moves extracted etcd and etcdctl binaries to a system path.
type CopyEtcdBinariesToPathStep struct {
	meta                   spec.StepMeta
	ExtractedDirCacheKey   string // Task cache key providing the path to the directory containing extracted etcd/etcdctl
	TargetDir              string // System directory to move binaries to, e.g., /usr/local/bin
	Sudo                   bool   // Whether to use sudo for moving and chmoding files
	ExpectedVersion        string // Expected version string to verify (optional, e.g., "3.5.9")
	RemoveSourceAfterCopy  bool   // Whether to remove the ExtractedDir after successful copy
}

// NewCopyEtcdBinariesToPathStep creates a new CopyEtcdBinariesToPathStep.
func NewCopyEtcdBinariesToPathStep(instanceName, extractedDirCacheKey, targetDir, expectedVersion string, sudo, removeSource bool) step.Step {
	if extractedDirCacheKey == "" {
		extractedDirCacheKey = ExtractedEtcdDirCacheKey
	}
	if targetDir == "" {
		targetDir = "/usr/local/bin"
	}
	name := instanceName
	if name == "" {
		name = "CopyEtcdBinariesToPath"
	}
	return &CopyEtcdBinariesToPathStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Copies etcd and etcdctl from extracted source to %s", targetDir),
		},
		ExtractedDirCacheKey:  extractedDirCacheKey,
		TargetDir:             targetDir,
		Sudo:                  sudo,
		ExpectedVersion:       expectedVersion,
		RemoveSourceAfterCopy: removeSource,
	}
}

func (s *CopyEtcdBinariesToPathStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CopyEtcdBinariesToPathStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		binPath := filepath.Join(s.TargetDir, binName)
		exists, errExist := runnerSvc.Exists(ctx.GoContext(), conn, binPath)
		if errExist != nil {
			logger.Warn("Failed to check existence of binary, assuming not installed correctly.", "path", binPath, "error", errExist)
			return false, nil
		}
		if !exists {
			logger.Debug("Etcd binary does not exist in target path.", "path", binPath)
			return false, nil
		}

		if s.ExpectedVersion != "" {
			versionCmd := ""
			if binName == "etcd" {
				versionCmd = fmt.Sprintf("%s --version", binPath)
			} else if binName == "etcdctl" {
				versionCmd = fmt.Sprintf("%s version", binPath)
			}

			// Expect version commands to exit 0. If they error, we can't confirm version.
			stdoutBytes, stderrBytes, execErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, versionCmd, &connector.ExecOptions{Sudo: false})
			output := string(stdoutBytes) // Combine stdout for parsing
			if len(stderrBytes) > 0 {
				// Some tools print version to stderr or errors that might still be parsable
				// output += "\nStderr: " + string(stderrBytes) // Keep it simple for now, just stdout primarily
			}

			fullOutput := string(stdoutBytes) + string(stderrBytes)


			if execErr != nil {
				logger.Warn("Failed to get version of existing binary, assuming not correct.", "path", binPath, "error", execErr, "output", fullOutput)
				return false, nil
			}

			expectedVersionStr := strings.TrimPrefix(s.ExpectedVersion, "v")
			versionLineFound := false
			if binName == "etcd" && (strings.Contains(output, "etcd Version: "+expectedVersionStr) || strings.Contains(output, "etcd version "+expectedVersionStr)) {
				versionLineFound = true
			}
			if binName == "etcdctl" && (strings.Contains(output, "etcdctl version: "+expectedVersionStr) || strings.Contains(output, `"etcdserver":"`+expectedVersionStr+`"`)) {
				versionLineFound = true
			}

			if !versionLineFound {
				logger.Info("Etcd binary exists, but version does not match.", "path", binPath, "expected", expectedVersionStr, "actualOutput", output)
				return false, nil
			}
			logger.Debug("Etcd binary version matches.", "binary", binName, "path", binPath)
		}
	}
	logger.Info("All etcd binaries exist in target path and version matches (if specified).")
	return true, nil
}

func (s *CopyEtcdBinariesToPathStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	extractedDirVal, found := ctx.TaskCache().Get(s.ExtractedDirCacheKey)
	if !found {
		return fmt.Errorf("extracted etcd directory path not found in task cache with key '%s'", s.ExtractedDirCacheKey)
	}
	extractedDir, ok := extractedDirVal.(string)
	if !ok || extractedDir == "" {
		return fmt.Errorf("invalid extracted etcd directory path in task cache: got '%v'", extractedDirVal)
	}
	logger.Info("Retrieved extracted etcd directory path from cache.", "path", extractedDir)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Ensuring target directory exists.", "path", s.TargetDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.TargetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", s.TargetDir, err)
	}

	binariesToCopy := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToCopy {
		// Source path might be directly in extractedDir or in a subdirectory like etcd-vX.Y.Z-linux-amd64/
		// The ExtractedDirCacheKey should point to the directory *containing* etcd and etcdctl.
		srcPath := filepath.Join(extractedDir, binName)
		dstPath := filepath.Join(s.TargetDir, binName)

		srcExists, errSrcExist := runnerSvc.Exists(ctx.GoContext(), conn, srcPath)
		if errSrcExist != nil {
			logger.Warn("Could not verify existence of source binary, attempting copy anyway.", "path", srcPath, "error", errSrcExist)
		} else if !srcExists {
			// Try to find in a potential versioned subdirectory if not directly in extractedDir
			// This logic might be too complex for this step; ExtractedDirCacheKey should be precise.
			// For now, assume ExtractedDirCacheKey is the immediate parent of etcd/etcdctl.
			return fmt.Errorf("source binary %s not found in specified extracted path %s", binName, extractedDir)
		}

		logger.Info("Copying binary.", "source", srcPath, "destination", dstPath)
		// Use mv for atomicity if possible, or cp then rm. Runner might abstract this.
		// For simplicity, using cp.
		cpCmd := fmt.Sprintf("cp -fp %s %s", srcPath, dstPath)
		if _, errCp := runnerSvc.Run(ctx.GoContext(), conn, cpCmd, s.Sudo); errCp != nil {
			return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, errCp)
		}

		logger.Info("Setting permissions for binary.", "path", dstPath)
		if errChmod := runnerSvc.Chmod(ctx.GoContext(), conn, dstPath, "0755", s.Sudo); errChmod != nil { // Ensure executable
			return fmt.Errorf("failed to make %s executable: %w", dstPath, errChmod)
		}
	}

	if s.RemoveSourceAfterCopy {
		logger.Info("Cleaning up source extraction directory.", "path", extractedDir)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, extractedDir, s.Sudo); err != nil { // Sudo might be needed if extraction was sudo
			logger.Warn("Failed to cleanup etcd extraction directory (best effort).", "path", extractedDir, "error", err)
		}
	}

	logger.Info("Etcd binaries copied to target path successfully.")
	return nil
}

func (s *CopyEtcdBinariesToPathStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	if s.TargetDir == "" {
		logger.Error("TargetDir is not set, cannot perform rollback.")
		return fmt.Errorf("TargetDir not set for etcd rollback on host %s", host.GetName())
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	binariesToRemove := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToRemove {
		binPath := filepath.Join(s.TargetDir, binName)
		// Only remove if version matches, if version was specified for safety?
		// For now, assume if rollback is called, we remove what Run put there.
		logger.Info("Attempting to remove binary for rollback.", "path", binPath)
		if errRm := runnerSvc.Remove(ctx.GoContext(), conn, binPath, s.Sudo); errRm != nil {
			// Log as warning because other binaries might still need removal.
			logger.Warn("Failed to remove binary during rollback (best effort).", "path", binPath, "error", errRm)
		}
	}
	logger.Info("Rollback attempt for copying etcd binaries finished.")
	return nil // Best effort for rollback
}

var _ step.Step = (*CopyEtcdBinariesToPathStep)(nil)
