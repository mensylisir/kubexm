package kubeadm

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

// GenerateFirstMasterConfigStep is a step to generate the kubeadm config for the first master.
type GenerateFirstMasterConfigStep struct {
	step.Base
}

// GenerateFirstMasterConfigStepBuilder is a builder for GenerateFirstMasterConfigStep.
type GenerateFirstMasterConfigStepBuilder struct {
	step.Builder[GenerateFirstMasterConfigStepBuilder, *GenerateFirstMasterConfigStep]
}

// NewGenerateFirstMasterConfigStepBuilder creates a new GenerateFirstMasterConfigStepBuilder.
func NewGenerateFirstMasterConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateFirstMasterConfigStepBuilder {
	s := &GenerateFirstMasterConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeadm init configuration for the first master", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateFirstMasterConfigStepBuilder).Init(s)
	return b
}

// Meta returns the step's metadata.
func (s *GenerateFirstMasterConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// TemplateData holds the data for the kubeadm config template.
type TemplateData struct {
	ClusterConfiguration   ClusterConfigurationTemplate
	InitConfiguration      InitConfigurationTemplate
	KubeProxyConfiguration KubeProxyConfigurationTemplate
	KubeletConfiguration   KubeletConfigurationTemplate
	ImageRepository        string
	ClusterName            string
	KubernetesVersion      string
}

// ClusterConfigurationTemplate holds the data for the ClusterConfiguration section.
type ClusterConfigurationTemplate struct {
	Etcd                 EtcdTemplate
	DNS                  DNSTemplate
	Networking           NetworkingTemplate
	ApiServer            ApiServerTemplate
	ControllerManager    ControllerManagerTemplate
	Scheduler            SchedulerTemplate
	CertificatesDir      string
	ControlPlaneEndpoint string
}

// EtcdTemplate holds the data for the etcd section.
type EtcdTemplate struct {
	IsExternal bool
	Endpoints  []string
	CaFile     string
	CertFile   string
	KeyFile    string
}

// DNSTemplate holds the data for the DNS section.
type DNSTemplate struct {
	ImageTag string
}

// NetworkingTemplate holds the data for the networking section.
type NetworkingTemplate struct {
	PodSubnet     string
	ServiceSubnet string
	DNSDomain     string
}

// ApiServerTemplate holds the data for the apiServer section.
type ApiServerTemplate struct {
	ExtraArgs map[string]string
	CertSANs  []string
}

// ControllerManagerTemplate holds the data for the controllerManager section.
type ControllerManagerTemplate struct {
	ExtraArgs map[string]string
}

// SchedulerTemplate holds the data for the scheduler section.
type SchedulerTemplate struct {
	ExtraArgs map[string]string
}

// InitConfigurationTemplate holds the data for the InitConfiguration section.
type InitConfigurationTemplate struct {
	LocalAPIEndpoint LocalAPIEndpointTemplate
	NodeRegistration NodeRegistrationTemplate
}

// LocalAPIEndpointTemplate holds the data for the localAPIEndpoint section.
type LocalAPIEndpointTemplate struct {
	AdvertiseAddress string
	BindPort         int
}

// NodeRegistrationTemplate holds the data for the nodeRegistration section.
type NodeRegistrationTemplate struct {
	CRISocket        string
	CgroupDriver     string
	KubeletExtraArgs map[string]string
}

// KubeProxyConfigurationTemplate holds the data for the KubeProxyConfiguration section.
type KubeProxyConfigurationTemplate struct {
	Mode     string
	Iptables IptablesTemplate
}

// IptablesTemplate holds the data for the iptables section.
type IptablesTemplate struct {
	MasqueradeAll bool
	MasqueradeBit int
	MinSyncPeriod string
	SyncPeriod    string
}

// KubeletConfigurationTemplate holds the data for the KubeletConfiguration section.
type KubeletConfigurationTemplate struct {
	ClusterDNS                        string
	ContainerLogMaxSize               string
	EvictionPressureTransitionPeriod  string
	ContainerLogMaxFiles              int
	EvictionMaxPodGracePeriod         int
	MaxPods                           int
	PodPidsLimit                      int64
	RotateCertificates                bool
	SerializeImagePulls               bool
	EvictionHard                      map[string]string
	EvictionSoft                      map[string]string
	EvictionSoftGracePeriod           map[string]string
	KubeReserved                      map[string]string
	SystemReserved                    map[string]string
	FeatureGates                      map[string]bool
}

func (s *GenerateFirstMasterConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()
	currentHost := ctx.GetHost()

	imageProvider := images.NewImageProvider(ctx)
	corednsImage, err := imageProvider.GetImage("coredns")
	if err != nil {
		return nil, fmt.Errorf("failed to get coredns image: %w", err)
	}

	// --- 1. Set default values ---
	data := TemplateData{
		ImageRepository:   corednsImage.Registry,
		KubernetesVersion: common.DefaultK8sVersion,
		ClusterName:       common.DefaultClusterName,
		ClusterConfiguration: ClusterConfigurationTemplate{
			CertificatesDir: common.DefaultKubernetesPKIDir,
			Networking: NetworkingTemplate{
				PodSubnet:     common.DefaultKubePodsCIDR,
				ServiceSubnet: common.DefaultKubeServiceCIDR,
				DNSDomain:     common.DefaultClusterDNSDomain,
			},
			ApiServer: ApiServerTemplate{
				ExtraArgs: map[string]string{
					"audit-log-maxage":          "30",
					"audit-log-maxbackup":       "10",
					"audit-log-maxsize":         "100",
					"audit-log-path":            "/var/log/kubernetes/kube-apiserver.log",
					"bind-address":              "0.0.0.0",
					"feature-gates":             "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
				},
			},
			ControllerManager: ControllerManagerTemplate{
				ExtraArgs: map[string]string{
					"node-cidr-mask-size":      "24",
					"bind-address":             "0.0.0.0",
					"cluster-signing-duration": "87600h",
					"feature-gates":            "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
				},
			},
			Scheduler: SchedulerTemplate{
				ExtraArgs: map[string]string{
					"bind-address":  "0.0.0.0",
					"feature-gates": "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
				},
			},
		},
		InitConfiguration: InitConfigurationTemplate{
			LocalAPIEndpoint: LocalAPIEndpointTemplate{
				BindPort: common.DefaultAPIServerPort,
			},
		},
		KubeProxyConfiguration: KubeProxyConfigurationTemplate{
			Mode: common.KubeProxyModeIPVS,
			Iptables: IptablesTemplate{
				MasqueradeAll: false,
				MasqueradeBit: 14,
				SyncPeriod:    "30s",
				MinSyncPeriod: "0s",
			},
		},
		KubeletConfiguration: KubeletConfigurationTemplate{
			ContainerLogMaxFiles:             5,
			ContainerLogMaxSize:              "10Mi",
			EvictionPressureTransitionPeriod: "30s",
			EvictionMaxPodGracePeriod:        120,
			MaxPods:                          110,
			PodPidsLimit:                     10000,
			RotateCertificates:               true,
			SerializeImagePulls:              true,
			EvictionHard: map[string]string{
				"memory.available": "5%",
				"pid.available":    "10%",
			},
			EvictionSoft: map[string]string{
				"memory.available": "10%",
			},
			EvictionSoftGracePeriod: map[string]string{
				"memory.available": "2m",
			},
			FeatureGates: map[string]bool{
				"CSIStorageCapacity":             true,
				"ExpandCSIVolumes":               true,
				"RotateKubeletServerCertificate": true,
			},
			KubeReserved: map[string]string{
				"cpu":    "200m",
				"memory": "250Mi",
			},
			SystemReserved: map[string]string{},
		},
	}

	// --- 2. Override defaults with user-provided values from the cluster spec ---

	// Basic cluster info
	data.KubernetesVersion = helpers.FirstNonEmpty(cluster.Spec.Kubernetes.Version, data.KubernetesVersion)
	data.ClusterName = helpers.FirstNonEmpty(cluster.Spec.Kubernetes.ClusterName, data.ClusterName)
	data.ClusterConfiguration.Networking.DNSDomain = helpers.FirstNonEmpty(cluster.Spec.Kubernetes.DNSDomain, data.ClusterConfiguration.Networking.DNSDomain)

	// Networking
	data.ClusterConfiguration.Networking.PodSubnet = helpers.FirstNonEmpty(cluster.Spec.Network.KubePodsCIDR, data.ClusterConfiguration.Networking.PodSubnet)
	data.ClusterConfiguration.Networking.ServiceSubnet = helpers.FirstNonEmpty(cluster.Spec.Network.KubeServiceCIDR, data.ClusterConfiguration.Networking.ServiceSubnet)

	// Control plane endpoint
	cpEndpoint := cluster.Spec.ControlPlaneEndpoint
	cpDomain := helpers.FirstNonEmpty(cpEndpoint.Domain, cpEndpoint.Address)
	cpPort := helpers.FirstNonZeroInteger(cpEndpoint.Port, data.InitConfiguration.LocalAPIEndpoint.BindPort)
	data.ClusterConfiguration.ControlPlaneEndpoint = fmt.Sprintf("%s:%d", cpDomain, cpPort)
	data.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress = currentHost.GetInternalAddress() // This is the first master
	data.InitConfiguration.LocalAPIEndpoint.BindPort = cpPort

	// CRI
	var cgroupDriver string
	var criSocket string
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		criSocket = common.ContainerdDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Containerd.CgroupDriver
	case common.RuntimeTypeCRIO:
		criSocket = common.CRIODefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Crio.CgroupDriver
	case common.RuntimeTypeDocker:
		criSocket = common.CriDockerdSocketPath
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Docker.CgroupDriver
	case common.RuntimeTypeIsula:
		criSocket = common.IsuladDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Isulad.CgroupDriver
	default:
		// Fallback to a sensible default if type is not specified
		criSocket = common.ContainerdDefaultEndpoint
		cgroupDriver = common.CgroupDriverSystemd
	}
	data.InitConfiguration.NodeRegistration.CRISocket = criSocket
	data.InitConfiguration.NodeRegistration.CgroupDriver = cgroupDriver

	// Etcd configuration
	etcdSpec := cluster.Spec.Etcd
	data.ClusterConfiguration.Etcd.IsExternal = etcdSpec.Type == string(common.EtcdDeploymentTypeExternal)
	if data.ClusterConfiguration.Etcd.IsExternal {
		if etcdSpec.External != nil {
			data.ClusterConfiguration.Etcd.Endpoints = etcdSpec.External.Endpoints
			data.ClusterConfiguration.Etcd.CaFile = etcdSpec.External.CAFile
			data.ClusterConfiguration.Etcd.CertFile = etcdSpec.External.CertFile
			data.ClusterConfiguration.Etcd.KeyFile = etcdSpec.External.KeyFile
		}
	} else if etcdSpec.Type == string(common.EtcdDeploymentTypeKubexm) {
		etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
		endpoints := make([]string, len(etcdNodes))
		for i, node := range etcdNodes {
			endpoints[i] = fmt.Sprintf("https://%s:%d", node.GetInternalAddress(), common.EtcdDefaultClientPort)
		}
		data.ClusterConfiguration.Etcd.Endpoints = endpoints
		data.ClusterConfiguration.Etcd.CaFile = filepath.Join(common.DefaultEtcdPKIDir, common.EtcdCaPemFileName)
		data.ClusterConfiguration.Etcd.CertFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, currentHost.GetName()))
		data.ClusterConfiguration.Etcd.KeyFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, currentHost.GetName()))
	}
	// For stacked etcd (kubeadm default), the etcd fields are left empty.

	// DNS
	data.ClusterConfiguration.DNS.ImageTag = helpers.FirstNonEmpty(cluster.Spec.DNS.Image.Tag, corednsImage.Tag)

	// KubeProxy
	kProxySpec := cluster.Spec.Kubernetes.KubeProxy
	data.KubeProxyConfiguration.Mode = helpers.FirstNonEmpty(kProxySpec.Mode, data.KubeProxyConfiguration.Mode)
	if kProxySpec.MasqueradeAll != nil {
		data.KubeProxyConfiguration.Iptables.MasqueradeAll = *kProxySpec.MasqueradeAll
	}

	// Extra Args & Feature Gates
	data.ClusterConfiguration.ApiServer.ExtraArgs = mergeMaps(cluster.Spec.Kubernetes.APIServer.ExtraArgs, data.ClusterConfiguration.ApiServer.ExtraArgs)
	data.ClusterConfiguration.ApiServer.ExtraArgs["feature-gates"] = formatFeatureGates(cluster.Spec.Kubernetes.APIServer.FeatureGates, data.ClusterConfiguration.ApiServer.ExtraArgs["feature-gates"])

	if audit := cluster.Spec.Kubernetes.APIServer.AuditConfig; audit != nil && audit.Enabled != nil && *audit.Enabled {
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-path"] = audit.LogPath
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxage"] = strconv.Itoa(*audit.MaxAge)
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxbackup"] = strconv.Itoa(*audit.MaxBackups)
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxsize"] = strconv.Itoa(*audit.MaxSize)
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-policy-file"] = audit.PolicyFile
	}

	data.ClusterConfiguration.ControllerManager.ExtraArgs = mergeMaps(cluster.Spec.Kubernetes.ControllerManager.ExtraArgs, data.ClusterConfiguration.ControllerManager.ExtraArgs)
	if cluster.Spec.Kubernetes.ControllerManager.NodeCidrMaskSize != nil {
		data.ClusterConfiguration.ControllerManager.ExtraArgs["node-cidr-mask-size"] = strconv.Itoa(*cluster.Spec.Kubernetes.ControllerManager.NodeCidrMaskSize)
	}
	data.ClusterConfiguration.ControllerManager.ExtraArgs["feature-gates"] = formatFeatureGates(cluster.Spec.Kubernetes.ControllerManager.FeatureGates, data.ClusterConfiguration.ControllerManager.ExtraArgs["feature-gates"])

	data.ClusterConfiguration.Scheduler.ExtraArgs = mergeMaps(cluster.Spec.Kubernetes.Scheduler.ExtraArgs, data.ClusterConfiguration.Scheduler.ExtraArgs)
	data.ClusterConfiguration.Scheduler.ExtraArgs["feature-gates"] = formatFeatureGates(cluster.Spec.Kubernetes.Scheduler.FeatureGates, data.ClusterConfiguration.Scheduler.ExtraArgs["feature-gates"])

	data.InitConfiguration.NodeRegistration.KubeletExtraArgs = cluster.Spec.Kubernetes.Kubelet.ExtraArgs

	// Kubelet configuration
	kletSpec := cluster.Spec.Kubernetes.Kubelet
	if kletSpec.ContainerLogMaxFiles != nil {
		data.KubeletConfiguration.ContainerLogMaxFiles = *kletSpec.ContainerLogMaxFiles
	}
	data.KubeletConfiguration.ContainerLogMaxSize = helpers.FirstNonEmpty(kletSpec.ContainerLogMaxSize, data.KubeletConfiguration.ContainerLogMaxSize)
	data.KubeletConfiguration.EvictionPressureTransitionPeriod = helpers.FirstNonEmpty(kletSpec.EvictionPressureTransitionPeriod, data.KubeletConfiguration.EvictionPressureTransitionPeriod)
	if kletSpec.EvictionMaxPodGracePeriod != nil {
		data.KubeletConfiguration.EvictionMaxPodGracePeriod = *kletSpec.EvictionMaxPodGracePeriod
	}
	if kletSpec.MaxPods != nil {
		data.KubeletConfiguration.MaxPods = *kletSpec.MaxPods
	}
	if kletSpec.PodPidsLimit != nil {
		data.KubeletConfiguration.PodPidsLimit = *kletSpec.PodPidsLimit
	}
	data.KubeletConfiguration.EvictionHard = mergeMaps(kletSpec.EvictionHard, data.KubeletConfiguration.EvictionHard)
	data.KubeletConfiguration.EvictionSoft = mergeMaps(kletSpec.EvictionSoft, data.KubeletConfiguration.EvictionSoft)
	data.KubeletConfiguration.EvictionSoftGracePeriod = mergeMaps(kletSpec.EvictionSoftGracePeriod, data.KubeletConfiguration.EvictionSoftGracePeriod)
	data.KubeletConfiguration.KubeReserved = mergeMaps(kletSpec.KubeReserved, data.KubeletConfiguration.KubeReserved)
	data.KubeletConfiguration.SystemReserved = mergeMaps(kletSpec.SystemReserved, data.KubeletConfiguration.SystemReserved)
	for k, v := range kletSpec.FeatureGates {
		data.KubeletConfiguration.FeatureGates[k] = v
	}

	// Cluster DNS
	dnsIP, err := helpers.GetDNSIPFromCIDR(data.ClusterConfiguration.Networking.ServiceSubnet)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS IP from service subnet: %w", err)
	}
	data.KubeletConfiguration.ClusterDNS = dnsIP
	if cluster.Spec.DNS.NodeLocalDNS.Enabled != nil && *cluster.Spec.DNS.NodeLocalDNS.Enabled {
		data.KubeletConfiguration.ClusterDNS = common.DefaultLocalDNS
	}

	// Certificate SANs
	sans := []string{"kubernetes", "kubernetes.default", "kubernetes.default.svc", "127.0.0.1", "localhost", cpDomain}
	if dnsDomain := data.ClusterConfiguration.Networking.DNSDomain; dnsDomain != "" {
		sans = append(sans, fmt.Sprintf("kubernetes.default.svc.%s", dnsDomain))
	}

	kubernetesServiceIP, err := helpers.GetFirstIPFromCIDR(data.ClusterConfiguration.Networking.ServiceSubnet)
	if err == nil {
		sans = append(sans, kubernetesServiceIP)
	}

	for _, host := range ctx.GetHostsByRole("") {
		sans = append(sans, host.GetName(), host.GetInternalAddress())
	}
	sans = append(sans, cluster.Spec.Kubernetes.APIServer.CertExtraSans...)
	data.ClusterConfiguration.ApiServer.CertSANs = helpers.UniqueStringSlice(sans)

	// --- 3. Render the template ---
	templateContent, err := templates.Get("kubernetes/kubeadm-init-config.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeadm init template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render kubeadm init template: %w", err)
	}
	return []byte(renderedConfig), nil
}

