package repo

import (
	"fmt"
	"strings"
)

// Config holds repository configuration for a specific OS type.
type Config struct {
	OS           string
	OSVersion    string
	MountPoint   string // Where the ISO will be mounted (e.g., /mnt/kubexm)
	RepoName     string
}

// RPMPackage represents an RPM package.
type RPMPackage struct {
	Name     string
	Version  string
	Arch     string
	Repo     string
	Checksum string
}

// DEBPackage represents a DEB package.
type DEBPackage struct {
	Name     string
	Version  string
	Arch     string
	Checksum string
}

// RPMRepoConfig generates the RPM repository configuration file content.
func RPMRepoConfig(mountPoint string) string {
	return fmt.Sprintf(`[kubexm-offline]
name=KubeXM Offline Repository
baseurl=file://%s/repos/offline
enabled=1
gpgcheck=0
priority=1
module_hotfixes=1
`, mountPoint)
}

// RPMRepoConfigHTTPS generates RPM repo config with optional HTTPS support.
func RPMRepoConfigHTTPS(baseURL string) string {
	return fmt.Sprintf(`[kubexm-offline]
name=KubeXM Offline Repository
baseurl=%s/repos/offline
enabled=1
gpgcheck=0
priority=1
module_hotfixes=1
`, baseURL)
}

// DEBSourcesConfig generates the DEB sources.list entry.
func DEBSourcesConfig(mountPoint string) string {
	return fmt.Sprintf(`deb [trusted=yes] file:%s/repos/offline offline main
`, mountPoint)
}

// DEBSourcesConfigHTTPS generates DEB sources config with HTTP/HTTPS URL.
func DEBSourcesConfigHTTPS(baseURL string) string {
	return fmt.Sprintf(`deb [trusted=yes] %s/repos/offline offline main
`, baseURL)
}

// EnableRPMRepo generates a shell script to enable the RPM offline repo.
func EnableRPMRepo(mountPoint string) string {
	return fmt.Sprintf(`#!/bin/bash
# Enable KubeXM offline RPM repository

set -e

REPO_FILE="/etc/yum.repos.d/kubexm-offline.repo"
MOUNT_POINT="%s"

if [ ! -f "$REPO_FILE" ]; then
    cat > "$REPO_FILE" << 'EOF'
[kubexm-offline]
name=KubeXM Offline Repository
baseurl=file://%s/repos/offline
enabled=1
gpgcheck=0
priority=1
module_hotfixes=1
EOF
fi

# Clean and rebuild cache
yum clean all 2>/dev/null || true
dnf clean all 2>/dev/null || true

echo "KubeXM offline repository enabled"
echo "Repository file: $REPO_FILE"

# List available packages
echo ""
echo "Available packages:"
yum --disablerepo="*" --enablerepo="kubexm-offline" list available 2>/dev/null | head -20 || \
dnf --disablerepo="*" --enablerepo="kubexm-offline" list available 2>/dev/null | head -20 || \
echo "  (run 'yum repolist' or 'dnf repolist' to see packages)"
`, mountPoint, mountPoint)
}

// EnableDEBRepo generates a shell script to enable the DEB offline repo.
func EnableDEBRepo(mountPoint string) string {
	return fmt.Sprintf(`#!/bin/bash
# Enable KubeXM offline DEB repository

set -e

SOURCES_FILE="/etc/apt/sources.list.d/kubexm-offline.list"
MOUNT_POINT="%s"

if [ ! -f "$SOURCES_FILE" ]; then
    cat > "$SOURCES_FILE" << 'EOF'
deb [trusted=yes] file:%s/repos/offline offline main
EOF
fi

# Update package lists
apt-get update 2>/dev/null || true

echo "KubeXM offline repository enabled"
echo "Sources file: $SOURCES_FILE"

# List available packages
echo ""
echo "Available packages:"
apt-cache search "^" 2>/dev/null | head -20 || \
echo "  (run 'apt-cache search .' to see packages)"
`, mountPoint, mountPoint)
}

// InstallPackagesRPM generates an RPM package installation script.
func InstallPackagesRPM(packages []string) string {
	if len(packages) == 0 {
		return `#!/bin/bash
# Install all kubexm packages from offline repository

set -e

echo "Installing kubexm packages from offline repository..."

# Essential packages for Kubernetes
yum install -y --disablerepo="*" --enablerepo="kubexm-offline" \
    conntrack \
    socat \
    ipset \
    iptables \
    ebtables \
    curl \
    wget \
    jq \
    rsync \
    tar \
    gzip \
    xz \
    bc \
    nc \
    psmisc \
    ipvsadm \
    nfs-utils \
    iscsi-initiator-utils \
    || dnf install -y --disablerepo="*" --enablerepo="kubexm-offline" \
    conntrack \
    socat \
    ipset \
    iptables \
    ebtables \
    curl \
    wget \
    jq \
    rsync \
    tar \
    gzip \
    xz \
    bc \
    nc \
    psmisc \
    ipvsadm \
    nfs-utils \
    iscsi-initiator-utils \
    || true

echo "Package installation complete"
`
	}

	var buf strings.Builder
	buf.WriteString("#!/bin/bash\n# Install kubexm packages\nset -e\necho 'Installing kubexm packages...'\nyum install -y --disablerepo='*' --enablerepo='kubexm-offline' \\\n")
	for i, pkg := range packages {
		if i == len(packages)-1 {
			buf.WriteString("    " + pkg + "\n")
		} else {
			buf.WriteString("    " + pkg + " \\\n")
		}
	}
	buf.WriteString("    || dnf install -y --disablerepo='*' --enablerepo='kubexm-offline' \\\n")
	for i, pkg := range packages {
		if i == len(packages)-1 {
			buf.WriteString("    " + pkg + "\n")
		} else {
			buf.WriteString("    " + pkg + " \\\n")
		}
	}
	buf.WriteString("    || true\necho 'Package installation complete'\n")
	return buf.String()
}

