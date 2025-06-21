package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CleanupContainerdDataStep removes the containerd data directory (typically /var/lib/containerd).
// This is a destructive operation and will remove all images, containers, etc., managed by containerd.
type CleanupContainerdDataStep struct {
	meta    spec.StepMeta
	DataDir string // Path to the containerd data directory
	Sudo    bool
}

// NewCleanupContainerdDataStep creates a new CleanupContainerdDataStep.
func NewCleanupContainerdDataStep(instanceName, dataDir string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "CleanupContainerdDataDirectory"
	}
	dd := dataDir
	if dd == "" {
		dd = "/var/lib/containerd" // Default data directory for containerd
	}
	return &CleanupContainerdDataStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes the containerd data directory %s. This is destructive.", dd),
		},
		DataDir: dd,
		Sudo:    true, // Removing /var/lib/containerd usually requires sudo
	}
}

func (s *CleanupContainerdDataStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CleanupContainerdDataStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	if s.DataDir == "" {
		err := fmt.Errorf("DataDir is not specified")
		logger.Error("Precheck failed: DataDir is empty.", "error", err)
		return false, err
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.DataDir)
	if err != nil {
		logger.Warn("Failed to check for existence of containerd data directory, assuming it might exist.", "path", s.DataDir, "error", err)
		return false, nil
	}

	if !exists {
		logger.Info("Containerd data directory already removed or does not exist.", "path", s.DataDir)
		return true, nil
	}
	logger.Info("Containerd data directory exists and needs removal.", "path", s.DataDir)
	return false, nil
}

func (s *CleanupContainerdDataStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.DataDir == "" {
		return fmt.Errorf("DataDir is not specified for host %s", host.GetName())
	}
	if s.DataDir == "/" || s.DataDir == "/var" || s.DataDir == "/var/lib" || s.DataDir == "/etc" { // Basic safety net
		return fmt.Errorf("refusing to remove potentially critical directory: %s", s.DataDir)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	// Ensure containerd service is stopped before removing data dir.
	// This should ideally be handled by a preceding ManageContainerdServiceStep(ActionStop).
	isActive, _ := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, "containerd")
	if isActive {
		logger.Warn("Containerd service is active. It should be stopped before removing data directory. Attempting to stop now...")
		stopCmd := "systemctl stop containerd"
		_, _, stopErr := runnerSvc.Run(ctx.GoContext(),conn, stopCmd, s.Sudo)
		if stopErr != nil {
			logger.Error("Failed to stop containerd service before data dir removal. Aborting data removal.", "error", stopErr)
			return fmt.Errorf("failed to stop containerd before data dir removal: %w", stopErr)
		}
		logger.Info("Containerd service stopped.")
	}

	logger.Info("Removing containerd data directory.", "path", s.DataDir)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.DataDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to remove containerd data directory %s: %w", s.DataDir, err)
	}

	logger.Info("Containerd data directory removed successfully.")
	return nil
}

func (s *CleanupContainerdDataStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for CleanupContainerdDataStep is not applicable (would mean restoring data, which is not feasible for this step).")
	return nil
}

var _ step.Step = (*CleanupContainerdDataStep)(nil)
