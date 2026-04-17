package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestoreFetchedConfigsStep struct {
	step.Base
	remoteKubeDir string
	localNodeDir  string
}

type RestoreFetchedConfigsStepBuilder struct {
	step.Builder[RestoreFetchedConfigsStepBuilder, *RestoreFetchedConfigsStep]
}

func NewRestoreFetchedConfigsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestoreFetchedConfigsStepBuilder {
	s := &RestoreFetchedConfigsStep{
		remoteKubeDir: DefaultRemoteK8sConfigDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restore Kubernetes configuration by pushing the locally-fetched backup back to the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(RestoreFetchedConfigsStepBuilder).Init(s)
	return b
}

func (s *RestoreFetchedConfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestoreFetchedConfigsStep) getLocalNodeDir(ctx runtime.ExecutionContext) string {
	baseWorkDir := ctx.GetClusterWorkDir()
	return filepath.Join(baseWorkDir, "remote-configs", ctx.GetHost().GetName())
}

func (s *RestoreFetchedConfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for restoring fetched configs...")

	s.localNodeDir = s.getLocalNodeDir(ctx)

	if _, err := os.Stat(s.localNodeDir); err != nil {
		return false, fmt.Errorf("precheck failed: local backup directory '%s' not found on bastion. Cannot restore", s.localNodeDir)
	}

	entries, err := os.ReadDir(s.localNodeDir)
	if err != nil || len(entries) == 0 {
		return false, fmt.Errorf("precheck failed: local backup directory '%s' is empty or unreadable", s.localNodeDir)
	}

	logger.Infof("Precheck passed: Found valid local backup directory at '%s'.", s.localNodeDir)
	return false, nil
}

func (s *RestoreFetchedConfigsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	s.localNodeDir = s.getLocalNodeDir(ctx)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	logger.Warnf("!!! EXECUTING CRITICAL RESTORE OPERATION !!!")
	logger.Warnf("Restoring Kubernetes configuration from local backup: '%s' -> '%s:%s'...",
		s.localNodeDir, ctx.GetHost().GetName(), s.remoteKubeDir)

	// First back up the current remote config in case we need to recover
	timestamp := time.Now().Format("20060102-150405")
	currentBackupDir := fmt.Sprintf("%s.kubexm-restore-backup-%s", s.remoteKubeDir, timestamp)
	logger.Infof("Creating a safety backup of current remote config: '%s'...", currentBackupDir)
	if err := runner.CopyFile(ctx.GoContext(), conn, s.remoteKubeDir, currentBackupDir, true, s.Sudo); err != nil {
		logger.Warnf("Failed to create safety backup at '%s'. Continuing anyway. Error: %v", currentBackupDir, err)
	}

	logger.Infof("Removing current configuration directory: '%s'...", s.remoteKubeDir)
	if err := runner.Remove(ctx.GoContext(), conn, s.remoteKubeDir, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove current config directory. Will attempt to restore anyway. Error: %v", err)
	}

	logger.Infof("Pushing local backup '%s' to remote '%s'...", s.localNodeDir, s.remoteKubeDir)
	if err := runner.Upload(ctx.GoContext(), conn, s.localNodeDir, s.remoteKubeDir, s.Sudo); err != nil {
		err = fmt.Errorf("failed to push backup directory to host '%s': %w", ctx.GetHost().GetName(), err)
		result.MarkFailed(err, "failed to restore configs")
		return result, err
	}

	logger.Info("Restarting kubelet to apply restored configuration...")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		err = fmt.Errorf("failed to gather host facts: %w", err)
		result.MarkFailed(err, "failed to gather host facts")
		return result, err
	}
	if err := runner.RestartService(ctx.GoContext(), conn, facts, "kubelet"); err != nil {
		err = fmt.Errorf("failed to restart kubelet after restore on host '%s': %w", ctx.GetHost().GetName(), err)
		result.MarkFailed(err, "failed to restart kubelet")
		return result, err
	}

	logger.Info("Restore operation completed. The node is attempting to revert to its pre-backup state.")
	result.MarkCompleted("restore operation completed")
	return result, nil
}

func (s *RestoreFetchedConfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Error("Rollback is not possible for a restore operation. The node is in its pre-backup state.")
	return nil
}

var _ step.Step = (*RestoreFetchedConfigsStep)(nil)
