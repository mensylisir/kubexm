package kubelet

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DistributeKubeletConfigForAllNodesStep struct {
	step.Base
	LocalCertsDir       string
	LocalKubeconfigsDir string
	RemotePKIDir        string
	RemoteConfigDir     string
}

type DistributeKubeletConfigForAllNodesStepBuilder struct {
	step.Builder[DistributeKubeletConfigForAllNodesStepBuilder, *DistributeKubeletConfigForAllNodesStep]
}

func NewDistributeKubeletConfigForAllNodesStepBuilder(ctx runtime.Context, instanceName string) *DistributeKubeletConfigForAllNodesStepBuilder {
	s := &DistributeKubeletConfigForAllNodesStep{
		LocalCertsDir:       filepath.Join(ctx.GetGlobalWorkDir(), "certs", "kubernetes"),
		LocalKubeconfigsDir: filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs"),
		RemotePKIDir:        common.KubeletPKIDirTarget,
		RemoteConfigDir:     common.KubernetesConfigDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute kubelet credentials to node:%s", instanceName, ctx.GetHost().GetName())
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(DistributeKubeletConfigForAllNodesStepBuilder).Init(s)
	return b
}

func (s *DistributeKubeletConfigForAllNodesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type fileToDistribute struct {
	localSrcFile  string
	remoteDstFile string
	permissions   string
}

func (s *DistributeKubeletConfigForAllNodesStep) getFilesToDistribute(ctx runtime.ExecutionContext) []fileToDistribute {
	nodeName := ctx.GetHost().GetName()

	return []fileToDistribute{
		{
			localSrcFile:  filepath.Join(s.LocalCertsDir, fmt.Sprintf("kubelet-%s.crt", nodeName)),
			remoteDstFile: filepath.Join(s.RemotePKIDir, "kubelet.crt"),
			permissions:   "0644",
		},
		{
			localSrcFile:  filepath.Join(s.LocalCertsDir, fmt.Sprintf("kubelet-%s.key", nodeName)),
			remoteDstFile: filepath.Join(s.RemotePKIDir, "kubelet.key"),
			permissions:   "0600",
		},
		{
			localSrcFile:  filepath.Join(s.LocalKubeconfigsDir, fmt.Sprintf("kubelet-%s.conf", nodeName)),
			remoteDstFile: filepath.Join(s.RemoteConfigDir, common.KubeletKubeconfigFileName),
			permissions:   "0600",
		},
	}
}

func (s *DistributeKubeletConfigForAllNodesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	for _, file := range s.getFilesToDistribute(ctx) {
		exists, err := runner.Exists(ctx.GoContext(), conn, file.remoteDstFile)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}

	ctx.GetLogger().Info("All required kubelet credentials already exist on the remote master node. Step is done.")
	return true, nil
}

func (s *DistributeKubeletConfigForAllNodesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemotePKIDir, "0755", s.Sudo); err != nil {
		return err
	}
	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteConfigDir, "0755", s.Sudo); err != nil {
		return err
	}

	for _, file := range s.getFilesToDistribute(ctx) {
		if _, err := os.Stat(file.localSrcFile); os.IsNotExist(err) {
			return fmt.Errorf("local source file '%s' not found, ensure previous generation steps ran successfully", file.localSrcFile)
		}

		logger.Infof("Uploading %s to %s:%s", file.localSrcFile, ctx.GetHost().GetName(), file.remoteDstFile)
		if err := runner.Upload(ctx.GoContext(), conn, file.localSrcFile, file.remoteDstFile, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", filepath.Base(file.localSrcFile), err)
		}

		logger.Infof("Setting permissions for %s to %s", file.remoteDstFile, file.permissions)
		if err := runner.Chmod(ctx.GoContext(), conn, file.remoteDstFile, file.permissions, s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on '%s': %w", file.remoteDstFile, err)
		}
	}

	logger.Info("Kubelet credentials have been distributed successfully to the master node.")
	return nil
}

func (s *DistributeKubeletConfigForAllNodesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	for _, file := range s.getFilesToDistribute(ctx) {
		logger.Warnf("Rolling back by removing remote file: %s", file.remoteDstFile)
		if err := runner.Remove(ctx.GoContext(), conn, file.remoteDstFile, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", file.remoteDstFile, err)
		}
	}
	return nil
}

var _ step.Step = (*DistributeKubeletConfigForAllNodesStep)(nil)
