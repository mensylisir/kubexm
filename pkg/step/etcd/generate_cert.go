package etcd

import (
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type GenerateEtcdCertsStep struct {
	step.Base
	LocalCertsDir  string
	EtcdNodes      []connector.Host
	CertDuration   time.Duration
	CaCertFileName string
	CaKeyFileName  string
	Permission     string
}

type GenerateEtcdCertsStepBuilder struct {
	step.Builder[GenerateEtcdCertsStepBuilder, *GenerateEtcdCertsStep]
}

func NewGenerateEtcdCertsStepBuilder(ctx runtime.Context, instanceName string) *GenerateEtcdCertsStepBuilder {
	s := &GenerateEtcdCertsStep{
		LocalCertsDir:  filepath.Join(ctx.GetGlobalWorkDir(), "certs", "etcd"),
		EtcdNodes:      ctx.GetHostsByRole(common.RoleEtcd),
		CertDuration:   365 * 24 * time.Hour * 10,
		CaCertFileName: common.EtcdCaPemFileName,
		CaKeyFileName:  common.EtcdCaKeyPemFileName,
		Permission:     "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate etcd node and client certificates", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(GenerateEtcdCertsStepBuilder).Init(s)
	return b
}

func (b *GenerateEtcdCertsStepBuilder) WithCaCertFileName(name string) *GenerateEtcdCertsStepBuilder {
	b.Step.CaCertFileName = name
	return b
}

func (b *GenerateEtcdCertsStepBuilder) WithCaKeyFileName(name string) *GenerateEtcdCertsStepBuilder {
	b.Step.CaKeyFileName = name
	return b
}

func (b *GenerateEtcdCertsStepBuilder) WithLocalCertsDir(path string) *GenerateEtcdCertsStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *GenerateEtcdCertsStepBuilder) WithCADuration(duration time.Duration) *GenerateEtcdCertsStepBuilder {
	b.Step.CertDuration = duration
	return b
}

func (b *GenerateEtcdCertsStepBuilder) WithPermission(permission string) *GenerateEtcdCertsStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *GenerateEtcdCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateEtcdCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	for _, node := range s.EtcdNodes {
		nodeName := node.GetName()
		adminCertFile := fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)
		adminKeyFile := fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)
		nodeCertFile := fmt.Sprintf(common.EtcdNodeCertFileNamePattern, nodeName)
		nodeKeyFile := fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, nodeName)
		memberCertFile := fmt.Sprintf(common.EtcdMemberCertFileNamePattern, nodeName)
		memberKeyFile := fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, nodeName)

		if !fileExists(s.LocalCertsDir, adminCertFile) ||
			!fileExists(s.LocalCertsDir, adminKeyFile) ||
			!fileExists(s.LocalCertsDir, nodeCertFile) ||
			!fileExists(s.LocalCertsDir, nodeKeyFile) ||
			!fileExists(s.LocalCertsDir, memberCertFile) ||
			!fileExists(s.LocalCertsDir, memberKeyFile) {
			return false, nil
		}
	}
	return true, nil
}

