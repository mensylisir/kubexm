package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RemoveEtcdDataDirStep removes the etcd data directory. This is a destructive operation.
type RemoveEtcdDataDirStep struct {
	meta    spec.StepMeta
	DataDir string // Path to the etcd data directory, e.g., /var/lib/etcd
	Sudo    bool
}

// NewRemoveEtcdDataDirStep creates a new RemoveEtcdDataDirStep.
func NewRemoveEtcdDataDirStep(instanceName, dataDir string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "RemoveEtcdDataDirectory"
	}
	dd := dataDir
	if dd == "" {
		dd = "/var/lib/etcd" // Common default, but should be derived from config
	}
	return &RemoveEtcdDataDirStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes the etcd data directory %s. This is destructive.", dd),
		},
		DataDir: dd,
		Sudo:    true, // Removing /var/lib/etcd usually requires sudo
	}
}

func (s *RemoveEtcdDataDirStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *RemoveEtcdDataDirStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if s.DataDir == "" {
		err := fmt.Errorf("DataDir is not specified for RemoveEtcdDataDirStep")
		logger.Error("Precheck failed: DataDir is empty.", "error", err)
		return false, err
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.DataDir)
	if err != nil {
		logger.Warn("Failed to check for existence of etcd data directory, assuming it might exist.", "path", s.DataDir, "error", err)
		return false, nil // Let Run attempt removal
	}

	if !exists {
		logger.Info("Etcd data directory already removed or does not exist.", "path", s.DataDir)
		return true, nil
	}
	logger.Info("Etcd data directory exists and needs removal.", "path", s.DataDir)
	return false, nil
}

func (s *RemoveEtcdDataDirStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.DataDir == "" {
		return fmt.Errorf("DataDir is not specified for RemoveEtcdDataDirStep on host %s", host.GetName())
	}
	if s.DataDir == "/" || s.DataDir == "/var" || s.DataDir == "/var/lib" || s.DataDir == "/etc" { // Basic safety net
		return fmt.Errorf("refusing to remove potentially critical directory: %s", s.DataDir)
	}


	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure etcd service is stopped before removing data dir (important!)
	// This should ideally be handled by a preceding ManageEtcdServiceStep(ActionStop).
	// Adding a check here for safety, though.
	isActive, _ := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, "etcd")
	if isActive {
		logger.Warn("etcd service is active. It should be stopped before removing data directory. Attempting to stop now...")
		stopCmd := "systemctl stop etcd"
		_, _, stopErr := runnerSvc.Run(ctx.GoContext(),conn, stopCmd, s.Sudo)
		if stopErr != nil {
			logger.Error("Failed to stop etcd service before data dir removal. Aborting data removal.", "error", stopErr)
			return fmt.Errorf("failed to stop etcd before data dir removal: %w", stopErr)
		}
		logger.Info("etcd service stopped.")
	}


	logger.Info("Removing etcd data directory.", "path", s.DataDir)
	// Remove with Recursive and potentially Force flags. Runner's Remove should handle this.
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.DataDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to remove etcd data directory %s: %w", s.DataDir, err)
	}

	logger.Info("Etcd data directory removed successfully.")
	return nil
}

func (s *RemoveEtcdDataDirStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for RemoveEtcdDataDirStep is not applicable (would mean restoring data from backup, which is a separate Restore step).")
	// If this step failed, the data directory might be partially removed or intact.
	// No simple rollback here.
	return nil
}

var _ step.Step = (*RemoveEtcdDataDirStep)(nil)
