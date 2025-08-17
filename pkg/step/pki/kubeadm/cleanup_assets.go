package kubeadm

import (
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmLocalCleanupStep struct {
	step.Base
}

type KubeadmLocalCleanupStepBuilder struct {
	step.Builder[KubeadmLocalCleanupStepBuilder, *KubeadmLocalCleanupStep]
}

func NewKubeadmLocalCleanupStepBuilder(ctx runtime.Context, instanceName string) *KubeadmLocalCleanupStepBuilder {
	s := &KubeadmLocalCleanupStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Clean up temporary directories from the local workspace"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmLocalCleanupStepBuilder).Init(s)
	return b
}

func (s *KubeadmLocalCleanupStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmLocalCleanupStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *KubeadmLocalCleanupStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	logger.Info("Starting local workspace cleanup...")

	baseCertsDir := ctx.GetKubernetesCertsDir()

	for _, host := range ctx.GetHostsByRole(common.RoleMaster) {
		nodeDir := filepath.Join(baseCertsDir, host.GetName())
		log := logger.With("path", nodeDir)

		if _, err := os.Stat(nodeDir); err == nil {
			log.Info("Removing local node-specific PKI backup...")
			if err := os.RemoveAll(nodeDir); err != nil {
				log.Errorf("Failed to remove local node directory: %v", err)
			}
		}
	}

	dirsToClean := []string{
		filepath.Join(baseCertsDir, "certs-new"),
		filepath.Join(baseCertsDir, "certs-old"),
	}

	for _, dir := range dirsToClean {
		log := logger.With("path", dir)
		if _, err := os.Stat(dir); err == nil {
			log.Infof("Removing temporary directory...")
			if err := os.RemoveAll(dir); err != nil {
				log.Errorf("Failed to remove temporary directory: %v", err)
			}
		}
	}

	logger.Info("Local workspace cleanup finished.")
	return nil
}

func (s *KubeadmLocalCleanupStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*KubeadmLocalCleanupStep)(nil)
