package kubelet

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type GenerateKubeletKubeconfigForNodesStep struct {
	step.Base
	CertsDir         string
	KubeconfigsDir   string
	ClusterName      string
	APIServerAddress string
}

type GenerateKubeletKubeconfigForNodesStepBuilder struct {
	step.Builder[GenerateKubeletKubeconfigForNodesStepBuilder, *GenerateKubeletKubeconfigForNodesStep]
}

func NewGenerateKubeletKubeconfigForNodesStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeletKubeconfigForNodesStepBuilder {
	s := &GenerateKubeletKubeconfigForNodesStep{
		CertsDir:         filepath.Join(ctx.GetGlobalWorkDir(), "certs", "kubernetes"),
		KubeconfigsDir:   filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs"),
		ClusterName:      ctx.GetClusterConfig().ObjectMeta.Name,
		APIServerAddress: fmt.Sprintf("https://%s:%s", ctx.GetClusterConfig().Spec.ControlPlaneEndpoint.Domain, common.KubeAPIServerDefaultPort),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubelet kubeconfig for all worker nodes", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(GenerateKubeletKubeconfigForNodesStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeletKubeconfigForNodesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateKubeletKubeconfigForNodesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	allHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	allHosts = helpers.UnionBy(allHosts, masterHosts, func(h connector.Host) string {
		return h.GetName()
	})
	for _, node := range allHosts {
		kubeconfigFileName := fmt.Sprintf("kubelet-%s.kubeconfig", node.GetName())
		if !helpers.FileExists(s.KubeconfigsDir, kubeconfigFileName) {
			logger.Infof("Kubelet kubeconfig for worker node '%s' not found. Generation is required.", node.GetName())
			return false, nil
		}
	}

	logger.Info("All required kubelet kubeconfigs for worker nodes already exist. Step is done.")
	return true, nil
}

func (s *GenerateKubeletKubeconfigForNodesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	caCertPath := filepath.Join(s.CertsDir, common.CACertFileName)
	caData, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	allHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	allHosts = helpers.UnionBy(allHosts, masterHosts, func(h connector.Host) string {
		return h.GetName()
	})
	for _, node := range allHosts {
		nodeName := node.GetName()
		logger.Infof("Generating kubelet kubeconfig for worker node: %s", nodeName)

		contextName := fmt.Sprintf("system:node:%s@%s", nodeName, s.ClusterName)
		userName := fmt.Sprintf("%s%s", common.KubeletCertificateCNPrefix, nodeName)

		certPath := filepath.Join(s.CertsDir, fmt.Sprintf("kubelet-%s.crt", nodeName))
		keyPath := filepath.Join(s.CertsDir, fmt.Sprintf("kubelet-%s.key", nodeName))

		config := clientcmdapi.Config{
			Clusters: map[string]*clientcmdapi.Cluster{
				s.ClusterName: {
					Server:                   s.APIServerAddress,
					CertificateAuthorityData: caData,
				},
			},
			Contexts: map[string]*clientcmdapi.Context{
				contextName: {
					Cluster:   s.ClusterName,
					AuthInfo:  userName,
					Namespace: "default",
				},
			},
			AuthInfos: map[string]*clientcmdapi.AuthInfo{
				userName: {
					ClientCertificate: certPath,
					ClientKey:         keyPath,
				},
			},
			CurrentContext: contextName,
		}

		kubeconfigFileName := fmt.Sprintf("kubelet-%s.kubeconfig", nodeName)
		kubeconfigFilePath := filepath.Join(s.KubeconfigsDir, kubeconfigFileName)
		if err := clientcmd.WriteToFile(config, kubeconfigFilePath); err != nil {
			return fmt.Errorf("failed to write kubeconfig for node %s: %w", nodeName, err)
		}
	}

	logger.Info("All kubelet kubeconfigs for worker nodes generated successfully.")
	return nil
}

func (s *GenerateKubeletKubeconfigForNodesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	allHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	allHosts = helpers.UnionBy(allHosts, masterHosts, func(h connector.Host) string {
		return h.GetName()
	})
	for _, node := range allHosts {
		kubeconfigFileName := fmt.Sprintf("kubelet-%s.conf", node.GetName())
		path := filepath.Join(s.KubeconfigsDir, kubeconfigFileName)
		logger.Warnf("Rolling back by deleting file: %s", path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	}

	logger.Info("Rollback completed for worker node kubelet kubeconfigs.")
	return nil
}

var _ step.Step = (*GenerateKubeletKubeconfigForNodesStep)(nil)
