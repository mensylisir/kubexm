package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// InstallFlannelStepSpec defines the configuration for installing Flannel CNI
type InstallFlannelStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// Version specifies the Flannel version to install
	Version string `json:"version" yaml:"version"`

	// PodCIDR specifies the pod network CIDR
	PodCIDR string `json:"podCIDR,omitempty" yaml:"podCIDR,omitempty"`

	// Backend specifies the backend type (vxlan, host-gw, udp)
	Backend string `json:"backend,omitempty" yaml:"backend,omitempty"`

	// Port specifies the backend port
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// VNI specifies the VXLAN VNI
	VNI int `json:"vni,omitempty" yaml:"vni,omitempty"`

	// DirectRouting enables direct routing for host-gw
	DirectRouting bool `json:"directRouting,omitempty" yaml:"directRouting,omitempty"`

	// DownloadURL specifies the base URL for downloading Flannel manifests
	DownloadURL string `json:"downloadURL,omitempty" yaml:"downloadURL,omitempty"`

	// ManifestPath specifies where to save the Flannel manifest
	ManifestPath string `json:"manifestPath,omitempty" yaml:"manifestPath,omitempty"`

	// KubeconfigPath specifies the path to kubeconfig
	KubeconfigPath string `json:"kubeconfigPath,omitempty" yaml:"kubeconfigPath,omitempty"`

	// Interface specifies the interface to use
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty"`

	// PublicIP specifies the public IP to use
	PublicIP string `json:"publicIP,omitempty" yaml:"publicIP,omitempty"`

	// WaitForReady indicates whether to wait for Flannel to be ready
	WaitForReady bool `json:"waitForReady,omitempty" yaml:"waitForReady,omitempty"`

	// TimeoutSeconds specifies the timeout for waiting
	TimeoutSeconds int `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}

// InstallFlannelStep implements the Step interface for installing Flannel CNI
type InstallFlannelStep struct {
	spec InstallFlannelStepSpec
}

// NewInstallFlannelStep creates a new InstallFlannelStep
func NewInstallFlannelStep(spec InstallFlannelStepSpec) *InstallFlannelStep {
	// Set default values
	if spec.PodCIDR == "" {
		spec.PodCIDR = "10.244.0.0/16"
	}
	if spec.Backend == "" {
		spec.Backend = "vxlan"
	}
	if spec.Port == 0 {
		if spec.Backend == "vxlan" {
			spec.Port = 8472
		} else if spec.Backend == "udp" {
			spec.Port = 8285
		}
	}
	if spec.VNI == 0 {
		spec.VNI = 1
	}
	if spec.DownloadURL == "" {
		spec.DownloadURL = "https://raw.githubusercontent.com/flannel-io/flannel/%s/Documentation/kube-flannel.yml"
	}
	if spec.ManifestPath == "" {
		spec.ManifestPath = "/tmp/flannel.yaml"
	}
	if spec.KubeconfigPath == "" {
		spec.KubeconfigPath = "/etc/kubernetes/admin.conf"
	}
	if spec.TimeoutSeconds == 0 {
		spec.TimeoutSeconds = 300 // 5 minutes
	}

	return &InstallFlannelStep{spec: spec}
}

// Meta returns the step metadata
func (s *InstallFlannelStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if Flannel is already installed and running
func (s *InstallFlannelStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if kubectl is available
	kubectlPath := "/usr/local/bin/kubectl"
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, kubectlPath)
	if err != nil || !exists {
		ctx.GetLogger().Infof("kubectl not found, cannot check Flannel status")
		return false, nil
	}

	// Check if Flannel daemonset exists and is ready
	checkCmd := fmt.Sprintf("KUBECONFIG=%s %s get daemonset kube-flannel-ds -n kube-flannel", 
		s.spec.KubeconfigPath, kubectlPath)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, checkCmd)
	if err != nil {
		// Try alternative namespace
		checkCmd = fmt.Sprintf("KUBECONFIG=%s %s get daemonset kube-flannel-ds -n kube-system", 
			s.spec.KubeconfigPath, kubectlPath)
		output, err = runnerSvc.Run(ctx.GoContext(), conn, checkCmd)
		if err != nil {
			ctx.GetLogger().Infof("Flannel daemonset not found: %v", err)
			return false, nil
		}
	}

	// Check if daemonset is ready
	if containsString(output, "kube-flannel-ds") {
		// Check pods are running
		podsCmd := fmt.Sprintf("KUBECONFIG=%s %s get pods -n kube-flannel -l app=flannel", 
			s.spec.KubeconfigPath, kubectlPath)
		podsOutput, err := runnerSvc.Run(ctx.GoContext(), conn, podsCmd)
		if err != nil {
			// Try alternative namespace
			podsCmd = fmt.Sprintf("KUBECONFIG=%s %s get pods -n kube-system -l app=flannel", 
				s.spec.KubeconfigPath, kubectlPath)
			podsOutput, err = runnerSvc.Run(ctx.GoContext(), conn, podsCmd)
		}
		if err == nil && containsString(podsOutput, "Running") {
			ctx.GetLogger().Infof("Flannel appears to be installed and running")
			return true, nil
		}
	}

	return false, nil
}

// Run executes the Flannel installation
func (s *InstallFlannelStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Installing Flannel CNI on host %s", host.GetName())

	// Download Flannel manifest
	err = s.downloadFlannelManifest(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to download Flannel manifest: %w", err)
	}

	// Customize Flannel manifest
	err = s.customizeFlannelManifest(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to customize Flannel manifest: %w", err)
	}

	// Apply Flannel manifest
	err = s.applyFlannelManifest(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to apply Flannel manifest: %w", err)
	}

	// Wait for Flannel to be ready if requested
	if s.spec.WaitForReady {
		err = s.waitForFlannelReady(ctx, runner, conn)
		if err != nil {
			return fmt.Errorf("Flannel installation failed to become ready: %w", err)
		}
	}

	logger.Infof("Flannel CNI installation completed successfully")
	return nil
}

// Rollback attempts to remove Flannel installation
func (s *InstallFlannelStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back Flannel CNI installation on host %s", host.GetName())

	kubectlPath := "/usr/local/bin/kubectl"

	// Delete Flannel manifest if it exists
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.ManifestPath)
	if err == nil && exists {
		deleteCmd := fmt.Sprintf("KUBECONFIG=%s %s delete -f %s", 
			s.spec.KubeconfigPath, kubectlPath, s.spec.ManifestPath)
		output, err := runnerSvc.Run(ctx.GoContext(), conn, deleteCmd)
		if err != nil {
			logger.Warnf("failed to delete Flannel resources: %v\nOutput: %s", err, output)
		} else {
			logger.Infof("Flannel resources deleted successfully")
		}

		// Remove manifest file
		rmCmd := fmt.Sprintf("rm -f %s", s.spec.ManifestPath)
		_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
		if err != nil {
			logger.Warnf("failed to remove manifest file: %v", err)
		}
	}

	logger.Infof("Flannel CNI rollback completed")
	return nil
}

// downloadFlannelManifest downloads the Flannel manifest
func (s *InstallFlannelStep) downloadFlannelManifest(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	downloadURL := fmt.Sprintf(s.spec.DownloadURL, s.spec.Version)
	logger.Infof("Downloading Flannel manifest from %s", downloadURL)

	// Download the manifest
	err := runner.DownloadFile(ctx.GoContext(), conn, downloadURL, s.spec.ManifestPath, 0644)
	if err != nil {
		return fmt.Errorf("failed to download Flannel manifest: %w", err)
	}

	return nil
}

// customizeFlannelManifest customizes the Flannel manifest based on configuration
func (s *InstallFlannelStep) customizeFlannelManifest(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	// Read the manifest
	content, err := runner.ReadFile(ctx.GoContext(), conn, s.spec.ManifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	logger.Infof("Customizing Flannel manifest")

	// Replace CIDR if needed
	if s.spec.PodCIDR != "10.244.0.0/16" {
		content = replaceString(content, "10.244.0.0/16", s.spec.PodCIDR)
	}

	// Add backend configuration
	if s.spec.Backend != "vxlan" || s.spec.Port != 8472 || s.spec.VNI != 1 {
		backendConfig := fmt.Sprintf(`    net-conf.json: |
      {
        "Network": "%s",
        "Backend": {
          "Type": "%s"`, s.spec.PodCIDR, s.spec.Backend)

		if s.spec.Backend == "vxlan" {
			backendConfig += fmt.Sprintf(`,
          "Port": %d,
          "VNI": %d`, s.spec.Port, s.spec.VNI)
			
			if s.spec.DirectRouting {
				backendConfig += `,
          "DirectRouting": true`
			}
		} else if s.spec.Backend == "host-gw" {
			// host-gw doesn't need additional config
		} else if s.spec.Backend == "udp" {
			backendConfig += fmt.Sprintf(`,
          "Port": %d`, s.spec.Port)
		}

		backendConfig += `
        }
      }`

		// Find and replace the net-conf.json section
		oldConfig := `    net-conf.json: |
      {
        "Network": "10.244.0.0/16",
        "Backend": {
          "Type": "vxlan"
        }
      }`
		content = replaceString(content, oldConfig, backendConfig)
	}

	// Add interface specification if provided
	if s.spec.Interface != "" {
		interfaceArg := fmt.Sprintf(`        - --iface=%s`, s.spec.Interface)
		// Find the args section and add the interface arg
		if !containsString(content, "--iface=") {
			content = replaceString(content, `        - --kube-subnet-mgr`, 
				fmt.Sprintf(`        - --kube-subnet-mgr
