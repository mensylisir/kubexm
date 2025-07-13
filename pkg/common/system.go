package common

// System configuration constants

const (
	DefaultRemoteWorkDirTarget  = "/tmp/kubexms_work"
	DefaultTmpDirNameLocal      = ".kubexm_tmp"
	DefaultLocalRegistryDataDir = "/var/lib/registry"

	DefaultHelmHome                = "/root/.helm"
	DefaultHelmCache               = "/root/.cache/helm"
	SysctlDefaultConfFileTarget    = "/etc/sysctl.conf"
	ModulesLoadDefaultDirTarget    = "/etc/modules-load.d"
	KubernetesSysctlConfFileTarget = "/etc/sysctl.d/99-kubexm.conf"
	ModulesLoadDefaultFileTarget   = "/etc/modules-load.d/99-kubexm.conf"
	DefaultSystemdPath             = "/etc/systemd/system"
	DefaultSystemdDropinPath       = "/etc/systemd/system/%s.service.d"
	DefaultBinPath                 = "/usr/local/bin"
	DefaultConfigPath              = "/etc/kubexm"
	DefaultLogPath                 = "/var/log/kubexm"
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
	ValidInternalLoadbalancerTypes = []string{string(InternalLBTypeKubeVIP), string(InternalLBTypeHAProxy), string(InternalLBTypeNginx)}
	ValidKubeVIPModes              = []string{KubeVIPModeARP, KubeVIPModeBGP}
	ValidNginxLBModes              = []string{NginxLBTCPModes, NginxLBHTTPModes}
	ValidNginxLBAlgorithms         = []string{NginxLBRoundRobin, NginxLBLeastConn, NginxLBIPHash, NginxLBHash, NginxLBRandom, NginxLBLeastTime}
	ValidHAProxyBalanceAlgorithms  = []string{HAProxyRoundrobin, HAProxyStaticRR, HAProxyLeastconn, HAProxyFirst, HAProxySource, HAProxyURI, HAProxyUrlParam, HAProxyHdr, HAProxyRdpCookie}
	ValidHAProxyModes              = []string{HAProxyModeTCP, HAProxyModeHTTP}
	ValidVRRPStates                = []string{DefaultKeepaliveMaster, DefaultKeepaliveBackup}
	ValidLVSAlgos                  = []string{DefaultKeepalivedLVSRRcheduler, DefaultKeepalivedLVSWRRcheduler, DefaultKeepalivedLVSLCcheduler, DefaultKeepalivedLVSWLCcheduler, DefaultKeepalivedLVSLBLCcheduler, DefaultKeepalivedLVSSHcheduler, DefaultKeepalivedLVSDHcheduler}
	ValidLVSKinds                  = []string{DefaultKeepalivedNAT, DefaultKeepalivedDR, DefaultKeepalivedTUN}
	ValidProtocols                 = []string{DefaultKeepalivedTCPProtocol, DefaultKeepalivedUDPProtocol}
	ValidKeepalivedAuthTypes       = []string{KeepalivedAuthTypePASS, KeepalivedAuthTypeAH}
	ValidSELinuxModes              = []string{PermissiveSELinuxMode, EnforceSELinuxMode, DisabledSELinuxMode, ""}
	ValidIPTablesModes             = []string{LegacyIPTablesMode, NftIPTablesMode, ""}
	SupportedArches                = []string{ArchAMD64, ArchARM64}
	SupportedArchitectures         = SupportedArches
	SupportedOperatingSystems      = []string{OSLinux, OSDarwin, OSWindows}
	SupportedLinuxDistributions    = []string{
		DistroUbuntu, DistroDebian, DistroCentOS, DistroRHEL, DistroRocky, DistroAlmalinux, DistroFedora,
		DistroSUSE, DistroAmazonLinux, DistroOracle, DistroPhoton, DistroFlatcar, DistroCoreos, DistroKylin, DistroUOS,
	}

	SupportedContainerRuntimes = []string{
		string(RuntimeTypeDocker),
		string(RuntimeTypeContainerd),
		string(RuntimeTypeCRIO),
		string(RuntimeTypeIsula),
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
		string(InternalLBTypeKubeVIP),
		string(InternalLBTypeHAProxy),
		string(InternalLBTypeNginx),
	}

	SupportedExternalLoadBalancerTypes = []string{
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
)
