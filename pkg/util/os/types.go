package os

// OSType represents the type of operating system
type OSType string

const (
	// Linux distributions
	OSUbuntu     OSType = "ubuntu"
	OSCentOS     OSType = "centos"
	OSRockyLinux OSType = "rockylinux"
	OSAlmaLinux  OSType = "almalinux"
	OSRHEL       OSType = "rhel"
	OSDebian     OSType = "debian"
	OSFedora     OSType = "fedora"
	OSOpenSUSE   OSType = "opensuse"
	OSAmazon     OSType = "amazon"
	OSAlpine     OSType = "alpine"
	
	// Other OS types
	OSWindows    OSType = "windows"
	OSMacOS      OSType = "macos"
)

// OSVersion represents the version of an operating system
type OSVersion string

// OSPackage represents a package that should be installed on an OS
type OSPackage struct {
	Name         string     `json:"name" yaml:"name"`
	Version      string     `json:"version,omitempty" yaml:"version,omitempty"`
	Arch         string     `json:"arch,omitempty" yaml:"arch,omitempty"`
	Dependencies []string   `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// OSBOM represents the Bill of Materials for an operating system
type OSBOM struct {
	OS          OSType     `json:"os" yaml:"os"`
	Version     OSVersion  `json:"version" yaml:"version"`
	Packages    []OSPackage `json:"packages" yaml:"packages"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
}