%s`, interfaceArg))
		}
	}

	// Add public IP specification if provided
	if s.spec.PublicIP != "" {
		publicIPArg := fmt.Sprintf(`        - --public-ip=%s`, s.spec.PublicIP)
		// Find the args section and add the public IP arg
		if !containsString(content, "--public-ip=") {
			content = replaceString(content, `        - --kube-subnet-mgr`, 
				fmt.Sprintf(`        - --kube-subnet-mgr
%s`, publicIPArg))
		}
	}

	// Write the customized manifest
	err = runner.WriteFile(ctx.GoContext(), conn, content, s.spec.ManifestPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write customized manifest: %w", err)
	}

	return nil
}

// applyFlannelManifest applies the Flannel manifest to the cluster
func (s *InstallFlannelStep) applyFlannelManifest(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	kubectlPath := "/usr/local/bin/kubectl"
	
	logger.Infof("Applying Flannel manifest to cluster")
	
	applyCmd := fmt.Sprintf("KUBECONFIG=%s %s apply -f %s", 
		s.spec.KubeconfigPath, kubectlPath, s.spec.ManifestPath)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, applyCmd)
	if err != nil {
		return fmt.Errorf("failed to apply manifest: %w\nOutput: %s", err, output)
	}

	logger.Infof("Flannel manifest applied successfully")
	return nil
}

// waitForFlannelReady waits for Flannel to be ready
func (s *InstallFlannelStep) waitForFlannelReady(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	kubectlPath := "/usr/local/bin/kubectl"
	
	logger.Infof("Waiting for Flannel to be ready...")
	
	// Try both possible namespaces
	namespaces := []string{"kube-flannel", "kube-system"}
	
	for _, namespace := range namespaces {
		// Wait for daemonset to be ready
		waitCmd := fmt.Sprintf("KUBECONFIG=%s %s rollout status daemonset/kube-flannel-ds -n %s --timeout=%ds", 
			s.spec.KubeconfigPath, kubectlPath, namespace, s.spec.TimeoutSeconds)
		output, err := runnerSvc.Run(ctx.GoContext(), conn, waitCmd)
		if err == nil {
			logger.Infof("Flannel is ready in namespace %s", namespace)
			return nil
		}
		logger.Debugf("Failed to find Flannel in namespace %s: %v", namespace, err)
	}

	return fmt.Errorf("Flannel daemonset failed to become ready in any namespace")
}