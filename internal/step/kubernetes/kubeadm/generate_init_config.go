package kubeadm

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/util/images"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/mensylisir/kubexm/internal/util"
)

type GenerateInitConfigStep struct {
	step.Base
}
type GenerateInitConfigStepBuilder struct {
	step.Builder[GenerateInitConfigStepBuilder, *GenerateInitConfigStep]
}

func NewGenerateInitConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateInitConfigStepBuilder {
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
	ExtraArgs    map[string]string
	CertSANs     []string
	ExtraVolumes []VolumeTemplate
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

	// Safe access to spec fields
	k8sSpec := &v1alpha1.Kubernetes{}
	if cluster.Spec.Kubernetes != nil {
		k8sSpec = cluster.Spec.Kubernetes
	}
	networkSpec := &v1alpha1.Network{}
	if cluster.Spec.Network != nil {
		networkSpec = cluster.Spec.Network
	}
	etcdSpec := &v1alpha1.Etcd{}
	if cluster.Spec.Etcd != nil {
		etcdSpec = cluster.Spec.Etcd
	}
	cpEndpoint := &v1alpha1.ControlPlaneEndpointSpec{}
	if cluster.Spec.ControlPlaneEndpoint != nil {
		cpEndpoint = cluster.Spec.ControlPlaneEndpoint
	}

	data.KubernetesVersion = helpers.FirstNonEmpty(k8sSpec.Version, common.DefaultK8sVersion)
	data.ClusterConfiguration.KubernetesVersion = data.KubernetesVersion
	data.ClusterName = helpers.FirstNonEmpty(k8sSpec.ClusterName, common.DefaultClusterLocal)
	data.ClusterConfiguration.CertificatesDir = common.DefaultKubernetesPKIDir
	data.ClusterConfiguration.ControlPlaneEndpoint = fmt.Sprintf("%s:%d", cpEndpoint.Domain, cpEndpoint.Port)

	imageProvider := images.NewImageProvider(ctx)
	corednsImage := imageProvider.GetImage("coredns")
	data.ImageRepository = imageProvider.GetImage("kube-apiserver").RegistryAddrWithNamespace()
	data.ClusterConfiguration.DNS.ImageTag = corednsImage.Tag()

	data.ClusterConfiguration.Networking.PodSubnet = helpers.FirstNonEmpty(networkSpec.KubePodsCIDR, common.DefaultKubePodsCIDR)
	data.ClusterConfiguration.Networking.ServiceSubnet = helpers.FirstNonEmpty(networkSpec.KubeServiceCIDR, common.DefaultKubeServiceCIDR)

	var defaultCRISocket, cgroupDriver string
	crSpec := k8sSpec.ContainerRuntime
	if crSpec != nil {
		switch crSpec.Type {
		case common.RuntimeTypeContainerd:
			defaultCRISocket = common.ContainerdDefaultEndpoint
			if crSpec.Containerd != nil && crSpec.Containerd.CgroupDriver != nil {
				cgroupDriver = *crSpec.Containerd.CgroupDriver
			}
		case common.RuntimeTypeCRIO:
			defaultCRISocket = common.CRIODefaultEndpoint
			if crSpec.Crio != nil && crSpec.Crio.CgroupDriver != nil {
				cgroupDriver = *crSpec.Crio.CgroupDriver
			}
		case common.RuntimeTypeDocker:
			defaultCRISocket = common.CriDockerdSocketPath
			if crSpec.Docker != nil && crSpec.Docker.CgroupDriver != nil {
				cgroupDriver = *crSpec.Docker.CgroupDriver
			}
		case common.RuntimeTypeIsula:
			defaultCRISocket = common.IsuladDefaultEndpoint
			if crSpec.Isulad != nil && crSpec.Isulad.CgroupDriver != nil {
				cgroupDriver = *crSpec.Isulad.CgroupDriver
			}
		}
	}
	data.InitConfiguration.NodeRegistration.CRISocket = defaultCRISocket
	data.InitConfiguration.NodeRegistration.CgroupDriver = cgroupDriver

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	data.ClusterConfiguration.Etcd.IsExternal = false
	if etcdSpec.Type == string(common.EtcdDeploymentTypeExternal) && etcdSpec.External != nil {
		data.ClusterConfiguration.Etcd.IsExternal = true
		ext := etcdSpec.External
		data.ClusterConfiguration.Etcd.Endpoints = ext.Endpoints
		data.ClusterConfiguration.Etcd.CaFile = ext.CAFile
		data.ClusterConfiguration.Etcd.CertFile = ext.CertFile
		data.ClusterConfiguration.Etcd.KeyFile = ext.KeyFile
	} else if etcdSpec.Type == string(common.EtcdDeploymentTypeKubexm) {
		data.ClusterConfiguration.Etcd.IsExternal = true
		endpoints := make([]string, len(etcdNodes))
		for i, node := range etcdNodes {
			endpoints[i] = fmt.Sprintf("https://%s:%d", node.GetInternalAddress(), common.EtcdDefaultClientPort)
		}
		data.ClusterConfiguration.Etcd.Endpoints = endpoints
		data.ClusterConfiguration.Etcd.CaFile = filepath.Join(common.DefaultEtcdPKIDir, common.EtcdCaPemFileName)
		data.ClusterConfiguration.Etcd.CertFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, currentHost.GetName()))
		data.ClusterConfiguration.Etcd.KeyFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, currentHost.GetName()))

		// CRITICAL: When using kubexm-deployed etcd, the apiserver pod needs to mount
		// the /etc/etcd/pki directory from the host filesystem. Without this extraVolume,
		// the apiserver pod cannot read the etcd client certificates.
		data.ClusterConfiguration.ApiServer.ExtraVolumes = []VolumeTemplate{
			{
				Name:      "etcd-certs",
				HostPath:  common.DefaultEtcdPKIDir,
				MountPath: common.DefaultEtcdPKIDir,
				ReadOnly:  true,
			},
		}
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
	data.ClusterConfiguration.ApiServer.ExtraArgs = helpers.MergeStringMaps(apiServerDefaultArgs, k8sSpec.APIServer.ExtraArgs)
	if k8sSpec.APIServer.AuditConfig != nil && k8sSpec.APIServer.AuditConfig.Enabled != nil && *k8sSpec.APIServer.AuditConfig.Enabled {
		audit := k8sSpec.APIServer.AuditConfig
		if audit.LogPath != "" {
			data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-path"] = audit.LogPath
		}
		if audit.MaxAge != nil {
			data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxage"] = strconv.Itoa(*audit.MaxAge)
		}
		if audit.MaxBackups != nil {
			data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxbackup"] = strconv.Itoa(*audit.MaxBackups)
		}
		if audit.MaxSize != nil {
			data.ClusterConfiguration.ApiServer.ExtraArgs["audit-log-maxsize"] = strconv.Itoa(*audit.MaxSize)
		}
		if audit.PolicyFile != "" {
			data.ClusterConfiguration.ApiServer.ExtraArgs["audit-policy-file"] = audit.PolicyFile
		}
	}
	data.ClusterConfiguration.ApiServer.ExtraArgs["feature-gates"] = mergeFeatureGates(apiServerDefaultArgs["feature-gates"], k8sSpec.APIServer.FeatureGates)

	sans := []string{"kubernetes", "kubernetes.default", "kubernetes.default.svc", "127.0.0.1", "localhost", cpEndpoint.Domain}
	sans = append(sans, fmt.Sprintf("kubernetes.default.svc.%s", data.ClusterName))
	kubernetesServiceIP, _ := helpers.GetFirstIPFromCIDR(data.ClusterConfiguration.Networking.ServiceSubnet)
	sans = append(sans, kubernetesServiceIP)
	for _, node := range ctx.GetHostsByRole("") {
		sans = append(sans, node.GetInternalAddress(), node.GetName())
	}
	if k8sSpec.APIServer.CertExtraSans != nil {
		sans = append(sans, k8sSpec.APIServer.CertExtraSans...)
	}
	data.ClusterConfiguration.ApiServer.CertSANs = helpers.UniqueStringSlice(sans)

	controllerManagerDefaultArgs := map[string]string{
		"node-cidr-mask-size":      "24",
		"bind-address":             "0.0.0.0",
		"cluster-signing-duration": "87600h",
		"feature-gates":            "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
	}
	data.ClusterConfiguration.ControllerManager.ExtraArgs = helpers.MergeStringMaps(controllerManagerDefaultArgs, k8sSpec.ControllerManager.ExtraArgs)
	if k8sSpec.ControllerManager.NodeCidrMaskSize != nil {
		data.ClusterConfiguration.ControllerManager.ExtraArgs["node-cidr-mask-size"] = strconv.Itoa(*k8sSpec.ControllerManager.NodeCidrMaskSize)
	}
	data.ClusterConfiguration.ControllerManager.ExtraArgs["feature-gates"] = mergeFeatureGates(controllerManagerDefaultArgs["feature-gates"], k8sSpec.ControllerManager.FeatureGates)

	schedulerDefaultArgs := map[string]string{
		"bind-address":  "0.0.0.0",
		"feature-gates": "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true",
	}
	data.ClusterConfiguration.Scheduler.ExtraArgs = helpers.MergeStringMaps(schedulerDefaultArgs, k8sSpec.Scheduler.ExtraArgs)
	data.ClusterConfiguration.Scheduler.ExtraArgs["feature-gates"] = mergeFeatureGates(schedulerDefaultArgs["feature-gates"], k8sSpec.Scheduler.FeatureGates)

	data.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress = currentHost.GetInternalAddress()
	data.InitConfiguration.LocalAPIEndpoint.BindPort = common.DefaultAPIServerPort
	data.InitConfiguration.NodeRegistration.KubeletExtraArgs = k8sSpec.Kubelet.ExtraArgs

	data.KubeProxyConfiguration.Mode = util.FirstNonEmpty(k8sSpec.KubeProxy.Mode, common.KubeProxyModeIPVS)
	data.KubeProxyConfiguration.Iptables.MasqueradeBit = 14
	data.KubeProxyConfiguration.Iptables.SyncPeriod = "30s"
	data.KubeProxyConfiguration.Iptables.MinSyncPeriod = "0s"
	if k8sSpec.KubeProxy.MasqueradeAll != nil {
		data.KubeProxyConfiguration.Iptables.MasqueradeAll = *k8sSpec.KubeProxy.MasqueradeAll
	}

	kubeletSpec := k8sSpec.Kubelet
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

	dnsSpec := &v1alpha1.DNS{}
	if cluster.Spec.DNS != nil {
		dnsSpec = cluster.Spec.DNS
	}
	if dnsSpec.NodeLocalDNS != nil && dnsSpec.NodeLocalDNS.Enabled != nil && *dnsSpec.NodeLocalDNS.Enabled {
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

func (s *GenerateInitConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	logger.Info("Rendering kubeadm init config")
	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render kubeadm init config")
		return result, err
	}

	remoteConfigDir := common.KubernetesConfigDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		err = fmt.Errorf("failed to create remote directory '%s': %w", remoteConfigDir, err)
		result.MarkFailed(err, "failed to create remote directory")
		return result, err
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, common.KubeadmInitConfigFileName)
	logger.Infof("Uploading/Updating rendered config to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)

	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteConfigPath, "0644", false); err != nil {
		err = fmt.Errorf("failed to upload kubeadm config file: %w", err)
		result.MarkFailed(err, "failed to upload kubeadm config file")
		return result, err
	}
	logger.Info("Kubeadm init configuration generated and uploaded successfully.")
	result.MarkCompleted("kubeadm init config generated and uploaded successfully")
	return result, nil
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
