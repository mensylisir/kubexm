package os

import (
	"fmt"
	"sort"
	"strings"
)

// OSComponent represents a component that has OS-specific dependencies
type OSComponent string

const (
	ComponentKubernetes  OSComponent = "kubernetes"
	ComponentEtcd        OSComponent = "etcd"
	ComponentDocker      OSComponent = "docker"
	ComponentContainerd  OSComponent = "containerd"
	ComponentRunc        OSComponent = "runc"
	ComponentCriDockerd  OSComponent = "cri-dockerd"
	ComponentCrio        OSComponent = "crio"
	ComponentCalico      OSComponent = "calico"
	ComponentCilium      OSComponent = "cilium"
	ComponentFlannel     OSComponent = "flannel"
	ComponentKubeOVN     OSComponent = "kubeovn"
	ComponentHybridnet   OSComponent = "hybridnet"
	ComponentMultus      OSComponent = "multus"
	ComponentHelm        OSComponent = "helm"
	ComponentOpenEBS     OSComponent = "openebs"
	ComponentLonghorn    OSComponent = "longhorn"
	ComponentNFS         OSComponent = "nfs"
	ComponentPodman      OSComponent = "podman"
	ComponentKeepalived  OSComponent = "keepalived"
	ComponentHAProxy     OSComponent = "haproxy"
)

// RuntimeType represents the container runtime type
type RuntimeType string

const (
	RuntimeDocker   RuntimeType = "docker"
	RuntimeContainerd RuntimeType = "containerd"
	RuntimeCrio     RuntimeType = "crio"
	RuntimePodman   RuntimeType = "podman"
)

