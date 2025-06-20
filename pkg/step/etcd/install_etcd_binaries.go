package etcd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time" // Keep for temporary directory generation based on time

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For DownloadAndExtractWithConnector
)

// InstallEtcdBinariesStep downloads, extracts, and installs etcd and etcdctl binaries.
type InstallEtcdBinariesStep struct {
	Version        string
	TargetDir      string
	InstallURLBase string
	Arch           string // Auto-detected if empty
	StepName       string

	// Internal fields for determined values
	determinedArch    string
	determinedVersion string // Store version with 'v' prefix for consistency in URL
}

// NewInstallEtcdBinariesStep creates a new InstallEtcdBinariesStep.
func NewInstallEtcdBinariesStep(version, targetDir, installURLBase, arch, stepName string) step.Step {
	s := &InstallEtcdBinariesStep{
		Version:        version,
		TargetDir:      targetDir,
		InstallURLBase: installURLBase,
		Arch:           arch,
		StepName:       stepName,
	}
	// Defaults will be applied in populateAndDetermineInternals using context from Precheck/Run
	return s
}

func (s *InstallEtcdBinariesStep) populateAndDetermineInternals(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName())

	// Determine Version
	if s.determinedVersion == "" {
		versionToUse := s.Version
		if versionToUse == "" {
			versionToUse = "v3.5.9" // Default version
			logger.Warn("Etcd version not specified, defaulting.", "defaultVersion", versionToUse)
		}
		if !strings.HasPrefix(versionToUse, "v") {
			versionToUse = "v" + versionToUse
		}
		s.determinedVersion = versionToUse
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
		archToUse := s.Arch
		if archToUse == "" {
			if host != nil {
				// Prefer GetArch directly from host if available and simple
				// facts, err := ctx.GetHostFacts(host) // This is heavier if only arch is needed
				// For now, let's assume host.GetArch() is available and preferred for simplicity
				hostArch := host.GetArch() // Assumes connector.Host has GetArch()
				if hostArch != "" {
					archToUse = hostArch
					if archToUse == "x86_64" { archToUse = "amd64" }
					if archToUse == "aarch64" { archToUse = "arm64" }
					logger.Debug("Host architecture determined for etcd", "rawArch", hostArch, "usingArch", archToUse)
				} else {
					archToUse = "amd64" // Fallback if GetArch() is empty or host is nil earlier
					logger.Warn("Could not auto-detect host architecture for etcd (GetArch empty), defaulting.", "defaultArch", archToUse)
				}
			} else { // Should not happen if called from Precheck/Run with a valid host
				archToUse = "amd64"
				logger.Error("No host context to auto-detect architecture for etcd, defaulting.", "defaultArch", archToUse)
			}
		}
		s.determinedArch = archToUse
	}
	return nil
}

func (s *InstallEtcdBinariesStep) Name() string {
	if s.StepName != "" {
		return s.StepName
	}
	// Use s.Version (original input) for display name consistency if determinedVersion adds 'v'
	return fmt.Sprintf("Install etcd binaries (version %s) to %s", s.Version, s.TargetDir)
}

func (s *InstallEtcdBinariesStep) Description() string {
	// Use s.Arch (original input) for display consistency
	return fmt.Sprintf("Downloads, extracts, and installs etcd and etcdctl version %s to %s on %s architecture.", s.Version, s.TargetDir, s.Arch)
}

