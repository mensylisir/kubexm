package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net"
	"path"
	"regexp"
	"strings"
	"time"
)

type Cluster struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec              *ClusterSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

type ClusterSpec struct {
	Hosts      []HostSpec      `json:"hosts" yaml:"hosts"`
	RoleGroups *RoleGroupsSpec `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	Global     *GlobalSpec     `json:"global,omitempty" yaml:"global,omitempty"`
	System     *SystemSpec     `json:"system,omitempty" yaml:"system,omitempty"`
	Kubernetes *Kubernetes     `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Etcd       *Etcd           `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	DNS        *DNS            `json:"dns,omitempty" yaml:"dns,omitempty"`

	Network              *Network                  `json:"network,omitempty" yaml:"network,omitempty"`
	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`

	Storage   *Storage   `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry  *Registry  `json:"registry,omitempty" yaml:"registry,omitempty"`
	HelmRepo  *HelmRepo  `json:"helmRepo,omitempty" yaml:"helmRepo,omitempty"`
	Addons    []Addon    `json:"addons,omitempty" yaml:"addons,omitempty"`
	Preflight *Preflight `json:"preflight,omitempty" yaml:"preflight,omitempty"`
	Extra     *Extra     `json:"extra,omitempty" yaml:"extra,omitempty"`
	Certs     *CertSpec  `json:"certs,omitempty" yaml:"certs,omitempty"`
}

type CertSpec struct {
	CADuration   string `json:"CADuration,omitempty" yaml:"CADuration,omitempty"`
	CertDuration string `json:"CertDuration,omitempty" yaml:"CertDuration,omitempty"`
}

type HostSpec struct {
	Name            string            `json:"name" yaml:"name"`
	Address         string            `json:"address" yaml:"address"`
	InternalAddress string            `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	Port            int               `json:"port,omitempty" yaml:"port,omitempty"`
	User            string            `json:"user,omitempty" yaml:"user,omitempty"`
	Password        string            `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey      string            `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	PrivateKeyPath  string            `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Taints          []TaintSpec       `json:"taints,omitempty" yaml:"taints,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	Arch            string            `json:"arch,omitempty" yaml:"arch,omitempty"`
	Timeout         int64             `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Roles           []string          `json:"-"`
	RoleTable       map[string]bool   `json:"-"`
	Cache           *cache.StepCache  `json:"-"`
}

type RoleGroupsSpec struct {
	Master       []string `json:"master,omitempty" yaml:"master,omitempty"`
	Worker       []string `json:"worker,omitempty" yaml:"worker,omitempty"`
	Etcd         []string `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	LoadBalancer []string `json:"loadbalancer,omitempty" yaml:"loadbalancer,omitempty"`
	Storage      []string `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry     []string `json:"registry,omitempty" yaml:"registry,omitempty"`
}

type MasterRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

type WorkerRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

type EtcdRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

type LoadBalancerRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

type StorageRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

type RegistryRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

type CustomRoleSpec struct {
	Name  string   `json:"name" yaml:"name"`
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

type SystemSpec struct {
	NTPServers         []string          `json:"ntpServers,omitempty" yaml:"ntpServers,omitempty"`
	Timezone           string            `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	RPMs               []string          `json:"rpms,omitempty" yaml:"rpms,omitempty"`
	Debs               []string          `json:"debs,omitempty" yaml:"debs,omitempty"`
	PackageManager     string            `json:"packageManager,omitempty" yaml:"packageManager,omitempty"`
	PreInstallScripts  []string          `json:"preInstallScripts,omitempty" yaml:"preInstall,omitempty"`
	PostInstallScripts []string          `json:"postInstallScripts,omitempty" yaml:"postInstall,omitempty"`
	SkipConfigureOS    bool              `json:"skipConfigureOS,omitempty" yaml:"skipConfigureOS,omitempty"`
	Modules            []string          `json:"modules,omitempty" yaml:"modules,omitempty"`
	SysctlParams       map[string]string `json:"sysctlParams,omitempty" yaml:"sysctlParams,omitempty"`
}

