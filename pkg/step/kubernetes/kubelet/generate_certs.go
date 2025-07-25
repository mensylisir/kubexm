package kubelet

import (
	"crypto/x509"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type GenerateKubeletCertsForAllNodesStep struct {
	step.Base
	CertsDir     string
	CertDuration time.Duration
}

type GenerateKubeletCertsForAllNodesStepBuilder struct {
	step.Builder[GenerateKubeletCertsForAllNodesStepBuilder, *GenerateKubeletCertsForAllNodesStep]
}

func NewGenerateKubeletCertsForAllNodesStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeletCertsForAllNodesStepBuilder {
	s := &GenerateKubeletCertsForAllNodesStep{
		CertsDir:     filepath.Join(ctx.GetGlobalWorkDir(), "certs", "kubernetes"),
		CertDuration: common.DefaultCertificateValidityDays * 24 * time.Hour,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubelet client certificates for all nodes", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(GenerateKubeletCertsForAllNodesStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeletCertsForAllNodesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateKubeletCertsForAllNodesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	allHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	allHosts = helpers.UnionBy(allHosts, masterHosts, func(h connector.Host) string {
		return h.GetName()
	})
	for _, node := range allHosts {
		nodeName := node.GetName()
		certFile := fmt.Sprintf("kubelet-%s.crt", nodeName)
		keyFile := fmt.Sprintf("kubelet-%s.key", nodeName)
		if !helpers.FileExists(s.CertsDir, certFile) || !helpers.FileExists(s.CertsDir, keyFile) {
			logger.Infof("Kubelet certificate for node '%s' not found. Generation is required.", nodeName)
			return false, nil
		}
	}
	logger.Info("All required kubelet certificates for all nodes already exist. Step is done.")
	return true, nil
}

func (s *GenerateKubeletCertsForAllNodesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	mainCA, mainKey, err := helpers.LoadCertificateAuthority(filepath.Join(s.CertsDir, common.CACertFileName), filepath.Join(s.CertsDir, common.CAKeyFileName))
	if err != nil {
		return err
	}

	allHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	allHosts = helpers.UnionBy(allHosts, masterHosts, func(h connector.Host) string {
		return h.GetName()
	})
	for _, node := range allHosts {
		nodeName := node.GetName()
		logger.Infof("Generating kubelet client certificate for node: %s", nodeName)

		kubeletClientCfg := helpers.CertConfig{
			CommonName:   fmt.Sprintf("%s%s", common.KubeletCertificateCNPrefix, nodeName),
			Organization: []string{common.KubeletCertificateOrganization},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			Duration:     s.CertDuration,
		}

		certFile := fmt.Sprintf("kubelet-%s.crt", nodeName)
		keyFile := fmt.Sprintf("kubelet-%s.key", nodeName)

		if err := helpers.NewSignedCertificate(s.CertsDir, certFile, keyFile, kubeletClientCfg, mainCA, mainKey); err != nil {
			return fmt.Errorf("failed to generate kubelet client cert for %s: %w", nodeName, err)
		}
	}

	logger.Info("All kubelet client certificates for all nodes generated successfully.")
	return nil
}

func (s *GenerateKubeletCertsForAllNodesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	allHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	allHosts = helpers.UnionBy(allHosts, masterHosts, func(h connector.Host) string {
		return h.GetName()
	})

	for _, node := range allHosts {
		nodeName := node.GetName()
		certFile := fmt.Sprintf("kubelet-%s.crt", nodeName)
		keyFile := fmt.Sprintf("kubelet-%s.key", nodeName)
		_ = os.Remove(filepath.Join(s.CertsDir, certFile))
		_ = os.Remove(filepath.Join(s.CertsDir, keyFile))
	}
	logger.Info("Rollback completed for worker node kubelet certificates.")
	return nil
}

func (s *GenerateKubeletCertsForAllNodesStep) getWorkerOnlyNodes(ctx runtime.ExecutionContext) []connector.Host {
	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)

	return helpers.DifferenceBy(workerHosts, masterHosts, func(h connector.Host) string {
		return h.GetName()
	})
}

var _ step.Step = (*GenerateKubeletCertsForAllNodesStep)(nil)
