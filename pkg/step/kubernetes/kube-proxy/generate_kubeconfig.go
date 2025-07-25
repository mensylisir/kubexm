package kube_proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type GenerateKubeProxyKubeconfigStep struct {
	step.Base
	CertsDir         string
	KubeconfigsDir   string
	ClusterName      string
	APIServerAddress string
}

type GenerateKubeProxyKubeconfigStepBuilder struct {
	step.Builder[GenerateKubeProxyKubeconfigStepBuilder, *GenerateKubeProxyKubeconfigStep]
}

func NewGenerateKubeProxyKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeProxyKubeconfigStepBuilder {
	s := &GenerateKubeProxyKubeconfigStep{
		CertsDir:         filepath.Join(ctx.GetGlobalWorkDir(), "certs", "kubernetes"),
		KubeconfigsDir:   filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs"),
		ClusterName:      ctx.GetClusterConfig().ObjectMeta.Name,
		APIServerAddress: fmt.Sprintf("https://%s:%s", ctx.GetClusterConfig().Spec.ControlPlaneEndpoint.Domain, common.DefaultAPIServerPort),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeconfig file for kube-proxy", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(GenerateKubeProxyKubeconfigStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeProxyKubeconfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateKubeProxyKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if !helpers.FileExists(s.KubeconfigsDir, common.KubeProxyKubeconfigFileName) {
		return false, nil
	}
	ctx.GetLogger().Info("Kube-proxy kubeconfig file already exists. Step is done.")
	return true, nil
}

func (s *GenerateKubeProxyKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	caCertData, err := os.ReadFile(filepath.Join(s.CertsDir, common.CACertFileName))
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certPath := filepath.Join(s.CertsDir, common.KubeProxyClientCertFileName)
	keyPath := filepath.Join(s.CertsDir, common.KubeProxyClientKeyFileName)

	logger.Infof("Generating %s", common.KubeProxyKubeconfigFileName)

	config := api.NewConfig()

	config.Clusters[s.ClusterName] = &api.Cluster{
		Server:                   s.APIServerAddress,
		CertificateAuthorityData: caCertData,
	}

	config.AuthInfos[common.KubeProxyUser] = &api.AuthInfo{
		ClientCertificate: certPath,
		ClientKey:         keyPath,
	}

	contextName := "default"
	config.Contexts[contextName] = &api.Context{
		Cluster:  s.ClusterName,
		AuthInfo: common.KubeProxyUser,
	}

	config.CurrentContext = contextName

	outputPath := filepath.Join(s.KubeconfigsDir, common.KubeProxyKubeconfigFileName)
	if err := clientcmd.WriteToFile(*config, outputPath); err != nil {
		return fmt.Errorf("failed to write %s: %w", common.KubeProxyKubeconfigFileName, err)
	}

	logger.Info("Kube-proxy kubeconfig file generated successfully.")
	return nil
}

func (s *GenerateKubeProxyKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	path := filepath.Join(s.KubeconfigsDir, common.KubeProxyKubeconfigFileName)
	logger.Warnf("Rolling back by deleting file: %s", path)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove file during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*GenerateKubeProxyKubeconfigStep)(nil)
