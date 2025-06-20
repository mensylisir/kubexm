package etcd

import (
	"context" // Required by runtime.Context
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.ExecOptions
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
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
// This method is called by the executor using runtime context.
func (s *InstallEtcdBinariesStepSpec) applyDefaults(ctx *runtime.Context) {
	if s.Version == "" {
		s.Version = "v3.5.9" // A common, recent version as default
		if ctx != nil && ctx.Logger != nil { // Check for nil ctx and Logger
			ctx.Logger.Warnf("Host %s: Etcd version not specified for install, defaulting to %s", ctx.Host.Name, s.Version)
		}
	}
	if s.TargetDir == "" { s.TargetDir = "/usr/local/bin" }
	if s.InstallURLBase == "" { s.InstallURLBase = "https://github.com/etcd-io/etcd/releases/download" }

	if s.Arch == "" { // Try to auto-detect Arch if not specified
		if ctx != nil && ctx.Host != nil && ctx.Host.Runner != nil &&
			ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil &&
			ctx.Host.Runner.Facts.OS.Arch != "" {
			s.Arch = ctx.Host.Runner.Facts.OS.Arch
		} else {
			s.Arch = "amd64" // Fallback default arch if detection is not possible
			if ctx != nil && ctx.Logger != nil {
				ctx.Logger.Warnf("Host %s: Could not auto-detect architecture for etcd download, defaulting to %s", ctx.Host.Name, s.Arch)
			}
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
func (e *InstallEtcdBinariesStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	rawSpec, rok := ctx.Step().GetCurrentStepSpec()
	if !rok {
		return false, fmt.Errorf("StepSpec not found in context for InstallEtcdBinariesStepExecutor Check method")
	}
	spec, ok := rawSpec.(*InstallEtcdBinariesStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for InstallEtcdBinariesStepExecutor Check method: %T", rawSpec, rawSpec)
	}

	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	spec.applyDefaults(ctx) // Ensure defaults are applied before checking
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		binPath := filepath.Join(spec.TargetDir, binName)
		exists, err := ctx.Host.Runner.Exists(ctx.GoContext, binPath)
		if err != nil { return false, fmt.Errorf("failed to check existence of %s on host %s: %w", binPath, ctx.Host.Name, err) }
		if !exists {
			hostCtxLogger.Debugf("Etcd binary %s does not exist at %s.", binName, binPath)
			return false, nil // Not done if any binary is missing
		}

		versionCmd := ""; expectedVersionString := strings.TrimPrefix(spec.Version, "v")
		if binName == "etcd" { versionCmd = fmt.Sprintf("%s --version", binPath) }
		if binName == "etcdctl" { versionCmd = fmt.Sprintf("%s version", binPath) }

		stdoutBytes, stderrBytes, execErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, versionCmd, &connector.ExecOptions{Sudo: false})
		if execErr != nil {
			hostCtxLogger.Warnf("Failed to get version of %s (command: '%s'): %v. Stderr: %s. Assuming not correct version.", binPath, versionCmd, execErr, string(stderrBytes))
			return false, nil // Cannot verify version, assume re-install needed
		}

		output := string(stdoutBytes)
		versionLineFound := false
		// etcd --version output: "etcd Version: 3.5.9" or "etcd version 3.5.9" (older)
		// etcdctl version output: "etcdctl version: 3.5.9"
		if binName == "etcd" && (strings.Contains(output, "etcd Version: "+expectedVersionString) || strings.Contains(output, "etcd version "+expectedVersionString)){
			versionLineFound = true
		}
		if binName == "etcdctl" && strings.Contains(output, "etcdctl version: "+expectedVersionString) {
			versionLineFound = true
		}

		if !versionLineFound {
			hostCtxLogger.Infof("Etcd binary %s exists, but version does not match '%s'. Found output: %s", binPath, expectedVersionString, output)
			return false, nil // Version mismatch
		}
		hostCtxLogger.Debugf("Etcd binary %s version %s already installed at %s.", binName, expectedVersionString, binPath)
	}
	hostCtxLogger.Infof("All etcd binaries (%s) exist in %s and match version %s.", strings.Join(binaries, ", "), spec.TargetDir, spec.Version)
	return true, nil // All binaries exist and versions match
}

// Execute downloads and installs etcd binaries.
func (e *InstallEtcdBinariesStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	rawSpec, rok := ctx.Step().GetCurrentStepSpec()
	if !rok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for InstallEtcdBinariesStepExecutor Execute method"))
	}
	spec, ok := rawSpec.(*InstallEtcdBinariesStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected spec type %T for InstallEtcdBinariesStepExecutor Execute method: %T", rawSpec, rawSpec))
	}

	spec.applyDefaults(ctx)
	res := step.NewResult(ctx, startTime, nil) // Use new NewResult signature
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", spec.Version, spec.Arch)
	downloadURL := fmt.Sprintf("%s/%s/%s", spec.InstallURLBase, spec.Version, archiveName)

	// Use a host-specific temporary directory for extraction, ideally from ctx.Host.WorkDir if set
	tmpExtractDirBase := ctx.Host.WorkDir
	if tmpExtractDirBase == "" {
		tmpExtractDirBase = "/tmp" // Fallback if no host workdir
	}
	extractDir := filepath.Join(tmpExtractDirBase, fmt.Sprintf("etcd-extract-%s-%d", spec.Version, time.Now().UnixNano()))

	hostCtxLogger.Infof("Downloading and extracting etcd %s from %s to %s", spec.Version, downloadURL, extractDir)
	// Sudo false for download/extract as it's to a temp/work directory.
	if err := ctx.Host.Runner.DownloadAndExtract(ctx.GoContext, downloadURL, extractDir, false); err != nil {
		res.Error = fmt.Errorf("failed to download and extract etcd from %s: %w", downloadURL, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Successf("Etcd archive downloaded and extracted to %s", extractDir)

	extractedBinDir := filepath.Join(extractDir, fmt.Sprintf("etcd-%s-linux-%s", spec.Version, spec.Arch))

	// Sudo true for Mkdirp if TargetDir is a system path like /usr/local/bin
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, spec.TargetDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create target directory %s: %w", spec.TargetDir, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	binariesToCopy := []string{"etcd", "etcdctl"}
	for _, binName := range binariesToCopy {
		srcPath := filepath.Join(extractedBinDir, binName)
		dstPath := filepath.Join(spec.TargetDir, binName)

		hostCtxLogger.Infof("Copying %s from %s to %s", binName, srcPath, dstPath)
		copyCmd := fmt.Sprintf("cp %s %s", srcPath, dstPath)
		// Sudo true for copying to system directories like /usr/local/bin
		_, stderrCp, errCp := ctx.Host.Runner.RunWithOptions(ctx.GoContext, copyCmd, &connector.ExecOptions{Sudo: true})
		if errCp != nil {
			res.Error = fmt.Errorf("failed to copy %s to %s: %w (stderr: %s)", srcPath, dstPath, errCp, string(stderrCp))
			res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}

		chmodCmd := fmt.Sprintf("chmod +x %s", dstPath)
		_, stderrChmod, errChmod := ctx.Host.Runner.RunWithOptions(ctx.GoContext, chmodCmd, &connector.ExecOptions{Sudo: true})
		if errChmod != nil {
			res.Error = fmt.Errorf("failed to make %s executable: %w (stderr: %s)", dstPath, errChmod, string(stderrChmod))
			res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
		hostCtxLogger.Successf("Copied and set +x for %s", dstPath)
	}

	hostCtxLogger.Infof("Cleaning up extraction directory %s", extractDir)
	// Sudo true for Remove if extractDir was created in a place that needs it, or if files are root-owned.
	// Since DownloadAndExtract sudo was false, files inside should be user-owned.
	// However, extractDir itself might be under a sudo-created path if Host.WorkDir was /root/something.
	// Use Sudo true for Remove to be safe, or adjust based on typical Host.WorkDir permissions.
	if err := ctx.Host.Runner.Remove(ctx.GoContext, extractDir, true); err != nil {
		hostCtxLogger.Warnf("Failed to cleanup etcd extraction directory %s: %v. This can be ignored or manually cleaned.", extractDir, err)
	}

	res.EndTime = time.Now()
	hostCtxLogger.Infof("Verifying etcd installation after run...")
	done, checkErr := e.Check(ctx) // Call Check with updated signature
	if checkErr != nil {
		res.Status = step.StatusFailed; res.Error = fmt.Errorf("failed to verify etcd installation after run: %w", checkErr)
		res.Message = res.Error.Error()
	} else if !done {
		res.Status = step.StatusFailed; res.Error = fmt.Errorf("etcd installation verification failed after run (binaries not found or version mismatch)")
		res.Message = res.Error.Error()
	} else {
		res.Status = step.StatusSucceeded; res.Message = fmt.Sprintf("Etcd %s binaries (etcd, etcdctl) installed successfully to %s.", spec.Version, spec.TargetDir)
	}

	if res.Status == step.StatusFailed {
		hostCtxLogger.Errorf("Step finished with errors: %s", res.Message)
	} else {
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	}
	return res
}
var _ step.StepExecutor = &InstallEtcdBinariesStepExecutor{}
