package common

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type SwapState string

const (
	SwapStateDisabled SwapState = "disabled"
	SwapStateEnabled  SwapState = "enabled"
)

type ManageSwapStep struct {
	step.Base
	State SwapState
}

type ManageSwapStepBuilder struct {
	step.Builder[ManageSwapStepBuilder, *ManageSwapStep]
}

func NewManageSwapStepBuilder(ctx runtime.ExecutionContext, instanceName string, state SwapState) *ManageSwapStepBuilder {
	cs := &ManageSwapStep{
		State: state,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Ensure swap state is [%s]", instanceName, state)
	cs.Base.Sudo = false
	cs.Base.Timeout = 2 * time.Minute
	return new(ManageSwapStepBuilder).Init(cs)
}

func (s *ManageSwapStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageSwapStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	stdout, _, err := ctx.GetRunner().OriginRun(ctx.GoContext(), conn, "swapon --show", s.Sudo)
	if err != nil {
		logger.Warnf("Command 'swapon --show' may have failed, proceeding with fstab check. Error: %v", err)
	}
	isCurrentlyActive := strings.TrimSpace(stdout) != ""

	fstabContent, err := s.readRemoteFile(ctx, "/etc/fstab")
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("precheck: failed to read /etc/fstab: %w", err)
	}

	isPermanentlyConfigured := false
	scanner := bufio.NewScanner(strings.NewReader(fstabContent))
	for scanner.Scan() {
		if s.isLineActiveSwap(scanner.Text()) {
			isPermanentlyConfigured = true
			break
		}
	}

	switch s.State {
	case SwapStateDisabled:
		if !isCurrentlyActive && !isPermanentlyConfigured {
			logger.Info("Swap is already disabled. Step considered done.")
			return true, nil
		}
		logger.Infof("Swap needs to be disabled (current active: %v, permanent: %v).", isCurrentlyActive, isPermanentlyConfigured)
		return false, nil
	case SwapStateEnabled:
		if isCurrentlyActive && isPermanentlyConfigured {
			logger.Info("Swap is already enabled. Step considered done.")
			return true, nil
		}
		logger.Infof("Swap needs to be enabled (current active: %v, permanent: %v).", isCurrentlyActive, isPermanentlyConfigured)
		return false, nil
	default:
		return false, fmt.Errorf("unsupported swap state for precheck: %s", s.State)
	}
}

func (s *ManageSwapStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	fstabContent, err := s.readRemoteFile(ctx, "/etc/fstab")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("run: failed to read /etc/fstab: %w", err)
	}

	newFstabContent, err := s.rebuildFstabContent(fstabContent)
	if err != nil {
		return fmt.Errorf("failed to rebuild fstab content: %w", err)
	}

	logger.Info("Atomically writing updated content to /etc/fstab")
	if err := s.atomicWriteRemoteFile(ctx, "/etc/fstab", newFstabContent); err != nil {
		return err
	}
	logger.Infof("Successfully updated /etc/fstab to set swap state to '%s'.", s.State)

	switch s.State {
	case SwapStateDisabled:
		logger.Info("Disabling swap for the current session...")
		_, stderr, err := runner.OriginRun(ctx.GoContext(), conn, "swapoff -a", s.Sudo)
		if err != nil {
			logger.Warnf("Command 'swapoff -a' may have failed (this is okay if swap was already off). Error: %v, Stderr: %s", err, stderr)
		}
	case SwapStateEnabled:
		logger.Info("Enabling swap for the current session...")
		_, stderr, err := runner.OriginRun(ctx.GoContext(), conn, "swapon -a", s.Sudo)
		if err != nil {
			return fmt.Errorf("failed to execute 'swapon -a': %w, stderr: %s", err, stderr)
		}
	default:
		return fmt.Errorf("unsupported swap state for run: %s", s.State)
	}

	logger.Infof("Swap successfully set to '%s'.", s.State)
	return nil
}

func (s *ManageSwapStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for ManageSwapStep is a no-op. To restore, manually inspect /etc/fstab and the original backup created by this step.")
	return nil
}

func (s *ManageSwapStep) rebuildFstabContent(originalContent string) ([]byte, error) {
	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(originalContent))

	for scanner.Scan() {
		line := scanner.Text()

		switch s.State {
		case SwapStateDisabled:
			if s.isLineActiveSwap(line) {
				newLines = append(newLines, "# "+line)
			} else {
				newLines = append(newLines, line)
			}
		case SwapStateEnabled:
			if s.isLineCommentedSwap(line) {
				newLines = append(newLines, strings.TrimSpace(strings.TrimPrefix(line, "#")))
			} else {
				newLines = append(newLines, line)
			}
		default:
			newLines = append(newLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return []byte(strings.Join(newLines, "\n") + "\n"), nil
}

func (s *ManageSwapStep) isLineActiveSwap(line string) bool {
	trimmedLine := strings.TrimSpace(line)
	if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
		return false
	}
	parts := strings.Fields(trimmedLine)
	return len(parts) >= 3 && parts[2] == "swap"
}

func (s *ManageSwapStep) isLineCommentedSwap(line string) bool {
	trimmedLine := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmedLine, "#") {
		return false
	}
	uncommentedLine := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "#"))
	parts := strings.Fields(uncommentedLine)
	return len(parts) >= 3 && parts[2] == "swap"
}

func (s *ManageSwapStep) readRemoteFile(ctx runtime.ExecutionContext, path string) (string, error) {
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

func (s *ManageSwapStep) atomicWriteRemoteFile(ctx runtime.ExecutionContext, destPath string, content []byte) error {
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

var _ step.Step = (*ManageSwapStep)(nil)
