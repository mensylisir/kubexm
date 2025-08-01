package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"net"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Kubernetes struct {
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
	Version     string `json:"version" yaml:"version"`
	ClusterName string `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	DNSDomain   string `json:"dnsDomain,omitempty" yaml:"dnsDomain,omitempty"`

	ContainerRuntime *ContainerRuntime `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`

	APIServer         *APIServerConfig         `json:"apiServer,omitempty" yaml:"apiServer,omitempty"`
	ControllerManager *ControllerManagerConfig `json:"controllerManager,omitempty" yaml:"controllerManager,omitempty"`
	Scheduler         *SchedulerConfig         `json:"scheduler,omitempty" yaml:"scheduler,omitempty"`
	Kubelet           *KubeletConfig           `json:"kubelet,omitempty" yaml:"kubelet,omitempty"`
	KubeProxy         *KubeProxyConfig         `json:"kubeProxy,omitempty" yaml:"kubeProxy,omitempty"`

	Addons *KubernetesAddons `json:"addons,omitempty" yaml:"addons,omitempty"`
}

type APIServerConfig struct {
	CertExtraSans        []string          `json:"certExtraSans,omitempty" yaml:"certExtraSans,omitempty"`
	ServiceNodePortRange string            `json:"serviceNodePortRange,omitempty" yaml:"serviceNodePortRange,omitempty"`
	AuditConfig          *AuditConfig      `json:"audit,omitempty" yaml:"audit,omitempty"`
	AutoRenewCerts       *bool             `json:"autoRenewCerts,omitempty" yaml:"autoRenewCerts,omitempty"`
	FeatureGates         map[string]bool   `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`
	ExtraArgs            map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`

	AuthorizationModes []string `json:"authorizationModes,omitempty" yaml:"authorizationModes,omitempty"`
	AdmissionPlugins   []string `json:"admissionPlugins,omitempty" yaml:"admissionPlugins,omitempty"`
	BindAddress        string   `json:"bindAddress,omitempty" yaml:"bindAddress,omitempty"`
	AllowPrivileged    *bool    `json:"allowPrivileged,omitempty" yaml:"allowPrivileged,omitempty"`
	EtcdPrefix         string   `json:"etcdPrefix,omitempty" yaml:"etcdPrefix,omitempty"`
	TlsCipherSuites    []string `json:"tlsCipherSuites,omitempty" yaml:"tlsCipherSuites,omitempty"`
}

type ControllerManagerConfig struct {
	NodeCidrMaskSize       *int              `json:"nodeCidrMaskSize,omitempty" yaml:"nodeCidrMaskSize,omitempty"`
	NodeCidrMaskSizeIPv6   *int              `json:"nodeCidrMaskSizeIPv6,omitempty" yaml:"nodeCidrMaskSizeIPv6,omitempty"`
	PodEvictionTimeout     string            `json:"podEvictionTimeout,omitempty" yaml:"podEvictionTimeout,omitempty"`
	NodeMonitorGracePeriod string            `json:"nodeMonitorGracePeriod,omitempty" yaml:"nodeMonitorGracePeriod,omitempty"`
	FeatureGates           map[string]bool   `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`
	ExtraArgs              map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
}

type SchedulerConfig struct {
	FeatureGates map[string]bool   `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`
	ExtraArgs    map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
}

