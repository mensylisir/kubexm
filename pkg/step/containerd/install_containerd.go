package containerd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"path/filepath"
)

type InstallContainerdStep struct {
	step.Base
	SystemdUnitFileSource string
	SystemdUnitFileTarget string
	Binaries              map[string]string
}

type InstallContainerdStepBuilder struct {
	step.Builder[InstallContainerdStepBuilder, *InstallContainerdStep]
}

func NewInstallContainerdStepBuilder(instanceName string) *InstallContainerdStepBuilder {
	cs := &InstallContainerdStep{}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Install containerd binaries and service from extracted archive", instanceName)
	cs.Base.Sudo = false

	b := new(InstallContainerdStepBuilder).Init(cs)

	defaultBinaries := map[string]string{
		"bin/containerd":              "/usr/local/bin/containerd",
		"bin/containerd-shim":         "/usr/local/bin/containerd-shim",
		"bin/containerd-shim-runc-v1": "/usr/local/bin/containerd-shim-runc-v1",
		"bin/containerd-shim-runc-v2": "/usr/local/bin/containerd-shim-runc-v2",
		"bin/ctr":                     "/usr/local/bin/ctr",
		"bin/crictl":                  "/usr/local/bin/crictl",
	}

	b.WithSystemdUnitFile("containerd.service", "/usr/lib/systemd/system/containerd.service")
	b.WithBinaries(defaultBinaries)
	return b
}

func (b *InstallContainerdStepBuilder) WithSystemdUnitFile(sourceRelPath, targetPath string) *InstallContainerdStepBuilder {
	b.Step.SystemdUnitFileSource = sourceRelPath
	b.Step.SystemdUnitFileTarget = targetPath
	return b
}

func (b *InstallContainerdStepBuilder) WithBinaries(binaries map[string]string) *InstallContainerdStepBuilder {
	b.Step.Binaries = binaries
	return b
}

func (s *InstallContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// getExtractedPath 是一个辅助函数，用于从 RuntimeConfig 和 TaskCache 中安全地获取解压路径
func (s *InstallContainerdStep) getExtractedPath(ctx runtime.ExecutionContext) (string, error) {
	inputKeyVal, ok := ctx.GetFromRuntimeConfig("inputCacheKey")
	if !ok {
		return "", fmt.Errorf("'inputCacheKey' is required but not provided in RuntimeConfig for step %s", s.Base.Meta.Name)
	}
	inputKey, isString := inputKeyVal.(string)
	if !isString || inputKey == "" {
		return "", fmt.Errorf("invalid 'inputCacheKey' in RuntimeConfig: expected a non-empty string, got %T", inputKeyVal)
	}

	extractedPathVal, found := ctx.GetTaskCache().Get(inputKey)
	if !found {
		return "", fmt.Errorf("path to extracted archive not found in Task Cache using key '%s'", inputKey)
	}
	extractedPath, okPath := extractedPathVal.(string)
	if !okPath || extractedPath == "" {
		return "", fmt.Errorf("invalid extracted archive path in Task Cache (key: '%s')", inputKey)
	}

	return extractedPath, nil
}

func (s *InstallContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	// **修正：正确处理 GetCurrentHostConnector 的返回值**
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	runner := ctx.GetRunner()

	extractedPath, err := s.getExtractedPath(ctx)
	if err != nil {
		logger.Infof("Could not determine extracted path, assuming step needs to run. Error: %v", err)
		return false, nil
	}

	for srcRelPath, targetPath := range s.Binaries {
		sourcePath := filepath.Join(extractedPath, srcRelPath)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("[ -f %s ]", sourcePath), s.Sudo); err != nil {
			return false, fmt.Errorf("precheck: source binary '%s' not found in extracted archive path '%s'", srcRelPath, extractedPath)
		}
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("[ -f %s ]", targetPath), s.Sudo); err != nil {
			logger.Infof("Target binary '%s' does not exist. Step needs to run.", targetPath)
			return false, nil
		}
	}

	if s.SystemdUnitFileTarget != "" && s.SystemdUnitFileSource != "" {
		sourcePath := filepath.Join(extractedPath, s.SystemdUnitFileSource)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("[ -f %s ]", sourcePath), s.Sudo); err == nil {
			if _, _, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("[ -f %s ]", s.SystemdUnitFileTarget), s.Sudo); err != nil {
				logger.Infof("Systemd unit file '%s' does not exist. Step needs to run.", s.SystemdUnitFileTarget)
				return false, nil
			}
		} else {
			logger.Warnf("Source systemd file '%s' not found, will skip its installation.", s.SystemdUnitFileSource)
		}
	}

	logger.Info("All required containerd binaries and the systemd service file already exist. Step considered done.")
	return true, nil
}

func (s *InstallContainerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	extractedPath, err := s.getExtractedPath(ctx)
	if err != nil {
		return err
	}
	logger.Infof("Using extracted containerd files from: %s", extractedPath)

	for srcRelPath, targetPath := range s.Binaries {
		sourcePath := filepath.Join(extractedPath, srcRelPath)
		if err := s.copyFile(ctx, sourcePath, targetPath, "0755"); err != nil {
			return err
		}
	}

	if s.SystemdUnitFileTarget != "" && s.SystemdUnitFileSource != "" {
		sourcePath := filepath.Join(extractedPath, s.SystemdUnitFileSource)

		// **修正：正确处理 GetCurrentHostConnector 的返回值**
		conn, err := ctx.GetCurrentHostConnector()
		if err != nil {
			return err
		}

		if _, _, err := ctx.GetRunner().OriginRun(ctx.GoContext(), conn, fmt.Sprintf("[ -f %s ]", sourcePath), s.Sudo); err == nil {
			if err := s.copyFile(ctx, sourcePath, s.SystemdUnitFileTarget, "0644"); err != nil {
				return err
			}
		} else {
			logger.Warnf("Source systemd file not found, skipping installation: %s", sourcePath)
		}
	}

	return nil
}

func (s *InstallContainerdStep) copyFile(ctx runtime.ExecutionContext, source, dest, perms string) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	// **修正：正确处理 GetCurrentHostConnector 的返回值**
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	runner := ctx.GetRunner()

	destDir := filepath.Dir(dest)

	logger.Infof("Ensuring directory exists: %s", destDir)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", destDir)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, mkdirCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w, stderr: %s", destDir, err, stderr)
	}

	logger.Infof("Copying %s to %s", source, dest)
	cpCmd := fmt.Sprintf("cp -fp %s %s", source, dest)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, cpCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to copy '%s' to '%s': %w, stderr: %s", source, dest, err, stderr)
	}

	logger.Infof("Setting permissions %s on %s", perms, dest)
	chmodCmd := fmt.Sprintf("chmod %s %s", perms, dest)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, chmodCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to set permissions on '%s': %w, stderr: %s", dest, err, stderr)
	}

	return nil
}

func (s *InstallContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	// **修正：正确处理 GetCurrentHostConnector 的返回值**
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	runner := ctx.GetRunner()

	filesToRemove := []string{s.SystemdUnitFileTarget}
	for _, targetPath := range s.Binaries {
		filesToRemove = append(filesToRemove, targetPath)
	}

	for _, path := range filesToRemove {
		if path == "" {
			continue
		}
		logger.Warnf("Rolling back by removing: %s", path)
		rmCmd := fmt.Sprintf("rm -f %s", path)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, rmCmd, s.Sudo); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback. Manual cleanup may be required. Error: %v", path, err)
		}
	}

	return nil
}

var _ step.Step = (*InstallContainerdStep)(nil)
