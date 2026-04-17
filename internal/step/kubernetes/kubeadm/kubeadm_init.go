package kubeadm

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
)

type KubeadmInitStep struct {
	step.Base
}
type KubeadmInitStepBuilder struct {
	step.Builder[KubeadmInitStepBuilder, *KubeadmInitStep]
}

func NewKubeadmInitStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmInitStepBuilder {
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

func (s *KubeadmInitStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmInitConfigFileName)
	cmd := fmt.Sprintf("kubeadm init --config %s --upload-certs", configPath)

	logger.Infof("Running command: %s", cmd)

	runResult, err := runner.Run(ctx.GoContext(), conn, cmd, true)
	if err != nil {
		err = fmt.Errorf("kubeadm init failed: %w. Output:\n%s", err, runResult.Stdout)
		result.MarkFailed(err, "kubeadm init failed")
		return result, err
	}
	token, err := helpers.ParseTokenFromOutput(runResult.Stdout)
	if err != nil {
		err = fmt.Errorf("failed to parse bootstrap token from cached output: %w", err)
		result.MarkFailed(err, "failed to parse bootstrap token")
		return result, err
	}

	certKey, err := helpers.ParseCertificateKeyFromOutput(runResult.Stdout)
	if err != nil {
		err = fmt.Errorf("failed to parse certificate key from cached output: %w", err)
		result.MarkFailed(err, "failed to parse certificate key")
		return result, err
	}

	caCertHash, err := helpers.ParseCaCertHashFromOutput(runResult.Stdout)
	if err != nil {
		err = fmt.Errorf("failed to parse ca cert hash from cached output: %w", err)
		result.MarkFailed(err, "failed to parse ca cert hash")
		return result, err
	}
	// Use stable task names for cache keys to ensure join tasks can find the data.
	cacheKey := fmt.Sprintf(common.CacheKubeadmInitToken, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), "KubeadmInit")
	ctx.GetTaskCache().Set(cacheKey, token)
	cacheKey = fmt.Sprintf(common.CacheKubeadmInitCertKey, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), "KubeadmInit")
	ctx.GetTaskCache().Set(cacheKey, certKey)
	cacheKey = fmt.Sprintf(common.CacheKubeadmInitCACertHash, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), "KubeadmInit")
	ctx.GetTaskCache().Set(cacheKey, caCertHash)
	logger.Info("Kubeadm init completed successfully.")
	result.MarkCompleted("kubeadm init completed successfully")
	return result, nil
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
