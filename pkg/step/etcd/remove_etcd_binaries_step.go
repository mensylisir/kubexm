package etcd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RemoveEtcdBinariesStep removes etcd and etcdctl binaries from the system path.
type RemoveEtcdBinariesStep struct {
	meta      spec.StepMeta
	TargetDir string // System directory where binaries were installed, e.g., /usr/local/bin
	Sudo      bool
}

// NewRemoveEtcdBinariesStep creates a new RemoveEtcdBinariesStep.
func NewRemoveEtcdBinariesStep(instanceName, targetDir string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "RemoveEtcdBinaries"
	}
	td := targetDir
	if td == "" {
		td = "/usr/local/bin" // Default path
	}
	return &RemoveEtcdBinariesStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes etcd and etcdctl binaries from %s.", td),
		},
		TargetDir: td,
		Sudo:      true, // Removing from system paths usually requires sudo
	}
}

func (s *RemoveEtcdBinariesStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *RemoveEtcdBinariesStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	binaries := []string{"etcd", "etcdctl"}
	allMissing := true
	for _, binName := range binaries {
		binPath := filepath.Join(s.TargetDir, binName)
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, binPath)
		if err != nil {
			logger.Warn("Failed to check for binary existence, assuming it might exist.", "path", binPath, "error", err)
			return false, nil // Let Run attempt removal
		}
		if exists {
			logger.Info("Etcd binary still exists.", "path", binPath)
			allMissing = false
		}
	}

	if allMissing {
		logger.Info("All etcd binaries already removed from target directory.")
		return true, nil
	}
	return false, nil
}

func (s *RemoveEtcdBinariesStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	binariesToRemove := []string{"etcd", "etcdctl"}
	var lastErr error
	for _, binName := range binariesToRemove {
		binPath := filepath.Join(s.TargetDir, binName)
		logger.Info("Removing binary.", "path", binPath)
		// Remove with IgnoreNotExist option if runner supports it, or check existence first.
		// For simplicity, just try to remove.
		if err := runnerSvc.Remove(ctx.GoContext(), conn, binPath, s.Sudo); err != nil {
			// Log error but continue trying to remove others.
			logger.Error("Failed to remove binary (best effort).", "path", binPath, "error", err)
			lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", binPath, err, lastErr)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("one or more errors occurred while removing etcd binaries: %w", lastErr)
	}
	logger.Info("Etcd binaries removed successfully from target directory.")
	return nil
}

func (s *RemoveEtcdBinariesStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for RemoveEtcdBinariesStep is not applicable (would mean reinstalling).")
	// If this step failed mid-way, some binaries might be removed, some not.
	// A sophisticated rollback might try to restore them from a backup or source,
	// but that's beyond typical step rollback logic.
	return nil
}

var _ step.Step = (*RemoveEtcdBinariesStep)(nil)