// osComponentBOMs defines the OS-specific package dependencies for each component
var osComponentBOMs = map[OSComponent]map[OSType][]OSBOM{
	ComponentKubernetes: {
		OSUbuntu: {
			{
				OS:      OSUbuntu,
				Version: "20.04",
				Packages: []OSPackage{
					{Name: "apt-transport-https"},
					{Name: "ca-certificates"},
					{Name: "curl"},
					{Name: "gnupg"},
					{Name: "lsb-release"},
				},
				Description: "Kubernetes dependencies for Ubuntu 20.04",
			},
			{
				OS:      OSUbuntu,
				Version: "22.04",
				Packages: []OSPackage{
					{Name: "apt-transport-https"},
					{Name: "ca-certificates"},
					{Name: "curl"},
					{Name: "gnupg"},
				},
				Description: "Kubernetes dependencies for Ubuntu 22.04",
			},
		},
		OSCentOS: {
			{
				OS:      OSCentOS,
				Version: "7",
				Packages: []OSPackage{
					{Name: "socat"},
					{Name: "conntrack-tools"},
					{Name: "ipset"},
					{Name: "ebtables"},
					{Name: "ethtool"},
					{Name: "ipvsadm"},
					{Name: "open-iscsi"},
					{Name: "nfs-utils"},
					{Name: "haproxy"},
					{Name: "keepalived"},
				},
				Description: "Kubernetes dependencies for CentOS 7",
			},
			{
				OS:      OSCentOS,
				Version: "8",
				Packages: []OSPackage{
					{Name: "socat"},
					{Name: "conntrack-tools"},
					{Name: "ipset"},
					{Name: "ebtables"},
					{Name: "ethtool"},
					{Name: "ipvsadm"},
					{Name: "open-iscsi"},
					{Name: "nfs-utils"},
					{Name: "haproxy"},
					{Name: "keepalived"},
				},
				Description: "Kubernetes dependencies for CentOS 8",
			},
		},
		OSRockyLinux: {
			{
				OS:      OSRockyLinux,
				Version: "8",
				Packages: []OSPackage{
					{Name: "socat"},
					{Name: "conntrack-tools"},
					{Name: "ipset"},
					{Name: "ebtables"},
					{Name: "ethtool"},
					{Name: "ipvsadm"},
					{Name: "open-iscsi"},
					{Name: "nfs-utils"},
					{Name: "haproxy"},
					{Name: "keepalived"},
				},
				Description: "Kubernetes dependencies for Rocky Linux 8",
			},
			{
				OS:      OSRockyLinux,
				Version: "9",
				Packages: []OSPackage{
					{Name: "socat"},
					{Name: "conntrack-tools"},
					{Name: "ipset"},
					{Name: "ebtables"},
					{Name: "ethtool"},
					{Name: "ipvsadm"},
					{Name: "open-iscsi"},
					{Name: "nfs-utils"},
					{Name: "haproxy"},
					{Name: "keepalived"},
				},
				Description: "Kubernetes dependencies for Rocky Linux 9",
			},
		},
		OSAlmaLinux: {
			{
				OS:      OSAlmaLinux,
				Version: "8",
				Packages: []OSPackage{
					{Name: "socat"},
					{Name: "conntrack-tools"},
					{Name: "ipset"},
					{Name: "ebtables"},
					{Name: "ethtool"},
					{Name: "ipvsadm"},
					{Name: "open-iscsi"},
					{Name: "nfs-utils"},
					{Name: "haproxy"},
					{Name: "keepalived"},
				},
				Description: "Kubernetes dependencies for AlmaLinux 8",
			},
			{
				OS:      OSAlmaLinux,
				Version: "9",
				Packages: []OSPackage{
					{Name: "socat"},
					{Name: "conntrack-tools"},
					{Name: "ipset"},
					{Name: "ebtables"},
					{Name: "ethtool"},
					{Name: "ipvsadm"},
					{Name: "open-iscsi"},
					{Name: "nfs-utils"},
					{Name: "haproxy"},
					{Name: "keepalived"},
				},
				Description: "Kubernetes dependencies for AlmaLinux 9",
			},
		},
	},

	ComponentDocker: {
		OSUbuntu: {
			{
				OS:      OSUbuntu,
				Version: "20.04",
				Packages: []OSPackage{
					{Name: "ca-certificates"},
					{Name: "curl"},
					{Name: "gnupg"},
					{Name: "lsb-release"},
				},
				Description: "Docker dependencies for Ubuntu 20.04",
			},
			{
				OS:      OSUbuntu,
				Version: "22.04",
				Packages: []OSPackage{
					{Name: "ca-certificates"},
					{Name: "curl"},
					{Name: "gnupg"},
				},
				Description: "Docker dependencies for Ubuntu 22.04",
			},
		},
		OSCentOS: {
			{
				OS:      OSCentOS,
				Version: "7",
				Packages: []OSPackage{
					{Name: "yum-utils"},
					{Name: "device-mapper-persistent-data"},
					{Name: "lvm2"},
				},
				Description: "Docker dependencies for CentOS 7",
			},
		},
		OSRockyLinux: {
			{
				OS:      OSRockyLinux,
				Version: "8",
				Packages: []OSPackage{
					{Name: "yum-utils"},
					{Name: "device-mapper-persistent-data"},
					{Name: "lvm2"},
				},
				Description: "Docker dependencies for Rocky Linux 8",
			},
		},
	},

	ComponentContainerd: {
		OSUbuntu: {
			{
				OS:      OSUbuntu,
				Version: "20.04",
				Packages: []OSPackage{
					{Name: "ca-certificates"},
					{Name: "curl"},
					{Name: "gnupg"},
				},
				Description: "Containerd dependencies for Ubuntu 20.04",
			},
		},
		OSCentOS: {
			{
				OS:      OSCentOS,
				Version: "7",
				Packages: []OSPackage{
					{Name: "yum-utils"},
				},
				Description: "Containerd dependencies for CentOS 7",
			},
		},
	},

	ComponentPodman: {
		OSCentOS: {
			{
				OS:      OSCentOS,
				Version: "7",
				Packages: []OSPackage{
					{Name: "podman"},
				},
				Description: "Podman dependencies for CentOS 7",
			},
			{
				OS:      OSCentOS,
				Version: "8",
				Packages: []OSPackage{
					{Name: "podman"},
				},
				Description: "Podman dependencies for CentOS 8",
			},
		},
		OSRockyLinux: {
			{
				OS:      OSRockyLinux,
				Version: "8",
				Packages: []OSPackage{
					{Name: "podman"},
				},
				Description: "Podman dependencies for Rocky Linux 8",
			},
			{
				OS:      OSRockyLinux,
				Version: "9",
				Packages: []OSPackage{
					{Name: "podman"},
				},
				Description: "Podman dependencies for Rocky Linux 9",
			},
		},
		OSAlmaLinux: {
			{
				OS:      OSAlmaLinux,
				Version: "8",
				Packages: []OSPackage{
					{Name: "podman"},
				},
				Description: "Podman dependencies for AlmaLinux 8",
			},
			{
				OS:      OSAlmaLinux,
				Version: "9",
				Packages: []OSPackage{
					{Name: "podman"},
				},
				Description: "Podman dependencies for AlmaLinux 9",
			},
		},
	},

	ComponentCrio: {
		OSCentOS: {
			{
				OS:      OSCentOS,
				Version: "7",
				Packages: []OSPackage{
					{Name: "cri-o"},
					{Name: "fio"},
					{Name: "sysbench"},
				},
				Description: "CRI-O dependencies for CentOS 7",
			},
			{
				OS:      OSCentOS,
				Version: "8",
				Packages: []OSPackage{
					{Name: "cri-o"},
					{Name: "fio"},
					{Name: "sysbench"},
				},
				Description: "CRI-O dependencies for CentOS 8",
			},
		},
		OSRockyLinux: {
			{
				OS:      OSRockyLinux,
				Version: "8",
				Packages: []OSPackage{
					{Name: "cri-o"},
					{Name: "fio"},
					{Name: "sysbench"},
				},
				Description: "CRI-O dependencies for Rocky Linux 8",
			},
			{
				OS:      OSRockyLinux,
				Version: "9",
				Packages: []OSPackage{
					{Name: "cri-o"},
					{Name: "fio"},
					{Name: "sysbench"},
				},
				Description: "CRI-O dependencies for Rocky Linux 9",
			},
		},
		OSAlmaLinux: {
			{
				OS:      OSAlmaLinux,
				Version: "8",
				Packages: []OSPackage{
					{Name: "cri-o"},
					{Name: "fio"},
					{Name: "sysbench"},
				},
				Description: "CRI-O dependencies for AlmaLinux 8",
			},
			{
				OS:      OSAlmaLinux,
				Version: "9",
				Packages: []OSPackage{
					{Name: "cri-o"},
					{Name: "fio"},
					{Name: "sysbench"},
				},
				Description: "CRI-O dependencies for AlmaLinux 9",
			},
		},
	},
}