func (s *InstallEtcdBinariesStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		logger.Error("Failed to populate internal fields", "error", err)
		return false, err
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	binaries := []string{"etcd", "etcdctl"}
	// Use s.determinedVersion (which has 'v') for consistency, then trim for comparison.
	expectedVersionString := strings.TrimPrefix(s.determinedVersion, "v")

	for _, binName := range binaries {
		binPath := filepath.Join(s.TargetDir, binName)
		exists, errExist := conn.Exists(ctx.GoContext(), binPath)
		if errExist != nil {
			logger.Warn("Failed to check existence of binary, assuming not installed correctly.", "path", binPath, "error", errExist)
			return false, nil // Let Run attempt.
		}
		if !exists {
			logger.Debug("Etcd binary does not exist.", "path", binPath)
			return false, nil
		}

		versionCmd := ""
		if binName == "etcd" { versionCmd = fmt.Sprintf("%s --version", binPath) }
		if binName == "etcdctl" { versionCmd = fmt.Sprintf("%s version", binPath) }

		stdoutBytes, stderrBytes, execErr := conn.Exec(ctx.GoContext(), versionCmd, &connector.ExecOptions{Sudo: false})
		output := string(stdoutBytes) + string(stderrBytes)

		if execErr != nil {
			logger.Warn("Failed to get version of binary, assuming not correct.", "path", binPath, "error", execErr, "output", output)
			return false, nil
		}

		versionLineFound := false
		if binName == "etcd" && (strings.Contains(output, "etcd Version: "+expectedVersionString) || strings.Contains(output, "etcd version "+expectedVersionString)) {
			versionLineFound = true
		}
		// etcdctl v3.4+ version output is like: "etcdctl version: 3.4.13", API version: 3.4"
		// etcdctl v3.5+ version output is JSON-like: {"etcdserver":"3.5.0","etcdcluster":"3.5.0"} or similar with "etcdctl version:"
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
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		logger.Error("Failed to populate internal fields", "error", err)
		return err
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	// Use s.determinedVersion (which has 'v') for filename and URL construction
	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", s.determinedVersion, s.determinedArch)
	downloadURL := fmt.Sprintf("%s/%s/%s", s.InstallURLBase, s.determinedVersion, archiveName)

	baseTmpDir := ctx.GetGlobalWorkDir(); if baseTmpDir == "" { baseTmpDir = "/tmp" }
	safeHostName := strings.ReplaceAll(host.GetName(), "/", "_") // Basic sanitization
	extractDir := filepath.Join(baseTmpDir, safeHostName, fmt.Sprintf("etcd-extract-%s-%d", s.determinedVersion, time.Now().UnixNano()))

	logger.Info("Downloading and extracting etcd.", "url", downloadURL, "extractDir", extractDir)
	// Assume utils.DownloadAndExtractWithConnector handles mkdir for extractDir. Sudo for extraction is false.
	if err := utils.DownloadAndExtractWithConnector(ctx.GoContext(), logger, conn, downloadURL, extractDir, false); err != nil {
		return fmt.Errorf("failed to download and extract etcd from %s for step %s on host %s: %w", downloadURL, s.Name(), host.GetName(), err)
	}
	logger.Info("Etcd archive downloaded and extracted.", "path", extractDir)

	// Path inside tarball is usually etcd-vX.Y.Z-linux-ARCH/
	extractedBinDir := filepath.Join(extractDir, fmt.Sprintf("etcd-%s-linux-%s", s.determinedVersion, s.determinedArch))

	// Ensure target directory exists (e.g., /usr/local/bin)
	// Using Exec for mkdir -p with sudo, as conn.Mkdir might not directly support sudo.
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.TargetDir)
	execOptsSudo := &connector.ExecOptions{Sudo: true}
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOptsSudo)
	if errMkdir != nil {
		return fmt.Errorf("failed to create target directory %s (stderr: %s) for step %s on host %s: %w", s.TargetDir, string(stderrMkdir), s.Name(), host.GetName(), errMkdir)
	}

	binariesToCopy := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToCopy {
		srcPath := filepath.Join(extractedBinDir, binName)
		dstPath := filepath.Join(s.TargetDir, binName)

		srcExists, errSrcExist := conn.Exists(ctx.GoContext(), srcPath)
		if errSrcExist != nil {
		    logger.Warn("Could not verify existence of source binary, attempting copy anyway.", "path", srcPath, "error", errSrcExist)
		} else if !srcExists {
		    return fmt.Errorf("source binary %s not found in extracted path %s for step %s on host %s", binName, extractedBinDir, s.Name(), host.GetName())
		}

		logger.Info("Copying binary.", "binary", binName, "destination", dstPath)
		cpCmd := fmt.Sprintf("cp -fp %s %s", srcPath, dstPath)
		_, stderrCp, errCp := conn.Exec(ctx.GoContext(), cpCmd, execOptsSudo)
		if errCp != nil {
			return fmt.Errorf("failed to copy %s to %s (stderr: %s) for step %s on host %s: %w", srcPath, dstPath, string(stderrCp), s.Name(), host.GetName(), errCp)
		}

		chmodCmd := fmt.Sprintf("chmod +x %s", dstPath)
		_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, execOptsSudo)
		if errChmod != nil {
			return fmt.Errorf("failed to make %s executable (stderr: %s) for step %s on host %s: %w", dstPath, string(stderrChmod), s.Name(), host.GetName(), errChmod)
		}
	}

	logger.Info("Cleaning up extraction directory.", "path", extractDir)
	// Assuming conn.Remove handles sudo if needed based on connector setup, for a temp dir it might not need sudo.
	// However, since it's in GlobalWorkDir which could be privileged, being cautious or using Exec for rm -rf.
	// For now, using conn.Remove and assuming it's fine for a subdir of GlobalWorkDir.
	if err := conn.Remove(ctx.GoContext(), extractDir, connector.RemoveOptions{Recursive: true, IgnoreNotExist: true}); err != nil {
		logger.Warn("Failed to cleanup etcd extraction directory (best effort).", "path", extractDir, "error", err)
	}
	logger.Info("Etcd binaries installed successfully.")
	return nil
}

func (s *InstallEtcdBinariesStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")
	// Populate internals to ensure TargetDir is set, especially if called standalone or after a Precheck=true
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
        // If host is nil here (e.g. context is not fully host-specific for some reason during cleanup),
        // populateAndDetermineInternals might have issues. TargetDir might remain empty.
		logger.Warn("Could not fully populate internals for rollback, TargetDir might be empty if not set initially.", "error", err)
	}

	if s.TargetDir == "" { // Should be set by populateAndDetermineInternals or constructor
	    logger.Error("TargetDir is not set, cannot perform rollback for etcd binaries.")
		return fmt.Errorf("TargetDir not set for etcd rollback on host %s", host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.Name(), err)
	}

	execOptsSudo := &connector.ExecOptions{Sudo: true}
	binariesToRemove := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToRemove {
		binPath := filepath.Join(s.TargetDir, binName)
		logger.Info("Attempting to remove binary for rollback.", "path", binPath)
		rmCmd := fmt.Sprintf("rm -f %s", binPath)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOptsSudo)
		if errRm != nil {
			logger.Error("Failed to remove binary during rollback (best effort).", "path", binPath, "stderr", string(stderrRm), "error", errRm)
		}
	}
	logger.Info("Rollback attempt for etcd binaries finished.")
	return nil
}

// Ensure InstallEtcdBinariesStep implements the step.Step interface.
var _ step.Step = (*InstallEtcdBinariesStep)(nil)
