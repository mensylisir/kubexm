package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util"
)

type SELinuxState string

const (
	SELinuxStateDisabled   SELinuxState = common.DisabledSELinuxMode
	SELinuxStatePermissive SELinuxState = common.PermissiveSELinuxMode
	SELinuxStateEnforcing  SELinuxState = common.EnforceSELinuxMode
)

type ManageSELinuxStep struct {
	step.Base
	State SELinuxState
}

type ManageSELinuxStepBuilder struct {
	step.Builder[ManageSELinuxStepBuilder, *ManageSELinuxStep]
}

func NewManageSELinuxStepBuilder(ctx runtime.ExecutionContext, instanceName string, state SELinuxState) *ManageSELinuxStepBuilder {
	cs := &ManageSELinuxStep{State: state}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Ensure SELinux state is [%s]", instanceName, state)
	cs.Base.Sudo = false
	cs.Base.Timeout = 2 * time.Minute
	return new(ManageSELinuxStepBuilder).Init(cs)
}

func (s *ManageSELinuxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageSELinuxStep) isSELinuxSupported(ctx runtime.ExecutionContext) (bool, error) {
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to get host facts: %w", err)
	}

	if util.ContainsString(common.RedHatFamilyDistributions, facts.OS.ID) {
		return true, nil
	} else if util.ContainsString(common.DebianFamilyDistributions, facts.OS.ID) {
		return false, nil
	} else {
		runner := ctx.GetRunner()
		conn, err := ctx.GetCurrentHostConnector()
		if err != nil {
			return false, err
		}
		_, _, err = runner.OriginRun(ctx.GoContext(), conn, "command -v sestatus", s.Sudo)
		return err == nil, nil
	}
}

func (s *ManageSELinuxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	supported, err := s.isSELinuxSupported(ctx)
	if err != nil {
		return false, err
	}
	if !supported {
		logger.Infof("SELinux is not supported on this OS. Step considered done.")
		return true, nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector: %w", err)
	}

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, "getenforce", s.Sudo)
	if err != nil {
		return false, fmt.Errorf("precheck: failed to run getenforce: %w, stderr: %s", err, stderr)
	}
	currentMode := strings.ToLower(strings.TrimSpace(stdout))

	configContent, err := s.ReadRemoteFile(ctx, "/etc/selinux/config")
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("precheck: failed to read /etc/selinux/config: %w", err)
	}

	configMode := s.parseSELinuxConfig(configContent)

	if string(s.State) == currentMode && string(s.State) == configMode {
		logger.Infof("SELinux is already in the desired state '%s' (both current and permanent). Step considered done.", s.State)
		return true, nil
	}

	logger.Infof("Current SELinux state (mode: %s, config: %s) does not match desired state '%s'. Step needs to run.", currentMode, configMode, s.State)
	return false, nil
}

func (s *ManageSELinuxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	supported, err := s.isSELinuxSupported(ctx)
	if err != nil {
		return err
	}
	if !supported {
		logger.Infof("SELinux is not supported or active on this OS. Skipping.")
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Attempting to set SELinux runtime state to '%s'...", s.State)
	var setEnforceCmd string
	switch s.State {
	case SELinuxStatePermissive:
		setEnforceCmd = "setenforce 0"
	case SELinuxStateEnforcing:
		setEnforceCmd = "setenforce 1"
	case SELinuxStateDisabled:
		logger.Info("SELinux 'disabled' state can only be applied permanently and requires a reboot.")
	default:
		return fmt.Errorf("unsupported SELinux state: %s", s.State)
	}
	if setEnforceCmd != "" {
		if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, setEnforceCmd, s.Sudo); err != nil {
			logger.Warnf("Failed to execute '%s'. A reboot might be required. Error: %v, Stderr: %s", setEnforceCmd, err, stderr)
		} else {
			logger.Infof("SELinux mode set to '%s' for the current session.", s.State)
		}
	}

	logger.Info("Updating permanent SELinux configuration in /etc/selinux/config...")
	configContent, err := s.ReadRemoteFile(ctx, "/etc/selinux/config")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("run: failed to read /etc/selinux/config: %w", err)
	}

	newConfigContent, err := s.rebuildSELinuxConfig(configContent, s.State)
	if err != nil {
		return fmt.Errorf("failed to rebuild selinux config: %w", err)
	}

	if err := s.AtomicWriteRemoteFile(ctx, "/etc/selinux/config", newConfigContent); err != nil {
		return err
	}
	logger.Info("SELinux configuration in /etc/selinux/config has been updated.")

	stdout, _, _ := runner.OriginRun(ctx.GoContext(), conn, "getenforce", s.Sudo)
	currentMode := strings.ToLower(strings.TrimSpace(stdout))
	if s.State == SELinuxStateDisabled {
		logger.Warn("A reboot is required to disable SELinux completely.")
	} else if string(s.State) != currentMode {
		logger.Warnf("The runtime SELinux state is still '%s'. A reboot is required for the permanent change to '%s' to take effect.", currentMode, s.State)
	}

	return nil
}

func (s *ManageSELinuxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for ManageSELinuxStep is a no-op.")
	return nil
}

func (s *ManageSELinuxStep) rebuildSELinuxConfig(originalContent string, targetState SELinuxState) ([]byte, error) {
	var newLines []string
	foundAndModified := false

	scanner := bufio.NewScanner(strings.NewReader(originalContent))
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "SELINUX=") {
			newLines = append(newLines, "SELINUX="+string(targetState))
			foundAndModified = true
		} else {
			newLines = append(newLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !foundAndModified {
		newLines = append(newLines, "SELINUX="+string(targetState))
	}

	return []byte(strings.Join(newLines, "\n") + "\n"), nil
}

func (s *ManageSELinuxStep) parseSELinuxConfig(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "SELINUX=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.ToLower(strings.TrimSpace(parts[1]))
			}
		}
	}
	return ""
}

func (s *ManageSELinuxStep) ReadRemoteFile(ctx runtime.ExecutionContext, path string) (string, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", err
	}
	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, "cat "+path, s.Sudo)
	if err != nil {
		if strings.Contains(stderr, "No such file or directory") {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("failed to read remote file %s: %w, stderr: %s", path, err, stderr)
	}
	return stdout, nil
}

func (s *ManageSELinuxStep) AtomicWriteRemoteFile(ctx runtime.ExecutionContext, destPath string, content []byte) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("mktemp -p %s", filepath.Dir(destPath)), s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w, stderr: %s", destPath, err, stderr)
	}
	tmpFilePath := strings.TrimSpace(stdout)

	if err := runner.WriteFile(ctx.GoContext(), conn, content, tmpFilePath, "0644", s.Sudo); err != nil {
		runner.OriginRun(ctx.GoContext(), conn, "rm -f "+tmpFilePath, s.Sudo)
		return fmt.Errorf("failed to write to temp file %s: %w", tmpFilePath, err)
	}

	_, stderr, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("mv -f %s %s", tmpFilePath, destPath), s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to move temp file to %s: %w, stderr: %s", destPath, err, stderr)
	}
	return nil
}

var _ step.Step = (*ManageSELinuxStep)(nil)