// GetComponentOSPkgBOM returns the OS package dependencies for a given component and OS
func GetComponentOSPkgBOM(component OSComponent, osType OSType, osVersion OSVersion) *OSBOM {
	osBOMs, ok := osComponentBOMs[component]
	if !ok {
		return nil
	}

	boms, ok := osBOMs[osType]
	if !ok {
		return nil
	}

	// Sort BOMs by version in descending order to find the best match
	sort.Slice(boms, func(i, j int) bool {
		return string(boms[i].Version) > string(boms[j].Version)
	})

	// Find the best matching BOM
	for _, bom := range boms {
		if strings.HasPrefix(string(osVersion), string(bom.Version)) {
			return &bom
		}
	}

	// If no exact match, return the first one (highest version)
	if len(boms) > 0 {
		return &boms[0]
	}

	return nil
}

// GetComponentOSPkgBOMWithRuntime returns the OS package dependencies for a given component, OS and runtime type
func GetComponentOSPkgBOMWithRuntime(component OSComponent, osType OSType, osVersion OSVersion, runtime RuntimeType) *OSBOM {
	bom := GetComponentOSPkgBOM(component, osType, osVersion)
	if bom == nil {
		return nil
	}

	// Create a copy of the BOM to avoid modifying the original
	bomCopy := *bom
	bomCopy.Packages = make([]OSPackage, len(bom.Packages))
	copy(bomCopy.Packages, bom.Packages)

	// Add runtime-specific packages
	switch runtime {
	case RuntimePodman:
		podmanBOM := GetComponentOSPkgBOM(ComponentPodman, osType, osVersion)
		if podmanBOM != nil {
			bomCopy.Packages = append(bomCopy.Packages, podmanBOM.Packages...)
		}
	case RuntimeCrio:
		crioBOM := GetComponentOSPkgBOM(ComponentCrio, osType, osVersion)
		if crioBOM != nil {
			bomCopy.Packages = append(bomCopy.Packages, crioBOM.Packages...)
		}
	}

	return &bomCopy
}

// GetAllOSComponents returns all components that have OS-specific dependencies
func GetAllOSComponents() []OSComponent {
	components := make([]OSComponent, 0, len(osComponentBOMs))
	for component := range osComponentBOMs {
		components = append(components, component)
	}
	sort.Slice(components, func(i, j int) bool {
		return string(components[i]) < string(components[j])
	})
	return components
}

// GetSupportedOSList returns all supported OS types for a given component
func GetSupportedOSList(component OSComponent) []OSType {
	osBOMs, ok := osComponentBOMs[component]
	if !ok {
		return nil
	}

	osList := make([]OSType, 0, len(osBOMs))
	for osType := range osBOMs {
		osList = append(osList, osType)
	}
	sort.Slice(osList, func(i, j int) bool {
		return string(osList[i]) < string(osList[j])
	})
	return osList
}

// GetSupportedOSVersions returns all supported OS versions for a given component and OS type
func GetSupportedOSVersions(component OSComponent, osType OSType) []OSVersion {
	osBOMs, ok := osComponentBOMs[component]
	if !ok {
		return nil
	}

	boms, ok := osBOMs[osType]
	if !ok {
		return nil
	}

	versions := make([]OSVersion, 0, len(boms))
	for _, bom := range boms {
		versions = append(versions, bom.Version)
	}
	// Remove duplicates and sort
	uniqueVersions := make(map[OSVersion]bool)
	for _, version := range versions {
		uniqueVersions[version] = true
	}

	sortedVersions := make([]OSVersion, 0, len(uniqueVersions))
	for version := range uniqueVersions {
		sortedVersions = append(sortedVersions, version)
	}
	sort.Slice(sortedVersions, func(i, j int) bool {
		return string(sortedVersions[i]) < string(sortedVersions[j])
	})

	return sortedVersions
}

// String returns a string representation of the OSComponent
func (c OSComponent) String() string {
	return string(c)
}

// String returns a string representation of the OSType
func (o OSType) String() string {
	return string(o)
}

// String returns a string representation of the OSVersion
func (v OSVersion) String() string {
	return string(v)
}

// String returns a string representation of the RuntimeType
func (r RuntimeType) String() string {
	return string(r)
}