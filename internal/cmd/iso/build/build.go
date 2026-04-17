package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/internal/cmd/iso/dep"
	"github.com/mensylisir/kubexm/internal/cmd/iso/util"
	"github.com/mensylisir/kubexm/internal/logger"
)

// Mode represents the build execution mode.
type Mode string

const (
	ModeHost   Mode = "host"   // Run directly on the current OS/architecture
	ModeDocker Mode = "docker" // Run in Docker for cross-OS/arch support
)

// Builder is the main ISO builder.
type Builder struct {
	Mode      Mode
	OSType    dep.OSType
	OSVersion string
	Arch      string
	OutputDir string
	WorkDir   string

	Log  func(string, ...interface{})
	DryRun bool
}

// BuildResult holds the result of an ISO build.
type BuildResult struct {
	ISOPath     string
	ISOSize    int64
	Duration   time.Duration
	PackagesCount int
	RepoPath   string
}

// NewBuilder creates a new ISO builder.
func NewBuilder(mode Mode, osType dep.OSType, osVersion, arch, outputDir string) *Builder {
	workDir := filepath.Join(outputDir, ".work")
	return &Builder{
		Mode:       mode,
		OSType:     osType,
		OSVersion:  osVersion,
		Arch:       arch,
		OutputDir:  outputDir,
		WorkDir:    workDir,
	}
}

// Build builds the complete ISO with all packages and repositories.
func (b *Builder) Build(cfg *dep.ClusterDepConfig) (*BuildResult, error) {
	start := time.Now()

	if b.Log == nil {
		stdlog := logger.Get()
		b.Log = stdlog.Infof
	}

	b.Log("Starting ISO build...")
	b.Log("  Mode: %s", b.Mode)
	b.Log("  OS: %s %s (%s)", b.OSType, b.OSVersion, b.Arch)
	b.Log("  Output: %s", b.OutputDir)

	// Step 1: Resolve dependencies
	b.Log("[1/6] Resolving dependencies...")
	resolved, err := b.resolveDependencies(cfg)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}
	b.Log("  Resolved %d packages", resolved.GetPackages().Len())
	if len(resolved.GetUnresolved()) > 0 {
		b.Log("  Warning: unresolved packages: %v", resolved.GetUnresolved())
	}

	// Step 2: Build local repository
	b.Log("[2/6] Building local repository...")
	repoPath, err := b.buildRepo(resolved)
	if err != nil {
		return nil, fmt.Errorf("repo build failed: %w", err)
	}
	b.Log("  Repository built at: %s", repoPath)

	// Step 3: Dump current system repos for reference
	b.dumpSystemRepos()

	// Step 4: Copy system repo configs
	if err := b.copyRepoConfigs(); err != nil {
		b.Log("  Warning: failed to copy repo configs: %v", err)
	}

	// Step 5: Copy kubexm binary packages
	b.Log("[3/6] Copying kubexm binary packages...")
	if err := b.copyKubexmPackages(cfg); err != nil {
		b.Log("  Warning: failed to copy kubexm packages: %v", err)
	}

	// Step 6: Generate ISO
	b.Log("[4/6] Generating ISO...")
	isoPath, err := b.generateISO()
	if err != nil {
		return nil, fmt.Errorf("ISO generation failed: %w", err)
	}

	// Step 7: Verify ISO
	b.Log("[5/6] Verifying ISO...")
	if err := b.verifyISO(isoPath); err != nil {
		b.Log("  Warning: ISO verification failed: %v", err)
	}

	elapsed := time.Since(start)
	isoSize, _ := util.GetFileSize(isoPath)

	b.Log("[6/6] Build complete!")
	b.Log("  ISO: %s", isoPath)
	b.Log("  Size: %s", util.FormatSize(isoSize))
	b.Log("  Duration: %v", elapsed.Round(time.Second))
	b.Log("  Packages: %d", resolved.GetPackages().Len())

	return &BuildResult{
		ISOPath:      isoPath,
		ISOSize:     isoSize,
		Duration:    elapsed,
		PackagesCount: resolved.GetPackages().Len(),
		RepoPath:   repoPath,
	}, nil
}

