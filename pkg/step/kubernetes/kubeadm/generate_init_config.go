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
	"github.com/mensylisir/kubexm/pkg/util"
)

type GenerateInitConfigStep struct {
	step.Base
}
type GenerateInitConfigStepBuilder struct {
	step.Builder[GenerateInitConfigStepBuilder, *GenerateInitConfigStep]
}

func NewGenerateInitConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateInitConfigStepBuilder {
	s := &GenerateInitConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kubeadm init configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateInitConfigStepBuilder).Init(s)
	return b
}
func (s *GenerateInitConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type TemplateData struct {
	ClusterConfiguration   ClusterConfigurationTemplate
	InitConfiguration      InitConfigurationTemplate
	KubeProxyConfiguration KubeProxyConfigurationTemplate
	KubeletConfiguration   KubeletConfigurationTemplate
	ImageRepository        string
	ClusterName            string
	KubernetesVersion      string
}

type ClusterConfigurationTemplate struct {
	Etcd                 EtcdTemplate
	DNS                  DNSTemplate
	Networking           NetworkingTemplate
	ApiServer            ApiServerTemplate
	ControllerManager    ControllerManagerTemplate
	Scheduler            SchedulerTemplate
	KubernetesVersion    string
	CertificatesDir      string
	ControlPlaneEndpoint string
}

type EtcdTemplate struct {
	IsExternal bool
	Endpoints  []string
	CaFile     string
	CertFile   string
	KeyFile    string
}

type DNSTemplate struct {
	ImageTag string
}

type NetworkingTemplate struct {
	PodSubnet     string
	ServiceSubnet string
}

type ApiServerTemplate struct {
	ExtraArgs map[string]string
	CertSANs  []string
}

type ControllerManagerTemplate struct {
	ExtraArgs    map[string]string
	ExtraVolumes []VolumeTemplate
}

type VolumeTemplate struct {
	Name      string
	HostPath  string
	MountPath string
	ReadOnly  bool
}

type SchedulerTemplate struct {
	ExtraArgs map[string]string
}

type InitConfigurationTemplate struct {
	LocalAPIEndpoint LocalAPIEndpointTemplate
	NodeRegistration NodeRegistrationTemplate
}

type LocalAPIEndpointTemplate struct {
	AdvertiseAddress string
	BindPort         int
}

type NodeRegistrationTemplate struct {
	CRISocket        string
	CgroupDriver     string
	KubeletExtraArgs map[string]string
}

type KubeProxyConfigurationTemplate struct {
	Mode     string
	Iptables IptablesTemplate
}

type IptablesTemplate struct {
	MasqueradeAll bool
	MasqueradeBit int
	MinSyncPeriod string
	SyncPeriod    string
}

type KubeletConfigurationTemplate struct {
	ClusterDNS                       string
	ContainerLogMaxSize              string
	EvictionPressureTransitionPeriod string
	ContainerLogMaxFiles             int
	EvictionMaxPodGracePeriod        int
	MaxPods                          int
	PodPidsLimit                     int64
	RotateCertificates               bool
	SerializeImagePulls              bool
	EvictionHard                     map[string]string
	EvictionSoft                     map[string]string
	EvictionSoftGracePeriod          map[string]string
	KubeReserved                     map[string]string
	SystemReserved                   map[string]string
	FeatureGates                     map[string]bool
}

func (s *GenerateInitConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()
	currentHost := ctx.GetHost()

	data := TemplateData{
		ClusterConfiguration:   ClusterConfigurationTemplate{},
		InitConfiguration:      InitConfigurationTemplate{},
		KubeProxyConfiguration: KubeProxyConfigurationTemplate{},
		KubeletConfiguration:   KubeletConfigurationTemplate{},
	}

	data.KubernetesVersion = helpers.FirstNonEmpty(cluster.Spec.Kubernetes.Version, common.DefaultK8sVersion)
	data.ClusterConfiguration.KubernetesVersion = data.KubernetesVersion
	data.ClusterName = helpers.FirstNonEmpty(cluster.Spec.Kubernetes.ClusterName, common.DefaultClusterLocal)
	data.ClusterConfiguration.CertificatesDir = common.DefaultKubernetesPKIDir
	data.ClusterConfiguration.ControlPlaneEndpoint = fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Domain, cluster.Spec.ControlPlaneEndpoint.Port)

	imageProvider := images.NewImageProvider(ctx)
	corednsImage := imageProvider.GetImage("coredns")
	data.ImageRepository = imageProvider.GetImage("kube-apiserver").RegistryAddrWithNamespace()
	data.ClusterConfiguration.DNS.ImageTag = corednsImage.Tag()

	data.ClusterConfiguration.Networking.PodSubnet = helpers.FirstNonEmpty(cluster.Spec.Network.KubePodsCIDR, common.DefaultKubePodsCIDR)
	data.ClusterConfiguration.Networking.ServiceSubnet = helpers.FirstNonEmpty(cluster.Spec.Network.KubeServiceCIDR, common.DefaultKubeServiceCIDR)

	var defaultCRISocket, cgroupDriver string
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		defaultCRISocket = common.ContainerdDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Containerd.CgroupDriver
	case common.RuntimeTypeCRIO:
		defaultCRISocket = common.CRIODefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Crio.CgroupDriver
	case common.RuntimeTypeDocker:
		defaultCRISocket = common.CriDockerdSocketPath
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Docker.CgroupDriver
	case common.RuntimeTypeIsula:
		defaultCRISocket = common.IsuladDefaultEndpoint
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Isulad.CgroupDriver
	default:
		cgroupDriver = common.CgroupDriverSystemd
	}
	data.InitConfiguration.NodeRegistration.CRISocket = defaultCRISocket
	data.InitConfiguration.NodeRegistration.CgroupDriver = cgroupDriver

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	data.ClusterConfiguration.Etcd.IsExternal = false
	if cluster.Spec.Etcd.Type == string(common.EtcdDeploymentTypeExternal) && cluster.Spec.Etcd.External != nil {
		data.ClusterConfiguration.Etcd.IsExternal = true
		ext := cluster.Spec.Etcd.External
		data.ClusterConfiguration.Etcd.Endpoints = ext.Endpoints
		data.ClusterConfiguration.Etcd.CaFile = ext.CAFile
		data.ClusterConfiguration.Etcd.CertFile = ext.CertFile
		data.ClusterConfiguration.Etcd.KeyFile = ext.KeyFile
	} else if cluster.Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubexm) {
		data.ClusterConfiguration.Etcd.IsExternal = true
		endpoints := make([]string, len(etcdNodes))
		for i, node := range etcdNodes {
			endpoints[i] = fmt.Sprintf("https://%s:%d", node.GetInternalAddress(), common.EtcdDefaultClientPort)
		}
		data.ClusterConfiguration.Etcd.Endpoints = endpoints
		data.ClusterConfiguration.Etcd.CaFile = filepath.Join(common.DefaultEtcdPKIDir, common.EtcdCaPemFileName)
		data.ClusterConfiguration.Etcd.CertFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, currentHost.GetName()))
		data.ClusterConfiguration.Etcd.KeyFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, currentHost.GetName()))
	}

	mergeFeatureGates := func(defaults string, overrides map[string]bool) string {
		fgMap := make(map[string]string)

		for _, pair := range strings.Split(defaults, ",") {
			if kv := strings.SplitN(pair, "=", 2); len(kv) == 2 && kv[0] != "" {
				fgMap[kv[0]] = kv[1]
			}
		}

		for k, v := range overrides {
			fgMap[k] = strconv.FormatBool(v)
		}

		var result []string
		for k, v := range fgMap {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(result)
		return strings.Join(result, ",")
	}

	apiServerDefaultArgs := map[string]string{
		"audit-log-maxage":    "30",
		"audit-log-maxbackup": "10",
		"audit-log-maxsize":   "100",
		"audit-log-path":      common.DefaultAuditLogFile,
		"bind-address":        "0.0.0.0",
		"feature-gates":       "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
	}
	data.ClusterConfiguration.ApiServer.ExtraArgs = helpers.MergeStringMaps(apiServerDefaultArgs, cluster.Spec.Kubernetes.APIServer.ExtraArgs)
	if cluster.Spec.Kubernetes.APIServer.AuditConfig.Enabled == helpers.BoolPtr(true) {
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-path"] = cluster.Spec.Kubernetes.APIServer.AuditConfig.LogPath
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxage"] = strconv.Itoa(*cluster.Spec.Kubernetes.APIServer.AuditConfig.MaxAge)
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxbackup"] = strconv.Itoa(*cluster.Spec.Kubernetes.APIServer.AuditConfig.MaxBackups)
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxsize"] = strconv.Itoa(*cluster.Spec.Kubernetes.APIServer.AuditConfig.MaxSize)
		data.ClusterConfiguration.ApiServer.ExtraArgs["audit-policy-file"] = cluster.Spec.Kubernetes.APIServer.AuditConfig.PolicyFile
	}
	data.ClusterConfiguration.ApiServer.ExtraArgs["feature-gates"] = mergeFeatureGates(apiServerDefaultArgs["feature-gates"], cluster.Spec.Kubernetes.APIServer.FeatureGates)

	sans := []string{"kubernetes", "kubernetes.default", "kubernetes.default.svc", "127.0.0.1", "localhost", cluster.Spec.ControlPlaneEndpoint.Domain}
	sans = append(sans, fmt.Sprintf("kubernetes.default.svc.%s", data.ClusterName))
	kubernetesServiceIP, _ := helpers.GetFirstIPFromCIDR(data.ClusterConfiguration.Networking.ServiceSubnet)
	sans = append(sans, kubernetesServiceIP)
	for _, node := range ctx.GetHostsByRole("") {
		sans = append(sans, node.GetInternalAddress(), node.GetName())
	}
	sans = append(sans, cluster.Spec.Kubernetes.APIServer.CertExtraSans...)
	data.ClusterConfiguration.ApiServer.CertSANs = helpers.UniqueStringSlice(sans)

	controllerManagerDefaultArgs := map[string]string{
		"node-cidr-mask-size":      "24",
		"bind-address":             "0.0.0.0",
		"cluster-signing-duration": "87600h",
		"feature-gates":            "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
	}
	data.ClusterConfiguration.ControllerManager.ExtraArgs = helpers.MergeStringMaps(controllerManagerDefaultArgs, cluster.Spec.Kubernetes.ControllerManager.ExtraArgs)
	if cluster.Spec.Kubernetes.ControllerManager.NodeCidrMaskSize != nil {
		data.ClusterConfiguration.ControllerManager.ExtraArgs["node-cidr-mask-size"] = strconv.Itoa(*cluster.Spec.Kubernetes.ControllerManager.NodeCidrMaskSize)
	}
	data.ClusterConfiguration.ControllerManager.ExtraArgs["feature-gates"] = mergeFeatureGates(controllerManagerDefaultArgs["feature-gates"], cluster.Spec.Kubernetes.ControllerManager.FeatureGates)

	schedulerDefaultArgs := map[string]string{
		"bind-address":  "0.0.0.0",
		"feature-gates": "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
	}
	data.ClusterConfiguration.Scheduler.ExtraArgs = helpers.MergeStringMaps(schedulerDefaultArgs, cluster.Spec.Kubernetes.Scheduler.ExtraArgs)
	data.ClusterConfiguration.Scheduler.ExtraArgs["feature-gates"] = mergeFeatureGates(schedulerDefaultArgs["feature-gates"], cluster.Spec.Kubernetes.Scheduler.FeatureGates)

	data.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress = currentHost.GetInternalAddress()
	data.InitConfiguration.LocalAPIEndpoint.BindPort = common.DefaultAPIServerPort
	data.InitConfiguration.NodeRegistration.KubeletExtraArgs = cluster.Spec.Kubernetes.Kubelet.ExtraArgs

	data.KubeProxyConfiguration.Mode = util.FirstNonEmpty(cluster.Spec.Kubernetes.KubeProxy.Mode, common.KubeProxyModeIPVS)
	data.KubeProxyConfiguration.Iptables.MasqueradeBit = 14
	data.KubeProxyConfiguration.Iptables.SyncPeriod = "30s"
	data.KubeProxyConfiguration.Iptables.MinSyncPeriod = "0s"
	if cluster.Spec.Kubernetes.KubeProxy.MasqueradeAll != nil {
		data.KubeProxyConfiguration.Iptables.MasqueradeAll = *cluster.Spec.Kubernetes.KubeProxy.MasqueradeAll
	}

	kubeletSpec := cluster.Spec.Kubernetes.Kubelet
	data.KubeletConfiguration.MaxPods = 110
	if kubeletSpec.MaxPods != nil {
		data.KubeletConfiguration.MaxPods = *kubeletSpec.MaxPods
	}
	data.KubeletConfiguration.PodPidsLimit = 10000
	if kubeletSpec.PodPidsLimit != nil {
		data.KubeletConfiguration.PodPidsLimit = int64(*kubeletSpec.PodPidsLimit)
	}
	data.KubeletConfiguration.ContainerLogMaxFiles = 3
	if kubeletSpec.ContainerLogMaxFiles != nil {
		data.KubeletConfiguration.ContainerLogMaxFiles = *kubeletSpec.ContainerLogMaxFiles
	}
	data.KubeletConfiguration.ContainerLogMaxSize = util.FirstNonEmpty(kubeletSpec.ContainerLogMaxSize, "5Mi")
	data.KubeletConfiguration.EvictionPressureTransitionPeriod = util.FirstNonEmpty(kubeletSpec.EvictionPressureTransitionPeriod, "30s")
	data.KubeletConfiguration.EvictionMaxPodGracePeriod = 120
	if kubeletSpec.EvictionMaxPodGracePeriod != nil {
		data.KubeletConfiguration.EvictionMaxPodGracePeriod = *kubeletSpec.EvictionMaxPodGracePeriod
	}
	data.KubeletConfiguration.RotateCertificates = true
	data.KubeletConfiguration.SerializeImagePulls = true

	data.KubeletConfiguration.EvictionHard = helpers.MergeStringMaps(map[string]string{"memory.available": "5%", "pid.available": "10%"}, kubeletSpec.EvictionHard)
	data.KubeletConfiguration.EvictionSoft = helpers.MergeStringMaps(map[string]string{"memory.available": "10%"}, kubeletSpec.EvictionSoft)
	data.KubeletConfiguration.EvictionSoftGracePeriod = helpers.MergeStringMaps(map[string]string{"memory.available": "2m"}, kubeletSpec.EvictionSoftGracePeriod)
	data.KubeletConfiguration.KubeReserved = helpers.MergeStringMaps(map[string]string{"cpu": "200m", "memory": "250Mi"}, kubeletSpec.KubeReserved)
	data.KubeletConfiguration.SystemReserved = kubeletSpec.SystemReserved                                                                                                                                   // No defaults specified in original code
	data.KubeletConfiguration.FeatureGates = helpers.MergeBoolMaps(map[string]bool{"CSIStorageCapacity": true, "ExpandCSIVolumes": true, "RotateKubeletServerCertificate": true}, kubeletSpec.FeatureGates) // Assuming util.MergeBoolMaps exists

	if cluster.Spec.DNS.NodeLocalDNS.Enabled == helpers.BoolPtr(true) {
		data.KubeletConfiguration.ClusterDNS = common.DefaultLocalDNS
	} else {
		dnsIP, err := helpers.GetDNSIPFromCIDR(data.ClusterConfiguration.Networking.ServiceSubnet)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate DNS IP from service subnet: %w", err)
		}
		data.KubeletConfiguration.ClusterDNS = dnsIP
	}

	templateContent, err := templates.Get("kubernetes/kubeadm/kubeadm-init-config.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeadm init template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render kubeadm init template: %w", err)
	}
	return []byte(renderedConfig), nil
}

func (s *GenerateInitConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

func (s *GenerateInitConfigStep) Run(ctx runtime.ExecutionContext) error {
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

	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteConfigPath, "0644", false); err != nil {
		return fmt.Errorf("failed to upload kubeadm config file: %w", err)
	}
	logger.Info("Kubeadm init configuration generated and uploaded successfully.")
	return nil
}

func (s *GenerateInitConfigStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*GenerateInitConfigStep)(nil)
