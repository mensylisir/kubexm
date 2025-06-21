package etcd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time" // Keep for temporary directory generation based on time

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
	// "github.com/mensylisir/kubexm/pkg/utils" // Removed
)

// InstallEtcdBinariesStep downloads, extracts, and installs etcd and etcdctl binaries.
type InstallEtcdBinariesStep struct {
	meta           spec.StepMeta
	Version        string
	TargetDir      string
	InstallURLBase string
	Arch           string // User-specified, can be empty for auto-detection
	Sudo           bool   // Sudo for install operations (mkdir, cp, chmod, rm)
	DownloadSudo   bool   // Sudo specifically for DownloadAndExtract (usually false for temp dirs)

	// Internal fields for determined values
	determinedArch    string
	determinedVersion string // Store version with 'v' prefix for consistency in URL
}

// NewInstallEtcdBinariesStep creates a new InstallEtcdBinariesStep.
func NewInstallEtcdBinariesStep(instanceName, version, targetDir, installURLBase, arch string, sudo, downloadSudo bool) step.Step {
	s := &InstallEtcdBinariesStep{
		// Meta will be populated by populateAndDetermineInternals after version/arch are determined
		Version:        version,
		TargetDir:      targetDir,
		InstallURLBase: installURLBase,
		Arch:           arch,
		Sudo:           sudo,
		DownloadSudo:   downloadSudo,
		// StepName:    instanceName, // Will be part of meta
	}
	// Set a temporary name for meta until populateAndDetermineInternals can refine it.
	s.meta.Name = instanceName
	if s.meta.Name == "" {
		s.meta.Name = "InstallEtcdBinaries"
	}
	s.meta.Description = "Pending determination of version/arch for full description."
	return s
}

func (s *InstallEtcdBinariesStep) populateAndDetermineInternals(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())

	// Determine Version
	if s.determinedVersion == "" {
		versionToUse := s.Version
		if versionToUse == "" {
			versionToUse = "3.5.9" // Default version (without 'v' initially for user input)
			logger.Warn("Etcd version not specified, defaulting.", "defaultVersion", versionToUse)
		}
		s.determinedVersion = versionToUse // Store user input or default
		if !strings.HasPrefix(s.determinedVersion, "v") {
			s.determinedVersion = "v" + s.determinedVersion // Add 'v' for internal use (URL, archive name)
		}
	}

	// Determine TargetDir
	if s.TargetDir == "" {
		s.TargetDir = "/usr/local/bin"
	}

	// Determine InstallURLBase
	if s.InstallURLBase == "" {
		s.InstallURLBase = "https://github.com/etcd-io/etcd/releases/download"
	}

	// Determine Architecture
	if s.determinedArch == "" {
		archToUse := s.Arch // User input takes precedence
		if archToUse == "" {
			if host != nil {
				facts, err := ctx.GetHostFacts(host)
				if err != nil {
					logger.Error("Failed to get host facts for architecture detection", "error", err)
					archToUse = "amd64" // Fallback
					logger.Warn("Could not auto-detect host architecture (facts error), defaulting.", "defaultArch", archToUse)
				} else if facts.OS != nil && facts.OS.Arch != "" {
					archToUse = facts.OS.Arch
					// Normalize common variations
					if archToUse == "x86_64" {
						archToUse = "amd64"
					}
					if archToUse == "aarch64" {
						archToUse = "arm64"
					}
					logger.Debug("Host architecture determined for etcd", "rawArch", facts.OS.Arch, "usingArch", archToUse)
				} else {
					archToUse = "amd64" // Fallback if OS.Arch is empty
					logger.Warn("Could not auto-detect host architecture (OS.Arch empty), defaulting.", "defaultArch", archToUse)
				}
			} else { // Should not happen if called from Precheck/Run with a valid host
				archToUse = "amd64"
				logger.Error("No host context to auto-detect architecture for etcd, defaulting.", "defaultArch", archToUse)
			}
		}
		s.determinedArch = archToUse
	}

	// Update meta with determined values if name was default
	if s.meta.Name == "InstallEtcdBinaries" || s.meta.Description == "Pending determination of version/arch for full description." {
		if s.meta.Name == "InstallEtcdBinaries" && s.Version != "" { // Only update name if it was the generic default and version is now known
			s.meta.Name = fmt.Sprintf("Install etcd %s", s.Version)
		}
		s.meta.Description = fmt.Sprintf("Downloads, extracts, and installs etcd and etcdctl (version %s, arch %s) to %s.",
			strings.TrimPrefix(s.determinedVersion, "v"), s.determinedArch, s.TargetDir)
	}
	return nil
}