func (s *GenerateEtcdCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	logger.Info("Loading ETCD CA...")
	caCertPath := filepath.Join(s.LocalCertsDir, s.CaCertFileName)
	caKeyPath := filepath.Join(s.LocalCertsDir, s.CaKeyFileName)
	caCert, caKey, err := helpers.LoadCertificateAuthority(caCertPath, caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load ETCD CA (cert: %s), please ensure CA generation step ran successfully: %w", caCertPath, caKeyPath, err)
	}

	altNames := helpers.AltNames{
		DNSNames: []string{"localhost", "etcd", "etcd.kube-system", "etcd.kube-system.svc",
			fmt.Sprintf("etcd.kube-system.svc.%s", ctx.GetClusterConfig().Spec.Kubernetes.ClusterName),
			ctx.GetClusterConfig().Spec.ControlPlaneEndpoint.Domain},
		IPs: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0:0:0:0:0:0:0:1")},
	}
	for _, node := range s.EtcdNodes {
		altNames.DNSNames = append(altNames.DNSNames, node.GetName())
		if ip := net.ParseIP(node.GetAddress()); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		}
		if node.GetInternalAddress() != "" && node.GetInternalAddress() != node.GetAddress() {
			if ip := net.ParseIP(node.GetInternalAddress()); ip != nil {
				altNames.IPs = append(altNames.IPs, ip)
			}
		}
	}

	for _, node := range s.EtcdNodes {
		//altNames := helpers.AltNames{
		//	DNSNames: []string{node.GetName(), "localhost", "etcd", "etcd.kube-system", "etcd.kube-system.svc",
		//		fmt.Sprintf("etcd.kube-system.svc.%s", ctx.GetClusterConfig().Spec.Kubernetes.ClusterName),
		//		ctx.GetClusterConfig().Spec.ControlPlaneEndpoint.Domain},
		//	IPs: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0:0:0:0:0:0:0:1")},
		//}
		//if ip := net.ParseIP(node.GetAddress()); ip != nil {
		//	altNames.IPs = append(altNames.IPs, ip)
		//}
		//if node.GetInternalAddress() != "" && node.GetInternalAddress() != node.GetAddress() {
		//	if ip := net.ParseIP(node.GetInternalAddress()); ip != nil {
		//		altNames.IPs = append(altNames.IPs, ip)
		//	}
		//}

		nodeName := node.GetName()
		logger.Info("Generating certificates for etcd node", "node", nodeName)
		adminCfg := helpers.CertConfig{
			CommonName:   fmt.Sprintf("%s-admin", nodeName),
			Organization: []string{"kubexm-admin"},
			AltNames:     altNames,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			Duration:     s.CertDuration,
		}
		adminCertFile := fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)
		adminKeyFile := fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)
		if err := helpers.NewSignedCertificate(s.LocalCertsDir, adminCertFile, adminKeyFile, adminCfg, caCert, caKey); err != nil {
			return fmt.Errorf("failed to generate etcd admin certificate for node %s: %w", nodeName, err)
		}

		nodeCfg := helpers.CertConfig{
			CommonName:   fmt.Sprintf("%s-node", nodeName),
			Organization: []string{"kubexm-node"},
			AltNames:     altNames,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			Duration:     s.CertDuration,
		}
		nodeCertFile := fmt.Sprintf(common.EtcdNodeCertFileNamePattern, nodeName)
		nodeKeyFile := fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, nodeName)
		if err := helpers.NewSignedCertificate(s.LocalCertsDir, nodeCertFile, nodeKeyFile, nodeCfg, caCert, caKey); err != nil {
			return fmt.Errorf("failed to generate etcd node certificate for node %s: %w", nodeName, err)
		}

		memberCfg := helpers.CertConfig{
			CommonName:   fmt.Sprintf("%s-member", nodeName),
			Organization: []string{"kubexm-etcd"},
			AltNames:     altNames,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			Duration:     s.CertDuration,
		}
		memberCertFile := fmt.Sprintf(common.EtcdMemberCertFileNamePattern, nodeName)
		memberKeyFile := fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, nodeName)
		if err := helpers.NewSignedCertificate(s.LocalCertsDir, memberCertFile, memberKeyFile, memberCfg, caCert, caKey); err != nil {
			return fmt.Errorf("failed to generate etcd member certificate for node %s: %w", nodeName, err)
		}
	}

	logger.Info("All etcd node and client certificates generated successfully.")
	return nil
}

func (s *GenerateEtcdCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	filesToRemove := []string{}
	for _, node := range s.EtcdNodes {
		nodeName := node.GetName()
		filesToRemove = append(filesToRemove,
			fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName),
			fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName),
			fmt.Sprintf(common.EtcdNodeCertFileNamePattern, nodeName),
			fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, nodeName),
			fmt.Sprintf(common.EtcdMemberCertFileNamePattern, nodeName),
			fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, nodeName),
		)
	}

	for _, file := range filesToRemove {
		path := filepath.Join(s.LocalCertsDir, file)
		logger.Warn("Rolling back by deleting file", "path", path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Error(err, "Failed to remove file during rollback", "path", path)
		}
	}

	return nil
}

func fileExists(dir, file string) bool {
	if file == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}

var _ step.Step = (*GenerateEtcdCertsStep)(nil)
