package kubeadm

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers" // USING YOUR HELPERS
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type KubeadmCreateKubeconfigsStep struct {
	step.Base
	caToUseDir string
	outputDir  string
}

type KubeadmCreateKubeconfigsStepBuilder struct {
	step.Builder[KubeadmCreateKubeconfigsStepBuilder, *KubeadmCreateKubeconfigsStep]
}

func NewKubeadmCreateKubeconfigsStepBuilder(ctx runtime.Context, instanceName string) *KubeadmCreateKubeconfigsStepBuilder {
	s := &KubeadmCreateKubeconfigsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Create new kubeconfig files (admin, controller-manager, scheduler, kubelet)"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmCreateKubeconfigsStepBuilder).Init(s)
	return b
}

func (s *KubeadmCreateKubeconfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmCreateKubeconfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for kubeconfig creation...")

	baseCertsDir := ctx.GetKubernetesCertsDir()
	s.caToUseDir = baseCertsDir
	s.outputDir = baseCertsDir

	if !helpers.IsFileExist(filepath.Join(s.caToUseDir, "ca.crt")) {
		return false, fmt.Errorf("precheck failed: CA certificate not found in '%s'", s.caToUseDir)
	}

	logger.Info("Precheck passed.")
	return false, nil
}

func (s *KubeadmCreateKubeconfigsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	clusterSpec := ctx.GetClusterConfig().Spec

	logger.Infof("Creating new kubeconfig files in '%s'...", s.outputDir)

	kubeconfigs := []struct {
		fileName string
		baseName string
		config   helpers.CertConfig
	}{
		{
			fileName: "admin.conf", baseName: "admin",
			config: helpers.CertConfig{
				CommonName:   "kubernetes-admin",
				Organization: []string{"system:masters"},
				Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
		},
		{
			fileName: "controller-manager.conf", baseName: "controller-manager",
			config: helpers.CertConfig{
				CommonName: "system:kube-controller-manager",
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
		},
		{
			fileName: "scheduler.conf", baseName: "scheduler",
			config: helpers.CertConfig{
				CommonName: "system:kube-scheduler",
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
		},
	}

	caCertPath := filepath.Join(s.caToUseDir, "ca.crt")
	caKeyPath := filepath.Join(s.caToUseDir, "ca.key")
	caCert, caKey, err := helpers.LoadCertificateAuthority(caCertPath, caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load main CA: %w", err)
	}

	serverURL := fmt.Sprintf("https://%s:%s", clusterSpec.ControlPlaneEndpoint.Domain, common.DefaultAPIServerPort)

	for _, cfg := range kubeconfigs {
		if err := s.generateKubeconfig(logger, cfg.fileName, cfg.baseName, cfg.config, serverURL, caCert, caKey); err != nil {
			return err
		}
	}

	for _, node := range ctx.GetHostsByRole("") {
		nodeName := node.GetName()
		kubeletBaseName := fmt.Sprintf("kubelet-%s", nodeName)
		kubeletFileName := fmt.Sprintf("%s.conf", kubeletBaseName)
		kubeletConfig := helpers.CertConfig{
			CommonName:   fmt.Sprintf("system:node:%s", nodeName),
			Organization: []string{"system:nodes"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		if err := s.generateKubeconfig(logger, kubeletFileName, kubeletBaseName, kubeletConfig, serverURL, caCert, caKey); err != nil {
			return err
		}
	}

	logger.Info("All kubeconfig files created successfully.")
	return nil
}

func (s *KubeadmCreateKubeconfigsStep) generateKubeconfig(logger *logger.Logger, fileName, baseName string, certCfg helpers.CertConfig, serverURL string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) error {
	log := logger.With("kubeconfig", fileName)
	log.Info("Generating client certificate and kubeconfig file...")

	certFile := fmt.Sprintf("%s.crt", baseName)
	keyFile := fmt.Sprintf("%s.key", baseName)
	if err := helpers.NewSignedCertificate(s.outputDir, certFile, keyFile, certCfg, caCert, caKey); err != nil {
		return fmt.Errorf("failed to create client certificate for '%s': %w", baseName, err)
	}

	caData, err := os.ReadFile(filepath.Join(s.caToUseDir, "ca.crt"))
	if err != nil {
		return err
	}
	clientCertData, err := os.ReadFile(filepath.Join(s.outputDir, certFile))
	if err != nil {
		return err
	}
	clientKeyData, err := os.ReadFile(filepath.Join(s.outputDir, keyFile))
	if err != nil {
		return err
	}

	config := clientcmdapi.NewConfig()
	config.Clusters["kubernetes"] = &clientcmdapi.Cluster{
		Server:                   serverURL,
		CertificateAuthorityData: caData,
	}
	config.AuthInfos[certCfg.CommonName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: clientCertData,
		ClientKeyData:         clientKeyData,
	}
	contextName := fmt.Sprintf("%s@kubernetes", certCfg.CommonName)
	config.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:  "kubernetes",
		AuthInfo: certCfg.CommonName,
	}
	config.CurrentContext = contextName

	outputKubeconfigPath := filepath.Join(s.outputDir, fileName)
	if err := clientcmd.WriteToFile(*config, outputKubeconfigPath); err != nil {
		return fmt.Errorf("failed to write kubeconfig file '%s': %w", outputKubeconfigPath, err)
	}
	log.Info("Successfully created new kubeconfig file.")
	return nil
}

func (s *KubeadmCreateKubeconfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	if s.outputDir == filepath.Join(ctx.GetKubernetesCertsDir(), "certs-new") {
		logger.Warn("Rolling back by deleting newly generated kubeconfig files and their associated assets from 'certs-new'...")

		baseNames := []string{"admin", "controller-manager", "scheduler"}
		for _, node := range ctx.GetHostsByRole("") {
			baseNames = append(baseNames, fmt.Sprintf("kubelet-%s", node.GetName()))
		}

		for _, baseName := range baseNames {
			log := logger.With("asset_base", baseName)

			confFile := filepath.Join(s.outputDir, fmt.Sprintf("%s.conf", baseName))
			certFile := filepath.Join(s.outputDir, fmt.Sprintf("%s.crt", baseName))
			keyFile := filepath.Join(s.outputDir, fmt.Sprintf("%s.key", baseName))

			log.Debugf("Removing '%s'...", confFile)
			if err := os.Remove(confFile); err != nil && !os.IsNotExist(err) {
				log.Errorf("Failed to remove kubeconfig file during rollback: %v", err)
			}

			log.Debugf("Removing '%s'...", certFile)
			if err := os.Remove(certFile); err != nil && !os.IsNotExist(err) {
				log.Errorf("Failed to remove certificate file during rollback: %v", err)
			}

			log.Debugf("Removing '%s'...", keyFile)
			if err := os.Remove(keyFile); err != nil && !os.IsNotExist(err) {
				log.Errorf("Failed to remove key file during rollback: %v", err)
			}
		}
		logger.Info("Rollback of kubeconfig generation finished.")
	} else {
		logger.Warn("Rollback for in-place kubeconfig generation is not performed automatically to avoid data loss.")
	}

	return nil
}