// ResolverResult is the interface for dependency resolution results.
type ResolverResult interface {
	GetPackages() dep.PackageList
	GetUnresolved() []string
	BuildRepo(outputDir string, log func(string, ...interface{})) (string, error)
}

func (b *Builder) resolveDependencies(cfg *dep.ClusterDepConfig) (ResolverResult, error) {
	ps := dep.NewPackageSet()

	resolved, err := ps.ResolveForCluster(cfg)
	if err != nil {
		return nil, err
	}

	if b.OSType.IsRPM() {
		resolver := dep.NewRPMResolver(b.OSType, b.OSVersion, b.Arch, b.WorkDir)
		return resolver.Resolve(resolved.Packages.Names())
	}

	if b.OSType.IsDEB() {
		resolver := dep.NewDEBResolver(b.OSType, b.OSVersion, b.Arch, b.WorkDir)
		return resolver.Resolve(resolved.Packages.Names())
	}

	return nil, fmt.Errorf("unsupported OS type: %s", b.OSType)
}

func (b *Builder) buildRepo(resolved ResolverResult) (string, error) {
	return resolved.BuildRepo(b.OutputDir, b.Log)
}

func (b *Builder) dumpSystemRepos() {
	if b.OSType.IsRPM() {
		data, err := dep.DumpYUMRepos()
		if err == nil && len(data) > 0 {
			dest := filepath.Join(b.OutputDir, "repos", "original-sources.list")
			os.WriteFile(dest, data, 0644)
			b.Log("  System YUM repos saved to: %s", dest)
		}
	}

	if b.OSType.IsDEB() {
		data, err := dep.DumpAptSources()
		if err == nil && len(data) > 0 {
			dest := filepath.Join(b.OutputDir, "repos", "original-sources.list")
			os.WriteFile(dest, data, 0644)
			b.Log("  System APT sources saved to: %s", dest)
		}
	}
}

func (b *Builder) copyRepoConfigs() error {
	if b.OSType.IsRPM() {
		// Write offline repo config
		repoFile := filepath.Join(b.OutputDir, "repos", "kubexm-offline.repo")
		content := `[kubexm-offline]
name=KubeXM Offline Repository
baseurl=file:///mnt/kubexm/repos/offline
enabled=1
gpgcheck=0
priority=1
module_hotfixes=1
`
		return os.WriteFile(repoFile, []byte(content), 0644)
	}

	if b.OSType.IsDEB() {
		// Write offline apt sources
		sourcesFile := filepath.Join(b.OutputDir, "repos", "kubexm-offline.list")
		content := `deb [trusted=yes] file:/mnt/kubexm/repos/offline offline main
`
		return os.WriteFile(sourcesFile, []byte(content), 0644)
	}

	return nil
}

func (b *Builder) copyKubexmPackages(cfg *dep.ClusterDepConfig) error {
	// Look for kubexm packages in common locations
	searchDirs := []string{
		"/tmp/kubexm",
		filepath.Join(os.Getenv("HOME"), ".kubexm"),
		"./packages",
	}

	var foundDir string
	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); err == nil {
			foundDir = dir
			break
		}
	}

	if foundDir == "" {
		b.Log("  No kubexm binary packages found (searched: %v)", searchDirs)
		return nil
	}

	dest := filepath.Join(b.OutputDir, "kubexm-packages")
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	// Copy available packages
	components := []string{
		"kubernetes",
		"etcd",
		"container_runtime",
		"cni",
		"helm",
		"registry",
	}

	for _, comp := range components {
		srcDir := filepath.Join(foundDir, comp)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			continue
		}

		destDir := filepath.Join(dest, comp)
		if err := util.CopyDirRecursive(srcDir, destDir); err != nil {
			b.Log("  Warning: failed to copy %s: %v", comp, err)
		} else {
			b.Log("  Copied %s packages", comp)
		}
	}

	return nil
}

