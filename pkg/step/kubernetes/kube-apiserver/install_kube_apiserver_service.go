package kube_apiserver

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type InstallKubeAPIServerServiceStep struct {
	step.Base
	AdvertiseAddress      string
	BindAddress           string
	BindPort              int
	ServiceClusterIPRange string
	ServiceNodePortRange  string
	AuthorizationMode     string
	AllowPrivileged       bool
	PKIDir                string
	AdmissionPlugins      string
	ServiceAccountIssuer  string
	EtcdPKIDir            string
	EtcdClientCertFile    string
	EtcdClientKeyFile     string
	EtcdServers           string
	EtcdPrefix            string
	TlsCipherSuites       string
	Audit                 *v1alpha1.AuditConfig
	FeatureGates          string
	ExtraArgs             map[string]string
	LogLevel              int
	RemoteServiceFile     string
	RemoteAuditPolicyFile string
}

type InstallKubeAPIServerServiceStepBuilder struct {
	step.Builder[InstallKubeAPIServerServiceStepBuilder, *InstallKubeAPIServerServiceStep]
}

func NewInstallKubeAPIServerServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeAPIServerServiceStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes
	apiCfg := k8sSpec.APIServer

	s := &InstallKubeAPIServerServiceStep{
		BindAddress:           "0.0.0.0",
		ServiceClusterIPRange: common.DefaultKubeServiceCIDR,
		ServiceNodePortRange:  common.DefaultServiceNodePortRange,
		AuthorizationMode:     "Node, RBAC",
		AllowPrivileged:       true,
		PKIDir:                common.KubernetesPKIDir,
		AdmissionPlugins:      "NodeRestriction",
		ServiceAccountIssuer:  fmt.Sprintf("https://kubernetes.default.svc.%s", k8sSpec.DNSDomain),
		EtcdPKIDir:            common.DefaultEtcdPKIDir,
		EtcdPrefix:            "/registry",
		LogLevel:              2,
		RemoteServiceFile:     common.DefaultKubeApiserverServiceFile,
	}

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	var etcdEndpoints []string
	for _, etcdNode := range etcdNodes {
		etcdEndpoints = append(etcdEndpoints, fmt.Sprintf("https://%s:%d", etcdNode.GetAddress(), common.DefaultEtcdClientPort))
	}
	s.EtcdServers = strings.Join(etcdEndpoints, ",")

	if apiCfg.TlsCipherSuites != nil {
		s.TlsCipherSuites = strings.Join(apiCfg.TlsCipherSuites, ",")
	}

	if apiCfg.AuditConfig != nil {
		s.Audit = apiCfg.AuditConfig
	}
	if apiCfg.ExtraArgs != nil {
		s.ExtraArgs = apiCfg.ExtraArgs
	}

	if apiCfg.AuditConfig != nil && apiCfg.AuditConfig.Enabled != nil && *apiCfg.AuditConfig.Enabled {
		s.RemoteAuditPolicyFile = apiCfg.AuditConfig.PolicyFile
	}

	if len(apiCfg.FeatureGates) > 0 {
		var fg []string
		for k, v := range apiCfg.FeatureGates {
			fg = append(fg, fmt.Sprintf("%s=%t", k, v))
		}
		s.FeatureGates = strings.Join(fg, ",")
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kube-apiserver systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeAPIServerServiceStepBuilder).Init(s)
	return b
}

func (s *InstallKubeAPIServerServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeAPIServerServiceStep) render(ctx runtime.ExecutionContext) (string, error) {
	currentNode := ctx.GetHost()
	s.AdvertiseAddress = currentNode.GetInternalAddress()
	if s.AdvertiseAddress == "" {
		s.AdvertiseAddress = currentNode.GetAddress()
	}
	firstEtcdNodeName := ctx.GetHostsByRole(common.RoleEtcd)[0].GetName()
	s.EtcdClientCertFile = fmt.Sprintf(common.EtcdNodeCertFileNamePattern, firstEtcdNodeName)
	s.EtcdClientKeyFile = fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, firstEtcdNodeName)

	tmplContent, err := templates.Get("kubernetes/kube-apiserver.service.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-apiserver service template: %w", err)
	}

	tmpl, err := template.New("kube-apiserver.service").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse kube-apiserver service template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", fmt.Errorf("failed to render kube-apiserver service template: %w", err)
	}

	return buffer.String(), nil
}

