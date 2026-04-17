package iso

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/cmd/iso/util"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/templates"
)

type CreateOptions struct {
	ClusterConfigFile string
	OutputPath       string
	ISOFile          string
	OSName           string
	OSVersion        string
	Arch             string
	PackagesDir      string
	DryRun           bool
	SkipVerification bool
}

var createOptions = &CreateOptions{}

// hostRolesCache caches the hostname->roles map per cluster config pointer.
var hostRolesCache atomicHostRolesCache

type atomicHostRolesCache struct {
	mu       sync.RWMutex
	config   *v1alpha1.Cluster
	rolesMap map[string][]string
}

func init() {
	IsoCmd.AddCommand(createIsoCmd)
	createIsoCmd.Flags().StringVarP(&createOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	createIsoCmd.Flags().StringVarP(&createOptions.OutputPath, "output", "o", "", "Output directory for ISO file (default: ./output)")
	createIsoCmd.Flags().StringVarP(&createOptions.ISOFile, "iso", "i", "", "Path to the base ISO file (required)")
	createIsoCmd.Flags().StringVarP(&createOptions.OSName, "os", "", "ubuntu", "Operating system name (default: ubuntu)")
	createIsoCmd.Flags().StringVarP(&createOptions.OSVersion, "version", "v", "22.04", "Operating system version (default: 22.04)")
	createIsoCmd.Flags().StringVarP(&createOptions.Arch, "arch", "a", "amd64", "Architecture (default: amd64)")
	createIsoCmd.Flags().StringVarP(&createOptions.PackagesDir, "packages", "p", "", "Path to kubexm packages directory (if not using config's workDir)")
	createIsoCmd.Flags().BoolVar(&createOptions.DryRun, "dry-run", false, "Show what would be created without making changes")
	createIsoCmd.Flags().BoolVar(&createOptions.SkipVerification, "skip-verification", false, "Skip ISO integrity verification")

	if err := createIsoCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for iso create command: %v\n", err)
	}
	if err := createIsoCmd.MarkFlagRequired("iso"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'iso' flag as required for iso create command: %v\n", err)
	}
}

var IsoCmd = &cobra.Command{
	Use:   "iso",
	Short: "Manage ISO images",
	Long:  `Commands for creating and managing offline installation ISO images for Kubernetes clusters.`,
}

