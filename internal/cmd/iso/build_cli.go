package iso

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/cmd/iso/build"
	"github.com/mensylisir/kubexm/internal/cmd/iso/dep"
	"github.com/mensylisir/kubexm/internal/cmd/iso/util"
	"github.com/mensylisir/kubexm/internal/logger"
)

type BuildOptions struct {
	// Target OS/arch
	OS           string
	OSVersion    string
	Arch         string

	// Cluster config for dependency resolution
	ClusterConfigFile string

	// Build mode
	Mode       string // "host" or "docker"
	MultiArch  bool
	Registry   string // Docker registry

	// Output
	OutputPath string

	// Package options
	PackagesDir    string
	IncludeKubexm bool

	// Dependency overrides
	Runtime       string
	CNIPlugin     string
	LoadBalancer  string
	StorageType   string
	ExtraPackages []string

	DryRun bool
}

var buildOptions = &BuildOptions{}

func init() {
	IsoCmd.AddCommand(buildIsoCmd)

	buildIsoCmd.Flags().StringVar(&buildOptions.OS, "os", "", "Operating system (ubuntu, centos, rocky, debian) (required for docker mode)")
	buildIsoCmd.Flags().StringVarP(&buildOptions.OSVersion, "version", "v", "", "OS version (e.g., 22.04 for Ubuntu, 7/8 for CentOS) (required for docker mode)")
	buildIsoCmd.Flags().StringVarP(&buildOptions.Arch, "arch", "a", "", "Architecture (amd64, arm64) (default: current host arch)")
	buildIsoCmd.Flags().StringVarP(&buildOptions.ClusterConfigFile, "config", "f", "", "Cluster configuration file for automatic dependency resolution")
	buildIsoCmd.Flags().StringVarP(&buildOptions.Mode, "mode", "m", "host", "Build mode: 'host' (current OS/arch) or 'docker' (cross-platform)")
	buildIsoCmd.Flags().BoolVar(&buildOptions.MultiArch, "multi-arch", false, "Build for multiple architectures using Docker Buildx")
	buildIsoCmd.Flags().StringVar(&buildOptions.Registry, "registry", "", "Docker registry for builder images")
	buildIsoCmd.Flags().StringVarP(&buildOptions.OutputPath, "output", "o", "", "Output directory for ISO file (default: ./output)")
	buildIsoCmd.Flags().StringVarP(&buildOptions.PackagesDir, "packages", "p", "", "Path to kubexm binary packages directory")
	buildIsoCmd.Flags().BoolVar(&buildOptions.IncludeKubexm, "include-kubexm", true, "Include kubexm binary packages in ISO")
	buildIsoCmd.Flags().StringVar(&buildOptions.Runtime, "runtime", "", "Container runtime (containerd, docker, cri-o)")
	buildIsoCmd.Flags().StringVar(&buildOptions.CNIPlugin, "cni", "", "CNI plugin (calico, cilium, flannel, kubeovn, hybridnet)")
	buildIsoCmd.Flags().StringVar(&buildOptions.LoadBalancer, "lb", "", "Load balancer type (kubexm_kh, kubexm_kn, haproxy, nginx)")
	buildIsoCmd.Flags().StringVar(&buildOptions.StorageType, "storage", "", "Storage type (nfs, longhorn, openebs)")
	buildIsoCmd.Flags().StringSliceVar(&buildOptions.ExtraPackages, "extra-pkgs", nil, "Extra packages to include")
	buildIsoCmd.Flags().BoolVar(&buildOptions.DryRun, "dry-run", false, "Show what would be built without building")
}

