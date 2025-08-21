package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmRemoteCleanupStep struct {
	step.Base
}

type KubeadmRemoteCleanupStepBuilder struct {
	step.Builder[KubeadmRemoteCleanupStepBuilder, *KubeadmRemoteCleanupStep]
}

func NewKubeadmRemoteCleanupStepBuilder(ctx runtime.Context, instanceName string) *KubeadmRemoteCleanupStepBuilder {
	s := &KubeadmRemoteCleanupStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Clean up temporary backup directories from the remote node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmRemoteCleanupStepBuilder).Init(s)
	return b
}

func (s *KubeadmRemoteCleanupStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmRemoteCleanupStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for remote cleanup...")
	logger.Info("Precheck passed: Cleanup will always be attempted if the step is reached.")
	return false, nil
}

func (s *KubeadmRemoteCleanupStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Starting remote backup cleanup...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("cannot connect to host for cleanup: %w", err)
	}

	var pkiBackupPath string
	if rawVal, ok := ctx.GetTaskCache().Get(common.CacheKubeCertsBackupPath); ok {
		if path, isString := rawVal.(string); isString {
			pkiBackupPath = path
		}
	}

	var kubeconfigBackupPath string
	if rawVal, ok := ctx.GetTaskCache().Get(common.CacheKubeconfigsBackupPath); ok {
		if path, isString := rawVal.(string); isString {
			kubeconfigBackupPath = path
		}
	}

	pathsToClean := []string{pkiBackupPath, kubeconfigBackupPath}
	var cleanedCount int

	for _, path := range pathsToClean {
		if path == "" {
			continue
		}

		log := logger.With("path", path)
		log.Info("Removing remote backup directory...")

		cleanupCmd := fmt.Sprintf("rm -rf %s", path)
		if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
			log.Errorf("Failed to remove remote backup directory. Manual cleanup may be needed. Error: %v", err)
		} else {
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		logger.Infof("Successfully cleaned up %d remote backup director(y/ies).", cleanedCount)
	} else {
		logger.Info("No remote backup paths found in cache for this host, nothing to clean up.")
	}

	return nil
}

func (s *KubeadmRemoteCleanupStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*KubeadmRemoteCleanupStep)(nil)