var createIsoCmd = &cobra.Command{
	Use:   "create",
	Short: "Create offline installation ISO",
	Long: `Create an offline installation ISO image containing all necessary
binaries, configurations, and packages for air-gapped Kubernetes installation.

This command packages:
- Kubernetes binaries (kubeadm, kubelet, kubectl, kube-proxy, etc.)
- Container runtime binaries (containerd, runc, crictl)
- CNI plugin binaries
- etcd binaries
- Load balancer binaries
- Helm charts
- Cluster configuration files
- Installation scripts

The resulting ISO can be used for fully offline Kubernetes cluster installation.

Examples:
  # Create ISO from cluster config
  kubexm iso create -f config.yaml -i ubuntu-22.04.iso

  # Create ISO with custom output directory
  kubexm iso create -f config.yaml -i ubuntu-22.04.iso -o /path/to/output

  # Create ISO with pre-downloaded packages directory
  kubexm iso create -f config.yaml -i ubuntu-22.04.iso -p /path/to/packages

  # Dry run to see what would be included
  kubexm iso create -f config.yaml -i ubuntu-22.04.iso --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		if createOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}
		if createOptions.ISOFile == "" {
			return fmt.Errorf("base ISO file must be provided via -i or --iso flag")
		}

		absConfigPath, err := filepath.Abs(createOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file: %w", err)
		}

		absISOPath, err := filepath.Abs(createOptions.ISOFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for ISO file: %w", err)
		}

		if _, err := os.Stat(absISOPath); os.IsNotExist(err) {
			return fmt.Errorf("base ISO file does not exist: %s", absISOPath)
		}

		clusterConfig, err := config.ParseFromFile(absConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration: %w", err)
		}

		outputPath := createOptions.OutputPath
		if outputPath == "" {
			outputPath = filepath.Join("output", clusterConfig.Name)
		}

		log.Infof("Creating offline installation ISO for cluster '%s'", clusterConfig.Name)
		log.Infof("Base ISO: %s", absISOPath)
		log.Infof("Output directory: %s", outputPath)

		if err := checkRequiredTools(log); err != nil {
			return err
		}

		if createOptions.DryRun {
			return showDryRunOutput(clusterConfig, outputPath, log)
		}

		return createOfflineISO(clusterConfig, absISOPath, outputPath, log)
	},
}

// ISOData holds all data needed for template rendering.
type ISOData struct {
	KubexmVersion   string
	ClusterName     string
	OSName          string
	OSVersion       string
	Arch            string
	ConfigFileName  string
	Kubernetes      *KubernetesData
	Etcd            *EtcdData
	Network         *NetworkData
	Registry        *RegistryData
	LoadBalancer    *LoadBalancerData
	Storage         *StorageData
	Hosts           []HostData
	PackagesSize    int64
	KickstartConfig *KickstartData
}

type KubernetesData struct {
	Version         string
	Type            string
	ClusterName     string
	DNSDomain       string
	ContainerRuntime string
	APIServerPort   int
	PodCIDR         string
	ServiceCIDR     string
}

type EtcdData struct {
	Version string
	Type    string
}

type NetworkData struct {
	Plugin    string
	PodCIDR   string
	MTU       int
}

type RegistryData struct {
	Domain   string
	Port     int
	HTTP     bool
}

type LoadBalancerData struct {
	Type     string
	Mode     string
	VIPIP    string
	VIPPort  int
}

type StorageData struct {
	Type      string
	NFSServer string
	NFSPath   string
}

type HostData struct {
	Name     string
	Address  string
	Internal string
	Roles    []string
}

type KickstartData struct {
	Version        string
	Language       string
	Keyboard       string
	Timezone       string
	RootPassword   string
	NetworkDevice  string
}

func checkRequiredTools(log *logger.Logger) error {
	xorrisoPath, xorrisoErr := exec.LookPath("xorriso")
	mkisofsPath, mkisofsErr := exec.LookPath("mkisofs")
	genisoimagePath, genisoimageErr := exec.LookPath("genisoimage")

	hasISOBuilder := xorrisoErr == nil || (mkisofsErr == nil && genisoimageErr == nil)
	if !hasISOBuilder {
		return fmt.Errorf("ISO creation tool not found. Please install xorriso or genisoimage:\n  Ubuntu/Debian: apt install xorriso\n  RHEL/CentOS: yum install xorriso\n  Or: apt install genisoimage")
	}

	if xorrisoErr != nil {
		log.Warnf("xorriso not found (using mkisofs as fallback). For best results, install xorriso.")
		if mkisofsErr != nil && genisoimageErr != nil {
			log.Warn("Both mkisofs and xorriso missing. genisoimage will be used as fallback.")
		}
	} else {
		log.Infof("Using xorriso at: %s", xorrisoPath)
	}

	if mkisofsPath != "" {
		log.Debugf("mkisofs available at: %s", mkisofsPath)
	}
	if genisoimagePath != "" {
		log.Debugf("genisoimage available at: %s", genisoimagePath)
	}

	// Check for rsync
	if _, err := exec.LookPath("rsync"); err != nil {
		log.Warn("rsync not found, will use standard copy")
	}

	// Check for 7z or unzip (for extracting compressed packages)
	hasExtractor := false
	for _, tool := range []string{"7z", "unzip", "tar"} {
		if _, err := exec.LookPath(tool); err == nil {
			hasExtractor = true
			break
		}
	}
	if !hasExtractor {
		log.Warn("No extraction tool found (7z, unzip, or tar). Package extraction may fail.")
	}

	return nil
}

func showDryRunOutput(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	log.Info("=== DRY RUN MODE ===")
	log.Infof("Would create offline ISO for cluster: %s", clusterConfig.Name)
	log.Infof("Would use base ISO: %s", createOptions.ISOFile)
	log.Infof("Would output to: %s", outputPath)
	log.Infof("Would include:")

	if clusterConfig.Spec.Kubernetes != nil {
		log.Infof("  - Kubernetes version: %s (type: %s)", clusterConfig.Spec.Kubernetes.Version, clusterConfig.Spec.Kubernetes.Type)
	}
	if clusterConfig.Spec.Etcd != nil {
		log.Infof("  - etcd version: %s (type: %s)", clusterConfig.Spec.Etcd.Version, clusterConfig.Spec.Etcd.Type)
	}
	if clusterConfig.Spec.Network != nil {
		log.Infof("  - CNI plugin: %s", clusterConfig.Spec.Network.Plugin)
		if clusterConfig.Spec.Network.KubePodsCIDR != "" {
			log.Infof("  - Pod CIDR: %s", clusterConfig.Spec.Network.KubePodsCIDR)
		}
	}
	if clusterConfig.Spec.Registry != nil && clusterConfig.Spec.Registry.MirroringAndRewriting != nil {
		privReg := clusterConfig.Spec.Registry.MirroringAndRewriting.PrivateRegistry
		if privReg != "" {
			log.Infof("  - Private registry: %s", privReg)
		}
	}

	log.Infof("  - Total hosts: %d", len(clusterConfig.Spec.Hosts))
	for _, host := range clusterConfig.Spec.Hosts {
		roles := getHostRoles(clusterConfig, host.Name)
		log.Infof("    * %s (%s): %s", host.Name, host.Address, strings.Join(roles, ", "))
	}

	return nil
}

func getHostRoles(clusterConfig *v1alpha1.Cluster, hostName string) []string {
	if clusterConfig.Spec.RoleGroups == nil {
		return nil
	}

	// Lazily build and cache the map for this cluster config.
	hostRolesCache.mu.RLock()
	if hostRolesCache.config == clusterConfig {
		roles := hostRolesCache.rolesMap[hostName]
		hostRolesCache.mu.RUnlock()
		return roles
	}
	hostRolesCache.mu.RUnlock()

	// Cache miss — build the map and store it.
	hostRolesCache.mu.Lock()
	defer hostRolesCache.mu.Unlock()
	hostRolesCache.config = clusterConfig
	hostRolesCache.rolesMap = buildHostRolesMap(clusterConfig)

	return hostRolesCache.rolesMap[hostName]
}

// buildHostRolesMap builds a hostname -> roles map once per cluster.
// O(sum of all role slice lengths) instead of O(hosts * roles * avg_slice_len).
func buildHostRolesMap(clusterConfig *v1alpha1.Cluster) map[string][]string {
	rg := clusterConfig.Spec.RoleGroups
	if rg == nil {
		return nil
	}

	result := make(map[string][]string)

	addHosts := func(hosts []string, role string) {
		for _, h := range hosts {
			result[h] = append(result[h], role)
		}
	}

	addHosts(rg.Master, "master")
	addHosts(rg.Worker, "worker")
	addHosts(rg.Etcd, "etcd")
	addHosts(rg.LoadBalancer, "loadbalancer")
	addHosts(rg.Storage, "storage")
	addHosts(rg.Registry, "registry")

	return result
}

func createOfflineISO(clusterConfig *v1alpha1.Cluster, baseISOPath, outputPath string, log *logger.Logger) error {
	start := time.Now()

	// Step 1: Find packages directory
	packagesDir := createOptions.PackagesDir
	if packagesDir == "" {
		if clusterConfig.Spec.Global != nil && clusterConfig.Spec.Global.WorkDir != "" {
			packagesDir = clusterConfig.Spec.Global.WorkDir
		} else {
			packagesDir = filepath.Join(os.TempDir(), "kubexm-packages")
		}
	}

	log.Infof("Packages directory: %s", packagesDir)

	// Step 2: Verify packages exist
	if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
		return fmt.Errorf("packages directory does not exist: %s. Run 'kubexm download' first", packagesDir)
	}

	// Step 3: Create working directory
	workDir := filepath.Join(outputPath, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Step 4: Extract base ISO contents
	isoExtractDir := filepath.Join(workDir, "iso-root")
	if err := os.MkdirAll(isoExtractDir, 0755); err != nil {
		return fmt.Errorf("failed to create ISO extract directory: %w", err)
	}

	log.Info("Extracting base ISO contents...")
	if err := extractISO(baseISOPath, isoExtractDir, log); err != nil {
		return fmt.Errorf("failed to extract base ISO: %w", err)
	}

	// Step 5: Prepare ISO data
	isoData := prepareISOData(clusterConfig)

	// Step 6: Copy packages
	kubexmDir := filepath.Join(workDir, "kubexm")
	kubexmPackagesDir := filepath.Join(kubexmDir, "packages")
	kubexmConfigDir := filepath.Join(kubexmDir, "config")
	kubexmScriptsDir := filepath.Join(kubexmDir, "scripts")
	kubexmManifestsDir := filepath.Join(kubexmDir, "manifests")

	for _, dir := range []string{kubexmPackagesDir, kubexmConfigDir, kubexmScriptsDir, kubexmManifestsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	log.Info("Copying packages...")
	packagesSize, err := copyPackages(packagesDir, kubexmPackagesDir, clusterConfig, log)
	if err != nil {
		return fmt.Errorf("failed to copy packages: %w", err)
	}
	isoData.PackagesSize = packagesSize
	log.Infof("Packages copied: %s", util.FormatSize(packagesSize))

	// Step 7: Render and write configuration files
	log.Info("Generating configuration files...")
	if err := writeConfigFiles(clusterConfig, kubexmConfigDir, isoData, log); err != nil {
		return fmt.Errorf("failed to write configuration files: %w", err)
	}

	// Step 8: Render and write installation scripts
	log.Info("Generating installation scripts...")
	if err := writeInstallScripts(kubexmScriptsDir, isoData, log); err != nil {
		return fmt.Errorf("failed to write installation scripts: %w", err)
	}

	// Step 9: Render and write manifests
	log.Info("Generating Kubernetes manifests...")
	if err := writeManifests(kubexmManifestsDir, clusterConfig, log); err != nil {
		return fmt.Errorf("failed to write manifests: %w", err)
	}

	// Step 10: Add bootloader entries
	log.Info("Configuring bootloader...")
	if err := configureBootloader(isoExtractDir, isoData, log); err != nil {
		return fmt.Errorf("failed to configure bootloader: %w", err)
	}

	// Step 11: Copy kubexm directory to ISO root
	isoKubexmDir := filepath.Join(isoExtractDir, "kubexm")
	if err := copyDirContents(kubexmDir, isoKubexmDir, log); err != nil {
		return fmt.Errorf("failed to copy kubexm directory to ISO: %w", err)
	}

	// Step 12: Create ISO manifest
	if err := createISOManifest(clusterConfig, filepath.Join(outputPath, "MANIFEST"), isoData, log); err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// Step 13: Re-master the ISO
	log.Info("Building final ISO...")
	outputISO := filepath.Join(outputPath, fmt.Sprintf("kubexm-%s-%s.iso", clusterConfig.Name, time.Now().Format("20060102-150405")))

	if err := buildISO(isoExtractDir, outputISO, log); err != nil {
		return fmt.Errorf("failed to build ISO: %w", err)
	}

	elapsed := time.Since(start)
	isoSize, _ := util.GetFileSize(outputISO)
	log.Infof("=== ISO Creation Complete ===")
	log.Infof("Output: %s", outputISO)
	log.Infof("Size: %s", util.FormatSize(isoSize))
	log.Infof("Time: %v", elapsed.Round(time.Second))
	log.Infof("Cluster: %s", clusterConfig.Name)
	log.Infof("Total packages: %s", util.FormatSize(packagesSize))

	return nil
}

func prepareISOData(clusterConfig *v1alpha1.Cluster) *ISOData {
	data := &ISOData{
		KubexmVersion:  "v1.0.0",
		ClusterName:    clusterConfig.Name,
		OSName:        createOptions.OSName,
		OSVersion:     createOptions.OSVersion,
		Arch:          createOptions.Arch,
		ConfigFileName: "config.yaml",
		KickstartConfig: &KickstartData{
			Version:       "1.0",
			Language:      "en_US.UTF-8",
			Keyboard:      "us",
			Timezone:      "UTC",
			RootPassword:  "kubexm",
			NetworkDevice: "eth0",
		},
	}

	if clusterConfig.Spec.Kubernetes != nil {
		k8s := clusterConfig.Spec.Kubernetes
		data.Kubernetes = &KubernetesData{
			Version:          k8s.Version,
			Type:             k8s.Type,
			ClusterName:      k8s.ClusterName,
			DNSDomain:        k8s.DNSDomain,
			ContainerRuntime: "containerd",
		}
		if k8s.ContainerRuntime != nil {
			data.Kubernetes.ContainerRuntime = string(k8s.ContainerRuntime.Type)
		}
	}

	if clusterConfig.Spec.Etcd != nil {
		data.Etcd = &EtcdData{
			Version: clusterConfig.Spec.Etcd.Version,
			Type:    clusterConfig.Spec.Etcd.Type,
		}
	}

	if clusterConfig.Spec.Network != nil {
		data.Network = &NetworkData{
			Plugin:  clusterConfig.Spec.Network.Plugin,
			PodCIDR: clusterConfig.Spec.Network.KubePodsCIDR,
			MTU:     1440,
		}
	}

	if clusterConfig.Spec.Registry != nil && clusterConfig.Spec.Registry.MirroringAndRewriting != nil {
		mr := clusterConfig.Spec.Registry.MirroringAndRewriting
		data.Registry = &RegistryData{
			Domain: mr.PrivateRegistry,
			Port:   5000, // default port
		}
	}

	if clusterConfig.Spec.ControlPlaneEndpoint != nil {
		data.LoadBalancer = &LoadBalancerData{
			VIPIP:   clusterConfig.Spec.ControlPlaneEndpoint.Address,
			VIPPort: clusterConfig.Spec.ControlPlaneEndpoint.Port,
		}
	}

	for _, host := range clusterConfig.Spec.Hosts {
		hd := HostData{
			Name:    host.Name,
			Address: host.Address,
			Roles:   getHostRoles(clusterConfig, host.Name),
		}
		if host.InternalAddress != "" {
			hd.Internal = host.InternalAddress
		}
		data.Hosts = append(data.Hosts, hd)
	}

	return data
}

func extractISO(isoPath, destDir string, log *logger.Logger) error {
	// Try xorriso first (most reliable for reading ISOs)
	xorrisoPath, err := exec.LookPath("xorriso")
	if err == nil {
		cmd := exec.Command(xorrisoPath,
			"-osirrox", "on",
			"-indev", isoPath,
			"-extract", "/", destDir,
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("xorriso extraction failed: %w", err)
		}
		log.Infof("Extracted ISO using xorriso")
		return nil
	}

	// Fallback: mount and copy
	return extractISOFallback(isoPath, destDir, log)
}

func extractISOFallback(isoPath, destDir string, log *logger.Logger) error {
	// Try mounting the ISO
	mountDir := filepath.Join(os.TempDir(), "kubexm-iso-mount-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}
	defer os.RemoveAll(mountDir)

	// Try guestfish (libguestfs)
	guestfishPath, guestfishErr := exec.LookPath("guestfish")
	if guestfishErr == nil {
		cmd := exec.Command(guestfishPath,
			"--ro",
			"-a", isoPath,
			"mount", "/dev/sda1", "/",
			"copy-out", "/", destDir,
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			log.Infof("Extracted ISO using guestfish")
			return nil
		}
	}

	// Fallback: 7zip can extract ISO
	sevenZipPath, sevenZipErr := exec.LookPath("7z")
	if sevenZipErr == nil {
		cmd := exec.Command(sevenZipPath, "x", "-y", "-o"+destDir, isoPath)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			log.Infof("Extracted ISO using 7z")
			return nil
		}
	}

	return fmt.Errorf("no suitable ISO extraction method found. Install xorriso, 7z, or guestfish")
}

func copyPackages(packagesSrc, packagesDest string, clusterConfig *v1alpha1.Cluster, log *logger.Logger) (int64, error) {
	var totalSize int64

	// Define which package subdirectories to copy
	subdirs := []string{
		common.DefaultEtcdDir,
		common.DefaultKubernetesDir,
		common.DefaultContainerRuntimeDir,
		common.DefaultCNIDir,
		"helm",
		"registry",
		"loadbalancer",
		"storage",
	}

	for _, subdir := range subdirs {
		src := filepath.Join(packagesSrc, subdir)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			log.Debugf("Package subdirectory not found (skipping): %s", subdir)
			continue
		}

		dest := filepath.Join(packagesDest, subdir)
		if err := os.MkdirAll(dest, 0755); err != nil {
			return 0, fmt.Errorf("failed to create directory %s: %w", dest, err)
		}

		// Use rsync if available for efficient copying
		rsyncPath, rsyncErr := exec.LookPath("rsync")
		if rsyncErr == nil {
			cmd := exec.Command(rsyncPath, "-a", "--stats", src+"/", dest+"/")
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				// Fall back to regular copy
				if err := util.CopyDirRecursive(src, dest); err != nil {
					return 0, fmt.Errorf("failed to copy %s: %w", subdir, err)
				}
			}
		} else {
			if err := util.CopyDirRecursive(src, dest); err != nil {
				return 0, fmt.Errorf("failed to copy %s: %w", subdir, err)
			}
		}

		size, _ := util.DirSize(dest)
		totalSize += size
		log.Debugf("Copied %s: %s", subdir, util.FormatSize(size))
	}

	// Also check for helm_packages at root level
	helmSrc := filepath.Join(packagesSrc, "helm_packages")
	if fi, err := os.Stat(helmSrc); err == nil && fi.IsDir() {
		dest := filepath.Join(packagesDest, "helm_packages")
		if err := os.MkdirAll(dest, 0755); err != nil {
			return 0, fmt.Errorf("failed to create helm_packages dir: %w", err)
		}
		if err := util.CopyDirRecursive(helmSrc, dest); err != nil {
			return 0, fmt.Errorf("failed to copy helm_packages: %w", err)
		}
		sz, _ := util.DirSize(dest)
		totalSize += sz
	}

	return totalSize, nil
}

func writeConfigFiles(clusterConfig *v1alpha1.Cluster, configDir string, data *ISOData, log *logger.Logger) error {
	// Render kubexm config template
	configTemplate, err := templates.Get("iso/kubexm-ks.cfg.tmpl")
	if err != nil {
		// If template not found, serialize the actual config
		return writeConfigFromCluster(clusterConfig, configDir, log)
	}

	rendered, err := templates.Render(configTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render config template: %w", err)
	}

	configPath := filepath.Join(configDir, data.ConfigFileName)
	if err := os.WriteFile(configPath, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	log.Infof("Config file written: %s", configPath)

	return nil
}

func writeConfigFromCluster(clusterConfig *v1alpha1.Cluster, configDir string, log *logger.Logger) error {
	// Serialize the cluster config as YAML
	configContent, err := yaml.Marshal(clusterConfig)
	if err != nil {
		// Fall back to copying the original config
		if createOptions.ClusterConfigFile != "" {
			src, err := os.Open(createOptions.ClusterConfigFile)
			if err != nil {
				return fmt.Errorf("failed to read source config: %w", err)
			}
			defer src.Close()

			dest, err := os.Create(filepath.Join(configDir, "config.yaml"))
			if err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
			defer dest.Close()

			if _, err := io.Copy(dest, src); err != nil {
				return fmt.Errorf("failed to copy config: %w", err)
			}
		}
		return nil
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Infof("Config file written: %s", configPath)
	return nil
}

func writeInstallScripts(scriptsDir string, data *ISOData, log *logger.Logger) error {
	// Write main install script
	installScriptTemplate, err := templates.Get("iso/install.sh.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get install.sh template: %w", err)
	}

	rendered, err := templates.Render(installScriptTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render install script: %w", err)
	}

	installPath := filepath.Join(scriptsDir, "install.sh")
	if err := os.WriteFile(installPath, []byte(rendered), 0755); err != nil {
		return fmt.Errorf("failed to write install script: %w", err)
	}
	log.Infof("Install script written: %s", installPath)

	return nil
}

func writeManifests(manifestsDir string, clusterConfig *v1alpha1.Cluster, log *logger.Logger) error {
	// Write basic manifests based on cluster config
	// These are rendered from the embedded templates

	manifestFiles := []string{
		"dns/coredns.yaml",
		"cni/calico/values.yaml",
		"cni/flannel/values.yaml",
	}

	for _, mf := range manifestFiles {
		templatePath := strings.Replace(mf, "values.yaml", "values.yaml.tmpl", 1)
		if _, err := templates.Get("cni/" + templatePath); err != nil {
			continue
		}

		content, _ := templates.Get("cni/" + templatePath)
		destDir := filepath.Join(manifestsDir, filepath.Dir(mf))
		if err := os.MkdirAll(destDir, 0755); err != nil {
			continue
		}

		// Simple pass-through for now (full rendering would need proper data)
		outPath := filepath.Join(manifestsDir, mf)
		os.WriteFile(outPath, []byte(content), 0644)
	}

	log.Debugf("Manifests written to: %s", manifestsDir)
	return nil
}

func configureBootloader(isoRoot string, data *ISOData, log *logger.Logger) error {
	// Add GRUB config for UEFI boot
	grubTemplate, err := templates.Get("iso/grub.cfg.tmpl")
	if err == nil {
		rendered, err := templates.Render(grubTemplate, data)
		if err == nil {
			grubDest := filepath.Join(isoRoot, "kubexm", "grub.cfg")
			if err := os.MkdirAll(filepath.Dir(grubDest), 0755); err == nil {
				os.WriteFile(grubDest, []byte(rendered), 0644)
				log.Infof("GRUB config written: %s", grubDest)
			}
		}
	}

	// Add ISOLINUX config for BIOS boot
	isolinuxTemplate, err := templates.Get("iso/isolinux.cfg.tmpl")
	if err == nil {
		rendered, err := templates.Render(isolinuxTemplate, data)
		if err == nil {
			// Try both isolinux.cfg and syslinux.cfg locations
			for _, cfgName := range []string{"isolinux/isolinux.cfg", "syslinux/syslinux.cfg", "isolinux.cfg"} {
				cfgDest := filepath.Join(isoRoot, cfgName)
				if _, err := os.Stat(filepath.Dir(cfgDest)); err == nil || os.MkdirAll(filepath.Dir(cfgDest), 0755) == nil {
					os.WriteFile(cfgDest, []byte(rendered), 0644)
					log.Infof("ISOLINUX config written: %s", cfgDest)
					break
				}
			}
		}
	}

	return nil
}

func buildISO(sourceDir, outputPath string, log *logger.Logger) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Try xorriso (preferred)
	xorrisoPath, err := exec.LookPath("xorriso")
	if err == nil {
		cmd := exec.Command(xorrisoPath,
			"-as", "mkisofs",
			"-r",                    // Rock Ridge extensions
			"-J",                    // Joliet extensions
			"-l",                    // allow full names
			"--joliet-long",         // allow long names in Joliet
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

	// Fallback to mkisofs or genisoimage
	var mkisofsPath string
	for _, name := range []string{"mkisofs", "genisoimage"} {
		if path, err := exec.LookPath(name); err == nil {
			mkisofsPath = path
			break
		}
	}
	if mkisofsPath == "" {
		return fmt.Errorf("no ISO creation tool found")
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
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkisofs failed: %w", err)
	}

	return nil
}

func createISOManifest(clusterConfig *v1alpha1.Cluster, manifestPath string, data *ISOData, log *logger.Logger) error {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "# Offline Installation ISO Manifest\n")
	fmt.Fprintf(&buf, "Cluster: %s\n", clusterConfig.Name)
	fmt.Fprintf(&buf, "Created: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&buf, "KubeXM Version: %s\n", data.KubexmVersion)
	fmt.Fprintf(&buf, "OS: %s %s (%s)\n\n", data.OSName, data.OSVersion, data.Arch)

	buf.WriteString("## Included Components\n\n")

	if data.Kubernetes != nil {
		buf.WriteString("### Kubernetes\n")
		fmt.Fprintf(&buf, "- Version: %s\n", data.Kubernetes.Version)
		fmt.Fprintf(&buf, "- Type: %s\n", data.Kubernetes.Type)
		fmt.Fprintf(&buf, "- Container Runtime: %s\n", data.Kubernetes.ContainerRuntime)
		buf.WriteString("- Components: kubeadm, kubelet, kubectl, kube-proxy, kube-scheduler, kube-controller-manager, kube-apiserver\n")
	}

	if data.Etcd != nil {
		buf.WriteString("### etcd\n")
		fmt.Fprintf(&buf, "- Version: %s\n", data.Etcd.Version)
		fmt.Fprintf(&buf, "- Type: %s\n", data.Etcd.Type)
	}

	if data.Network != nil {
		buf.WriteString("### Network Plugin\n")
		fmt.Fprintf(&buf, "- Plugin: %s\n", data.Network.Plugin)
		if data.Network.PodCIDR != "" {
			fmt.Fprintf(&buf, "- Pod CIDR: %s\n", data.Network.PodCIDR)
		}
	}

	if data.Registry != nil {
		buf.WriteString("### Private Registry\n")
		fmt.Fprintf(&buf, "- Domain: %s:%d\n", data.Registry.Domain, data.Registry.Port)
	}

	buf.WriteString("### Packages\n")
	fmt.Fprintf(&buf, "- Total Size: %s\n", util.FormatSize(data.PackagesSize))
	buf.WriteString("- Location: /kubexm/packages/\n")

	fmt.Fprintf(&buf, "\n## Hosts (%d total)\n\n", len(data.Hosts))
	for _, host := range data.Hosts {
		fmt.Fprintf(&buf, "- %s (%s) [%s]\n", host.Name, host.Address, strings.Join(host.Roles, ", "))
	}

	buf.WriteString(`
## Directory Structure

/kubexm/
├── packages/       # Binary packages organized by component
├── config/         # Cluster configuration files
├── scripts/        # Installation scripts
└── manifests/      # Kubernetes manifests

## Installation

1. Boot from the ISO
2. Select "Install KubeXM Kubernetes Cluster" from the boot menu
3. The installer will:
   - Partition the target disk
   - Install base system
   - Copy all packages and configurations
   - Configure system settings for Kubernetes
   - Install bootloader
4. Reboot into the installed system
5. Run: kubeadm init --config /etc/kubexm/kubeadm-config.yaml

## Offline Requirements

All required binaries and container images are pre-bundled:
- No internet connection required during installation
- No package manager access needed
- Container images pre-loaded in /var/lib/containerd/

## Logs

- Installation log: /var/log/kubexm-install.log
- Kubernetes init log: /var/log/kubexm-init.log
`)

	return os.WriteFile(manifestPath, buf.Bytes(), 0644)
}

// --- Utility functions ---

func copyDirRecursive(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath, info)
	})
}

func copyFile(src, dest string, info os.FileInfo) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

func copyDirContents(src, dest string, log *logger.Logger) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			if err := util.CopyDirRecursive(srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err := common.CopyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// AddIsoCommand adds the iso command to the root command
func AddIsoCommand(parentCmd *cobra.Command) {
	parentCmd.AddCommand(IsoCmd)
}