var buildIsoCmd = &cobra.Command{
	Use:   "build",
	Short: "Build offline installation ISO with OS packages",
	Long: `Build an offline installation ISO containing all necessary OS packages
and dependencies for air-gapped Kubernetes cluster installation.

This command resolves, downloads, and packages:
- OS-level dependencies for Kubernetes (conntrack, socat, iptables, etc.)
- Container runtime OS dependencies
- CNI plugin dependencies
- Load balancer dependencies (keepalived, haproxy, nginx)
- Storage plugin dependencies (nfs-utils, iscsi, etc.)
- System utilities

Two build modes are supported:

  Host Mode (default):
    - Runs directly on the current OS and architecture
    - Uses local package managers (yum/dnf or apt-get)
    - Simple and fast, but limited to current platform

  Docker Mode:
    - Runs in Docker containers for cross-OS/arch support
    - Can build Ubuntu ISOs from CentOS and vice versa
    - Supports multi-architecture builds (amd64 + arm64)

Examples:
  # Build ISO for current OS (host mode)
  kubexm iso build -f config.yaml

  # Build ISO for Ubuntu 22.04 using Docker
  kubexm iso build --os ubuntu --version 22.04 --mode docker

  # Build multi-architecture ISOs (amd64 + arm64)
  kubexm iso build --os ubuntu --version 22.04 --multi-arch --mode docker

  # Build with specific component overrides
  kubexm iso build --os centos --version 8 --cni calico --lb kubexm_kh --storage longhorn

  # Dry run to see what would be included
  kubexm iso build --os ubuntu --version 22.04 --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		// Determine build mode
		mode := build.ModeHost
		if buildOptions.Mode == "docker" {
			mode = build.ModeDocker
		}

		// Determine OS/arch
		osType, osVersion, arch, err := resolveTargetOS(buildOptions.OS, buildOptions.OSVersion, buildOptions.Arch)
		if err != nil {
			return err
		}

		// Determine output directory
		outputDir := buildOptions.OutputPath
		if outputDir == "" {
			outputDir = filepath.Join("output", fmt.Sprintf("%s-%s-%s", osType, osVersion, arch))
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		// Create dependency config from cluster config or flags
		cfg, err := buildDepConfig(buildOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to build dependency config: %w", err)
		}

		// Override from CLI flags
		if buildOptions.Runtime != "" {
			cfg.Runtime = buildOptions.Runtime
		}
		if buildOptions.CNIPlugin != "" {
			cfg.CNIPlugin = buildOptions.CNIPlugin
		}
		if buildOptions.LoadBalancer != "" {
			cfg.LoadBalancer = buildOptions.LoadBalancer
		}
		if buildOptions.StorageType != "" {
			cfg.StorageType = buildOptions.StorageType
		}
		if len(buildOptions.ExtraPackages) > 0 {
			cfg.ExtraPackages = buildOptions.ExtraPackages
		}

		cfg.Arch = arch

		if buildOptions.DryRun {
			showDryRunBuild(cfg, osType, osVersion, arch, mode, outputDir, log)
			return nil
		}

		log.Infof("=== KubeXM ISO Build ===")
		log.Infof("OS: %s %s (%s)", osType, osVersion, arch)
		log.Infof("Mode: %s", mode)
		log.Infof("Output: %s", outputDir)
		log.Infof("Runtime: %s", cfg.Runtime)
		log.Infof("CNI: %s", cfg.CNIPlugin)
		log.Infof("LoadBalancer: %s", cfg.LoadBalancer)
		log.Infof("Storage: %s", cfg.StorageType)

		// Check Docker availability for docker mode
		if mode == build.ModeDocker {
			if err := checkDockerAvailable(log); err != nil {
				return err
			}
		}

		// Perform build
		if buildOptions.MultiArch && mode == build.ModeDocker {
			return runMultiArchBuild(osType, osVersion, arch, outputDir, log)
		}

		return runBuild(mode, osType, osVersion, arch, outputDir, cfg, log)
	},
}

func resolveTargetOS(osFlag, versionFlag, archFlag string) (dep.OSType, string, string, error) {
	// Determine OS type
	var osType dep.OSType
	if osFlag != "" {
		switch strings.ToLower(osFlag) {
		case "ubuntu":
			osType = dep.OSTypeUbuntu
		case "centos":
			osType = dep.OSTypeCentOS
		case "rocky":
			osType = dep.OSTypeRocky
		case "rhel", "redhat":
			osType = dep.OSTypeRHEL
		case "debian":
			osType = dep.OSTypeDebian
		default:
			return "", "", "", fmt.Errorf("unsupported OS: %s (supported: centos, rocky, rhel, oracle, almalinux, fedora, uos, anolis, openeuler, kylin, ubuntu, debian)", osFlag)
		}
	} else {
		// Detect from current system
		detected, version := dep.DetectOS()
		if detected == dep.OSTypeUnknown {
			return "", "", "", fmt.Errorf("cannot detect OS. Please specify --os explicitly")
		}
		osType = detected
		versionFlag = version
	}

	// Determine version
	if versionFlag == "" {
		if osType.IsRPM() {
			versionFlag = "7"
		} else {
			versionFlag = "22.04"
		}
	}

	// Determine arch
	arch := archFlag
	if arch == "" {
		if osType.IsDEB() {
			arch = dep.DetectHostArch()
		} else {
			arch = dep.DetectHostArchRPM()
		}
	}

	// Normalize architecture
	switch strings.ToLower(arch) {
	case "x86_64", "amd64":
		arch = "amd64"
	case "aarch64", "arm64":
		arch = "arm64"
	case "x86", "i386", "i686":
		arch = "amd64" // x86 builds default to amd64
	}

	return osType, versionFlag, arch, nil
}

func buildDepConfig(configFile string) (*dep.ClusterDepConfig, error) {
	cfg := &dep.ClusterDepConfig{
		ExtraPackages: []string{},
	}

	if configFile == "" {
		return cfg, nil
	}

	// Try to parse the cluster config
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return cfg, nil
	}

	// Try to load as kubexm cluster config
	data, err := os.ReadFile(absPath)
	if err != nil {
		return cfg, nil
	}

	// Extract key values from YAML (lightweight parsing without full schema)
	content := string(data)

	// Extract runtime
	if idx := strings.Index(content, "containerRuntime:"); idx >= 0 {
		line := extractYAMLValue(content[idx:])
		for _, rt := range []string{"containerd", "docker", "cri-o", "isula"} {
			if strings.Contains(line, rt) {
				cfg.Runtime = rt
				break
			}
		}
	}

	// Extract CNI plugin
	if idx := strings.Index(content, "plugin:"); idx >= 0 {
		line := extractYAMLValue(content[idx:])
		for _, cni := range []string{"calico", "cilium", "flannel", "kubeovn", "hybridnet", "multus"} {
			if strings.Contains(line, cni) {
				cfg.CNIPlugin = cni
				break
			}
		}
	}

	// Extract load balancer
	if idx := strings.Index(content, "loadbalancer"); idx >= 0 {
		line := extractYAMLValue(content[idx:])
		for _, lb := range []string{"kubexm_kh", "kubexm_kn", "haproxy", "nginx"} {
			if strings.Contains(line, lb) {
				cfg.LoadBalancer = lb
				break
			}
		}
	}

	// Extract storage
	if idx := strings.Index(content, "storage:"); idx >= 0 {
		line := extractYAMLValue(content[idx:])
		for _, st := range []string{"nfs", "longhorn", "openebs"} {
			if strings.Contains(line, st) {
				cfg.StorageType = st
				break
			}
		}
	}

	return cfg, nil
}

func extractYAMLValue(from string) string {
	lines := strings.Split(from, "\n")
	if len(lines) < 2 {
		return ""
	}
	// Get the value on the next non-empty line
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			return line
		}
	}
	return ""
}

func checkDockerAvailable(log *logger.Logger) error {
	cmd := exec.Command("docker", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not available. Please install docker first")
	}

	cmd = exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not running. Please start docker")
	}

	log.Infof("Docker available")
	return nil
}

func showDryRunBuild(cfg *dep.ClusterDepConfig, osType dep.OSType, osVersion, arch string, mode build.Mode, outputDir string, log *logger.Logger) {
	log.Info("=== DRY RUN MODE ===")
	log.Infof("Would build ISO:")
	log.Infof("  OS: %s %s (%s)", osType, osVersion, arch)
	log.Infof("  Mode: %s", mode)
	log.Infof("  Output: %s", outputDir)
	log.Infof("")
	log.Infof("Would resolve dependencies for:")
	log.Infof("  Runtime: %s", cfg.Runtime)
	log.Infof("  CNI Plugin: %s", cfg.CNIPlugin)
	log.Infof("  Load Balancer: %s", cfg.LoadBalancer)
	log.Infof("  Storage: %s", cfg.StorageType)

	// Show what packages would be included
	ps := dep.NewPackageSet()
	log.Infof("")
	log.Infof("Core packages (%d):", len(ps.K8sPrereqs))
	for _, pkg := range ps.K8sPrereqs {
		log.Infof("  - %s", pkg)
	}

	if deps, ok := ps.RuntimeDeps[cfg.Runtime]; ok && len(deps) > 0 {
		log.Infof("")
		log.Infof("Runtime deps (%s):", cfg.Runtime)
		for _, pkg := range deps {
			log.Infof("  - %s", pkg)
		}
	}

	if deps, ok := ps.LoadBalancerDeps[cfg.LoadBalancer]; ok && len(deps) > 0 {
		log.Infof("")
		log.Infof("Load balancer deps (%s):", cfg.LoadBalancer)
		for _, pkg := range deps {
			log.Infof("  - %s", pkg)
		}
	}

	if deps, ok := ps.StorageDeps[cfg.StorageType]; ok && len(deps) > 0 {
		log.Infof("")
		log.Infof("Storage deps (%s):", cfg.StorageType)
		for _, pkg := range deps {
			log.Infof("  - %s", pkg)
		}
	}

	log.Infof("")
	log.Infof("Would create:")
	log.Infof("  - Local package repository (yum/apt)")
	log.Infof("  - Repo configuration files")
	log.Infof("  - Installation scripts")
	log.Infof("  - Final ISO image")
}

func runBuild(mode build.Mode, osType dep.OSType, osVersion, arch, outputDir string, cfg *dep.ClusterDepConfig, log *logger.Logger) error {
	cfg.OS = osType
	cfg.OSVersion = osVersion

	var result *build.BuildResult
	var err error

	if mode == build.ModeDocker {
		builder := build.NewDockerBuilder(osType, osVersion, arch, outputDir)
		builder.Builder.Log = log.Infof
		result, err = builder.Build(cfg)
	} else {
		builder := build.NewBuilder(mode, osType, osVersion, arch, outputDir)
		builder.Log = log.Infof
		result, err = builder.Build(cfg)
	}

	if err != nil {
		return err
	}

	log.Infof("=== Build Complete ===")
	log.Infof("ISO: %s", result.ISOPath)
	log.Infof("Size: %s", util.FormatSize(result.ISOSize))
	log.Infof("Packages: %d", result.PackagesCount)
	log.Infof("Duration: %v", result.Duration.Round(time.Second))

	return nil
}

func runMultiArchBuild(osType dep.OSType, osVersion, arch, outputDir string, log *logger.Logger) error {
	archs := []string{arch}
	if !contains(archs, "amd64") {
		archs = append(archs, "amd64")
	}
	if !contains(archs, "arm64") {
		archs = append(archs, "arm64")
	}

	results, err := build.BuildMultiArch(osType, osVersion, archs, outputDir, log.Infof)
	if err != nil {
		return fmt.Errorf("multi-arch build failed: %w", err)
	}

	log.Infof("=== Multi-Arch Build Complete ===")
	for arch, isoPath := range results {
		size, _ := getFileSizeISO(isoPath)
		log.Infof("  %s: %s (%s)", arch, isoPath, util.FormatSize(size))
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getFileSizeISO(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

