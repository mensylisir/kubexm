package kubeadm

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
	"github.com/mensylisir/kubexm/pkg/util" // 假设有一个包含辅助函数的 util 包
)

// GenerateInitConfigStep 及其构建器
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
	IsExternal                                  bool
	Endpoints                                   []string
	CaFile, CertFile, KeyFile, Version, DataDir string
	ExtraArgs                                   map[string]string
}

type DNSTemplate struct {
	ImageTag string
}

type NetworkingTemplate struct {
	PodSubnet, ServiceSubnet string
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
	CRISocket, CgroupDriver string
	KubeletExtraArgs        map[string]string
}

type KubeProxyConfigurationTemplate struct {
	Mode     string
	Iptables IptablesTemplate
}

type IptablesTemplate struct {
	MasqueradeAll             bool
	MasqueradeBit             int
	MinSyncPeriod, SyncPeriod string
}

type KubeletConfigurationTemplate struct {
	ClusterDNS, ContainerLogMaxSize, EvictionPressureTransitionPeriod                 string
	ContainerLogMaxFiles, EvictionMaxPodGracePeriod, MaxPods                          int
	PodPidsLimit                                                                      int64
	RotateCertificates, SerializeImagePulls                                           bool
	EvictionHard, EvictionSoft, EvictionSoftGracePeriod, KubeReserved, SystemReserved map[string]string
	FeatureGates                                                                      map[string]bool
}

