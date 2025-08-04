package kube_apiserver

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type ConfigureKubeAPIServerStep struct {
	step.Base
	AdvertiseAddress       string
	BindAddress            string
	ServiceClusterIPRange  string
	ServiceNodePortRange   string
	AuthorizationMode      string
	AuthorizationModeSlice []string
	AllowPrivileged        bool
	PKIDir                 string
	AdmissionPlugins       string
	AdmissionPluginsSlice  []string
	ServiceAccountIssuer   string
	EtcdPKIDir             string
	EtcdClientCertFile     string
	EtcdClientKeyFile      string
	EtcdServers            string
	EtcdServersSlice       []string
	EtcdPrefix             string
	TlsCipherSuites        string
	TlsCipherSuitesSlice   []string
	Audit                  *v1alpha1.AuditConfig
	FeatureGates           map[string]bool
	RemoteConfigFile       string
	RemoteAuditPolicyFile  string
}

type ConfigureKubeAPIServerStepBuilder struct {
	step.Builder[ConfigureKubeAPIServerStepBuilder, *ConfigureKubeAPIServerStep]
}

func NewConfigureKubeAPIServerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureKubeAPIServerStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes
	apiCfg := k8sSpec.APIServer

	s := &ConfigureKubeAPIServerStep{
		BindAddress:           "0.0.0.0",
		ServiceClusterIPRange: common.DefaultKubeServiceCIDR,
		ServiceNodePortRange:  common.DefaultServiceNodePortRange,
		AuthorizationMode:     "Node,RBAC",
		AllowPrivileged:       true,
		PKIDir:                common.KubernetesPKIDir,
		AdmissionPlugins:      "NodeRestriction",
		ServiceAccountIssuer:  fmt.Sprintf("https://kubernetes.default.svc.%s", k8sSpec.DNSDomain),
		EtcdPKIDir:            common.DefaultEtcdPKIDir,
		EtcdPrefix:            "/registry",
		RemoteConfigFile:      filepath.Join(common.KubernetesConfigDir, "kube-apiserver.yaml"),
	}

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	var etcdEndpoints []string
	for _, etcdNode := range etcdNodes {
		etcdEndpoints = append(etcdEndpoints, fmt.Sprintf("https://%s:%d", etcdNode.GetAddress(), common.DefaultEtcdClientPort))
	}
	s.EtcdServers = strings.Join(etcdEndpoints, ",")
	s.EtcdServersSlice = etcdEndpoints
	s.AuthorizationModeSlice = strings.Split(s.AuthorizationMode, ",")
	s.AdmissionPluginsSlice = strings.Split(s.AdmissionPlugins, ",")

	if apiCfg.TlsCipherSuites != nil {
		s.TlsCipherSuites = strings.Join(apiCfg.TlsCipherSuites, ",")
		s.TlsCipherSuitesSlice = apiCfg.TlsCipherSuites
	}
	if apiCfg.AuditConfig != nil {
		s.Audit = apiCfg.AuditConfig
		if apiCfg.AuditConfig.Enabled != nil && *apiCfg.AuditConfig.Enabled {
			s.RemoteAuditPolicyFile = apiCfg.AuditConfig.PolicyFile
			if s.RemoteAuditPolicyFile == "" {
				s.RemoteAuditPolicyFile = common.DefaultAuditPolicyFile
			}
			if s.Audit.LogPath == "" {
				s.Audit.LogPath = common.DefaultAuditLogFile
			}
			if s.Audit.PolicyFile == "" {
				s.Audit.PolicyFile = common.DefaultAuditPolicyFile
			}
			if s.Audit.MaxSize == nil {
				s.Audit.MaxSize = helpers.IntPtr(100)
			}
			if s.Audit.MaxBackups == nil {
				s.Audit.MaxBackups = helpers.IntPtr(10)
			}
			if s.Audit.MaxAge == nil {
				s.Audit.MaxAge = helpers.IntPtr(30)
			}
		}
	}
	if len(apiCfg.FeatureGates) > 0 {
		s.FeatureGates = apiCfg.FeatureGates
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kube-apiserver config files", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(ConfigureKubeAPIServerStepBuilder).Init(s)
	return b
}

func (s *ConfigureKubeAPIServerStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *ConfigureKubeAPIServerStep) renderConfig(ctx runtime.ExecutionContext) (string, error) {
	currentNode := ctx.GetHost()
	s.AdvertiseAddress = currentNode.GetInternalAddress()
	if s.AdvertiseAddress == "" {
		s.AdvertiseAddress = currentNode.GetAddress()
	}
	firstEtcdNodeName := ctx.GetHostsByRole(common.RoleEtcd)[0].GetName()
	s.EtcdClientCertFile = fmt.Sprintf(common.EtcdNodeCertFileNamePattern, firstEtcdNodeName)
	s.EtcdClientKeyFile = fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, firstEtcdNodeName)

	tmplContent, err := templates.Get("kubernetes/kube-apiserver.yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-apiserver config template: %w", err)
	}

	return templates.Render(tmplContent, s)
}

func (s *ConfigureKubeAPIServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedConfig, err := s.renderConfig(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to render expected config for precheck: %w", err)
	}

	remoteConfig, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteConfigFile)
	if err != nil {
		logger.Infof("Remote config file %s not found, configuration is required.", s.RemoteConfigFile)
		return false, nil
	}
	if string(remoteConfig) != expectedConfig {
		logger.Warn("Remote kube-apiserver config file content mismatch. Re-configuration is required.")
		return false, nil
	}

	if s.Audit != nil && s.Audit.Enabled != nil && *s.Audit.Enabled {
		expectedAudit, err := templates.Get("kubernetes/audit-policy.yaml.tmpl")
		if err != nil {
			return false, fmt.Errorf("failed to get audit policy template: %w", err)
		}
		remoteAudit, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteAuditPolicyFile)
		if err != nil {
			logger.Infof("Audit is enabled, but policy file %s not found. Configuration is required.", s.RemoteAuditPolicyFile)
			return false, nil
		}
		if string(remoteAudit) != expectedAudit {
			logger.Warn("Remote audit policy file content mismatch. Re-configuration is required.")
			return false, nil
		}
	}

	logger.Info("All kube-apiserver configuration files are up to date. Step is done.")
	return true, nil
}

func (s *ConfigureKubeAPIServerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.Audit != nil && s.Audit.Enabled != nil && *s.Audit.Enabled {
		policyContent, err := templates.Get("kubernetes/audit-policy.yaml.tmpl")
		if err != nil {
			return fmt.Errorf("failed to get default audit policy template: %w", err)
		}
		err = helpers.WriteContentToRemote(ctx, conn, policyContent, s.RemoteAuditPolicyFile, "0644", s.Sudo)
		if err != nil {
			return fmt.Errorf("failed to write audit policy file: %w", err)
		}
	}

	configContent, err := s.renderConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to render kube-apiserver config: %w", err)
	}
	err = helpers.WriteContentToRemote(ctx, conn, configContent, s.RemoteConfigFile, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write kube-apiserver config file: %w", err)
	}

	logger.Info("kube-apiserver configuration files have been created successfully.")
	return nil
}

func (s *ConfigureKubeAPIServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.RemoteConfigFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteConfigFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove config file during rollback: %v", err)
	}
	if s.Audit != nil && s.Audit.Enabled != nil && *s.Audit.Enabled {
		logger.Warnf("Rolling back by removing audit policy file: %s", s.RemoteAuditPolicyFile)
		if err := runner.Remove(ctx.GoContext(), conn, s.RemoteAuditPolicyFile, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove audit policy file during rollback: %v", err)
		}
	}
	return nil
}

var _ step.Step = (*ConfigureKubeAPIServerStep)(nil)