func (b *Builder) generateISO() (string, error) {
	// Create a staging directory for ISO contents
	stagingDir := filepath.Join(b.WorkDir, "iso-staging")
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	// Copy everything from output directory to staging
	if err := util.CopyDirRecursive(b.OutputDir, stagingDir); err != nil {
		return "", fmt.Errorf("failed to copy to staging: %w", err)
	}

	// Generate repo configuration for installation
	if err := b.generateInstallConfigs(stagingDir); err != nil {
		b.Log("  Warning: failed to generate install configs: %v", err)
	}

	// Generate the ISO
	isoName := fmt.Sprintf("kubexm-%s-%s-%s-%s.iso",
		b.OSType, b.OSVersion, b.Arch, time.Now().Format("20060102-150405"))
	isoPath := filepath.Join(b.OutputDir, isoName)

	if err := b.buildISO(stagingDir, isoPath); err != nil {
		return "", err
	}

	return isoPath, nil
}

func (b *Builder) generateInstallConfigs(isoRoot string) error {
	installDir := filepath.Join(isoRoot, "install")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}

	// Generate repo enablement script
	var buf bytes.Buffer

	if b.OSType.IsRPM() {
		buf.WriteString(`#!/bin/bash
# Enable kubexm offline repository

REPO_FILE="/etc/yum.repos.d/kubexm-offline.repo"

if [ ! -f "$REPO_FILE" ]; then
    cat > "$REPO_FILE" << 'EOF'
[kubexm-offline]
name=KubeXM Offline Repository
baseurl=file:///mnt/kubexm/repos/offline
enabled=1
gpgcheck=0
priority=1
module_hotfixes=1
EOF
fi

echo "KubeXM offline repository enabled"
yum clean all
`)
	}

	if b.OSType.IsDEB() {
		buf.WriteString(`#!/bin/bash
# Enable kubexm offline repository

SOURCES_FILE="/etc/apt/sources.list.d/kubexm-offline.list"

if [ ! -f "$SOURCES_FILE" ]; then
    cat > "$SOURCES_FILE" << 'EOF'
deb [trusted=yes] file:/mnt/kubexm/repos/offline offline main
EOF
fi

echo "KubeXM offline repository enabled"
apt-get update
`)
	}

	installScript := filepath.Join(installDir, "enable-repo.sh")
	if err := os.WriteFile(installScript, buf.Bytes(), 0755); err != nil {
		return err
	}

	// Generate package install script
	var installBuf bytes.Buffer

	if b.OSType.IsRPM() {
		installBuf.WriteString(`#!/bin/bash
# Install all kubexm packages from offline repo

set -e

echo "Installing kubexm packages from offline repository..."

# Install essential packages
yum install -y \
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
    2>/dev/null || true

echo "Package installation complete"
`)
	}

	if b.OSType.IsDEB() {
		installBuf.WriteString(`#!/bin/bash
# Install all kubexm packages from offline repo

set -e

echo "Installing kubexm packages from offline repository..."

# Install essential packages
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
    netcat \
    psmisc \
    ipvsadm \
    nfs-common \
    open-iscsi \
    2>/dev/null || true

echo "Package installation complete"
`)
	}

	pkgInstallScript := filepath.Join(installDir, "install-packages.sh")
	if err := os.WriteFile(pkgInstallScript, installBuf.Bytes(), 0755); err != nil {
		return err
	}

	// Generate README
	readme := fmt.Sprintf(`KubeXM Offline Installation ISO
================================
OS: %s %s (%s)
Generated: %s
Mode: %s

Contents:
- /repos/offline/         : Local package repository
- /kubexm-packages/        : Kubexm binary packages
- /install/                : Installation scripts

Usage:
1. Mount this ISO on target machines
2. Run: bash /mnt/kubexm/install/enable-repo.sh
3. Run: bash /mnt/kubexm/install/install-packages.sh
4. Install Kubernetes using kubexm binary packages

Repository Configuration:
- RPM: /etc/yum.repos.d/kubexm-offline.repo
- DEB: /etc/apt/sources.list.d/kubexm-offline.list
`,
		b.OSType, b.OSVersion, b.Arch, time.Now().Format(time.RFC3339), b.Mode)

	readmePath := filepath.Join(isoRoot, "README.txt")
	return os.WriteFile(readmePath, []byte(readme), 0644)
}