func (s *GenerateFirstMasterConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmInitConfigFileName)
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", remoteConfigPath, ctx.GetHost().GetName(), err)
	}
	if !exists {
		logger.Info("Remote config file does not exist. Step needs to run.")
		return false, nil
	}

	logger.Info("Remote config file exists. Comparing content.")
	expectedContent, err := s.renderContent(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to render expected config for precheck: %w", err)
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to read remote config file '%s': %w", remoteConfigPath, err)
	}
	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		logger.Info("Remote config file content matches the expected content. Step is done.")
		return true, nil
	}

	logger.Info("Remote config file content differs from expected content. Step needs to run to update it.")
	return false, nil
}

func (s *GenerateFirstMasterConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Rendering kubeadm init config")
	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	remoteConfigDir := common.KubernetesConfigDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		return fmt.Errorf("failed to create remote directory '%s': %w", remoteConfigDir, err)
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, common.KubeadmInitConfigFileName)
	logger.Infof("Uploading/Updating rendered config to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)

	if err := runner.WriteFile(ctx.GoContext(), conn, renderedConfig, remoteConfigPath, "0644", false); err != nil {
		return fmt.Errorf("failed to upload kubeadm config file: %w", err)
	}
	logger.Info("Kubeadm init configuration generated and uploaded successfully.")
	return nil
}

func (s *GenerateFirstMasterConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	remoteConfigPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmInitConfigFileName)
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

// mergeMaps merges two maps, with values from the `override` map taking precedence.
func mergeMaps(override, base map[string]string) map[string]string {
	if override == nil && base == nil {
		return nil
	}
	merged := make(map[string]string)
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// formatFeatureGates combines default and override feature gates into a comma-separated string.
func formatFeatureGates(override map[string]string, defaultGates string) string {
	// This is a simplified approach. A more robust implementation would parse the defaultGates string.
	// For now, we assume override takes full precedence if not empty.
	if len(override) > 0 {
		var pairs []string
		for k, v := range override {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(pairs) // for consistent output
		return strings.Join(pairs, ",")
	}
	return defaultGates
}

var _ step.Step = (*GenerateFirstMasterConfigStep)(nil)