// Meta returns the step's metadata.
func (s *InstallEtcdBinariesStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *InstallEtcdBinariesStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	// Call populateAndDetermineInternals first to set meta.Name correctly for logger
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		// Log with a temporary step name if meta.Name is not yet final
		tempLogger := ctx.GetLogger().With("step", "InstallEtcdBinaries.PrecheckInit", "host", host.GetName())
		tempLogger.Error("Failed to populate internal fields", "error", err)
		return false, err
	}
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	binaries := []string{"etcd", "etcdctl"}
	expectedVersionString := strings.TrimPrefix(s.determinedVersion, "v")

	for _, binName := range binaries {
		binPath := filepath.Join(s.TargetDir, binName)
		exists, errExist := runnerSvc.Exists(ctx.GoContext(), conn, binPath)
		if errExist != nil {
			logger.Warn("Failed to check existence of binary, assuming not installed correctly.", "path", binPath, "error", errExist)
			return false, nil // Let Run attempt.
		}
		if !exists {
			logger.Debug("Etcd binary does not exist.", "path", binPath)
			return false, nil
		}

		versionCmd := ""
		if binName == "etcd" {
			versionCmd = fmt.Sprintf("%s --version", binPath)
		}
		if binName == "etcdctl" {
			versionCmd = fmt.Sprintf("%s version", binPath)
		}

		// Use RunWithOptions for more control and to get CommandError
		stdoutBytes, stderrBytes, execErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, versionCmd, &connector.ExecOptions{Sudo: false, Check: true}) // Check:true means non-zero might not be fatal
		output := string(stdoutBytes) + string(stderrBytes)

		if execErr != nil {
			// If Check:true, execErr might be a CommandError with non-zero exit, or other error
			logger.Warn("Failed to get version of binary (or command failed), assuming not correct.", "path", binPath, "error", execErr, "output", output)
			return false, nil
		}

		versionLineFound := false
		if binName == "etcd" && (strings.Contains(output, "etcd Version: "+expectedVersionString) || strings.Contains(output, "etcd version "+expectedVersionString)) {
			versionLineFound = true
		}
		if binName == "etcdctl" && (strings.Contains(output, "etcdctl version: "+expectedVersionString) || strings.Contains(output, `"etcdserver":"`+expectedVersionString+`"`)) {
			versionLineFound = true
		}

		if !versionLineFound {
			logger.Info("Etcd binary exists, but version does not match.", "path", binPath, "expected", expectedVersionString, "actualOutput", output)
			return false, nil
		}
		logger.Debug("Etcd binary version matches.", "binary", binName, "path", binPath)
	}
	logger.Info("All etcd binaries exist and match version.")
	return true, nil
}