func (b *Builder) buildISO(sourceDir, outputPath string) error {
	// Try xorriso first
	xorrisoPath, err := exec.LookPath("xorriso")
	if err == nil {
		cmd := exec.Command(xorrisoPath,
			"-as", "mkisofs",
			"-r",
			"-J",
			"-l",
			"-o", outputPath,
			"-V", "KUBEXM",
			"--iso-level", "3",
			sourceDir,
		)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("xorriso mkisofs emulation failed: %w", err)
		}
		return nil
	}

	// Fall back to mkisofs/genisoimage
	var mkisofsPath string
	for _, name := range []string{"mkisofs", "genisoimage"} {
		if path, err := exec.LookPath(name); err == nil {
			mkisofsPath = path
			break
		}
	}
	if mkisofsPath == "" {
		return fmt.Errorf("no ISO creation tool found (install xorriso or mkisofs)")
	}

	cmd := exec.Command(mkisofsPath,
		"-r",
		"-J",
		"-l",
		"-o", outputPath,
		"-V", "KUBEXM",
		sourceDir,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func (b *Builder) verifyISO(isoPath string) error {
	if _, err := os.Stat(isoPath); os.IsNotExist(err) {
		return fmt.Errorf("ISO file not found: %s", isoPath)
	}

	// Basic size check
	info, err := os.Stat(isoPath)
	if err != nil {
		return err
	}
	if info.Size() < 1024*1024 { // Less than 1MB is suspicious
		return fmt.Errorf("ISO file suspiciously small: %d bytes", info.Size())
	}

	// Try to list ISO contents with xorriso
	xorrisoPath, err := exec.LookPath("xorriso")
	if err != nil {
		return nil // Can't verify, but ISO was built
	}

	cmd := exec.Command(xorrisoPath, "-indev", isoPath, "-ls")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ISO verification failed: %w", err)
	}

	if !bytes.Contains(output, []byte("repos")) && !bytes.Contains(output, []byte("kubexm")) {
		b.Log("  Warning: ISO contents may be incomplete")
	}

	return nil
}

// --- Docker Mode ---

// DockerBuilder builds ISO using Docker containers for cross-platform support.
type DockerBuilder struct {
	Builder
	Registry string // Docker registry for builder images
}

// NewDockerBuilder creates a new Docker-based builder.
func NewDockerBuilder(osType dep.OSType, osVersion, arch, outputDir string) *DockerBuilder {
	return &DockerBuilder{
		Builder: Builder{
			Mode:      ModeDocker,
			OSType:    osType,
			OSVersion: osVersion,
			Arch:      arch,
			OutputDir: outputDir,
			WorkDir:   filepath.Join(outputDir, ".work"),
		},
		Registry: "", // Use official images by default
	}
}

// Build runs the ISO build inside a Docker container.
func (b *DockerBuilder) Build(cfg *dep.ClusterDepConfig) (*BuildResult, error) {
	image := b.getBuilderImage()
	b.Log("Using Docker builder image: %s", image)

	// Ensure the image exists
	if err := b.ensureBuilderImage(image); err != nil {
		return nil, err
	}

	// Run build in Docker
	return b.runInDocker(image, cfg)
}