type KubeletConfig struct {
	MaxPods       *int                  `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`
	CgroupDriver  string                `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`
	EvictionHard  map[string]string     `json:"evictionHard,omitempty" yaml:"evictionHard,omitempty"`
	FeatureGates  map[string]bool       `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`
	ExtraArgs     map[string]string     `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	Configuration *runtime.RawExtension `json:"configuration,omitempty" yaml:"configuration,omitempty"`

	CpuManagerPolicy                 string            `json:"cpuManagerPolicy,omitempty" yaml:"cpuManagerPolicy,omitempty"`
	KubeReserved                     map[string]string `json:"kubeReserved,omitempty" yaml:"kubeReserved,omitempty"`
	SystemReserved                   map[string]string `json:"systemReserved,omitempty" yaml:"systemReserved,omitempty"`
	EvictionSoft                     map[string]string `json:"evictionSoft,omitempty" yaml:"evictionSoft,omitempty"`
	EvictionSoftGracePeriod          map[string]string `json:"evictionSoftGracePeriod,omitempty" yaml:"evictionSoftGracePeriod,omitempty"`
	EvictionMaxPodGracePeriod        *int              `json:"evictionMaxPodGracePeriod,omitempty" yaml:"evictionMaxPodGracePeriod,omitempty"`
	EvictionPressureTransitionPeriod string            `json:"evictionPressureTransitionPeriod,omitempty" yaml:"evictionPressureTransitionPeriod,omitempty"`
	PodPidsLimit                     *int              `json:"podPidsLimit,omitempty" yaml:"podPidsLimit,omitempty"`
	HairpinMode                      string            `json:"hairpinMode,omitempty" yaml:"hairpinMode,omitempty"`
	ContainerLogMaxSize              string            `json:"containerLogMaxSize,omitempty" yaml:"containerLogMaxSize,omitempty"`
	ContainerLogMaxFiles             *int              `json:"containerLogMaxFiles,omitempty" yaml:"containerLogMaxFiles,omitempty"`
}

type KubeProxyConfig struct {
	Enable        *bool                 `json:"enable,omitempty" yaml:"enable,omitempty"`
	Mode          string                `json:"mode,omitempty" yaml:"mode,omitempty"`
	MasqueradeAll *bool                 `json:"masqueradeAll,omitempty" yaml:"masqueradeAll,omitempty"`
	FeatureGates  map[string]bool       `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`
	ExtraArgs     map[string]string     `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	Configuration *runtime.RawExtension `json:"configuration,omitempty" yaml:"configuration,omitempty"`
}

type KubernetesAddons struct {
	Nodelocaldns         *NodelocaldnsConfig         `json:"nodelocaldns,omitempty" yaml:"nodelocaldns,omitempty"`
	NodeFeatureDiscovery *NodeFeatureDiscoveryConfig `json:"nodeFeatureDiscovery,omitempty" yaml:"nodeFeatureDiscovery,omitempty"`
	Kata                 *KataConfig                 `json:"kata,omitempty" yaml:"kata,omitempty"`
	NvidiaRuntime        *NvidiaRuntimeConfig        `json:"nvidiaRuntime,omitempty" yaml:"nvidiaRuntime,omitempty"`
}

