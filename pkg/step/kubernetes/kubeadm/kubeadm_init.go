package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmInitStep struct {
	step.Base
}
type KubeadmInitStepBuilder struct {
	step.Builder[KubeadmInitStepBuilder, *KubeadmInitStep]
}

func NewKubeadmInitStepBuilder(ctx runtime.Context, instanceName string) *KubeadmInitStepBuilder {
	s := &KubeadmInitStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Initialize the first master node with kubeadm", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute
	b := new(KubeadmInitStepBuilder).Init(s)
	return b
}
func (s *KubeadmInitStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmInitStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Info("Kubeadm has already been initialized on this node (admin.conf exists). Step is done.")
		return true, nil
	}

	logger.Info("Kubeadm has not been initialized on this node yet. Step needs to run.")
	return false, nil
}

func (s *KubeadmInitStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmInitConfigFileName)
	cmd := fmt.Sprintf("kubeadm init --config %s --upload-certs", configPath)

	logger.Infof("Running command: %s", cmd)

	output, err := runner.Run(ctx.GoContext(), conn, cmd, true)
	if err != nil {
		return fmt.Errorf("kubeadm init failed: %w. Output:\n%s", err, output)
	}
	token, err := helpers.ParseTokenFromOutput(output)
	if err != nil {
		return fmt.Errorf("failed to parse bootstrap token from cached output: %w", err)
	}

	certKey, err := helpers.ParseCertificateKeyFromOutput(output)
	if err != nil {
		return fmt.Errorf("failed to parse certificate key from cached output: %w", err)
	}

	caCertHash, err := helpers.ParseCaCertHashFromOutput(output)
	if err != nil {
		return fmt.Errorf("failed to parse ca cert hash from cached output: %w", err)
	}
	ctx.GetTaskCache().Set(common.CacheKubeadmInitToken, token)
	ctx.GetTaskCache().Set(common.CacheKubeadmInitCertKey, certKey)
	ctx.GetTaskCache().Set(common.CacheKubeadmInitCACertHash, caCertHash)
	logger.Info("Kubeadm init completed successfully.")
	return nil
}

func (s *KubeadmInitStep) Rollback(ctx runtime.ExecutionContext) error {
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

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmInitConfigFileName)
	logger.Warnf("Rolling back by removing: %s", configPath)
	if err := runner.Remove(ctx.GoContext(), conn, configPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", configPath, err)
	}

	return nil
}
func getCriSocketFromSpec(cluster *v1alpha1.Cluster) string {
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		return common.ContainerdDefaultEndpoint
	case common.RuntimeTypeCRIO:
		return common.CRIODefaultEndpoint
	case common.RuntimeTypeDocker:
		return common.CriDockerdSocketPath
	case common.RuntimeTypeIsula:
		return common.IsuladDefaultEndpoint
	default:
		return common.ContainerdDefaultEndpoint
	}
}

var _ step.Step = (*KubeadmInitStep)(nil)
