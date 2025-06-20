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
// and installing the binaries.
type ExtractEtcdStepSpec struct {
	spec.StepMeta // Embed common meta fields

	ArchivePathKey   string `json:"archivePathKey,omitempty"`   // Key to retrieve the downloaded archive path from shared data (e.g., EtcdDownloadedArchiveKey)
	Version          string `json:"version,omitempty"`          // Etcd version, e.g., "v3.5.9". Used for determining path inside tarball.
	Arch             string `json:"arch,omitempty"`             // Architecture, e.g., "amd64". Used for determining path inside tarball.
	TargetInstallDir string `json:"targetInstallDir,omitempty"` // Directory to install etcd/etcdctl binaries, e.g., "/usr/local/bin"
	// ExtractedBinDirKey string `json:"extractedBinDirKey,omitempty"` // Optional: Key to store the path of the temp extraction dir that contains the binaries
}

// NewExtractEtcdStepSpec creates a new ExtractEtcdStepSpec.
func NewExtractEtcdStepSpec(stepName, archivePathKey, version, arch, targetInstallDir string) *ExtractEtcdStepSpec {
	if stepName == "" {
		stepName = "Extract and Install Etcd Binaries"
	}

	inKey := archivePathKey
	if inKey == "" {
		inKey = EtcdDownloadedArchiveKey // Default key from download step
	}

	normalizedVersion := version
	if !strings.HasPrefix(normalizedVersion, "v") && normalizedVersion != "" {
		normalizedVersion = "v" + normalizedVersion
	}
	// Version and Arch are important for constructing the path within the tarball,
	// e.g., etcd-v3.5.9-linux-amd64/etcd
	// If version or arch is empty, the executor might need to determine them or raise an error.

	installDir := targetInstallDir
	if installDir == "" {
		installDir = "/usr/local/bin" // Default install directory
	}

	return &ExtractEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Extracts etcd binaries from archive (input key: %s, version: %s, arch: %s) and installs them to %s.", inKey, normalizedVersion, arch, installDir),
		},
		ArchivePathKey:   inKey,
		Version:          normalizedVersion,
		Arch:             arch,
		TargetInstallDir: installDir,
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


