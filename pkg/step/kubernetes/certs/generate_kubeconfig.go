package certs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type GenerateKubeconfigsStep struct {
	step.Base
	CertsDir         string
	OutputDir        string
	ClusterName      string
	APIServerAddress string
}

type GenerateKubeconfigsStepBuilder struct {
	step.Builder[GenerateKubeconfigsStepBuilder, *GenerateKubeconfigsStep]
}

func NewGenerateKubeconfigsStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeconfigsStepBuilder {
	s := &GenerateKubeconfigsStep{
		CertsDir:         filepath.Join(ctx.GetGlobalWorkDir(), "certs", "kubernetes"),
		OutputDir:        filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs"),
		ClusterName:      ctx.GetClusterConfig().ObjectMeta.Name,
		APIServerAddress: fmt.Sprintf("https://%s", ctx.GetClusterConfig().Spec.ControlPlaneEndpoint.Domain),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeconfig files for control plane components", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(GenerateKubeconfigsStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeconfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type kubeconfigDefinition struct {
	FileName string
	UserName string
	CertFile string
	KeyFile  string
}

func (s *GenerateKubeconfigsStep) getKubeconfigDefinitions() []kubeconfigDefinition {
	return []kubeconfigDefinition{
		{
			FileName: common.ControllerManagerKubeconfigFileName,
			UserName: common.KubeControllerManagerUser,
			CertFile: common.ControllerManagerCertFileName,
			KeyFile:  common.ControllerManagerKeyFileName,
		},
		{
			FileName: common.SchedulerKubeconfigFileName,
			UserName: common.KubeSchedulerUser,
			CertFile: common.SchedulerCertFileName,
			KeyFile:  common.SchedulerKeyFileName,
		},
		{
			FileName: common.AdminKubeconfigFileName,
			UserName: "kubernetes-admin",
			CertFile: common.AdminCertFileName,
			KeyFile:  common.AdminKeyFileName,
		},
	}
}

func (s *GenerateKubeconfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	for _, def := range s.getKubeconfigDefinitions() {
		targetPath := filepath.Join(s.OutputDir, def.FileName)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			logger.Infof("Kubeconfig file '%s' not found. Generation is required.", targetPath)
			return false, nil
		}
	}

	logger.Info("All required kubeconfig files already exist. Step is done.")
	return true, nil
}

func (s *GenerateKubeconfigsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory for kubeconfigs: %w", err)
	}
	caCertPath := filepath.Join(s.CertsDir, common.CACertFileName)
	caCertData, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate from %s: %w", caCertPath, err)
	}

	for _, def := range s.getKubeconfigDefinitions() {
		logger.Infof("Generating kubeconfig file: %s", def.FileName)

		clientCertPath := filepath.Join(s.CertsDir, def.CertFile)
		clientKeyPath := filepath.Join(s.CertsDir, def.KeyFile)

		config := api.NewConfig()

		config.Clusters[s.ClusterName] = &api.Cluster{
			Server:                   s.APIServerAddress,
			CertificateAuthorityData: caCertData,
		}

		config.AuthInfos[def.UserName] = &api.AuthInfo{
			ClientCertificate: clientCertPath,
			ClientKey:         clientKeyPath,
		}

		contextName := fmt.Sprintf("%s@%s", def.UserName, s.ClusterName)
		config.Contexts[contextName] = &api.Context{
			Cluster:  s.ClusterName,
			AuthInfo: def.UserName,
		}

		config.CurrentContext = contextName

		outputPath := filepath.Join(s.OutputDir, def.FileName)
		if err := clientcmd.WriteToFile(*config, outputPath); err != nil {
			return fmt.Errorf("failed to write kubeconfig file %s: %w", outputPath, err)
		}
	}

	logger.Info("All kubeconfig files generated successfully.")
	return nil
}

func (s *GenerateKubeconfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	for _, def := range s.getKubeconfigDefinitions() {
		path := filepath.Join(s.OutputDir, def.FileName)
		logger.Warnf("Rolling back by deleting kubeconfig file: %s", path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	}
	return nil
}

var _ step.Step = (*GenerateKubeconfigsStep)(nil)