func (s *InstallEtcdBinariesStep) Run(ctx runtime.StepContext, host connector.Host) error {
	// Call populateAndDetermineInternals first to set meta.Name correctly for logger
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		tempLogger := ctx.GetLogger().With("step", "InstallEtcdBinaries.RunInit", "host", host.GetName())
		tempLogger.Error("Failed to populate internal fields", "error", err)
		return err
	}
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}
	facts, err := ctx.GetHostFacts(host) // Needed for runner.DownloadAndExtract
	if err != nil {
		return fmt.Errorf("failed to get host facts for %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", s.determinedVersion, s.determinedArch)
	downloadURL := fmt.Sprintf("%s/%s/%s", s.InstallURLBase, s.determinedVersion, archiveName)

	baseTmpDir := ctx.GetGlobalWorkDir()
	if baseTmpDir == "" {
		baseTmpDir = "/tmp"
	} // Should be set by runtime builder
	safeHostName := strings.ReplaceAll(host.GetName(), "/", "_")
	extractDir := filepath.Join(baseTmpDir, safeHostName, fmt.Sprintf("etcd-extract-%s-%d", s.determinedVersion, time.Now().UnixNano()))

	logger.Info("Downloading and extracting etcd.", "url", downloadURL, "extractDir", extractDir)
	// runner.DownloadAndExtract takes sudo for extraction, typically false for temp dirs.
	if err := runnerSvc.DownloadAndExtract(ctx.GoContext(), conn, facts, downloadURL, extractDir, s.DownloadSudo); err != nil {
		return fmt.Errorf("failed to download and extract etcd from %s for step %s on host %s: %w", downloadURL, s.meta.Name, host.GetName(), err)
	}
	logger.Info("Etcd archive downloaded and extracted.", "path", extractDir)

	extractedBinDir := filepath.Join(extractDir, fmt.Sprintf("etcd-%s-linux-%s", s.determinedVersion, s.determinedArch))

	logger.Info("Ensuring target directory exists.", "path", s.TargetDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.TargetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target directory %s for step %s on host %s: %w", s.TargetDir, s.meta.Name, host.GetName(), err)
	}

	binariesToCopy := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToCopy {
		srcPath := filepath.Join(extractedBinDir, binName)
		dstPath := filepath.Join(s.TargetDir, binName)

		srcExists, errSrcExist := runnerSvc.Exists(ctx.GoContext(), conn, srcPath)
		if errSrcExist != nil {
			logger.Warn("Could not verify existence of source binary, attempting copy anyway.", "path", srcPath, "error", errSrcExist)
		} else if !srcExists {
			return fmt.Errorf("source binary %s not found in extracted path %s for step %s on host %s", binName, extractedBinDir, s.meta.Name, host.GetName())
		}

		logger.Info("Copying binary.", "binary", binName, "destination", dstPath)
		cpCmd := fmt.Sprintf("cp -fp %s %s", srcPath, dstPath)
		if _, errCp := runnerSvc.Run(ctx.GoContext(), conn, cpCmd, s.Sudo); errCp != nil {
			return fmt.Errorf("failed to copy %s to %s for step %s on host %s: %w", srcPath, dstPath, s.meta.Name, host.GetName(), errCp)
		}

		logger.Info("Setting permissions for binary.", "path", dstPath)
		if errChmod := runnerSvc.Chmod(ctx.GoContext(), conn, dstPath, "+x", s.Sudo); errChmod != nil { // +x is more common than 0755 for chmod by runner
			return fmt.Errorf("failed to make %s executable for step %s on host %s: %w", dstPath, s.meta.Name, host.GetName(), errChmod)
		}
	}

	logger.Info("Cleaning up extraction directory.", "path", extractDir)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, extractDir, false); err != nil { // Sudo typically false for temp dir cleanup
		logger.Warn("Failed to cleanup etcd extraction directory (best effort).", "path", extractDir, "error", err)
	}
	logger.Info("Etcd binaries installed successfully.")
	return nil
}

func (s *InstallEtcdBinariesStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// Call populateAndDetermineInternals first to set meta.Name correctly for logger
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		tempLogger := ctx.GetLogger().With("step", "InstallEtcdBinaries.RollbackInit", "host", host.GetName())
		tempLogger.Warn("Could not fully populate internals for rollback, TargetDir might be empty if not set initially.", "error", err)
		// Continue if TargetDir was set initially
		if s.TargetDir == "" {
			tempLogger.Error("TargetDir is not set, cannot perform rollback for etcd binaries.")
			return fmt.Errorf("TargetDir not set for etcd rollback on host %s", host.GetName())
		}
	}
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")


	if s.TargetDir == "" { // Double check after populate
		logger.Error("TargetDir is not set after populate, cannot perform rollback.")
		return fmt.Errorf("TargetDir not set for etcd rollback on host %s after populate", host.GetName())
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.meta.Name, err)
	}

	binariesToRemove := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToRemove {
		binPath := filepath.Join(s.TargetDir, binName)
		logger.Info("Attempting to remove binary for rollback.", "path", binPath)
		if errRm := runnerSvc.Remove(ctx.GoContext(), conn, binPath, s.Sudo); errRm != nil {
			logger.Error("Failed to remove binary during rollback (best effort).", "path", binPath, "error", errRm)
		}
	}
	logger.Info("Rollback attempt for etcd binaries finished.")
	return nil
}

// Ensure InstallEtcdBinariesStep implements the step.Step interface.
var _ step.Step = (*InstallEtcdBinariesStep)(nil)
