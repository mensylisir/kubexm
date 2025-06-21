package docker

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RemoveCriDockerdStep removes the cri-dockerd binary and its systemd unit files.
type RemoveCriDockerdStep struct {
	meta             spec.StepMeta
	TargetBinaryDir  string // System path where binary was installed, e.g., /usr/local/bin
	TargetSystemdDir string // System path where systemd units were installed, e.g., /etc/systemd/system
	Sudo             bool
}

// NewRemoveCriDockerdStep creates a new RemoveCriDockerdStep.
func NewRemoveCriDockerdStep(instanceName, targetBinaryDir, targetSystemdDir string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "RemoveCriDockerd"
	}
	binDir := targetBinaryDir
	if binDir == "" {
		binDir = "/usr/local/bin"
	}
	sysdDir := targetSystemdDir
	if sysdDir == "" {
		sysdDir = "/etc/systemd/system"
	}

	return &RemoveCriDockerdStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes cri-dockerd binary from %s and systemd units from %s.", binDir, sysdDir),
		},
		TargetBinaryDir:  binDir,
		TargetSystemdDir: sysdDir,
		Sudo:             true,
	}
}

func (s *RemoveCriDockerdStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *RemoveCriDockerdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	itemsToCheck := []string{
		filepath.Join(s.TargetBinaryDir, "cri-dockerd"),
		filepath.Join(s.TargetSystemdDir, "cri-dockerd.service"),
		filepath.Join(s.TargetSystemdDir, "cri-dockerd.socket"),
	}

	allMissing := true
	for _, itemPath := range itemsToCheck {
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, itemPath)
		if err != nil {
			logger.Warn("Failed to check for item existence, assuming it might exist.", "path", itemPath, "error", err)
			return false, nil
		}
		if exists {
			logger.Info("cri-dockerd related item still exists.", "path", itemPath)
			allMissing = false
		}
	}

	if allMissing {
		logger.Info("All cri-dockerd related items already removed.")
		return true, nil
	}
	return false, nil
}

func (s *RemoveCriDockerdStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	itemsToRemove := []string{
		filepath.Join(s.TargetBinaryDir, "cri-dockerd"),
		filepath.Join(s.TargetSystemdDir, "cri-dockerd.service"),
		filepath.Join(s.TargetSystemdDir, "cri-dockerd.socket"),
	}
	var lastErr error

	for _, itemPath := range itemsToRemove {
		logger.Info("Removing cri-dockerd item.", "path", itemPath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, itemPath, s.Sudo); err != nil {
			logger.Error("Failed to remove item (best effort).", "path", itemPath, "error", err)
			lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", itemPath, err, lastErr)
		}
	}

	// After removing service files, a daemon-reload is typically needed.
	// This step doesn't do it automatically. A task can schedule ManageCriDockerdServiceStep(ActionDaemonReload).

	if lastErr != nil {
		return fmt.Errorf("one or more errors occurred while removing cri-dockerd items: %w", lastErr)
	}
	logger.Info("cri-dockerd items removed successfully.")
	return nil
}

func (s *RemoveCriDockerdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for RemoveCriDockerdStep is not applicable (would mean reinstalling).")
	return nil
}

var _ step.Step = (*RemoveCriDockerdStep)(nil)