type AuditConfig struct {
	Enabled           *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	PolicyFileContent string `json:"policyFileContent,omitempty" yaml:"policyFileContent,omitempty"`

	LogPath    string `json:"logPath,omitempty" yaml:"logPath,omitempty"`
	PolicyFile string `json:"policyFile,omitempty" yaml:"policyFile,omitempty"`
	MaxAge     *int   `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
	MaxBackups *int   `json:"maxBackups,omitempty" yaml:"maxBackups,omitempty"`
	MaxSize    *int   `json:"maxSize,omitempty" yaml:"maxSize,omitempty"`
}

type NodelocaldnsConfig struct {
	Enabled *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	IP      string `json:"ip,omitempty" yaml:"ip,omitempty"`
}

type NodeFeatureDiscoveryConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type KataConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type NvidiaRuntimeConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

func SetDefaults_Kubernetes(cfg *Kubernetes) {
	if cfg == nil {
		return
	}

	if cfg.DNSDomain == "" {
		cfg.DNSDomain = common.DefaultClusterLocal
	}

	if cfg.ContainerRuntime == nil {
		cfg.ContainerRuntime = &ContainerRuntime{}
	}
	SetDefaults_ContainerRuntimeConfig(cfg.ContainerRuntime)

	if cfg.APIServer == nil {
		cfg.APIServer = &APIServerConfig{}
	}
	SetDefaults_APIServerConfig(cfg.APIServer)

	if cfg.ControllerManager == nil {
		cfg.ControllerManager = &ControllerManagerConfig{}
	}
	SetDefaults_ControllerManagerConfig(cfg.ControllerManager)

	if cfg.Kubelet == nil {
		cfg.Kubelet = &KubeletConfig{}
	}
	SetDefaults_KubeletConfig(cfg.Kubelet)

	if cfg.KubeProxy == nil {
		cfg.KubeProxy = &KubeProxyConfig{}
	}
	SetDefaults_KubeProxyConfig(cfg.KubeProxy)

	if cfg.Addons == nil {
		cfg.Addons = &KubernetesAddons{}
	}
	if cfg.Addons.Nodelocaldns == nil {
		cfg.Addons.Nodelocaldns = &NodelocaldnsConfig{}
	}
	SetDefaults_NodelocaldnsConfig(cfg.Addons.Nodelocaldns)

	if cfg.Addons.NodeFeatureDiscovery == nil {
		cfg.Addons.NodeFeatureDiscovery = &NodeFeatureDiscoveryConfig{}
	}
	if cfg.Addons.NodeFeatureDiscovery.Enabled == nil {
		cfg.Addons.NodeFeatureDiscovery.Enabled = helpers.BoolPtr(false)
	}

	if cfg.Addons.Kata == nil {
		cfg.Addons.Kata = &KataConfig{}
	}
	if cfg.Addons.Kata.Enabled == nil {
		cfg.Addons.Kata.Enabled = helpers.BoolPtr(false)
	}

	if cfg.Addons.NvidiaRuntime == nil {
		cfg.Addons.NvidiaRuntime = &NvidiaRuntimeConfig{}
	}
	if cfg.Addons.NvidiaRuntime.Enabled == nil {
		cfg.Addons.NvidiaRuntime.Enabled = helpers.BoolPtr(false)
	}
}

func SetDefaults_APIServerConfig(cfg *APIServerConfig) {
	if cfg.AutoRenewCerts == nil {
		cfg.AutoRenewCerts = helpers.BoolPtr(common.DefaultAutoRenewCerts)
	}
	if cfg.ServiceNodePortRange == "" {
		cfg.ServiceNodePortRange = common.DefaultServiceNodePortRange
	}
	if len(cfg.AuthorizationModes) == 0 {
		cfg.AuthorizationModes = []string{"Node", "RBAC"}
	}
	if len(cfg.AdmissionPlugins) == 0 {
		cfg.AdmissionPlugins = []string{"NodeRestriction"}
	}
	if cfg.BindAddress == "" {
		cfg.BindAddress = "0.0.0.0"
	}
	if cfg.AllowPrivileged == nil {
		cfg.AllowPrivileged = helpers.BoolPtr(true)
	}
	if cfg.EtcdPrefix == "" {
		cfg.EtcdPrefix = "/registry"
	}
	if cfg.AuditConfig == nil {
		cfg.AuditConfig = &AuditConfig{}
	}
	if cfg.AuditConfig.Enabled == nil || cfg.AuditConfig.Enabled == helpers.BoolPtr(false) {
		cfg.AuditConfig.Enabled = helpers.BoolPtr(common.DefaultAuditEnable)
	} else {
		SetDefaults_AuditConfig(cfg.AuditConfig)
	}

}

func SetDefaults_AuditConfig(cfg *AuditConfig) {
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(true)
	}
	if *cfg.Enabled {
		if cfg.LogPath == "" {
			cfg.LogPath = "/var/log/kubernetes/audit.log"
		}
		if cfg.PolicyFile == "" {
			cfg.PolicyFile = "/etc/kubernetes/audit-policy.yaml"
		}
		if cfg.MaxAge == nil {
			cfg.MaxAge = helpers.IntPtr(30)
		}
		if cfg.MaxBackups == nil {
			cfg.MaxBackups = helpers.IntPtr(10)
		}
		if cfg.MaxSize == nil {
			cfg.MaxSize = helpers.IntPtr(100)
		}
	}
}

func SetDefaults_ControllerManagerConfig(cfg *ControllerManagerConfig) {
	if cfg.NodeCidrMaskSize == nil {
		cfg.NodeCidrMaskSize = helpers.IntPtr(24)
	}
}

func SetDefaults_KubeletConfig(cfg *KubeletConfig) {
	if cfg.CgroupDriver == "" {
		cfg.CgroupDriver = common.CgroupDriverSystemd
	}
	if cfg.MaxPods == nil {
		cfg.MaxPods = helpers.IntPtr(common.DefaultMaxPods)
	}

	if cfg.CpuManagerPolicy == "" {
		cfg.CpuManagerPolicy = "none"
	}
	if len(cfg.KubeReserved) == 0 {
		cfg.KubeReserved = map[string]string{"cpu": "200m", "memory": "250Mi"}
	}
	if len(cfg.SystemReserved) == 0 {
		cfg.SystemReserved = map[string]string{"cpu": "200m", "memory": "250Mi"}
	}
	if len(cfg.EvictionHard) == 0 {
		cfg.EvictionHard = map[string]string{
			"memory.available": "5%",
			"pid.available":    "10%",
		}
	}
	if cfg.EvictionMaxPodGracePeriod == nil {
		cfg.EvictionMaxPodGracePeriod = helpers.IntPtr(120)
	}
	if cfg.EvictionPressureTransitionPeriod == "" {
		cfg.EvictionPressureTransitionPeriod = "30s"
	}
	if cfg.PodPidsLimit == nil {
		cfg.PodPidsLimit = helpers.IntPtr(10000)
	}
	if cfg.HairpinMode == "" {
		cfg.HairpinMode = common.DefaultKubeletHairpinMode
	}
	if cfg.ContainerLogMaxSize == "" {
		cfg.ContainerLogMaxSize = "5Mi"
	}
	if cfg.ContainerLogMaxFiles == nil {
		cfg.ContainerLogMaxFiles = helpers.IntPtr(3)
	}
	if cfg.FeatureGates == nil {
		cfg.FeatureGates = make(map[string]bool)
	}
	if _, ok := cfg.FeatureGates["RotateKubeletServerCertificate"]; !ok {
		cfg.FeatureGates["RotateKubeletServerCertificate"] = true
	}
}

func SetDefaults_KubeProxyConfig(cfg *KubeProxyConfig) {
	if cfg.Enable == nil {
		cfg.Enable = helpers.BoolPtr(common.KubeProxyEnable)
	}
	if cfg.Mode == "" {
		cfg.Mode = common.DefaultKubeProxyMode
	}
	if cfg.MasqueradeAll == nil {
		cfg.MasqueradeAll = helpers.BoolPtr(common.DefaultMasqueradeAll)
	}
}

func SetDefaults_NodelocaldnsConfig(cfg *NodelocaldnsConfig) {
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(false)
	}
	if *cfg.Enabled && cfg.IP == "" {
		cfg.IP = common.DefaultLocalDNS
	}
}

func Validate_Kubernetes(cfg *Kubernetes, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add(pathPrefix + ": kubernetes configuration section cannot be nil")
		return
	}
	p := path.Join(pathPrefix)

	// 顶层字段校验
	if cfg.Version == "" {
		verrs.Add(p + ".version: is a required field")
	} else if !helpers.IsValidSemanticVersion(cfg.Version) {
		verrs.Add(fmt.Sprintf("%s.version: invalid semantic version format for '%s'", p, cfg.Version))
	}
	if cfg.ClusterName == "" {
		verrs.Add(p + ".clusterName: is a required field")
	}
	if cfg.DNSDomain != "" && !helpers.IsValidDomainName(cfg.DNSDomain) {
		verrs.Add(fmt.Sprintf("%s.domain: invalid domain format for '%s'", p, cfg.DNSDomain))
	}

	if cfg.ContainerRuntime != nil {
		Validate_ContainerRuntimeConfig(cfg.ContainerRuntime, verrs, path.Join(p, "containerRuntime"))
	}
	if cfg.APIServer != nil {
		Validate_APIServerConfig(cfg.APIServer, verrs, path.Join(p, "apiServer"))
	}
	if cfg.ControllerManager != nil {
		Validate_ControllerManagerConfig(cfg.ControllerManager, verrs, path.Join(p, "controllerManager"))
	}
	if cfg.Kubelet != nil {
		Validate_KubeletConfig(cfg.Kubelet, verrs, path.Join(p, "kubelet"))
	}
	if cfg.KubeProxy != nil {
		Validate_KubeProxyConfig(cfg.KubeProxy, verrs, path.Join(p, "kubeProxy"))
	}

	if cfg.Addons != nil {
		addonsPath := path.Join(p, "addons")
		if cfg.Addons.Nodelocaldns != nil {
			Validate_NodelocaldnsConfig(cfg.Addons.Nodelocaldns, verrs, path.Join(addonsPath, "nodelocaldns"))
		}
		if cfg.Addons.NodeFeatureDiscovery != nil {
		}

		if cfg.Addons.Kata != nil {
		}

		if cfg.Addons.NvidiaRuntime != nil {
		}
	}
}

func Validate_APIServerConfig(cfg *APIServerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg.ServiceNodePortRange != "" {
		parts := strings.Split(cfg.ServiceNodePortRange, "-")
		if len(parts) != 2 {
			verrs.Add(fmt.Sprintf("%s.serviceNodePortRange: invalid format '%s', must be 'start-end'",
				pathPrefix, cfg.ServiceNodePortRange))
		} else {
			startPort, err1 := strconv.Atoi(parts[0])
			endPort, err2 := strconv.Atoi(parts[1])

			if err1 != nil || err2 != nil {
				verrs.Add(fmt.Sprintf("%s.serviceNodePortRange: invalid port number in range '%s'",
					pathPrefix, cfg.ServiceNodePortRange))
			} else {
				if startPort < 1 || startPort > 65535 {
					verrs.Add(fmt.Sprintf("%s.serviceNodePortRange: start port %d is out of valid range (1-65535)",
						pathPrefix, startPort))
				}
				if endPort < 1 || endPort > 65535 {
					verrs.Add(fmt.Sprintf("%s.serviceNodePortRange: end port %d is out of valid range (1-65535)",
						pathPrefix, endPort))
				}

				if startPort >= 1 && startPort <= 65535 && endPort >= 1 && endPort <= 65535 {
					if startPort >= endPort {
						verrs.Add(fmt.Sprintf("%s.serviceNodePortRange: start port %d must be less than end port %d",
							pathPrefix, startPort, endPort))
					}
				}
			}
		}
	}
	if cfg.AuditConfig != nil {
		Validate_AuditConfig(cfg.AuditConfig, verrs, path.Join(pathPrefix, "audit"))
	}
}

func Validate_AuditConfig(cfg *AuditConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.LogPath == "" {
			verrs.Add(pathPrefix + ".logPath: cannot be empty when audit is enabled")
		} else if !path.IsAbs(cfg.LogPath) {
			verrs.Add(fmt.Sprintf("%s.logPath: must be an absolute path, got '%s'", pathPrefix, cfg.LogPath))
		}

		if cfg.PolicyFile == "" {
			verrs.Add(pathPrefix + ".policyFile: cannot be empty when audit is enabled")
		} else if !path.IsAbs(cfg.PolicyFile) {
			verrs.Add(fmt.Sprintf("%s.policyFile: must be an absolute path, got '%s'", pathPrefix, cfg.PolicyFile))
		}

		if cfg.PolicyFileContent != "" && cfg.PolicyFile != "" {
			if cfg.PolicyFileContent == "" {
				verrs.Add(pathPrefix + ".policyFileContent: cannot be empty when audit is enabled and policy file is managed by kubexm")
			}
		} else if cfg.PolicyFileContent == "" {
			verrs.Add(pathPrefix + ".policyFileContent: is required when audit is enabled")
		}

		if cfg.MaxAge != nil && *cfg.MaxAge < 0 {
			verrs.Add(fmt.Sprintf("%s.maxAge: cannot be negative, got %d", pathPrefix, *cfg.MaxAge))
		}
		if cfg.MaxBackups != nil && *cfg.MaxBackups < 0 {
			verrs.Add(fmt.Sprintf("%s.maxBackups: cannot be negative, got %d", pathPrefix, *cfg.MaxBackups))
		}
		if cfg.MaxSize != nil && *cfg.MaxSize < 0 {
			verrs.Add(fmt.Sprintf("%s.maxSize: cannot be negative, got %d", pathPrefix, *cfg.MaxSize))
		}
	}
}
func Validate_ControllerManagerConfig(cfg *ControllerManagerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg.NodeCidrMaskSize != nil && (*cfg.NodeCidrMaskSize < 16 || *cfg.NodeCidrMaskSize > 28) {
		verrs.Add(fmt.Sprintf("%s.nodeCidrMaskSize: must be between 16 and 28, got %d", pathPrefix, *cfg.NodeCidrMaskSize))
	}
	if cfg.NodeCidrMaskSizeIPv6 != nil && (*cfg.NodeCidrMaskSizeIPv6 < 64 || *cfg.NodeCidrMaskSizeIPv6 > 124) {
		verrs.Add(fmt.Sprintf("%s.nodeCidrMaskSizeIPv6: must be between 64 and 124, got %d", pathPrefix, *cfg.NodeCidrMaskSizeIPv6))
	}
}

func Validate_KubeletConfig(cfg *KubeletConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	p := path.Join(pathPrefix)

	if !helpers.ContainsStringWithEmpty(common.ValidCgroupDrivers, cfg.CgroupDriver) {
		verrs.Add(fmt.Sprintf("%s.cgroupDriver: invalid driver '%s', must be one of [%s] or empty",
			p, cfg.CgroupDriver, strings.Join(common.ValidCgroupDrivers, ", ")))
	}

	if cfg.CpuManagerPolicy != "" {
		validCPUPolicies := []string{"none", "static"}
		if !helpers.ContainsString(validCPUPolicies, cfg.CpuManagerPolicy) {
			verrs.Add(fmt.Sprintf("%s.cpuManagerPolicy: invalid policy '%s', must be one of [%s]",
				p, cfg.CpuManagerPolicy, strings.Join(validCPUPolicies, ", ")))
		}
	}

	if cfg.HairpinMode != "" {
		validHairpinModes := []string{"promiscuous-bridge", "hairpin-veth", "none"}
		if !helpers.ContainsString(validHairpinModes, cfg.HairpinMode) {
			verrs.Add(fmt.Sprintf("%s.hairpinMode: invalid mode '%s', must be one of [%s]",
				p, cfg.HairpinMode, strings.Join(validHairpinModes, ", ")))
		}
	}

	validateResourceMap := func(m map[string]string, fieldName string) {
		for k, v := range m {
			if strings.TrimSpace(v) == "" {
				verrs.Add(fmt.Sprintf("%s.%s['%s']: value cannot be empty", p, fieldName, k))
			}
		}
	}
	if len(cfg.KubeReserved) > 0 {
		validateResourceMap(cfg.KubeReserved, "kubeReserved")
	}
	if len(cfg.SystemReserved) > 0 {
		validateResourceMap(cfg.SystemReserved, "systemReserved")
	}

	if cfg.PodPidsLimit != nil && *cfg.PodPidsLimit < -1 {
		verrs.Add(fmt.Sprintf("%s.podPidsLimit: must be -1 (unlimited) or a positive integer, got %d", p, *cfg.PodPidsLimit))
	}

	if cfg.ContainerLogMaxFiles != nil && *cfg.ContainerLogMaxFiles < 1 {
		verrs.Add(fmt.Sprintf("%s.containerLogMaxFiles: must be at least 1, got %d", p, *cfg.ContainerLogMaxFiles))
	}

	percentRegex := regexp.MustCompile(`^([0-9]{1,2}|100)%$`)
	validateEvictionMap := func(m map[string]string, fieldName string) {
		for k, v := range m {
			if strings.HasSuffix(v, "%") {
				if !percentRegex.MatchString(v) {
					verrs.Add(fmt.Sprintf("%s.%s['%s']: invalid percentage format for '%s'", p, fieldName, k, v))
				}
			}
		}
	}
	if len(cfg.EvictionHard) > 0 {
		validateEvictionMap(cfg.EvictionHard, "evictionHard")
	}
	if len(cfg.EvictionSoft) > 0 {
		validateEvictionMap(cfg.EvictionSoft, "evictionSoft")
	}
}

func Validate_KubeProxyConfig(cfg *KubeProxyConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if *cfg.Enable {
		if !helpers.ContainsString(common.ValidKubeProxyModes, cfg.Mode) {
			verrs.Add(fmt.Sprintf("%s.mode: invalid mode '%s', must be one of [%s]",
				pathPrefix, cfg.Mode, strings.Join(common.ValidKubeProxyModes, ", ")))
		}
	} else {
		if cfg.Mode != "" {
			verrs.Add(fmt.Sprintf("%s.mode: should be ' ' or empty when kube-proxy is disabled", pathPrefix))
		}
	}
}

func Validate_NodelocaldnsConfig(cfg *NodelocaldnsConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if *cfg.Enabled {
		if cfg.IP == "" {
			verrs.Add(pathPrefix + ".ip: cannot be empty when nodelocaldns is enabled")
		} else if net.ParseIP(cfg.IP) == nil {
			verrs.Add(fmt.Sprintf("%s.ip: invalid IP address format for '%s'", pathPrefix, cfg.IP))
		}
	}
}
