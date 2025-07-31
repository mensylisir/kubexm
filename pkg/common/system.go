package common

// System configuration constants

const (
	DefaultRemoteWorkDirTarget  = "/tmp/kubexms_work"
	DefaultTmpDirNameLocal      = ".kubexm_tmp"
	DefaultLocalRegistryDataDir = "/var/lib/registry"

	DefaultHelmHome                 = "/root/.helm"
	DefaultHelmCache                = "/root/.cache/helm"
	SysctlDefaultConfFileTarget     = "/etc/sysctl.conf"
	ModulesLoadDefaultDirTarget     = "/etc/modules-load.d"
	KubernetesSysctlConfFileTarget  = "/etc/sysctl.d/99-kubexm.conf"
	ModulesLoadDefaultFileTarget    = "/etc/modules-load.d/99-kubexm.conf"
	SecuriryLimitsDefaultFileTarget = "/etc/security/limits.conf"
	DefaultSystemdPath              = "/etc/systemd/system"
	DefaultSystemdDropinPath        = "/etc/systemd/system/%s.service.d"
	DefaultBinPath                  = "/usr/local/bin"
	DefaultConfigPath               = "/etc/kubexm"
	DefaultLogPath                  = "/var/log/kubexm"
	SecuriryLimitsDefaultFile       = "/etc/security/limits.d/99-kubexm.conf"
)

const (
	DefaultSecretFilePermission  = 0600
	DefaultSecretDirPermission   = 0700
	DefaultExecutablePermission  = 0755
	DefaultConfigFilePermission  = 0644
	DefaultCertificatePermission = 0644

	DefaultKubeletUser    = "root"
	DefaultEtcdUser       = "root"
	DefaultContainerdUser = "root"
	DefaultDockerUser     = "root"
	DefaultKubeUser       = "root"
	DefaultSystemUser     = "root"

	DefaultReadTimeout         = 30
	DefaultWriteTimeout        = 30
	DefaultRetryCount          = 3
	DefaultRetryInterval       = 5
	DefaultHealthCheckInterval = 30
	DefaultBackupInterval      = 24
)

const (
	DefaultMinDiskGB        = 20
	DefaultMaxPods          = 110
	DefaultNodeCidrMaskSize = 24
	DefaultMaxPodPidsLimit  = 4096
	DefaultMaxOpenFiles     = 1000000
	DefaultMaxMapCount      = 262144
	DefaultMaxUserInstances = 8192

	DefaultMaxNodes      = 5000
	DefaultMaxNamespaces = 1000
	DefaultMaxServices   = 5000
	DefaultMaxIngresses  = 1000
	DefaultMaxPVs        = 1000
	DefaultMaxPVCs       = 1000
)

const (
	DefaultArch = "amd64"
	ArchAMD64   = "amd64"
	ArchARM64   = "arm64"
	ArchPPC64LE = "ppc64le"
	ArchS390X   = "s390x"
	ArchX8664   = "x86_64"
	ArchAarch64 = "aarch64"

	DefaultOS = "linux"
	OSLinux   = "linux"
	OSDarwin  = "darwin"
	OSWindows = "windows"

	DistroUbuntu      = "ubuntu"
	DistroCentOS      = "centos"
	DistroRHEL        = "rhel"
	DistroDebian      = "debian"
	DistroFedora      = "fedora"
	DistroRocky       = "rocky"
	DistroAlmalinux   = "almalinux"
	DistroSUSE        = "suse"
	DistroOracle      = "oracle"
	DistroPhoton      = "photon"
	DistroFlatcar     = "flatcar"
	DistroAmazonLinux = "amzn"
	DistroCoreos      = "coreos"
	DistroKylin       = "kylin"
	DistroUOS         = "uos"
)

const (
	DefaultSELinuxMode    = "permissive"
	PermissiveSELinuxMode = "permissive"
	EnforceSELinuxMode    = "enforcing"
	DisabledSELinuxMode   = "disabled"
	DefaultIPTablesMode   = "legacy"
	LegacyIPTablesMode    = "legacy"
	NftIPTablesMode       = "nft"
)

const (
	KernelModuleBrNetfilter = "br_netfilter"
	KernelModuleIpvs        = "ip_vs"
)

