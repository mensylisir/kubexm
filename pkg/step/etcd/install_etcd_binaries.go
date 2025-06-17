package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// InstallEtcdBinariesStep downloads and extracts etcd binaries.
type InstallEtcdBinariesStep struct {
	// Version specifies the etcd version to install (e.g., "v3.5.9").
	Version string
	// TargetDir is the directory where etcd binaries (etcd, etcdctl) will be placed (e.g., "/usr/local/bin").
	TargetDir string
	// InstallURLBase is the base URL for downloading etcd releases.
	// If empty, a default like "https://github.com/etcd-io/etcd/releases/download" will be used.
	InstallURLBase string
	// Arch can be used to specify architecture like "amd64", "arm64".
	// If empty, it will be auto-detected from Runner.Facts.OS.Arch.
	Arch string
}

// Name returns a human-readable name for the step.
func (s *InstallEtcdBinariesStep) Name() string {
	// Use effective values in name if possible, but they are set in defaultValues which needs context.
	// For now, use the struct's direct values.
	version := s.Version
	if version == "" { version = "default" }
	targetDir := s.TargetDir
	if targetDir == "" { targetDir = "/usr/local/bin" }
	return fmt.Sprintf("Install etcd binaries (version %s) to %s", version, targetDir)
}

// defaultValues sets default values for the step's fields if they are not provided.
// It uses runtime context for auto-detection (e.g., architecture).
func (s *InstallEtcdBinariesStep) defaultValues(ctx *runtime.Context) {
	if s.Version == "" {
		s.Version = "v3.5.9" // Specify a recent, common default
		if ctx != nil && ctx.Logger != nil { // Check for nil ctx and Logger
			ctx.Logger.Warnf("Etcd version not specified for host %s, defaulting to %s", ctx.Host.Name, s.Version)
		}
	}
	if s.TargetDir == "" {
		s.TargetDir = "/usr/local/bin"
	}
	if s.InstallURLBase == "" {
		s.InstallURLBase = "https://github.com/etcd-io/etcd/releases/download"
	}
	if s.Arch == "" {
		if ctx != nil && ctx.Host != nil && ctx.Host.Runner != nil &&
			ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil &&
			ctx.Host.Runner.Facts.OS.Arch != "" {
			s.Arch = ctx.Host.Runner.Facts.OS.Arch
		} else {
			s.Arch = "amd64" // Fallback default arch
			if ctx != nil && ctx.Logger != nil {
				ctx.Logger.Warnf("Could not auto-detect architecture for etcd download on host %s, defaulting to %s", ctx.Host.Name, s.Arch)
			}
		}
	}
}

// Check determines if the etcd binaries (etcd, etcdctl) already exist in TargetDir and match the specified version.
func (s *InstallEtcdBinariesStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	s.defaultValues(ctx) // Ensure defaults are applied before checking
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()


	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		binPath := filepath.Join(s.TargetDir, binName)
		exists, err := ctx.Host.Runner.Exists(ctx.GoContext, binPath)
		if err != nil {
			return false, fmt.Errorf("failed to check existence of %s on host %s: %w", binPath, ctx.Host.Name, err)
		}
		if !exists {
			hostCtxLogger.Debugf("Etcd binary %s does not exist at %s.", binName, binPath)
			return false, nil // Not done if any binary is missing
		}

		// Check version
		versionCmd := ""
		if binName == "etcd" {
			versionCmd = fmt.Sprintf("%s --version", binPath)
		} else if binName == "etcdctl" {
			versionCmd = fmt.Sprintf("%s version", binPath) // etcdctl uses 'version'
		} else {
			hostCtxLogger.Warnf("Unknown binary name %s for version check.", binName)
			return false, nil // Should not happen with current binaries list
		}

		stdout, stderr, execErr := ctx.Host.Runner.Run(ctx.GoContext, versionCmd, false) // Sudo typically not needed for --version
		if execErr != nil {
			hostCtxLogger.Warnf("Failed to get version of %s (command: '%s') on host %s: %v. Stderr: %s. Assuming not correct version.", binPath, versionCmd, ctx.Host.Name, execErr, string(stderr))
			return false, nil // Cannot verify version, assume re-install needed
		}

		output := string(stdout)
		// etcd --version output: "etcd Version: 3.5.9 ..." or "etcd version 3.5.9 ..."
		// etcdctl version output: "etcdctl version: 3.5.9 ..."
		// We need to trim "v" from s.Version for comparison if it exists.
		cleanStepVersion := strings.TrimPrefix(s.Version, "v")

		versionLineFound := false
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if binName == "etcd" && (strings.Contains(trimmedLine, "etcd Version: "+cleanStepVersion) || strings.Contains(trimmedLine, "etcd version "+cleanStepVersion)) {
				versionLineFound = true; break
			}
			if binName == "etcdctl" && strings.Contains(trimmedLine, "etcdctl version: "+cleanStepVersion) {
				versionLineFound = true; break
			}
		}

		if !versionLineFound {
			hostCtxLogger.Infof("Etcd binary %s exists, but version does not match '%s'. Found output: %s", binPath, cleanStepVersion, output)
			return false, nil // Version mismatch
		}
		hostCtxLogger.Debugf("Etcd binary %s version %s already installed at %s.", binName, cleanStepVersion, binPath)
	}

	hostCtxLogger.Infof("All etcd binaries (%s) exist in %s and match version %s.", strings.Join(binaries, ", "), s.TargetDir, s.Version)
	return true, nil // All binaries exist and versions match
}

