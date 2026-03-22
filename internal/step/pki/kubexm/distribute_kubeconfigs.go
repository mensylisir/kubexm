package kubexm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type DistributeMasterKubeconfigsStep struct {
	step.Base
	localKubeconfigDir  string
	remoteKubeconfigDir string
}

// DistributeMasterKubeconfigsStepBuilder builds a DistributeMasterKubeconfigsStep.
type DistributeMasterKubeconfigsStepBuilder struct {
	step.Builder[DistributeMasterKubeconfigsStepBuilder, *DistributeMasterKubeconfigsStep]
}

func NewDistributeMasterKubeconfigsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DistributeMasterKubeconfigsStepBuilder {
	s := &DistributeMasterKubeconfigsStep{
		localKubeconfigDir:  filepath.Join(filepath.Dir(ctx.GetKubernetesCertsDir()), "kubeconfig"),
		remoteKubeconfigDir: common.KubernetesConfigDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute all necessary kubeconfigs (admin, controller-manager, scheduler, kube-proxy, kubelet) to a master node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(DistributeMasterKubeconfigsStepBuilder).Init(s)
	return b
}

func (s *DistributeMasterKubeconfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeMasterKubeconfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for master kubeconfig distribution...")

	configsToCheck := []string{
		common.AdminKubeconfigFileName,
		common.ControllerManagerKubeconfigFileName,
		common.SchedulerKubeconfigFileName,
		common.KubeProxyKubeconfigFileName,
		common.KubeletKubeconfigFileName,
	}

	for _, confFile := range configsToCheck {
		localPath := filepath.Join(s.localKubeconfigDir, confFile)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return false, fmt.Errorf("precheck failed: local source kubeconfig file '%s' not found in '%s'", confFile, s.localKubeconfigDir)
		}
	}

	logger.Info("Precheck passed: backup found and all local source files exist.")
	return false, nil
}

func (s *DistributeMasterKubeconfigsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get host connector")
		return result, err
	}

	nodeName := ctx.GetHost().GetName()

	filesToUpload := map[string]string{
		common.AdminKubeconfigFileName:         common.AdminKubeconfigFileName,
		common.ControllerManagerKubeconfigFileName: common.ControllerManagerKubeconfigFileName,
		common.SchedulerKubeconfigFileName:     common.SchedulerKubeconfigFileName,
		common.KubeProxyKubeconfigFileName:     common.KubeProxyKubeconfigFileName,
		common.KubeletKubeconfigFileName:       "kubelet.conf",
	}

	logger.Infof("Distributing %d kubeconfig files to '%s' on host %s...", len(filesToUpload), s.remoteKubeconfigDir, nodeName)

	for localFile, remoteFile := range filesToUpload {
		localPath := filepath.Join(s.localKubeconfigDir, localFile)
		remotePath := filepath.Join(s.remoteKubeconfigDir, remoteFile)

		log := logger.With("file", remoteFile)
		log.Infof("Uploading %s to %s", localPath, remotePath)

		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to upload kubeconfig '%s'", localFile))
			return result, err
		}
	}

	logger.Info("Successfully distributed all required kubeconfigs to the master node.")
	result.MarkCompleted("master kubeconfigs distributed successfully")
	return result, nil
}

func (s *DistributeMasterKubeconfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Error("CRITICAL: No kubeconfig backup path found in cache. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
	return nil
}
