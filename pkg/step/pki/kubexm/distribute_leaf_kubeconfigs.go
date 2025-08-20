package kubexm

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DistributeWorkerKubeconfigsStep struct {
	step.Base
	localPkiDir         string
	localKubeconfigDir  string
	remotePkiDir        string
	remoteKubeconfigDir string
}

type DistributeWorkerKubeconfigsStepBuilder struct {
	step.Builder[DistributeWorkerKubeconfigsStepBuilder, *DistributeWorkerKubeconfigsStep]
}

func NewDistributeWorkerKubeconfigsStepBuilder(ctx runtime.Context, instanceName string) *DistributeWorkerKubeconfigsStepBuilder {
	s := &DistributeWorkerKubeconfigsStep{
		localPkiDir:         ctx.GetKubernetesCertsDir(),
		localKubeconfigDir:  filepath.Join(filepath.Dir(ctx.GetKubernetesCertsDir()), "kubeconfig"),
		remotePkiDir:        common.KubernetesPKIDir,
		remoteKubeconfigDir: common.KubernetesConfigDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute CA, kubelet.conf, and kube-proxy.conf to a worker node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(DistributeWorkerKubeconfigsStepBuilder).Init(s)
	return b
}

func (s *DistributeWorkerKubeconfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeWorkerKubeconfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for worker certificate distribution...")
	logger.Info("Precheck passed: File distribution will be attempted.")
	return false, nil
}

func (s *DistributeWorkerKubeconfigsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	currentNodeName := ctx.GetHost().GetName()

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.remotePkiDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote PKI directory '%s': %w", s.remotePkiDir, err)
	}
	if err := runner.Mkdirp(ctx.GoContext(), conn, s.remoteKubeconfigDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote kubeconfig directory '%s': %w", s.remoteKubeconfigDir, err)
	}

	//localCaPath := filepath.Join(s.localPkiDir, common.CACertFileName)
	//remoteCaPath := filepath.Join(s.remotePkiDir, common.CACertFileName)
	//logger.Infof("Uploading CA certificate to %s...", remoteCaPath)
	//if err := runner.Upload(ctx.GoContext(), conn, localCaPath, remoteCaPath, s.Sudo); err != nil {
	//	return fmt.Errorf("failed to upload ca.crt: %w", err)
	//}

	kubeletConfFileName := fmt.Sprintf(common.KubeletKubeconfigFileName, currentNodeName)
	localKubeletConfPath := filepath.Join(s.localKubeconfigDir, kubeletConfFileName)
	remoteKubeletConfPath := filepath.Join(s.remoteKubeconfigDir, "kubelet.conf")
	logger.Infof("Uploading kubelet kubeconfig for node %s to %s...", currentNodeName, remoteKubeletConfPath)
	if err := runner.Upload(ctx.GoContext(), conn, localKubeletConfPath, remoteKubeletConfPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload kubelet.conf for node %s: %w", currentNodeName, err)
	}

	localKubeProxyConfPath := filepath.Join(s.localKubeconfigDir, common.KubeProxyKubeconfigFileName)
	remoteKubeProxyConfPath := filepath.Join(s.remoteKubeconfigDir, common.KubeProxyKubeconfigFileName)
	logger.Infof("Uploading kube-proxy kubeconfig to %s...", remoteKubeProxyConfPath)
	if err := runner.Upload(ctx.GoContext(), conn, localKubeProxyConfPath, remoteKubeProxyConfPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload kube-proxy.conf: %w", err)
	}

	logger.Info("Successfully distributed all required certificates and kubeconfigs to the worker node.")
	return nil
}

func (s *DistributeWorkerKubeconfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for worker certificate distribution is not performed atomically. Recovery relies on the initial node backup.")
	return nil
}

var _ step.Step = (*DistributeWorkerKubeconfigsStep)(nil)