var (
	ValidContainerRuntimeTypes = []ContainerRuntimeType{
		RuntimeTypeContainerd,
		RuntimeTypeDocker,
		RuntimeTypeCRIO,
		RuntimeTypeIsula,
	}
	ValidUpstreamPolicies = []string{
		UpstreamForwardingConfigRandom,
		UpstreamForwardingConfigRoundRobin,
		UpstreamForwardingConfigSequential,
	}
	SupportedChecks                     = []string{"cpu", "memory", "swap", "firewalld", "selinux"}
	ValidPMs                            = []string{"yum", "dnf", "apt"}
	ValidRegistryTypes                  = []string{RegistryTypeHarbor, RegistryTypeDockerRegistry, RegistryTypeRegistry}
	ValidEtcdMetricsLevels              = []string{"basic", "extensive"}
	ValidEtcdLogLevels                  = []string{"debug", "info", "warn", "error", "panic", "fatal"}
	ValidKubeProxyModes                 = []string{KubeProxyModeIPTables, KubeProxyModeIPVS}
	ValidCgroupDrivers                  = []string{CgroupDriverSystemd, CgroupDriverCgroupfs}
	ValidHybridnetNetworkTypes          = []string{HybridnetDefaultNetworkConfigTypeUnderlay, HybridnetDefaultNetworkConfigTypeOverlay}
	ValidKubeOvnTunnelTypes             = []string{KubeOvnNetworkingTunnelTypeGeneve, KubeOvnNetworkingTunnelVXLAN}
	ValidFlannelBackendTypes            = []string{FlannelBackendConfigTypeVxlan, FlannelBackendConfigTypeHostGw, FlannelBackendConfigTypeUdp, FlannelBackendConfigTypeIpsec}
	ValidCalicoEncapsulationModes       = []string{CalicoIPIPModeAlways, CalicoIPIPModeCrossSubnet, CalicoIPIPModeNever}
	ValidCalicoIPPoolEncapsulationModes = []string{CalicoIPPoolEncapsulationIPIP, CalicoIPPoolEncapsulationIPIPCrossSubnet, CalicoIPPoolEncapsulationVXLAN, CalicoIPPoolEncapsulationVXLANCrossSubnet, CalicoIPPoolEncapsulationNone}
	ValidCalicoLogSeverities            = []string{CalicoFelixConfigurationLogSeverityScreenDebug, CalicoFelixConfigurationLogSeverityScreenInfo, CalicoFelixConfigurationLogSeverityScreenWarning, CalicoFelixConfigurationLogSeverityScreenError, CalicoFelixConfigurationLogSeverityScreenFatal}
	ValidCiliumTunnelModes              = []string{CiliumTunnelVxlanModes, CiliumTunnelGeneveModes, CiliumTunnelDisabledModes}
	ValidCiliumIPAMModes                = []string{CiliumIPAMClusterPoolsMode, CiliumIPAMKubernetesMode}
	ValidCiliumKPRModes                 = []string{CiliumKPRProbeModes, CiliumKPRStrictModes, CiliumKPRDisabledModes}
	ValidCiliumIdentModes               = []string{CiliumIdentCrdModes, CiliumIdentKvstoreModes}
	ValidInternalLoadbalancerTypes      = []string{string(InternalLBTypeHAProxy), string(InternalLBTypeNginx)}
	ValidKubeVIPModes                   = []string{KubeVIPModeARP, KubeVIPModeBGP}
	ValidNginxLBModes                   = []string{NginxLBTCPModes, NginxLBHTTPModes}
	ValidNginxLBAlgorithms              = []string{NginxLBRoundRobin, NginxLBLeastConn, NginxLBIPHash, NginxLBHash, NginxLBRandom, NginxLBLeastTime}
	ValidHAProxyBalanceAlgorithms       = []string{HAProxyRoundrobin, HAProxyStaticRR, HAProxyLeastconn, HAProxyFirst, HAProxySource, HAProxyURI, HAProxyUrlParam, HAProxyHdr, HAProxyRdpCookie}
	ValidHAProxyModes                   = []string{HAProxyModeTCP, HAProxyModeHTTP}
	ValidVRRPStates                     = []string{DefaultKeepaliveMaster, DefaultKeepaliveBackup}
	ValidLVSAlgos                       = []string{DefaultKeepalivedLVSRRcheduler, DefaultKeepalivedLVSWRRcheduler, DefaultKeepalivedLVSLCcheduler, DefaultKeepalivedLVSWLCcheduler, DefaultKeepalivedLVSLBLCcheduler, DefaultKeepalivedLVSSHcheduler, DefaultKeepalivedLVSDHcheduler}
	ValidLVSKinds                       = []string{DefaultKeepalivedNAT, DefaultKeepalivedDR, DefaultKeepalivedTUN}
	ValidProtocols                      = []string{DefaultKeepalivedTCPProtocol, DefaultKeepalivedUDPProtocol}
	ValidKeepalivedAuthTypes            = []string{KeepalivedAuthTypePASS, KeepalivedAuthTypeAH}
	ValidSELinuxModes                   = []string{PermissiveSELinuxMode, EnforceSELinuxMode, DisabledSELinuxMode, ""}
	ValidIPTablesModes                  = []string{LegacyIPTablesMode, NftIPTablesMode, ""}
	SupportedArches                     = []string{ArchAMD64, ArchARM64}
	SupportedArchitectures              = SupportedArches
	SupportedOperatingSystems           = []string{OSLinux, OSDarwin, OSWindows}
	SupportedLinuxDistributions         = []string{
		DistroUbuntu, DistroDebian, DistroCentOS, DistroRHEL, DistroRocky, DistroAlmalinux, DistroFedora,
		DistroSUSE, DistroAmazonLinux, DistroOracle, DistroPhoton, DistroFlatcar, DistroCoreos, DistroKylin, DistroUOS,
	}

	SupportedContainerRuntimes = []string{
		string(RuntimeTypeDocker),
		string(RuntimeTypeContainerd),
		string(RuntimeTypeCRIO),
		string(RuntimeTypeIsula),
	}

	SupportedCNIPlugins = []string{
		string(CNITypeCalico),
		string(CNITypeFlannel),
		string(CNITypeCilium),
		string(CNITypeKubeOvn),
		string(CNITypeHybridnet),
	}

	SupportedCNITypes = []string{
		string(CNITypeCalico),
		string(CNITypeFlannel),
		string(CNITypeCilium),
		string(CNITypeKubeOvn),
		string(CNITypeMultus),
		string(CNITypeHybridnet),
	}

	SupportedInternalLoadBalancerTypes = []string{
		string(InternalLBTypeHAProxy),
		string(InternalLBTypeNginx),
	}

	SupportedExternalLoadBalancerTypes = []string{
		string(ExternalLBTypeKubeVIP),
		string(ExternalLBTypeKubexmKH),
		string(ExternalLBTypeKubexmKN),
		string(ExternalLBTypeExternal),
		string(ExternalLBTypeNone),
	}

	SupportedKubernetesDeploymentTypes = []string{
		string(KubernetesDeploymentTypeKubeadm),
		string(KubernetesDeploymentTypeKubexm),
	}

	SupportedEtcdDeploymentTypes = []string{
		string(EtcdDeploymentTypeKubeadm),
		string(EtcdDeploymentTypeKubexm),
		string(EtcdDeploymentTypeExternal),
	}

	SupportedStorageTypes = []string{
		StorageComponentOpenEBS,
		StorageComponentNFS,
		StorageComponentRookCeph,
		StorageComponentLonghorn,
	}

	HostUnitMap = map[string]string{
		"K": "Ki",
		"M": "Mi",
		"G": "Gi",
		"T": "Ti",
		"P": "Pi",
	}

	SupportedAptDistributions = []string{
		DistroUbuntu,
		DistroDebian,
	}

	SupportedYumDnfDistributions = []string{
		DistroCentOS,
		DistroRHEL,
		DistroRocky,
		DistroAlmalinux,
		DistroFedora,
		DistroAmazonLinux,
		DistroOracle,
		DistroKylin,
		DistroUOS,
	}

	SupportedZypperDistributions = []string{
		DistroSUSE,
	}

	SupportedTdnfDistributions = []string{
		DistroPhoton,
	}

	SupportedSELinuxDistributions = []string{
		DistroCentOS,
		DistroRHEL,
		DistroRocky,
		DistroAlmalinux,
		DistroFedora,
		DistroAmazonLinux,
		DistroOracle,
		DistroCoreos,
		DistroKylin,
		DistroUOS,
	}

	SupportedAppArmorDistributions = []string{
		DistroUbuntu,
		DistroDebian,
		DistroSUSE,
	}

	RedHatFamilyDistributions = []string{
		DistroRHEL,
		DistroCentOS,
		DistroFedora,
		DistroRocky,
		DistroAlmalinux,
		DistroOracle,
		DistroAmazonLinux,
		DistroKylin,
		DistroUOS,
	}

	DebianFamilyDistributions = []string{
		DistroDebian,
		DistroUbuntu,
	}

	ContainerOptimizedDistributions = []string{
		DistroFlatcar,
		DistroCoreos,
		DistroPhoton,
	}
)