// getBuilderImage returns the Docker base image for each supported OS.
// NOTE: RHEL images require Red Hat subscription; for production use,
// build your own internal registry mirrors.
func (b *DockerBuilder) getBuilderImage() string {
	switch b.OSType {
	// --- RPM 系 ---
	case dep.OSTypeCentOS:
		if strings.HasPrefix(b.OSVersion, "7") {
			return "centos:7"
		}
		// CentOS 8/9 Stream
		return "quay.io/centos/centos:stream9"

	case dep.OSTypeRocky:
		return "rockylinux/rockylinux:" + b.OSVersion

	case dep.OSTypeRHEL:
		// RHEL 镜像需要订阅认证，生产环境建议使用内部镜像站
		// 这里用 CentOS Stream 作为 fallback
		if strings.HasPrefix(b.OSVersion, "7") {
			return "centos:7"
		}
		return "quay.io/centos/centos:stream9"

	case dep.OSTypeOracle:
		return "oraclelinux:" + b.OSVersion

	case dep.OSTypeAlmaLinux:
		return "almalinux:" + b.OSVersion

	case dep.OSTypeFedora:
		return "fedora:" + b.OSVersion

	case dep.OSTypeUOS:
		// UOS 20 Server 目前没有公开 Docker 镜像，需要自行构建或从内部镜像站获取
		// 这里用龙蜥作为 fallback（都是 RPM 系，兼容性好）
		return "openanolis/anolisos:" + b.OSVersion

	case dep.OSTypeAnolis:
		return "openanolis/anolisos:" + b.OSVersion

	case dep.OSTypeOpenEuler:
		return "openeuler/openeuler:" + b.OSVersion

	case dep.OSTypeKylin:
		// 麒麟 V10 目前没有公开 Docker 镜像，需要自行构建
		// 龙芯/飞腾架构建议从内部镜像站获取
		return "openanolis/anolisos:" + b.OSVersion

	// --- DEB 系 ---
	case dep.OSTypeUbuntu:
		return "ubuntu:" + b.OSVersion

	case dep.OSTypeDebian:
		return "debian:" + b.OSVersion

	default:
		return "ubuntu:22.04"
	}
}

func (b *DockerBuilder) ensureBuilderImage(image string) error {
	// Check if image exists locally
	cmd := exec.Command("docker", "image", "inspect", image)
	if cmd.Run() == nil {
		return nil // Image exists
	}

	// Pull the image
	pullCmd := exec.Command("docker", "pull", image)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	return pullCmd.Run()
}

func (b *DockerBuilder) runInDocker(image string, cfg *dep.ClusterDepConfig) (*BuildResult, error) {
	// Create a temporary directory for build context
	buildCtxDir := filepath.Join(b.WorkDir, "docker-build")
	if err := os.MkdirAll(buildCtxDir, 0755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(buildCtxDir)

	// Generate build script
	buildScript, err := b.generateBuildScript(cfg)
	if err != nil {
		return nil, err
	}

	scriptPath := filepath.Join(buildCtxDir, "build.sh")
	if err := os.WriteFile(scriptPath, []byte(buildScript), 0755); err != nil {
		return nil, err
	}

	// Create Dockerfile
	dockerfile, err := b.generateDockerfile()
	if err != nil {
		return nil, err
	}

	dfPath := filepath.Join(buildCtxDir, "Dockerfile")
	if err := os.WriteFile(dfPath, []byte(dockerfile), 0644); err != nil {
		return nil, err
	}

	// Run docker build
	b.Log("Building Docker image...")
	dockerfilePath := filepath.Join(buildCtxDir, "Dockerfile")

	// Build the builder image
	buildImgName := fmt.Sprintf("kubexm-builder-%s-%s:%s", b.OSType, b.OSVersion, b.Arch)
	buildCmd := exec.Command("docker", "build",
		"-t", buildImgName,
		"-f", dockerfilePath,
		buildCtxDir,
	)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	// Run the builder container
	isoOutput := filepath.Join(b.OutputDir, "iso-output")
	if err := os.MkdirAll(isoOutput, 0755); err != nil {
		return nil, err
	}

	b.Log("Running builder container...")
	runCmd := exec.Command("docker", "run", "--rm",
		"-v", isoOutput+":/output",
		buildImgName,
	)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return nil, fmt.Errorf("docker run failed: %w", err)
	}

	// Find the generated ISO
	isos, _ := filepath.Glob(filepath.Join(isoOutput, "*.iso"))
	if len(isos) == 0 {
		return nil, fmt.Errorf("no ISO found in output directory")
	}

	isoPath := isos[0]
	isoSize, _ := util.GetFileSize(isoPath)

	b.Log("ISO generated: %s (%s)", isoPath, util.FormatSize(isoSize))

	return &BuildResult{
		ISOPath:      isoPath,
		ISOSize:     isoSize,
		Duration:    time.Since(time.Now()),
		PackagesCount: 0,
	}, nil
}