func (s *InstallKubeAPIServerServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteServiceFile)
	if err != nil {
		return false, err
	}
	if !exists {
		logger.Info("kube-apiserver.service file does not exist. Configuration is required.")
		return false, nil
	}
	remoteSvcContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteServiceFile)
	if err != nil {
		return false, fmt.Errorf("failed to read remote service file %s: %w", s.RemoteServiceFile, err)
	}
	expectedSvcContent, err := s.render(ctx)
	if err != nil {
		return false, err
	}
	if string(remoteSvcContent) != expectedSvcContent {
		logger.Warn("Remote kube-apiserver.service file content mismatch. Re-configuration is required.")
		return false, nil
	}

	if s.Audit != nil && s.Audit.Enabled != nil && *s.Audit.Enabled {
		exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteAuditPolicyFile)
		if err != nil {
			return false, err
		}
		if !exists {
			logger.Info("Audit is enabled, but policy file does not exist. Configuration is required.")
			return false, nil
		}

		remoteAuditContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteAuditPolicyFile)
		if err != nil {
			return false, err
		}
		expectedAuditContent, err := templates.Get("kubernetes/audit-policy.yaml.tmpl")
		if err != nil {
			return false, err
		}

		if string(remoteAuditContent) != expectedAuditContent {
			logger.Warn("Remote audit policy file content mismatch. Re-configuration is required.")
			return false, nil
		}
	}

	logger.Info("All kube-apiserver configuration files are up to date. Step is done.")
	return true, nil
}

func (s *InstallKubeAPIServerServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.Audit != nil && s.Audit.Enabled != nil && *s.Audit.Enabled {
		policyContent, err := templates.Get("kubernetes/audit-policy.yaml.tmpl")
		if err != nil {
			return fmt.Errorf("failed to get default audit policy template: %w", err)
		}

		logger.Infof("Audit is enabled, writing default audit policy to %s", s.RemoteAuditPolicyFile)
		if err := runner.Mkdirp(ctx.GoContext(), conn, filepath.Dir(s.RemoteAuditPolicyFile), "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create directory for audit policy file: %w", err)
		}
		if err := runner.WriteFile(ctx.GoContext(), conn, []byte(policyContent), s.RemoteAuditPolicyFile, "0644", s.Sudo); err != nil {
			return fmt.Errorf("failed to write audit policy file: %w", err)
		}
	}

	svcContent, err := s.render(ctx)
	if err != nil {
		return err
	}
	logger.Info("Writing kube-apiserver.service file to remote host...")
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(svcContent), s.RemoteServiceFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write service file to %s: %w", s.RemoteServiceFile, err)
	}

	logger.Info("Reloading systemd daemon to apply changes...")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts, falling back to raw command for daemon-reload: %v", err)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
			return fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			return fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
		}
	}

	logger.Info("kube-apiserver has been configured successfully.")
	return nil
}

func (s *InstallKubeAPIServerServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.RemoteServiceFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteServiceFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove service file during rollback: %v", err)
	}

	if s.Audit != nil && s.Audit.Enabled != nil && *s.Audit.Enabled {
		logger.Warnf("Rolling back by removing audit policy file: %s", s.RemoteAuditPolicyFile)
		if err := runner.Remove(ctx.GoContext(), conn, s.RemoteAuditPolicyFile, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove audit policy file during rollback: %v", err)
		}
	}

	return nil
}

var _ step.Step = (*InstallKubeAPIServerServiceStep)(nil)
