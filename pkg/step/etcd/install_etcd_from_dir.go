package etcd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/octo-cli/core/pkg/runtime"
	"github.com/octo-cli/core/pkg/step"
	"github.com/octo-cli/core/pkg/step/spec"
)

// InstallEtcdFromDirStepSpec defines the specification for installing etcd binaries
// from a directory to the target binary directory.
type InstallEtcdFromDirStepSpec struct {
	TargetBinDir string `json:"targetBinDir"`
}

// GetName returns the name of the step.
func (s *InstallEtcdFromDirStepSpec) GetName() string {
	return "InstallEtcdFromDir"
}

// ApplyDefaults applies default values to the spec.
func (s *InstallEtcdFromDirStepSpec) ApplyDefaults(ctx *runtime.Context) error {
	if s.TargetBinDir == "" {
		s.TargetBinDir = "/usr/local/bin" // A common default for binaries
	}
	return nil
}

// InstallEtcdFromDirStepExecutor implements the logic for installing etcd binaries.
type InstallEtcdFromDirStepExecutor struct{}

func (e *InstallEtcdFromDirStepExecutor) getExpectedVersion(ctx *runtime.Context) (string, error) {
	etcdVersionVal, ok := ctx.SharedData.Get(EtcdVersionKey)
	if !ok {
		return "", fmt.Errorf("%s not found in SharedData", EtcdVersionKey)
	}
	etcdVersion, ok := etcdVersionVal.(string)
	if !ok || etcdVersion == "" {
		return "", fmt.Errorf("%s in SharedData is not a valid version string", EtcdVersionKey)
	}
	return strings.TrimPrefix(etcdVersion, "v"), nil // Remove "v" prefix for comparison
}

// Check verifies if etcd and etcdctl exist in TargetBinDir and match the expected version.
func (e *InstallEtcdFromDirStepExecutor) Check(ctx *runtime.Context, s spec.StepSpec) (bool, error) {
	spec, ok := s.(*InstallEtcdFromDirStepSpec)
	if !ok {
		return false, fmt.Errorf("invalid spec type %T for InstallEtcdFromDirStepExecutor", s)
	}
	if err := spec.ApplyDefaults(ctx); err != nil {
		return false, fmt.Errorf("failed to apply defaults: %w", err)
	}

	expectedVersion, err := e.getExpectedVersion(ctx)
	if err != nil {
		// If EtcdVersionKey is not in SharedData, it means download/extract didn't run or complete.
		// So, binaries are not installed yet.
		ctx.Logger.Debugf("Cannot determine expected etcd version from SharedData: %v. Assuming not installed.", err)
		return false, nil
	}

	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		targetPath := filepath.Join(spec.TargetBinDir, binName)
		if _, err := ctx.Host.Runner.Stat(targetPath); err != nil {
			ctx.Logger.Infof("%s not found at %s. Installation needed.", binName, targetPath)
			return false, nil // Binary doesn't exist
		}

		// Check version for etcd binary
		if binName == "etcd" {
			// Construct command to get version (e.g., etcd --version)
			// Output format for `etcd --version`:
			// etcd Version: 3.5.0
			// API Version: 3.5
			// Git SHA: ce9380d78
			// Go Version: go1.16.3
			// Go OS/Arch: linux/amd64
			versionCmd := fmt.Sprintf("%s --version", targetPath)
			output, err := ctx.Host.Runner.Run(versionCmd)
			if err != nil {
				ctx.Logger.Warnf("Failed to get version of %s at %s: %v. Re-installation might be needed.", binName, targetPath, err)
				return false, nil // Cannot determine version, assume not OK
			}

			versionLine := ""
			for _, line := range strings.Split(output, "\n") {
				if strings.HasPrefix(line, "etcd Version:") {
					versionLine = strings.TrimSpace(strings.TrimPrefix(line, "etcd Version:"))
					break
				}
			}

			if versionLine == "" {
				ctx.Logger.Warnf("Could not parse version output for %s: %s. Re-installation might be needed.", targetPath, output)
				return false, nil
			}

			if versionLine != expectedVersion {
				ctx.Logger.Infof("Version mismatch for %s: expected %s, found %s. Installation needed.", targetPath, expectedVersion, versionLine)
				return false, nil
			}
			ctx.Logger.Infof("Correct version %s of %s found at %s.", versionLine, binName, targetPath)
		}
	}

	ctx.Logger.Infof("Etcd binaries already installed in %s with correct version %s.", spec.TargetBinDir, expectedVersion)
	return true, nil
}

// Execute installs etcd and etcdctl binaries to the target directory.
func (e *InstallEtcdFromDirStepExecutor) Execute(ctx *runtime.Context, s spec.StepSpec) error {
	spec, ok := s.(*InstallEtcdFromDirStepSpec)
	if !ok {
		return fmt.Errorf("invalid spec type %T for InstallEtcdFromDirStepExecutor", s)
	}
	if err := spec.ApplyDefaults(ctx); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	extractedPathVal, ok := ctx.SharedData.Get(EtcdExtractedPathKey)
	if !ok {
		return fmt.Errorf("%s not found in SharedData, ensure ExtractEtcdArchiveStep ran successfully", EtcdExtractedPathKey)
	}
	extractedPath, ok := extractedPathVal.(string)
	if !ok || extractedPath == "" {
		return fmt.Errorf("%s in SharedData is not a valid path", EtcdExtractedPathKey)
	}

	if err := ctx.Host.Runner.Mkdirp(spec.TargetBinDir); err != nil {
		return fmt.Errorf("failed to create target binary directory %s: %w", spec.TargetBinDir, err)
	}

	binaries := []string{"etcd", "etcdctl"}
	for _, binName := range binaries {
		sourcePath := filepath.Join(extractedPath, binName)
		destinationPath := filepath.Join(spec.TargetBinDir, binName)

		ctx.Logger.Infof("Copying %s from %s to %s", binName, sourcePath, destinationPath)
		// Using Run("cp") for simplicity. A robust CopyFile in runner would be better.
		copyCmd := fmt.Sprintf("cp %s %s", sourcePath, destinationPath)
		if _, err := ctx.Host.Runner.Run(copyCmd); err != nil {
			return fmt.Errorf("failed to copy %s to %s: %w", binName, destinationPath, err)
		}

		chmodCmd := fmt.Sprintf("chmod +x %s", destinationPath)
		if _, err := ctx.Host.Runner.Run(chmodCmd); err != nil {
			return fmt.Errorf("failed to make %s executable: %w", destinationPath, err)
		}
		ctx.Logger.Infof("Successfully installed and set +x for %s", destinationPath)
	}

	// Verify installation by checking version (as a final confirmation)
	_, err := e.Check(ctx, spec)
	if err != nil {
		return fmt.Errorf("post-installation check failed: %w", err)
	}

	ctx.Logger.Infof("Etcd binaries installed successfully to %s.", spec.TargetBinDir)
	return nil
}

func init() {
	step.Register(&InstallEtcdFromDirStepSpec{}, &InstallEtcdFromDirStepExecutor{})
}