// Run downloads the specified version of etcd, extracts it, and copies binaries to TargetDir.
func (s *InstallEtcdBinariesStep) Run(ctx *runtime.Context) *step.Result {
	s.defaultValues(ctx) // Apply defaults using context
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", s.Version, s.Arch)
	downloadURL := fmt.Sprintf("%s/%s/%s", s.InstallURLBase, s.Version, archiveName)

	// Use a more specific temporary directory for extraction to avoid conflicts if /tmp is shared or not cleaned.
	extractDir := filepath.Join(ctx.Host.WorkDir, fmt.Sprintf("etcd-extract-%s-%d", s.Version, time.Now().UnixNano()))
	// If ctx.Host.WorkDir is empty, runner's DownloadAndExtract might default to /tmp for the archive,
	// but we control the final extraction dir for binaries here.

	hostCtxLogger.Infof("Downloading and extracting etcd %s from %s to %s", s.Version, downloadURL, extractDir)
	// Sudo false for download/extract to a workdir (assuming workdir is user-writable or step manages its creation).
	if err := ctx.Host.Runner.DownloadAndExtract(ctx.GoContext, downloadURL, extractDir, false); err != nil {
		res.Error = fmt.Errorf("failed to download and extract etcd from %s on host %s: %w", downloadURL, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Successf("Etcd archive downloaded and extracted to %s", extractDir)

	extractedBinDir := filepath.Join(extractDir, fmt.Sprintf("etcd-%s-linux-%s", s.Version, s.Arch))

	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, s.TargetDir, "0755", true); err != nil { // Sudo true for system target dirs
		res.Error = fmt.Errorf("failed to create target directory %s on host %s: %w", s.TargetDir, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	binariesToCopy := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToCopy {
		srcPath := filepath.Join(extractedBinDir, binName)
		dstPath := filepath.Join(s.TargetDir, binName)

		hostCtxLogger.Infof("Copying %s from %s to %s", binName, srcPath, dstPath)
		// Use `cp` command with sudo for placing binaries into system locations like /usr/local/bin
		copyCmd := fmt.Sprintf("cp %s %s", srcPath, dstPath)
		_, stderrCp, errCp := ctx.Host.Runner.Run(ctx.GoContext, copyCmd, true)
		if errCp != nil {
			res.Error = fmt.Errorf("failed to copy %s to %s on host %s: %w (stderr: %s)", srcPath, dstPath, ctx.Host.Name, errCp, string(stderrCp))
			res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}

		chmodCmd := fmt.Sprintf("chmod +x %s", dstPath)
		_, stderrChmod, errChmod := ctx.Host.Runner.Run(ctx.GoContext, chmodCmd, true)
		if errChmod != nil {
			res.Error = fmt.Errorf("failed to make %s executable on host %s: %w (stderr: %s)", dstPath, ctx.Host.Name, errChmod, string(stderrChmod))
			res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
		hostCtxLogger.Successf("Copied and set +x for %s", dstPath)
	}

	hostCtxLogger.Infof("Cleaning up extraction directory %s", extractDir)
	// Sudo true for remove if files within extractDir might have been created with sudo by extract,
	// or if extractDir itself was made with sudo (though DownloadAndExtract with sudo=false shouldn't do that).
	// Assuming extractDir is within a user-writable space or DownloadAndExtract handles permissions.
	// If extractDir is based on Host.WorkDir, it might not need sudo for removal.
	// Let's assume sudo might be needed if any part of extraction escalated.
	if err := ctx.Host.Runner.Remove(ctx.GoContext, extractDir, true); err != nil {
		hostCtxLogger.Warnf("Failed to cleanup etcd extraction directory %s on host %s: %v. This can be ignored or manually cleaned.", extractDir, ctx.Host.Name, err)
	}

	res.EndTime = time.Now()
	// Final verification by calling Check method again
	hostCtxLogger.Infof("Verifying etcd installation after run...")
	done, checkErr := s.Check(ctx)
	if checkErr != nil {
		res.Status = "Failed"
		res.Error = fmt.Errorf("failed to verify etcd installation after run on host %s: %w", ctx.Host.Name, checkErr)
		res.Message = res.Error.Error()
	} else if !done {
		res.Status = "Failed"
		res.Error = fmt.Errorf("etcd installation verification failed after run on host %s (binaries not found or version mismatch)", ctx.Host.Name)
		res.Message = res.Error.Error()
	} else {
		res.Status = "Succeeded"
		res.Message = fmt.Sprintf("Etcd %s binaries (etcd, etcdctl) installed successfully to %s on host %s.", s.Version, s.TargetDir, ctx.Host.Name)
	}

	hostCtxLogger.Infof("Step finished with status '%s': %s", res.Status, res.Message)
	return res
}

var _ step.Step = &InstallEtcdBinariesStep{}