func (b *DockerBuilder) generateDockerfile() (string, error) {
	var buf bytes.Buffer

	switch {
	case b.OSType.IsRPM():
		buf.WriteString(fmt.Sprintf(`FROM %s

# Install build tools
RUN dnf install -y dnf-plugins-core yum-utils createrepo_c && \\
    dnf clean all

# Install build script
COPY build.sh /build.sh
RUN chmod +x /build.sh

WORKDIR /output
ENTRYPOINT ["/build.sh"]
`, b.getBuilderImage()))

	case b.OSType.IsDEB():
		buf.WriteString(fmt.Sprintf(`FROM %s

# Install build tools
RUN apt-get update && apt-get install -y \\
    apt-utils \\
    dpkg-dev \\
    apt-rdepends \\
    gzip \\
    dpkg-sig \\
    && rm -rf /var/lib/apt/lists/*

# Install build script
COPY build.sh /build.sh
RUN chmod +x /build.sh

WORKDIR /output
ENTRYPOINT ["/build.sh"]
`, b.getBuilderImage()))
	}

	return buf.String(), nil
}

// allPKGs returns the consolidated package list from the package set.
func (b *DockerBuilder) allPKGs() []string {
	ps := dep.NewPackageSet()
	all := append([]string{}, ps.K8sPrereqs...)
	for _, deps := range ps.RuntimeDeps {
		all = append(all, deps...)
	}
	for _, deps := range ps.LoadBalancerDeps {
		all = append(all, deps...)
	}
	for _, deps := range ps.StorageDeps {
		all = append(all, deps...)
	}
	return all
}

// useYUM returns true if the OS should use yum (not dnf).
func (b *DockerBuilder) useYUM() bool {
	// CentOS 7 is the only major RPM OS that uses yum
	return b.OSType == dep.OSTypeCentOS && strings.HasPrefix(b.OSVersion, "7")
}

// useDNF returns true if the OS should use dnf.
func (b *DockerBuilder) useDNF() bool {
	return b.OSType.IsRPM() && !b.useYUM()
}

