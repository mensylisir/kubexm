package network

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// InstallCalicoStepSpec defines the configuration for installing Calico CNI
type InstallCalicoStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// Version specifies the Calico version to install
	Version string `json:"version" yaml:"version"`

	// PodCIDR specifies the pod network CIDR
	PodCIDR string `json:"podCIDR,omitempty" yaml:"podCIDR,omitempty"`

	// Mode specifies the networking mode (ipip, vxlan, none)
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty"`

	// IPPool specifies the IP pool configuration
	IPPool string `json:"ipPool,omitempty" yaml:"ipPool,omitempty"`

	// MTU specifies the maximum transmission unit
	MTU int `json:"mtu,omitempty" yaml:"mtu,omitempty"`

	// EnableBGP indicates whether to enable BGP
	EnableBGP bool `json:"enableBGP,omitempty" yaml:"enableBGP,omitempty"`

	// BGPPeers specifies BGP peer configurations
	BGPPeers []BGPPeer `json:"bgpPeers,omitempty" yaml:"bgpPeers,omitempty"`

	// EnableNetworkPolicy indicates whether to enable network policies
	EnableNetworkPolicy bool `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`

	// DownloadURL specifies the base URL for downloading Calico manifests
	DownloadURL string `json:"downloadURL,omitempty" yaml:"downloadURL,omitempty"`

	// ManifestPath specifies where to save the Calico manifest
	ManifestPath string `json:"manifestPath,omitempty" yaml:"manifestPath,omitempty"`

	// KubeconfigPath specifies the path to kubeconfig
	KubeconfigPath string `json:"kubeconfigPath,omitempty" yaml:"kubeconfigPath,omitempty"`

	// CustomConfig allows adding custom Calico configuration
	CustomConfig map[string]interface{} `json:"customConfig,omitempty" yaml:"customConfig,omitempty"`

	// WaitForReady indicates whether to wait for Calico to be ready
	WaitForReady bool `json:"waitForReady,omitempty" yaml:"waitForReady,omitempty"`

	// TimeoutSeconds specifies the timeout for waiting
	TimeoutSeconds int `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}

// BGPPeer defines a BGP peer configuration
type BGPPeer struct {
	PeerIP string `json:"peerIP" yaml:"peerIP"`
	ASNum  int    `json:"asNum" yaml:"asNum"`
}

// InstallCalicoStep implements the Step interface for installing Calico CNI
type InstallCalicoStep struct {
	spec InstallCalicoStepSpec
}

// NewInstallCalicoStep creates a new InstallCalicoStep
func NewInstallCalicoStep(spec InstallCalicoStepSpec) *InstallCalicoStep {
	// Set default values
	if spec.PodCIDR == "" {
		spec.PodCIDR = "10.244.0.0/16"
	}
	if spec.Mode == "" {
		spec.Mode = "ipip" // Default IPIP mode as specified by user
	}
	if spec.IPPool == "" {
		spec.IPPool = spec.PodCIDR
	}
	if spec.MTU == 0 {
		spec.MTU = 1440 // Default MTU for IPIP
	}
	if spec.DownloadURL == "" {
		spec.DownloadURL = "https://raw.githubusercontent.com/projectcalico/calico/%s/manifests/calico.yaml"
	}
	if spec.ManifestPath == "" {
		spec.ManifestPath = "/tmp/calico.yaml"
	}
	if spec.KubeconfigPath == "" {
		spec.KubeconfigPath = "/etc/kubernetes/admin.conf"
	}
	if spec.TimeoutSeconds == 0 {
		spec.TimeoutSeconds = 300 // 5 minutes
	}
	spec.EnableNetworkPolicy = true // Enable by default

	return &InstallCalicoStep{spec: spec}
}

// Meta returns the step metadata
func (s *InstallCalicoStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if Calico is already installed and running
func (s *InstallCalicoStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if kubectl is available
	kubectlPath := "/usr/local/bin/kubectl"
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, kubectlPath)
	if err != nil || !exists {
		ctx.GetLogger().Infof("kubectl not found, cannot check Calico status")
		return false, nil
	}

	// Check if Calico daemonset exists and is ready
	checkCmd := fmt.Sprintf("KUBECONFIG=%s %s get daemonset calico-node -n kube-system", 
		s.spec.KubeconfigPath, kubectlPath)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, checkCmd)
	if err != nil {
		ctx.GetLogger().Infof("Calico daemonset not found: %v", err)
		return false, nil
	}

	// Check if daemonset is ready
	if containsString(output, "calico-node") {
		// Check pods are running
		podsCmd := fmt.Sprintf("KUBECONFIG=%s %s get pods -n kube-system -l k8s-app=calico-node", 
			s.spec.KubeconfigPath, kubectlPath)
		podsOutput, err := runnerSvc.Run(ctx.GoContext(), conn, podsCmd)
		if err == nil && containsString(podsOutput, "Running") {
			ctx.GetLogger().Infof("Calico appears to be installed and running")
			return true, nil
		}
	}

	return false, nil
}

// Run executes the Calico installation
func (s *InstallCalicoStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Installing Calico CNI on host %s", host.GetName())

	// Download Calico manifest
	err = s.downloadCalicoManifest(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to download Calico manifest: %w", err)
	}

	// Customize Calico manifest
	err = s.customizeCalicoManifest(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to customize Calico manifest: %w", err)
	}

	// Apply Calico manifest
	err = s.applyCalicoManifest(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to apply Calico manifest: %w", err)
	}

	// Wait for Calico to be ready if requested
	if s.spec.WaitForReady {
		err = s.waitForCalicoReady(ctx, runner, conn)
		if err != nil {
			return fmt.Errorf("Calico installation failed to become ready: %w", err)
		}
	}

	logger.Infof("Calico CNI installation completed successfully")
	return nil
}

// Rollback attempts to remove Calico installation
func (s *InstallCalicoStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back Calico CNI installation on host %s", host.GetName())

	kubectlPath := "/usr/local/bin/kubectl"

	// Delete Calico manifest if it exists
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.ManifestPath)
	if err == nil && exists {
		deleteCmd := fmt.Sprintf("KUBECONFIG=%s %s delete -f %s", 
			s.spec.KubeconfigPath, kubectlPath, s.spec.ManifestPath)
		output, err := runnerSvc.Run(ctx.GoContext(), conn, deleteCmd)
		if err != nil {
			logger.Warnf("failed to delete Calico resources: %v\nOutput: %s", err, output)
		} else {
			logger.Infof("Calico resources deleted successfully")
		}

		// Remove manifest file
		rmCmd := fmt.Sprintf("rm -f %s", s.spec.ManifestPath)
		_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
		if err != nil {
			logger.Warnf("failed to remove manifest file: %v", err)
		}
	}

	logger.Infof("Calico CNI rollback completed")
	return nil
}

// downloadCalicoManifest downloads the Calico manifest
func (s *InstallCalicoStep) downloadCalicoManifest(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	downloadURL := fmt.Sprintf(s.spec.DownloadURL, s.spec.Version)
	logger.Infof("Downloading Calico manifest from %s", downloadURL)

	// Download the manifest
	err := runner.DownloadFile(ctx.GoContext(), conn, downloadURL, s.spec.ManifestPath, 0644)
	if err != nil {
		return fmt.Errorf("failed to download Calico manifest: %w", err)
	}

	return nil
}

// customizeCalicoManifest customizes the Calico manifest based on configuration
func (s *InstallCalicoStep) customizeCalicoManifest(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	// Read the manifest
	content, err := runner.ReadFile(ctx.GoContext(), conn, s.spec.ManifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	logger.Infof("Customizing Calico manifest")

	// Replace CIDR if needed
	if s.spec.PodCIDR != "192.168.0.0/16" {
		content = replaceString(content, "192.168.0.0/16", s.spec.PodCIDR)
	}

	// Configure IPIP mode
	if s.spec.Mode == "ipip" {
		content = replaceString(content, `"CALICO_IPV4POOL_IPIP": "Always"`, `"CALICO_IPV4POOL_IPIP": "Always"`)
		content = replaceString(content, `"CALICO_IPV4POOL_VXLAN": "Never"`, `"CALICO_IPV4POOL_VXLAN": "Never"`)
	} else if s.spec.Mode == "vxlan" {
		content = replaceString(content, `"CALICO_IPV4POOL_IPIP": "Always"`, `"CALICO_IPV4POOL_IPIP": "Never"`)
		content = replaceString(content, `"CALICO_IPV4POOL_VXLAN": "Never"`, `"CALICO_IPV4POOL_VXLAN": "Always"`)
	} else if s.spec.Mode == "none" {
		content = replaceString(content, `"CALICO_IPV4POOL_IPIP": "Always"`, `"CALICO_IPV4POOL_IPIP": "Never"`)
		content = replaceString(content, `"CALICO_IPV4POOL_VXLAN": "Never"`, `"CALICO_IPV4POOL_VXLAN": "Never"`)
	}

	// Set MTU if specified
	if s.spec.MTU != 1440 {
		mtuEnv := fmt.Sprintf(`            - name: CALICO_IPV4POOL_VXLAN
              value: "Never"
            - name: FELIX_IPINIPMTU
              value: "%d"`, s.spec.MTU)
		content = replaceString(content, `            - name: CALICO_IPV4POOL_VXLAN
              value: "Never"`, mtuEnv)
	}

	// Write the customized manifest
	err = runner.WriteFile(ctx.GoContext(), conn, content, s.spec.ManifestPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write customized manifest: %w", err)
	}

	return nil
}

// applyCalicoManifest applies the Calico manifest to the cluster
func (s *InstallCalicoStep) applyCalicoManifest(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	kubectlPath := "/usr/local/bin/kubectl"
	
	logger.Infof("Applying Calico manifest to cluster")
	
	applyCmd := fmt.Sprintf("KUBECONFIG=%s %s apply -f %s", 
		s.spec.KubeconfigPath, kubectlPath, s.spec.ManifestPath)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, applyCmd)
	if err != nil {
		return fmt.Errorf("failed to apply manifest: %w\nOutput: %s", err, output)
	}

	logger.Infof("Calico manifest applied successfully")
	return nil
}

// waitForCalicoReady waits for Calico to be ready
func (s *InstallCalicoStep) waitForCalicoReady(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	kubectlPath := "/usr/local/bin/kubectl"
	
	logger.Infof("Waiting for Calico to be ready...")
	
	// Wait for daemonset to be ready
	waitCmd := fmt.Sprintf("KUBECONFIG=%s %s rollout status daemonset/calico-node -n kube-system --timeout=%ds", 
		s.spec.KubeconfigPath, kubectlPath, s.spec.TimeoutSeconds)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, waitCmd)
	if err != nil {
		return fmt.Errorf("Calico daemonset failed to become ready: %w\nOutput: %s", err, output)
	}

	// Wait for controller deployment to be ready
	waitControllerCmd := fmt.Sprintf("KUBECONFIG=%s %s rollout status deployment/calico-kube-controllers -n kube-system --timeout=%ds", 
		s.spec.KubeconfigPath, kubectlPath, s.spec.TimeoutSeconds)
	output, err = runnerSvc.Run(ctx.GoContext(), conn, waitControllerCmd)
	if err != nil {
		logger.Warnf("Calico controller deployment check failed: %v\nOutput: %s", err, output)
		// Continue as this might not exist in all versions
	}

	logger.Infof("Calico is ready")
	return nil
}

// containsString checks if a string contains a substring
func containsString(content, substr string) bool {
	return len(content) > 0 && len(substr) > 0 && 
		   findString(content, substr) >= 0
}

// findString finds the index of substr in content
func findString(content, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(content) < len(substr) {
		return -1
	}
	
	for i := 0; i <= len(content)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if content[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// replaceString replaces all occurrences of old with new in s
func replaceString(s, old, new string) string {
	if old == new || len(old) == 0 {
		return s
	}
	
	result := ""
	i := 0
	for i < len(s) {
		if idx := findStringAt(s[i:], old); idx >= 0 {
			result += s[i:i+idx] + new
			i += idx + len(old)
		} else {
			result += s[i:]
			break
		}
	}
	return result
}

// findStringAt finds the index of substr in s starting from position 0
func findStringAt(s, substr string) int {
	return findString(s, substr)
}