type GlobalSpec struct {
	User              string        `json:"user,omitempty" yaml:"user,omitempty"`
	Port              int           `json:"port,omitempty" yaml:"port,omitempty"`
	Password          string        `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey        string        `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	PrivateKeyPath    string        `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	ConnectionTimeout time.Duration `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	WorkDir           string        `json:"workDir,omitempty" yaml:"workDir,omitempty"`
	HostWorkDir       string        `json:"hostWorkDir,omitempty" yaml:"hostWorkDir,omitempty"`
	Verbose           bool          `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	IgnoreErr         bool          `json:"ignoreErr,omitempty" yaml:"ignoreErr,omitempty"`
	SkipPreflight     bool          `json:"skipPreflight,omitempty" yaml:"skipPreflight,omitempty"`
}

type TaintSpec struct {
	Key    string `json:"key" yaml:"key"`
	Value  string `json:"value" yaml:"value"`
	Effect string `json:"effect" yaml:"effect"`
}

func SetDefaults_Cluster(obj *Cluster) {
	if obj.APIVersion == "" {
		obj.APIVersion = common.DefaultAPIVersion
	}
	if obj.Kind == "" {
		obj.Kind = common.DefaultKind
	}
	SetDefaults_ClusterSpec(obj)
}

func SetDefaults_ClusterSpec(cluster *Cluster) {
	if cluster.Spec == nil {
		return
	}

	if cluster.Spec.Global == nil {
		cluster.Spec.Global = &GlobalSpec{}
	}
	SetDefaults_GlobalSpec(cluster.Spec.Global)

	for i := range cluster.Spec.Hosts {
		SetDefaults_HostSpec(&cluster.Spec.Hosts[i], cluster)
	}

	if cluster.Spec.System == nil {
		cluster.Spec.System = &SystemSpec{}
	}
	SetDefaults_SystemSpec(cluster.Spec.System)

	if cluster.Spec.Kubernetes == nil {
		cluster.Spec.Kubernetes = &Kubernetes{}
	}
	SetDefaults_Kubernetes(cluster.Spec.Kubernetes)

	if cluster.Spec.Network == nil {
		cluster.Spec.Network = &Network{}
	}
	SetDefaults_Network(cluster.Spec.Network)

	if cluster.Spec.ControlPlaneEndpoint == nil {
		cluster.Spec.ControlPlaneEndpoint = &ControlPlaneEndpointSpec{}
	}
	SetDefaults_ControlPlaneEndpointSpec(cluster.Spec.ControlPlaneEndpoint)

	if cluster.Spec.Storage == nil {
		cluster.Spec.Storage = &Storage{}
	}
	SetDefaults_Storage(cluster.Spec.Storage)

	if cluster.Spec.Registry == nil {
		cluster.Spec.Registry = &Registry{}
	}
	SetDefaults_Registry(cluster.Spec.Registry)

	if cluster.Spec.Etcd == nil {
		cluster.Spec.Etcd = &Etcd{}
	}
	SetDefaults_Etcd(cluster.Spec.Etcd)
	if cluster.Spec.DNS == nil {
		cluster.Spec.DNS = &DNS{}
	}
	SetDefaults_DNS(cluster.Spec.DNS)
	if cluster.Spec.Preflight == nil {
		cluster.Spec.Preflight = &Preflight{}
	}
	SetDefaults_Preflight(cluster.Spec.Preflight)
}

func SetDefaults_GlobalSpec(spec *GlobalSpec) {
	if spec.User == "" {
		spec.User = common.DefaultUser
	}
	if spec.Port == 0 {
		spec.Port = common.DefaultPort
	}
	if spec.ConnectionTimeout == 0 {
		spec.ConnectionTimeout = common.DefaultTimeout
	}
	if spec.WorkDir == "" {
		workDir, _ := helpers.GenerateWorkDir()
		spec.WorkDir = workDir
	}
}

func SetDefaults_HostSpec(spec *HostSpec, cluster *Cluster) {
	if spec.User == "" {
		spec.User = cluster.Spec.Global.User
	}
	if spec.Port == 0 {
		spec.Port = cluster.Spec.Global.Port
	}
	if spec.Password == "" && cluster.Spec.Global.Password != "" {
		spec.Password = cluster.Spec.Global.Password
	}
	if spec.PrivateKey == "" && cluster.Spec.Global.PrivateKey != "" {
		spec.PrivateKey = cluster.Spec.Global.PrivateKey
	}
	if spec.PrivateKeyPath == "" && cluster.Spec.Global.PrivateKeyPath != "" {
		spec.PrivateKeyPath = cluster.Spec.Global.PrivateKeyPath
	}
	if spec.Arch == "" {
		spec.Arch = common.ArchAMD64
	}
	_, _ = helpers.GenerateHostWorkDir(cluster.ObjectMeta.Name, cluster.Spec.Global.WorkDir, spec.Name)
}

func SetDefaults_SystemSpec(spec *SystemSpec) {
	if spec.Timezone == "" {
		spec.Timezone = "UTC"
	}
}

func Validate_Cluster(obj *Cluster, verrs *validation.ValidationErrors) {
	if obj.APIVersion != common.DefaultAPIVersion {
		verrs.Add(fmt.Sprintf("apiVersion: must be '%s'", common.DefaultAPIVersion))
	}
	if obj.Kind != common.DefaultKind {
		verrs.Add("kind: must be %s", common.DefaultKind)
	}
	if obj.Name == "" {
		verrs.Add("metadata.name: cannot be empty")
	}
	Validate_ClusterSpec(obj.Spec, verrs, "spec")
}

func Validate_ClusterSpec(spec *ClusterSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if spec == nil {
		verrs.Add(pathPrefix + ": spec section cannot be nil")
		return
	}
	p := path.Join(pathPrefix)

	if len(spec.Hosts) == 0 {
		verrs.Add(p + ".hosts: at least one host must be defined")
	}
	hostNames := make(map[string]bool)
	hostAddresses := make(map[string]bool)
	for i, host := range spec.Hosts {
		hostPath := fmt.Sprintf("%s.hosts[%d]", p, i)
		Validate_HostSpec(&host, verrs, hostPath)
		if host.Name != "" {
			if hostNames[host.Name] {
				verrs.Add(fmt.Sprintf("%s.name: duplicate host name '%s'", hostPath, host.Name))
			}
			hostNames[host.Name] = true
		}
		if host.Address != "" {
			if hostAddresses[host.Address] {
				verrs.Add(fmt.Sprintf("%s.address: duplicate host address '%s'", hostPath, host.Address))
			}
			hostAddresses[host.Address] = true
		}
	}

	if spec.RoleGroups == nil {
		verrs.Add(p + ".roleGroups: is a required section")
	} else {
		allHostsInRoles := make(map[string]bool)
		roleGroupsPath := path.Join(p, "roleGroups")
		validateRoleGroup := func(roleHosts []string, roleName string) {
			for _, hostName := range roleHosts {
				if !hostNames[hostName] {
					verrs.Add(fmt.Sprintf("%s.%s: host '%s' is not defined in the hosts list", roleGroupsPath, roleName, hostName))
				}
				allHostsInRoles[hostName] = true
			}
		}
		validateRoleGroup(spec.RoleGroups.Master, "master")
		validateRoleGroup(spec.RoleGroups.Worker, "worker")
		validateRoleGroup(spec.RoleGroups.Etcd, "etcd")
		validateRoleGroup(spec.RoleGroups.Registry, "registry")
		validateRoleGroup(spec.RoleGroups.LoadBalancer, "loadbalancer")
		validateRoleGroup(spec.RoleGroups.Storage, "storage")
		for hostName := range hostNames {
			if !allHostsInRoles[hostName] {
				verrs.Add(fmt.Sprintf("%s: host '%s' is defined but not assigned to any role in roleGroups", p, hostName))
			}
		}

		if len(spec.RoleGroups.Master) == 0 {
			verrs.Add(roleGroupsPath + ".master: at least one master host is required")
		}
	}

	if spec.Global != nil {
		Validate_GlobalSpec(spec.Global, verrs, path.Join(p, "global"))
	}

	if spec.Kubernetes != nil {
		Validate_Kubernetes(spec.Kubernetes, verrs, path.Join(p, "kubernetes"))
	} else {
		verrs.Add(p + ".kubernetes: is a required section")
	}

	if spec.Etcd != nil {
		Validate_Etcd(spec.Etcd, verrs, path.Join(p, "etcd"))
	} else {
		verrs.Add(p + ".etcd: is a required section")
	}

	if spec.Network != nil {
		Validate_Network(spec.Network, verrs, path.Join(p, "network"))
	} else {
		verrs.Add(p + ".network: is a required section")
	}

	if spec.ControlPlaneEndpoint != nil {
		Validate_ControlPlaneEndpointSpec(spec.ControlPlaneEndpoint, verrs, path.Join(p, "controlPlaneEndpoint"))
	} else {
		if spec.RoleGroups != nil && len(spec.RoleGroups.Master) > 1 {
			verrs.Add(p + ".controlPlaneEndpoint: is a required section for multi-master setup")
		}
	}

	if spec.System != nil {
		Validate_SystemSpec(spec.System, verrs, path.Join(p, "system"))
	}

	if spec.DNS != nil {
		Validate_DNS(spec.DNS, verrs, path.Join(p, "dns"))
	}

	if spec.Storage != nil {
		Validate_Storage(spec.Storage, verrs, path.Join(p, "storage"))
	}

	if spec.Registry != nil {
		Validate_Registry(spec.Registry, verrs, path.Join(p, "registry"))
	}

	for i, addon := range spec.Addons {
		Validate_Addon(&addon, verrs, fmt.Sprintf("%s.addons[%d]", p, i))
	}

	if spec.Preflight != nil {
		Validate_Preflight(spec.Preflight, verrs, path.Join(p, "preflight"))
	}

	if spec.Extra != nil {
		Validate_Extra(spec.Extra, verrs, path.Join(p, "extra"))
	}
}

func Validate_HostSpec(spec *HostSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if spec.Name == "" {
		verrs.Add(pathPrefix + ".name: is a required field")
	}
	if spec.Address == "" {
		verrs.Add(pathPrefix + ".address: is a required field")
	} else if net.ParseIP(spec.Address) == nil {
		verrs.Add(fmt.Sprintf("%s.address: invalid IP address format for '%s'", pathPrefix, spec.Address))
	}

	authMethods := 0
	if spec.Password != "" {
		authMethods++
	}
	if spec.PrivateKey != "" {
		authMethods++
	}
	if spec.PrivateKeyPath != "" {
		authMethods++
	}
	if authMethods > 1 {
		verrs.Add(pathPrefix + ": only one of password, privateKey, or privateKeyPath can be set")
	}
	if authMethods == 0 {
		verrs.Add(pathPrefix + ": one of password, privateKey, or privateKeyPath is required for SSH authentication")
	}

	for i, taint := range spec.Taints {
		Validate_TaintSpec(&taint, verrs, fmt.Sprintf("%s.taints[%d]", pathPrefix, i))
	}
}

func Validate_TaintSpec(spec *TaintSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if spec.Key == "" {
		verrs.Add(pathPrefix + ".key: cannot be empty")
	}
	validEffects := []string{"NoSchedule", "PreferNoSchedule", "NoExecute"}
	if !helpers.ContainsString(validEffects, spec.Effect) {
		verrs.Add(fmt.Sprintf("%s.effect: invalid effect '%s', must be one of [%s]",
			pathPrefix, spec.Effect, strings.Join(validEffects, ", ")))
	}
}

func Validate_GlobalSpec(spec *GlobalSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if spec == nil {
		return
	}
	p := path.Join(pathPrefix)

	if strings.TrimSpace(spec.User) == "" {
		verrs.Add(p + ".user: cannot be empty")
	}

	if spec.Port <= 0 || spec.Port > 65535 {
		verrs.Add(fmt.Sprintf("%s.port: invalid port %d, must be between 1-65535", p, spec.Port))
	}

	authMethods := 0
	if spec.Password != "" {
		authMethods++
	}
	if spec.PrivateKey != "" {
		authMethods++
	}
	if spec.PrivateKeyPath != "" {
		authMethods++
	}
	if authMethods > 1 {
		verrs.Add(p + ": only one of password, privateKey, or privateKeyPath can be set at the global level")
	}

	if spec.ConnectionTimeout <= 0 {
		verrs.Add(fmt.Sprintf("%s.connectionTimeout: must be a positive duration, got %v", p, spec.ConnectionTimeout))
	}

	if strings.TrimSpace(spec.WorkDir) == "" {
		verrs.Add(p + ".workDir: cannot be empty")
	} else if !path.IsAbs(spec.WorkDir) {
		verrs.Add(fmt.Sprintf("%s.workDir: must be an absolute path, got '%s'", p, spec.WorkDir))
	}
}

func Validate_SystemSpec(spec *SystemSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if spec == nil {
		return
	}
	p := path.Join(pathPrefix)
	for i, server := range spec.NTPServers {
		if strings.TrimSpace(server) == "" {
			verrs.Add(fmt.Sprintf("%s.ntpServers[%d]: cannot be empty", p, i))
		} else if !helpers.IsValidHostPort(server) && !helpers.IsValidDomainName(server) {
			verrs.Add(fmt.Sprintf("%s.ntpServers[%d]: invalid format for '%s', must be a valid IP or hostname", p, i, server))
		}
	}

	if spec.Timezone != "" {
		if strings.TrimSpace(spec.Timezone) == "" {
			verrs.Add(p + ".timezone: cannot be only whitespace if specified")
		}
	}

	hasRPMs := len(spec.RPMs) > 0
	hasDebs := len(spec.Debs) > 0

	if spec.PackageManager != "" {
		if !helpers.ContainsString(common.ValidPMs, spec.PackageManager) {
			verrs.Add(fmt.Sprintf("%s.packageManager: invalid manager '%s', must be one of %v", p, spec.PackageManager, common.ValidPMs))
		}

		if (spec.PackageManager == "yum" || spec.PackageManager == "dnf") && hasDebs {
			verrs.Add(fmt.Sprintf("%s: debs list cannot be used with packageManager '%s'", p, spec.PackageManager))
		}
		if spec.PackageManager == "apt" && hasRPMs {
			verrs.Add(fmt.Sprintf("%s: rpms list cannot be used with packageManager 'apt'", p))
		}
	} else {
		if hasRPMs && hasDebs {
			verrs.Add(p + ": cannot specify both rpms and debs lists without a specific packageManager")
		}
	}

	sysctlKeyRegex := regexp.MustCompile(`^[a-z0-9\._-]+$`)
	for key, value := range spec.SysctlParams {
		if !sysctlKeyRegex.MatchString(key) {
			verrs.Add(fmt.Sprintf("%s.sysctlParams: invalid key format for '%s'", p, key))
		}
		if strings.TrimSpace(value) == "" {
			verrs.Add(fmt.Sprintf("%s.sysctlParams['%s']: value cannot be empty", p, key))
		}
	}
}
