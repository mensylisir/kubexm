package etcd

import (
	"context" // Required by runtime.Context
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.ExecOptions
	"github.com/mensylisir/kubexm/pkg/runtime"   // For runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For potential DownloadAndExtractWithConnector
)

// InstallEtcdBinariesStepSpec defines parameters for installing etcd binaries.
type InstallEtcdBinariesStepSpec struct {
	Version        string
	TargetDir      string // e.g., "/usr/local/bin"
	InstallURLBase string // e.g., "https://github.com/etcd-io/etcd/releases/download"
	Arch           string // e.g., "amd64", "arm64". Auto-detected if empty.
	StepName       string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *InstallEtcdBinariesStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	// Use applied defaults for naming if possible, but spec might be named before defaults applied.
	td := s.TargetDir; if td == "" { td = "/usr/local/bin"}
	ver := s.Version; if ver == "" { ver = "default" } // "default" indicates it will be resolved later
	return fmt.Sprintf("Install etcd binaries (version %s) to %s", ver, td)
}

// applyDefaults sets default values for the spec if they are not provided.
func (s *InstallEtcdBinariesStepSpec) applyDefaults(ctx runtime.StepContext) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	hostName := "unknown"
	if currentHost != nil {
		hostName = currentHost.GetName()
	}

	if s.Version == "" {
		s.Version = "v3.5.9" // A common, recent version as default
		logger.Warn("Etcd version not specified for install, defaulting.", "host", hostName, "defaultVersion", s.Version)
	}
	if s.TargetDir == "" { s.TargetDir = "/usr/local/bin" }
	if s.InstallURLBase == "" { s.InstallURLBase = "https://github.com/etcd-io/etcd/releases/download" }

	if s.Arch == "" {
		if currentHost != nil {
			facts, err := ctx.GetHostFacts(currentHost)
			if err == nil && facts != nil && facts.OS != nil && facts.OS.Arch != "" {
				s.Arch = facts.OS.Arch
				// Translate common arch names if needed (e.g., x86_64 -> amd64)
				if s.Arch == "x86_64" { s.Arch = "amd64" }
				if s.Arch == "aarch64" { s.Arch = "arm64" }
			} else {
				s.Arch = "amd64" // Fallback default arch
				logger.Warn("Could not auto-detect architecture for etcd download, defaulting.", "host", hostName, "defaultArch", s.Arch, "error", err)
			}
		} else {
			s.Arch = "amd64" // Fallback if no host context
			logger.Warn("No host context to auto-detect architecture for etcd download, defaulting.", "defaultArch", s.Arch)
		}
	}
}
var _ spec.StepSpec = &InstallEtcdBinariesStepSpec{}

// InstallEtcdBinariesStepExecutor implements logic for InstallEtcdBinariesStepSpec.
type InstallEtcdBinariesStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&InstallEtcdBinariesStepSpec{}), &InstallEtcdBinariesStepExecutor{})
}

// Check determines if etcd binaries are installed and match version.
func (e *InstallEtcdBinariesStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for InstallEtcdBinariesStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for InstallEtcdBinariesStepExecutor Check method")
	}
	spec, ok := rawSpec.(*InstallEtcdBinariesStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected spec type %T for InstallEtcdBinariesStepExecutor Check method", rawSpec)
	}

	spec.applyDefaults(ctx) // Pass StepContext
	logger = logger.With("step", spec.GetName())

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		binPath := filepath.Join(spec.TargetDir, binName)
		exists, err := conn.Exists(goCtx, binPath) // Use connector
		if err != nil {
			logger.Error("Failed to check existence of binary", "path", binPath, "error", err)
			return false, fmt.Errorf("failed to check existence of %s on host %s: %w", binPath, currentHost.GetName(), err)
		}
		if !exists {
			logger.Debug("Etcd binary does not exist.", "path", binPath)
			return false, nil
		}

		versionCmd := ""; expectedVersionString := strings.TrimPrefix(spec.Version, "v")
		if binName == "etcd" { versionCmd = fmt.Sprintf("%s --version", binPath) }
		if binName == "etcdctl" { versionCmd = fmt.Sprintf("%s version", binPath) }

		stdoutBytes, stderrBytes, execErr := conn.RunCommand(goCtx, versionCmd, &connector.ExecOptions{Sudo: false}) // Use connector
		if execErr != nil {
			logger.Warn("Failed to get version of binary. Assuming not correct version.", "path", binPath, "command", versionCmd, "error", execErr, "stderr", string(stderrBytes))
			return false, nil
		}

		output := string(stdoutBytes)
		versionLineFound := false
		if binName == "etcd" && (strings.Contains(output, "etcd Version: "+expectedVersionString) || strings.Contains(output, "etcd version "+expectedVersionString)){
			versionLineFound = true
		}
		if binName == "etcdctl" && strings.Contains(output, "etcdctl version: "+expectedVersionString) {
			versionLineFound = true
		}

		if !versionLineFound {
			logger.Info("Etcd binary exists, but version does not match.", "path", binPath, "expected", expectedVersionString, "output", output)
			return false, nil
		}
		logger.Debug("Etcd binary version already installed.", "binary", binName, "version", expectedVersionString, "path", binPath)
	}
	logger.Info("All etcd binaries exist and match version.", "targetDir", spec.TargetDir, "version", spec.Version)
	return true, nil
}

