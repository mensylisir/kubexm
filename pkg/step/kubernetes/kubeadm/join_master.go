package kubeadm

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmJoinMasterStep struct {
	step.Base
}
type KubeadmJoinMasterStepBuilder struct {
	step.Builder[KubeadmJoinMasterStepBuilder, *KubeadmJoinMasterStep]
}

func NewKubeadmJoinMasterStepBuilder(ctx runtime.Context, instanceName string) *KubeadmJoinMasterStepBuilder {
	s := &KubeadmJoinMasterStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Join a new master node to the cluster", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	b := new(KubeadmJoinMasterStepBuilder).Init(s)
	return b
}
func (s *KubeadmJoinMasterStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmJoinMasterStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	adminConfPath := filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)
	exists, err := runner.Exists(ctx.GoContext(), conn, adminConfPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s': %w", adminConfPath, err)
	}

	if exists {
		logger.Info("This node has already joined the cluster as a master (admin.conf exists). Step is done.")
		return true, nil
	}

	logger.Info("This node has not joined the cluster as a master yet. Step needs to run.")
	return false, nil
}

func (s *KubeadmJoinMasterStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinMasterConfigFileName)
	cmd := fmt.Sprintf("kubeadm join --config %s", configPath)

	logger.Infof("Running command: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, true); err != nil {
		return fmt.Errorf("kubeadm join master failed: %w", err)
	}

	logger.Info("Successfully joined the cluster as a master node.")
	return nil
}

func (s *KubeadmJoinMasterStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	criSocket := getCriSocketFromSpec(ctx.GetClusterConfig())
	cmd := fmt.Sprintf("kubeadm reset --cri-socket %s --force", criSocket)

	logger.Warnf("Rolling back by running command: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, true); err != nil {
		logger.Warnf("kubeadm reset command failed, but continuing rollback: %v", err)
	} else {
		logger.Info("Kubeadm reset completed.")
	}

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmJoinMasterConfigFileName)
	logger.Warnf("Rolling back by removing: %s", configPath)
	if err := runner.Remove(ctx.GoContext(), conn, configPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", configPath, err)
	}

	return nil
}

var _ step.Step = (*KubeadmJoinMasterStep)(nil)