// Precheck checks if the etcd binaries already exist in the target directory.
func (s *ExtractEtcdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.TargetInstallDir == "" || s.Version == "" || s.Arch == "" {
		return false, fmt.Errorf("TargetInstallDir, Version, and Arch must be specified for step: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	binaries := []string{"etcd", "etcdctl"} // etcdutl is sometimes included too
	expectedVersionString := strings.TrimPrefix(s.Version, "v")

	for _, binName := range binaries {
		binPath := filepath.Join(s.TargetInstallDir, binName)
		exists, errExist := conn.Exists(ctx.GoContext(), binPath)
		if errExist != nil {
			logger.Warn("Failed to check existence of binary, assuming not installed.", "path", binPath, "error", errExist)
			return false, nil
		}
		if !exists {
			logger.Debug("Etcd binary does not exist.", "path", binPath)
			return false, nil
		}

		// Check version (optional, but good for etcd)
		var versionCmd string
		if binName == "etcd" { versionCmd = fmt.Sprintf("%s --version", binPath) }
		if binName == "etcdctl" { versionCmd = fmt.Sprintf("%s version", binPath) }

		if versionCmd != "" {
			stdoutBytes, stderrBytes, execErr := conn.Exec(ctx.GoContext(), versionCmd, &connector.ExecOptions{})
			output := string(stdoutBytes) + string(stderrBytes)
			if execErr != nil {
				logger.Warn("Failed to get version of binary, assuming not correct.", "path", binPath, "error", execErr, "output", output)
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
				logger.Info("Etcd binary exists, but version mismatch.", "path", binPath, "expected", expectedVersionString, "output", output)
				return false, nil
			}
			logger.Debug("Etcd binary version matches.", "binary", binName, "path", binPath)
		}
	}
	logger.Info("All etcd binaries exist in target directory and match version.", "dir", s.TargetInstallDir)
	return true, nil
}

// Run extracts the etcd archive and installs binaries.
func (s *ExtractEtcdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	archivePath, err := s.getEffectiveArchivePath(ctx)
	if err != nil {
		return err
	}
	if s.TargetInstallDir == "" || s.Version == "" || s.Arch == "" {
		return fmt.Errorf("TargetInstallDir, Version, and Arch must be specified for step: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Create a temporary extraction directory
	baseTmpDir := ctx.GetGlobalWorkDir()
	if baseTmpDir == "" {
		baseTmpDir = "/tmp"
	}
	// Sanitize host name for directory creation
	safeHostName := strings.ReplaceAll(host.GetName(), "/", "_")
	tempExtractDir := filepath.Join(baseTmpDir, safeHostName, fmt.Sprintf("etcd-extract-temp-%s-%d", s.Version, time.Now().UnixNano()))

	defer func() {
		logger.Debug("Cleaning up temporary extraction directory.", "path", tempExtractDir)
		if err := conn.Remove(ctx.GoContext(), tempExtractDir, connector.RemoveOptions{Recursive: true, IgnoreNotExist: true}); err != nil {
			logger.Warn("Failed to cleanup temporary extraction directory.", "path", tempExtractDir, "error", err)
		}
	}()

	logger.Info("Ensuring temporary extraction directory exists.", "path", tempExtractDir)
	// Sudo for mkdir depends on baseTmpDir permissions. Assume utils.PathRequiresSudo.
	mkdirOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(tempExtractDir)}
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), fmt.Sprintf("mkdir -p %s", tempExtractDir), mkdirOpts)
	if errMkdir != nil {
		return fmt.Errorf("failed to create temporary extraction directory %s (stderr: %s): %w", tempExtractDir, string(stderrMkdir), errMkdir)
	}

	logger.Info("Extracting etcd archive.", "archive", archivePath, "destination", tempExtractDir)
	// tar -xzf etcd-v3.5.9-linux-amd64.tar.gz -C /tmp/etcd-extract-temp --strip-components=1 etcd-v3.5.9-linux-amd64/etcd etcd-v3.5.9-linux-amd64/etcdctl
	archiveInternalDir := fmt.Sprintf("etcd-%s-linux-%s", s.Version, s.Arch)
	filesToExtract := []string{
		filepath.Join(archiveInternalDir, "etcd"),
		filepath.Join(archiveInternalDir, "etcdctl"),
		// filepath.Join(archiveInternalDir, "etcdutl"), // If needed
	}
	// Sudo for tar command if tempExtractDir requires it (less likely for /tmp, more for GlobalWorkDir if privileged)
	tarOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(tempExtractDir)}
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s --strip-components=1 %s",
		archivePath,
		tempExtractDir,
		strings.Join(filesToExtract, " "))

	_, stderrTar, errTar := conn.Exec(ctx.GoContext(), extractCmd, tarOpts)
	if errTar != nil {
		return fmt.Errorf("failed to extract etcd binaries from %s (stderr: %s): %w", archivePath, string(stderrTar), errTar)
	}

	// Ensure target installation directory exists
	logger.Info("Ensuring target installation directory exists.", "path", s.TargetInstallDir)
	mkdirInstallOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.TargetInstallDir)}
	_, stderrMkdirInstall, errMkdirInstall := conn.Exec(ctx.GoContext(), fmt.Sprintf("mkdir -p %s", s.TargetInstallDir), mkdirInstallOpts)
	if errMkdirInstall != nil {
		return fmt.Errorf("failed to create target install directory %s (stderr: %s): %w", s.TargetInstallDir, string(stderrMkdirInstall), errMkdirInstall)
	}

	// Move binaries to target directory and set permissions
	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		srcPath := filepath.Join(tempExtractDir, binName)
		dstPath := filepath.Join(s.TargetInstallDir, binName)

		logger.Info("Moving etcd binary to target location.", "binary", binName, "destination", dstPath)
		mvOpts := &connector.ExecOptions{Sudo: true} // Assume /usr/local/bin needs sudo
		mvCmd := fmt.Sprintf("mv -f %s %s", srcPath, dstPath)
		_, stderrMv, errMv := conn.Exec(ctx.GoContext(), mvCmd, mvOpts)
		if errMv != nil {
			return fmt.Errorf("failed to move %s to %s (stderr: %s): %w", srcPath, dstPath, string(stderrMv), errMv)
		}

		chmodCmd := fmt.Sprintf("chmod +x %s", dstPath)
		_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, mvOpts) // mvOpts has Sudo:true
		if errChmod != nil {
			return fmt.Errorf("failed to make %s executable (stderr: %s): %w", dstPath, string(stderrChmod), errChmod)
		}
	}
	logger.Info("Etcd binaries installed successfully.", "dir", s.TargetInstallDir)
	return nil
}

// Rollback removes the installed etcd binaries.
func (s *ExtractEtcdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	if s.TargetInstallDir == "" {
		logger.Info("TargetInstallDir is empty, cannot determine files to roll back.")
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		binPath := filepath.Join(s.TargetInstallDir, binName)
		logger.Info("Attempting to remove etcd binary.", "path", binPath)
		rmOpts := &connector.ExecOptions{Sudo: true} // Assume /usr/local/bin needs sudo
		rmCmd := fmt.Sprintf("rm -f %s", binPath)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, rmOpts)
		if errRm != nil {
			logger.Error("Failed to remove etcd binary during rollback (best effort).", "path", binPath, "stderr", string(stderrRm), "error", errRm)
		} else {
			logger.Info("Etcd binary removed successfully.", "path", binPath)
		}
	}
	return nil
}

var _ step.Step = (*ExtractEtcdStepSpec)(nil)