// Execute downloads and installs etcd binaries.
func (e *InstallEtcdBinariesStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for InstallEtcdBinariesStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for InstallEtcdBinariesStepExecutor Execute method")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*InstallEtcdBinariesStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected spec type %T for InstallEtcdBinariesStepExecutor Execute method", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	spec.applyDefaults(ctx) // Pass StepContext
	logger = logger.With("step", spec.GetName())

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", spec.Version, spec.Arch)
	downloadURL := fmt.Sprintf("%s/%s/%s", spec.InstallURLBase, spec.Version, archiveName)

	// Determine temporary extraction directory using global work dir + host-specific + unique name
	// Fallback to /tmp if global work dir is not set (though it should be by builder).
	baseTmpDir := ctx.GetGlobalWorkDir()
	if baseTmpDir == "" { baseTmpDir = "/tmp"}
	extractDir := filepath.Join(baseTmpDir, currentHost.GetName(), fmt.Sprintf("etcd-extract-%s-%d", spec.Version, time.Now().UnixNano()))

	logger.Info("Downloading and extracting etcd.", "version", spec.Version, "url", downloadURL, "extractDir", extractDir)

	// Assuming a utility function that uses the connector to download and extract.
	// This function would handle MkdirAll for extractDir and the extraction process.
	// Sudo for DownloadAndExtractWithConnector might be false if extractDir is user-writable.
	if err := utils.DownloadAndExtractWithConnector(goCtx, logger, conn, downloadURL, extractDir, false /*sudo for extraction*/); err != nil {
		logger.Error("Failed to download and extract etcd.", "url", downloadURL, "error", err)
		res.Error = fmt.Errorf("failed to download and extract etcd from %s: %w", downloadURL, err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Etcd archive downloaded and extracted.", "extractDir", extractDir)

	extractedBinDir := filepath.Join(extractDir, fmt.Sprintf("etcd-%s-linux-%s", spec.Version, spec.Arch))

	if err := conn.Mkdir(goCtx, spec.TargetDir, "0755"); err != nil { // Sudo for Mkdir should be handled by connector if path needs it
		logger.Error("Failed to create target directory.", "path", spec.TargetDir, "error", err)
		res.Error = fmt.Errorf("failed to create target directory %s: %w", spec.TargetDir, err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	binariesToCopy := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToCopy {
		srcPath := filepath.Join(extractedBinDir, binName)
		dstPath := filepath.Join(spec.TargetDir, binName)

		logger.Info("Copying binary.", "binary", binName, "source", srcPath, "destination", dstPath)
		copyCmd := fmt.Sprintf("cp %s %s", srcPath, dstPath)
		_, stderrCp, errCp := conn.RunCommand(goCtx, copyCmd, &connector.ExecOptions{Sudo: true})
		if errCp != nil {
			logger.Error("Failed to copy binary.", "binary", binName, "stderr", string(stderrCp), "error", errCp)
			res.Error = fmt.Errorf("failed to copy %s to %s: %w (stderr: %s)", srcPath, dstPath, errCp, string(stderrCp))
			res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
		}

		chmodCmd := fmt.Sprintf("chmod +x %s", dstPath)
		_, stderrChmod, errChmod := conn.RunCommand(goCtx, chmodCmd, &connector.ExecOptions{Sudo: true})
		if errChmod != nil {
			logger.Error("Failed to make binary executable.", "path", dstPath, "stderr", string(stderrChmod), "error", errChmod)
			res.Error = fmt.Errorf("failed to make %s executable: %w (stderr: %s)", dstPath, errChmod, string(stderrChmod))
			res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
		}
		logger.Info("Copied and set +x for binary.", "path", dstPath)
	}

	logger.Info("Cleaning up extraction directory.", "path", extractDir)
	if err := conn.Remove(goCtx, extractDir, true); err != nil { // true for recursive
		logger.Warn("Failed to cleanup etcd extraction directory. This can be ignored or manually cleaned.", "path", extractDir, "error", err)
	}

	res.EndTime = time.Now()
	logger.Info("Verifying etcd installation after execution.")
	done, checkErr := e.Check(ctx)
	if checkErr != nil {
		logger.Error("Post-execution check failed.", "error", checkErr)
		res.Status = step.StatusFailed; res.Error = fmt.Errorf("failed to verify etcd installation after run: %w", checkErr)
	} else if !done {
		logger.Error("Post-execution check indicates installation not complete.")
		res.Status = step.StatusFailed; res.Error = fmt.Errorf("etcd installation verification failed after run (binaries not found or version mismatch)")
	} else {
		res.Status = step.StatusSucceeded; res.Message = fmt.Sprintf("Etcd %s binaries (etcd, etcdctl) installed successfully to %s.", spec.Version, spec.TargetDir)
	}

	if res.Status == step.StatusFailed && res.Error != nil { // Ensure message is set if error happened
	    res.Message = res.Error.Error()
		logger.Error("Step finished with errors.", "message", res.Message)
	} else if res.Status == step.StatusSucceeded {
		logger.Info("Step succeeded.", "message", res.Message)
	}
	return res
}
var _ step.StepExecutor = &InstallEtcdBinariesStepExecutor{}