func (s *GenerateInitConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()
	currentHost := ctx.GetHost()

	data := TemplateData{}
	clusterConfiguration := ClusterConfigurationTemplate{}

	var DefaultCRISocket string
	var CGroupDriver string
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		DefaultCRISocket = common.ContainerdDefaultEndpoint
		CGroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Containerd.CgroupDriver
	case common.RuntimeTypeCRIO:
		DefaultCRISocket = common.CRIODefaultEndpoint
		CGroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Crio.CgroupDriver
	case common.RuntimeTypeDocker:
		DefaultCRISocket = common.CriDockerdSocketPath
		CGroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Docker.CgroupDriver
	case common.RuntimeTypeIsula:
		DefaultCRISocket = common.IsuladDefaultEndpoint
		CGroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Isulad.CgroupDriver
	}

	etcdConfig := EtcdTemplate{}
	var isExternalEtcd bool
	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 && cluster.Spec.Etcd.Type == string(common.EtcdDeploymentTypeExternal) && cluster.Spec.Etcd.External != nil {
		isExternalEtcd = true
		etcdConfig.CaFile = filepath.Join(cluster.Spec.Etcd.External.CAFile)
		etcdConfig.CertFile = filepath.Join(cluster.Spec.Etcd.External.CertFile)
		etcdConfig.KeyFile = filepath.Join(cluster.Spec.Etcd.External.KeyFile)
		etcdConfig.Endpoints = cluster.Spec.Etcd.External.Endpoints
	} else if cluster.Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubexm) {
		isExternalEtcd = false
		etcdConfig.Endpoints = make([]string, len(etcdNodes))
		for i, node := range etcdNodes {
			etcdConfig.Endpoints[i] = fmt.Sprintf("https://%s:%d", node.GetInternalAddress(), common.EtcdDefaultClientPort)
		}
		etcdConfig.CaFile = filepath.Join(common.DefaultEtcdPKIDir, common.EtcdCaPemFileName)
		etcdConfig.CertFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, etcdNodes[0].GetName()))
		etcdConfig.CaFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, etcdNodes[0].GetName()))
	} else if cluster.Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) || cluster.Spec.Etcd.Type == "" {
		isExternalEtcd = false
	}

	clusterConfiguration.Etcd = etcdConfig

	imageProvider := images.NewImageProvider(ctx)
	corednsImage := imageProvider.GetImage("coredns")

	clusterConfiguration.DNS.ImageTag = corednsImage.Tag()

	var PodSubnet string
	var ServiceSubnet string
	if cluster.Spec.Network.KubePodsCIDR == "" {
		PodSubnet = common.DefaultKubePodsCIDR
	} else {
		PodSubnet = cluster.Spec.Network.KubePodsCIDR
	}

	if cluster.Spec.Network.KubeServiceCIDR == "" {
		ServiceSubnet = common.DefaultKubeServiceCIDR
	} else {
		ServiceSubnet = cluster.Spec.Network.KubeServiceCIDR
	}
	clusterConfiguration.Networking = NetworkingTemplate{
		PodSubnet:     PodSubnet,
		ServiceSubnet: ServiceSubnet,
	}

	ApiServerExtraArgs := make(map[string]string)
	ApiServerExtraArgs["audit-log-maxage"] = "30"
	ApiServerExtraArgs["audit-log-maxbackup"] = "10"
	ApiServerExtraArgs["audit-log-maxsize"] = "100"
	ApiServerExtraArgs["audit-log-path"] = "/var/log/kubernetes/kube-apiserver.log"
	ApiServerExtraArgs["bind-address"] = "0.0.0.0"
	ApiServerExtraArgs["feature-gates"] = "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true"
	if cluster.Spec.Kubernetes.APIServer.AuditConfig.Enabled == helpers.BoolPtr(true) {
		ApiServerExtraArgs["audit-log-path"] = cluster.Spec.Kubernetes.APIServer.AuditConfig.LogPath
		ApiServerExtraArgs["audit-log-maxage"] = strconv.Itoa(*cluster.Spec.Kubernetes.APIServer.AuditConfig.MaxAge)
		ApiServerExtraArgs["audit-log-maxbackup"] = strconv.Itoa(*cluster.Spec.Kubernetes.APIServer.AuditConfig.MaxBackups)
		ApiServerExtraArgs["audit-log-maxsize"] = strconv.Itoa(*cluster.Spec.Kubernetes.APIServer.AuditConfig.MaxSize)
		ApiServerExtraArgs["audit-policy-file"] = cluster.Spec.Kubernetes.APIServer.AuditConfig.PolicyFile
	}
	if cluster.Spec.Kubernetes.APIServer.FeatureGates != nil {
		for key, value := range cluster.Spec.Kubernetes.APIServer.FeatureGates {
			args := fmt.Sprintf("%s=%s", key, value)
			ApiServerExtraArgs["feature-gates"] += args
		}
	}
	if cluster.Spec.Kubernetes.APIServer.ExtraArgs != nil {
		for key, value := range cluster.Spec.Kubernetes.APIServer.ExtraArgs {
			ApiServerExtraArgs[key] = value
		}
	}
	clusterConfiguration.ApiServer.ExtraArgs = ApiServerExtraArgs

	var dnsDomain string
	sans := []string{"kubernetes", "kubernetes.default", "kubernetes.default.svc", "127.0.0.1", "localhost"}
	if cluster.Spec.Kubernetes.DNSDomain != "" {
		dnsDomain = cluster.Spec.Kubernetes.DNSDomain
	} else {
		dnsDomain = common.DefaultClusterLocal
	}
	sans = append(sans, fmt.Sprintf("kubernetes.default.svc.%s", dnsDomain))
	nodes := ctx.GetHostsByRole("")
	for _, node := range nodes {
		sans = append(sans, node.GetInternalAddress(), node.GetName(), fmt.Sprintf("%s.%s", node.GetName(), dnsDomain))
		if ip := node.GetInternalIPv4Address(); ip != "" {
			sans = append(sans, ip)
		}
		if ip := node.GetInternalIPv6Address(); ip != "" {
			sans = append(sans, ip)
		}
		if addr := node.GetInternalAddress(); addr != "" {
			sans = append(sans, addr)
		}
		kubernetesServiceIP, _ := helpers.GetFirstIPFromCIDR(ServiceSubnet)
		sans = append(sans, kubernetesServiceIP)
	}
	if cluster.Spec.Kubernetes.APIServer.CertExtraSans != nil {
		sans = append(sans, cluster.Spec.Kubernetes.APIServer.CertExtraSans...)
	}

	if cluster.Spec.Kubernetes.APIServer.CertExtraSans != nil {
		sans = append(sans, cluster.Spec.Kubernetes.APIServer.CertExtraSans...)
	}
	clusterConfiguration.ApiServer.CertSANs = sans

	ControllerManagerExtraArgs := make(map[string]string)
	ControllerManagerExtraArgs["node-cidr-mask-size"] = "24"
	ControllerManagerExtraArgs["bind-address"] = "0.0.0.0"
	ControllerManagerExtraArgs["cluster-signing-duration"] = "87600h"
	ControllerManagerExtraArgs["feature-gates"] = "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true"
	if cluster.Spec.Kubernetes.ControllerManager.ExtraArgs != nil {
		for key, value := range cluster.Spec.Kubernetes.ControllerManager.ExtraArgs {
			ControllerManagerExtraArgs[key] = value
		}
	}
	if cluster.Spec.Kubernetes.ControllerManager.NodeCidrMaskSize != nil {
		ControllerManagerExtraArgs["node-cidr-mask-size"] = string(rune(*cluster.Spec.Kubernetes.ControllerManager.NodeCidrMaskSize))
	}
	if cluster.Spec.Kubernetes.ControllerManager.FeatureGates != nil {
		for key, value := range cluster.Spec.Kubernetes.ControllerManager.FeatureGates {
			args := fmt.Sprintf("%s=%s", key, value)
			ControllerManagerExtraArgs["feature-gates"] += args
		}
	}

	clusterConfiguration.ControllerManager.ExtraArgs = ControllerManagerExtraArgs

	SchedulerExtraArgs := make(map[string]string)
	SchedulerExtraArgs["feature-gates"] = "RotateKubeletServerCertificate=true,ExpandCSIVolumes=true,CSIStorageCapacity=true"
	SchedulerExtraArgs["bind-address"] = "0.0.0.0"
	if cluster.Spec.Kubernetes.Scheduler.ExtraArgs != nil {
		for key, value := range cluster.Spec.Kubernetes.Scheduler.ExtraArgs {
			SchedulerExtraArgs[key] = value
		}
	}
	if cluster.Spec.Kubernetes.Scheduler.FeatureGates != nil {
		for key, value := range cluster.Spec.Kubernetes.Scheduler.FeatureGates {
			args := fmt.Sprintf("%s=%s", key, value)
			SchedulerExtraArgs["feature-gates"] += args
		}
	}

	clusterConfiguration.Scheduler.ExtraArgs = SchedulerExtraArgs

	clusterConfiguration.CertificatesDir = common.DefaultKubernetesPKIDir
	clusterConfiguration.ControlPlaneEndpoint = fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Domain, common.DefaultAPIServerPort)

	data.ClusterConfiguration = clusterConfiguration

	initConfiguration := InitConfigurationTemplate{}
	initConfiguration.LocalAPIEndpoint.AdvertiseAddress = ctx.GetHostsByRole(common.RoleMaster)[0].GetInternalAddress()
	initConfiguration.LocalAPIEndpoint.BindPort = common.DefaultAPIServerPort
	initConfiguration.NodeRegistration.CRISocket = DefaultCRISocket
	initConfiguration.NodeRegistration.CgroupDriver = CGroupDriver

	if cluster.Spec.Kubernetes.Kubelet.ExtraArgs != nil {
		initConfiguration.NodeRegistration.KubeletExtraArgs = cluster.Spec.Kubernetes.Kubelet.ExtraArgs
	}

	data.InitConfiguration = initConfiguration

	var proxyMode string
	iptables := IptablesTemplate{}
	if cluster.Spec.Kubernetes.KubeProxy.Mode == "" {
		proxyMode = common.KubeProxyModeIPVS
	} else {
		proxyMode = cluster.Spec.Kubernetes.KubeProxy.Mode
	}
	iptables.MasqueradeAll = false
	iptables.MasqueradeBit = 14
	iptables.SyncPeriod = "30s"
	iptables.MinSyncPeriod = "0s"

	if cluster.Spec.Kubernetes.KubeProxy.MasqueradeAll != nil {
		iptables.MasqueradeAll = *cluster.Spec.Kubernetes.KubeProxy.MasqueradeAll
	}

	kubeProxyConfiguration := KubeProxyConfigurationTemplate{}
	kubeProxyConfiguration.Mode = proxyMode
	kubeProxyConfiguration.Iptables = iptables

	kubeletConfig := KubeletConfigurationTemplate{}

	var clusterDNS string
	if cluster.Spec.DNS.NodeLocalDNS.Enabled == helpers.BoolPtr(true) {
		clusterDNS = common.DefaultLocalDNS
	} else {
		clusterDNS, _ = helpers.GetDNSIPFromCIDR(ServiceSubnet)
	}
	kubeletConfig.ClusterDNS = clusterDNS

	containerLogMaxFiles := 3
	containerLogMaxSize := "5Mi"
	evictionPressureTransitionPeriod := "30s"
	evictionMaxPodGracePeriod := "120"
	evictionHard := make(map[string]string)
	evictionSoft := make(map[string]string)
	evictionSoftGracePeriod := make(map[string]string)
	featureGates := make(map[string]bool)
	kubeReserved := make(map[string]string)
	maxPods := 110
	podPidsLimit := 10000
	rotateCertificates := true
	serializeImagePulls := true

	SystemReserved := make(map[string]string)
	if cluster.Spec.Kubernetes.Kubelet.ContainerLogMaxFiles != nil {
		containerLogMaxFiles = string(rune(*cluster.Spec.Kubernetes.Kubelet.ContainerLogMaxFiles))
	}
	if cluster.Spec.Kubernetes.Kubelet.ContainerLogMaxSize != "" {
		containerLogMaxSize = cluster.Spec.Kubernetes.Kubelet.ContainerLogMaxSize
	}
	if cluster.Spec.Kubernetes.Kubelet.EvictionPressureTransitionPeriod != "" {
		evictionPressureTransitionPeriod = cluster.Spec.Kubernetes.Kubelet.EvictionPressureTransitionPeriod
	}
	if cluster.Spec.Kubernetes.Kubelet.EvictionMaxPodGracePeriod != nil {
		evictionMaxPodGracePeriod = string(rune(*cluster.Spec.Kubernetes.Kubelet.EvictionMaxPodGracePeriod))
	}
	evictionHard["memory.available"] = "5%"
	evictionHard["pid.available"] = "10%"
	for key, value := range cluster.Spec.Kubernetes.Kubelet.EvictionHard {
		evictionHard[key] = value
	}

	evictionSoft["memory.available"] = "10%"
	for key, value := range cluster.Spec.Kubernetes.Kubelet.EvictionSoft {
		evictionSoft[key] = value
	}

	evictionSoftGracePeriod["memory.available"] = "2m"
	for key, value := range cluster.Spec.Kubernetes.Kubelet.EvictionSoftGracePeriod {
		evictionSoftGracePeriod[key] = value
	}

	featureGates["CSIStorageCapacity"] = true
	featureGates["ExpandCSIVolumes"] = true
	featureGates["RotateKubeletServerCertificate"] = true
	for key, value := range cluster.Spec.Kubernetes.Kubelet.FeatureGates {
		featureGates[key] = value
	}

	kubeReserved["cpu"] = "200m"
	kubeReserved["memory"] = "250Mi"
	for key, value := range cluster.Spec.Kubernetes.Kubelet.KubeReserved {
		kubeReserved[key] = value
	}

	if cluster.Spec.Kubernetes.Kubelet.MaxPods != nil {
		maxPods = *cluster.Spec.Kubernetes.Kubelet.MaxPods
	}
	if cluster.Spec.Kubernetes.Kubelet.PodPidsLimit != nil {
		podPidsLimit = *cluster.Spec.Kubernetes.Kubelet.PodPidsLimit
	}

	kubeletConfig.ContainerLogMaxSize = containerLogMaxSize
	kubeletConfig.ContainerLogMaxFiles = containerLogMaxFiles

	var clusterName string
	if cluster.Spec.Kubernetes.DNSDomain != "" {
		clusterName = cluster.Spec.Kubernetes.DNSDomain
	} else {
		clusterName = common.DefaultClusterLocal
	}

	var kuberetesVersion string
	if cluster.Spec.Kubernetes.Version != "" {
		kuberetesVersion = cluster.Spec.Kubernetes.Version
	} else {
		kuberetesVersion = common.DefaultK8sVersion
	}

	var cgroupDriver string
	switch cluster.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeDocker:
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Docker.CgroupDriver
	case common.RuntimeTypeContainerd:
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Containerd.CgroupDriver
	case common.RuntimeTypeCRIO:
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Crio.CgroupDriver
	case common.RuntimeTypeIsula:
		cgroupDriver = *cluster.Spec.Kubernetes.ContainerRuntime.Isulad.CgroupDriver
	default:
		cgroupDriver = common.CgroupDriverSystemd
	}

	var ClusterDNS string
	if cluster.Spec.DNS.NodeLocalDNS.Enabled == helpers.BoolPtr(false) {

	} else {
		ClusterDNS = common.DefaultLocalDNS
	}

	data := TemplateData{
		IsExternalEtcd:       isExternalEtcd,
		Etcd:                 etcdConfig,
		DNSImageTag:          corednsImage.Tag(),
		ImageRepository:      fmt.Sprintf("%s/%s", corednsImage.RegistryAddr(), corednsImage.Namespace()),
		KubernetesVersion:    kuberetesVersion,
		CertificatesDir:      common.DefaultKubernetesPKIDir,
		ClusterName:          clusterName,
		ControlPlaneEndpoint: fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Domain, common.DefaultAPIServerPort),
		PodSubnet:            PodSubnet,
		ServiceSubnet:        ServiceSubnet,
		AdvertiseAddress:     currentHost.GetInternalAddress(),
		CRISocket:            DefaultCRISocket,
		CgroupDriver:         cgroupDriver,
		ClusterDNS:           ClusterDNS,

		ApiServerExtraArgs:         ApiServerExtraArgs,
		ControllerManagerExtraArgs: ControllerManagerExtraArgs,
		SchedulerExtraArgs:         SchedulerExtraArgs,
		KubeletExtraArgs:           make(map[string]string),
		KubeProxy: KubeProxyConfig{
			Mode:          common.DefaultKubeProxyMode,
			MasqueradeAll: false,
			MasqueradeBit: 14,
			MinSyncPeriod: "0s",
			SyncPeriod:    "30s",
		},
		Kubelet: KubeletConfig{
			MaxPods:                          110,
			ContainerLogMaxFiles:             5,
			ContainerLogMaxSize:              "10Mi",
			EvictionMaxPodGracePeriod:        120,
			EvictionPressureTransitionPeriod: "30s",
			PodPidsLimit:                     -1,
			RotateCertificates:               true,
			SerializeImagePulls:              true,
			EvictionHard:                     common.DefaultEvictionHard,
			EvictionSoft:                     common.DefaultEvictionSoft,
			EvictionSoftGracePeriod:          common.DefaultEvictionSoftGracePeriod,
			FeatureGates:                     common.DefaultFeatureGates,
			KubeReserved:                     common.DefaultKubeReserved,
			SystemReserved:                   common.DefaultSystemReserved,
		},
	}

	// --- 2. 使用用户在 cluster.spec 中定义的值覆盖默认值 ---
	// a. 基础配置
	data.KubernetesVersion = util.FirstNonEmpty(cluster.Spec.Kubernetes.Version, data.KubernetesVersion)
	data.ClusterName = util.FirstNonEmpty(cluster.Spec.Kubernetes.ClusterName, data.ClusterName)
	data.CertificatesDir = common.KubernetesPKIDir

	data.BindPort = cluster.Spec.ControlPlaneEndpoint.Port
	data.ControlPlaneEndpoint = fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Address, data.BindPort)
	if cluster.Spec.ControlPlaneEndpoint.Domain != "" {
		data.ControlPlaneEndpoint = fmt.Sprintf("%s:%d", cluster.Spec.ControlPlaneEndpoint.Domain, data.BindPort)
	}

	// b. 镜像仓库
	if cluster.Spec.Kubernetes.ImageRepo != "" {
		data.ImageRepository = cluster.Spec.Kubernetes.ImageRepo
	}
	if cluster.Spec.Registry != nil && cluster.Spec.Registry.URL != "" {
		data.ImageRepository = filepath.Join(cluster.Spec.Registry.URL, data.ImageRepository)
	}
	data.DNSImageTag = util.FirstNonEmpty(cluster.Spec.DNS.Image.Tag, data.DNSImageTag)

	// c. ExtraArgs
	data.ApiServerExtraArgs = util.MergeStringMaps(cluster.Spec.Kubernetes.ApiServer.ExtraArgs, data.ApiServerExtraArgs)
	data.ControllerManagerExtraArgs = util.MergeStringMaps(cluster.Spec.Kubernetes.ControllerManager.ExtraArgs, data.ControllerManagerExtraArgs)
	data.SchedulerExtraArgs = util.MergeStringMaps(cluster.Spec.Kubernetes.Scheduler.ExtraArgs, data.SchedulerExtraArgs)

	// --- 3. 严格按照您的逻辑计算 Etcd 和 CertSANS ---

	// a. 计算 EtcdConfig
	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	data.IsExternalEtcd = (cluster.Spec.Etcd.Type == string(common.EtcdDeploymentTypeExternal))

	if data.IsExternalEtcd && cluster.Spec.Etcd.External != nil {
		ext := cluster.Spec.Etcd.External
		data.Etcd.Endpoints = ext.Endpoints
		data.Etcd.CaFile = ext.CAFile
		data.Etcd.CertFile = filepath.Join(ext.CertsPath, fmt.Sprintf("node-%s.pem", currentHost.GetName()))
		data.Etcd.KeyFile = filepath.Join(ext.CertsPath, fmt.Sprintf("node-%s-key.pem", currentHost.GetName()))
	} else if cluster.Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubexm) {
		endpoints := make([]string, len(etcdNodes))
		for i, node := range etcdNodes {
			endpoints[i] = fmt.Sprintf("https://%s:%d", node.GetInternalIPv4Address(), common.EtcdDefaultClientPort)
		}
		data.Etcd.Endpoints = endpoints
		data.Etcd.CaFile = filepath.Join(common.DefaultEtcdPKIDir, common.EtcdCaPemFileName)
		data.Etcd.CertFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, currentHost.GetName()))
		data.Etcd.KeyFile = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, currentHost.GetName()))
	}
	// Kubeadm (stacked) 模式下，etcd配置由kubeadm自行处理，无需在此定义

	// b. 计算 CertSANS
	sans := []string{"kubernetes", "kubernetes.default", "kubernetes.default.svc", "127.0.0.1", "localhost"}
	if data.ClusterName != "" {
		sans = append(sans, fmt.Sprintf("kubernetes.default.svc.%s", data.ClusterName))
	}
	for _, node := range cluster.Spec.Hosts {
		if node.Name != "" {
			sans = append(sans, node.Name)
			if data.ClusterName != "" {
				sans = append(sans, fmt.Sprintf("%s.%s", node.Name, data.ClusterName))
			}
		}
		if ip := node.GetInternalIPv4Address(); ip != "" {
			sans = append(sans, ip)
		}
		if ip := node.GetInternalIPv6Address(); ip != "" {
			sans = append(sans, ip)
		}
		if addr := node.GetInternalAddress(); addr != "" {
			sans = append(sans, addr)
		}
	}
	data.CertSANS = util.UniqueStringSlice(sans)

	// --- 4. 渲染模板 ---
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

// Precheck, Run, Rollback 方法调用 renderContent，无需修改。
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

	if err := runner.WriteFile(ctx.GoContext(), conn, renderedConfig, remoteConfigPath, "0644", false); err != nil {
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