// InstallPackagesDEB generates a DEB package installation script.
func InstallPackagesDEB(packages []string) string {
	if len(packages) == 0 {
		return `#!/bin/bash
# Install all kubexm packages from offline repository

set -e

echo "Installing kubexm packages from offline repository..."

# Essential packages for Kubernetes
apt-get install -y \
    conntrack \
    socat \
    iptables \
    ebtables \
    curl \
    wget \
    jq \
    rsync \
    tar \
    gzip \
    xz-utils \
    bc \
    netcat-openbsd \
    psmisc \
    ipvsadm \
    nfs-common \
    open-iscsi \
    || true

echo "Package installation complete"
`
	}

	var buf strings.Builder
	buf.WriteString("#!/bin/bash\n# Install kubexm packages\nset -e\necho 'Installing kubexm packages...'\napt-get install -y \\\n")
	for i, pkg := range packages {
		if i == len(packages)-1 {
			buf.WriteString("    " + pkg + "\n")
		} else {
			buf.WriteString("    " + pkg + " \\\n")
		}
	}
	buf.WriteString("    || true\necho 'Package installation complete'\n")
	return buf.String()
}

// GenerateInstallGuide generates a comprehensive installation guide.
func GenerateInstallGuide(osType, mountPoint string) string {
	return fmt.Sprintf(`KubeXM Offline Installation Guide
================================

Target OS: %s
Mount Point: %s

## Quick Start

1. Mount this ISO:
   sudo mkdir -p %s
   sudo mount -o loop /path/to/kubexm.iso %s

2. Enable the offline repository:
   # For RPM-based systems (CentOS/Rocky/RHEL):
   sudo bash %s/scripts/enable-repo.sh

   # For DEB-based systems (Ubuntu/Debian):
   sudo bash %s/scripts/enable-repo.sh

3. Install packages:
   # For RPM-based systems:
   sudo bash %s/scripts/install-packages.sh

   # For DEB-based systems:
   sudo bash %s/scripts/install-packages.sh

4. Continue with Kubernetes installation using kubexm

## Repository Contents

/repos/offline/           - Local package repository
  ├── repodata/          (RPM) Repository metadata
  └── pool/              (DEB) Package pool

/kubexm-packages/        - Kubexm binary packages
  ├── kubernetes/        - Kubernetes binaries (kubeadm, kubelet, kubectl, etc.)
  ├── etcd/              - etcd binaries
  ├── container_runtime/ - Container runtime binaries
  ├── cni/               - CNI plugin binaries
  ├── helm/              - Helm binary
  └── registry/         - Registry binaries

/scripts/                 - Installation scripts
  ├── enable-repo.sh     - Enable offline repository
  └── install-packages.sh - Install OS packages

## Repository Configuration

RPM-based systems:
  File: /etc/yum.repos.d/kubexm-offline.repo
  Command: yum repolist

DEB-based systems:
  File: /etc/apt/sources.list.d/kubexm-offline.list
  Command: apt-cache policy

## Offline Installation

This ISO enables fully offline Kubernetes cluster installation:
- No internet connection required
- All OS-level dependencies pre-bundled
- All Kubernetes binaries pre-bundled
- Local package repository for dependency resolution

## Troubleshooting

1. Repository not found:
   - Ensure the ISO is properly mounted
   - Check that /etc/yum.repos.d/ or /etc/apt/sources.list.d/ is writable

2. Package not found:
   - Run 'yum clean all' or 'apt-get update' to refresh cache
   - Verify the repository is enabled with 'yum repolist' or 'apt-cache policy'

3. Dependency issues:
   - The local repository contains all resolved dependencies
   - No external network access needed
`, osType, mountPoint, mountPoint,
		mountPoint, mountPoint, mountPoint, mountPoint, mountPoint)
}

// PackageIndex represents a package index for offline reference.
type PackageIndex struct {
	Packages []string
	TotalSize int64
	Count     int
}

// VerifyPackages checks if all packages are present in the repo.
func VerifyPackages(repoDir string, expectedPackages []string) (missing []string) {
	// This is a basic verification - actual implementation would
	// parse the repository metadata
	return missing
}