func (b *DockerBuilder) generateBuildScript(cfg *dep.ClusterDepConfig) (string, error) {
	var buf bytes.Buffer
	pkgs := b.allPKGs()

	if b.OSType.IsRPM() {
		if b.useYUM() {
			// CentOS 7 / RHEL 7 — yum path
			buf.WriteString(`#!/bin/bash
set -e

echo "Configuring package repositories..."
yum install -y epel-release || true
yum update -y || true

echo "Installing tools..."
yum install -y yum-utils createrepo || true

echo "Resolving and downloading packages..."
mkdir -p /output/repos/offline

yum install -y --downloadonly --downloaddir=/output/repos/offline `)
			buf.WriteString(strings.Join(pkgs, " "))
			buf.WriteString(` || true

echo "Building local repository..."
createrepo /output/repos/offline

echo "Build complete"
ls -la /output/repos/offline/
`)
		} else {
			// All other RPM OSes (Rocky/RHEL 8+/Fedora/UOS/Anolis/openEuler/Kylin/Oracle/Alma) — dnf path
			buf.WriteString(`#!/bin/bash
set -e

echo "Configuring package repositories..."
dnf install -y epel-release || true
dnf update -y || true

echo "Installing tools..."
dnf install -y dnf-plugins-core yum-utils createrepo_c || true

echo "Resolving and downloading packages..."
mkdir -p /output/repos/offline

dnf install -y --downloadonly --downloaddir=/output/repos/offline `)
			buf.WriteString(strings.Join(pkgs, " "))
			buf.WriteString(` || true

echo "Building local repository..."
createrepo_c /output/repos/offline

echo "Build complete"
ls -la /output/repos/offline/
`)
		}
	}

	if b.OSType.IsDEB() {
		buf.WriteString(`#!/bin/bash
set -e

echo "Updating package lists..."
apt-get update

echo "Installing tools..."
apt-get install -y apt-rdepends dpkg-dev gzip || true

echo "Resolving and downloading packages..."
mkdir -p /output/repos/offline

`)
		for _, pkg := range pkgs {
			buf.WriteString(fmt.Sprintf("cd /output/repos/offline && apt-get download %s 2>/dev/null || true\n", pkg))
		}

		buf.WriteString(`
echo "Building local repository..."
cd /output/repos/offline
dpkg-scanpackages . /dev/null | gzip -9c > Packages.gz

echo "Build complete"
ls -la /output/repos/offline/
`)
	}

	return buf.String(), nil
}

// --- Cross-arch support with Docker Buildx ---

// BuildxPlatform returns the docker buildx platform string.
func BuildxPlatform(arch string) string {
	switch arch {
	case "amd64":
		return "linux/amd64"
	case "arm64":
		return "linux/arm64"
	case "arm":
		return "linux/arm/v7"
	case "ppc64le":
		return "linux/ppc64le"
	case "s390x":
		return "linux/s390x"
	default:
		return "linux/amd64"
	}
}

// BuildMultiArch builds ISOs for multiple architectures using Docker Buildx.
func BuildMultiArch(osType dep.OSType, osVersion string, archs []string, outputDir string, log func(string, ...interface{})) (map[string]string, error) {
	results := make(map[string]string)

	// Enable docker buildx if not already enabled
	cmd := exec.Command("docker", "buildx", "inspect")
	if cmd.Run() != nil {
		createCmd := exec.Command("docker", "buildx", "create", "--name", "kubexm-buildx", "--use")
		if err := createCmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to create buildx builder: %w", err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := 0

	for _, arch := range archs {
		wg.Add(1)
		go func(arch string) {
			defer wg.Done()
			log("Building ISO for %s/%s...", arch, osVersion)

			builder := NewDockerBuilder(osType, osVersion, arch, filepath.Join(outputDir, arch))
			builder.Builder.Log = log

			result, err := builder.Build(nil)
			if err != nil {
				log("Failed to build ISO for %s: %v", arch, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			mu.Lock()
			results[arch] = result.ISOPath
			mu.Unlock()
		}(arch)
	}

	wg.Wait()

	if failed > 0 {
		return results, fmt.Errorf("%d architecture build(s) failed", failed)
	}
	return results, nil
}

// CreateTarball creates a tar.gz archive of the ISO and packages.
func CreateTarball(sourceDir, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(sourceDir, path)
		if relPath == "" || relPath == "." {
			return nil
		}

		header, _ := tar.FileInfoHeader(info, "")
		header.Name = relPath
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer data.Close()
			_, err = io.Copy(tw, data)
			return err
		}
		return nil
	})
}
