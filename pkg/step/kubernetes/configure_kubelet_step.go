package kubernetes

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ConfigureKubeletStepSpec defines the configuration for configuring kubelet
type ConfigureKubeletStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// KubeletConfigPath is where to place kubelet configuration
	KubeletConfigPath string `json:"kubeletConfigPath,omitempty" yaml:"kubeletConfigPath,omitempty"`

	// KubeletServicePath is where to place the systemd service file
	KubeletServicePath string `json:"kubeletServicePath,omitempty" yaml:"kubeletServicePath,omitempty"`

	// KubeletBinaryPath is where kubelet binary is located
	KubeletBinaryPath string `json:"kubeletBinaryPath,omitempty" yaml:"kubeletBinaryPath,omitempty"`

	// CgroupDriver specifies the cgroup driver (systemd or cgroupfs)
	CgroupDriver string `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`

	// ClusterDNS specifies the cluster DNS IP
	ClusterDNS string `json:"clusterDNS,omitempty" yaml:"clusterDNS,omitempty"`

	// ClusterDomain specifies the cluster domain
	ClusterDomain string `json:"clusterDomain,omitempty" yaml:"clusterDomain,omitempty"`

	// ContainerRuntimeEndpoint specifies the container runtime endpoint
	ContainerRuntimeEndpoint string `json:"containerRuntimeEndpoint,omitempty" yaml:"containerRuntimeEndpoint,omitempty"`

	// KubeReserved specifies resources reserved for Kubernetes components
	KubeReserved map[string]string `json:"kubeReserved,omitempty" yaml:"kubeReserved,omitempty"`

	// SystemReserved specifies resources reserved for system processes
	SystemReserved map[string]string `json:"systemReserved,omitempty" yaml:"systemReserved,omitempty"`

	// MaxPods specifies the maximum number of pods per node
	MaxPods int `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`

	// PodCIDR specifies the pod CIDR for this node
	PodCIDR string `json:"podCIDR,omitempty" yaml:"podCIDR,omitempty"`

	// ExtraArgs allows adding custom kubelet arguments
	ExtraArgs map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`

	// FailSwapOn specifies whether to fail if swap is enabled
	FailSwapOn bool `json:"failSwapOn,omitempty" yaml:"failSwapOn,omitempty"`
}

// ConfigureKubeletStep implements the Step interface for configuring kubelet
type ConfigureKubeletStep struct {
	spec ConfigureKubeletStepSpec
}

// NewConfigureKubeletStep creates a new ConfigureKubeletStep
func NewConfigureKubeletStep(spec ConfigureKubeletStepSpec) *ConfigureKubeletStep {
	// Set default values
	if spec.KubeletConfigPath == "" {
		spec.KubeletConfigPath = "/var/lib/kubelet/config.yaml"
	}
	if spec.KubeletServicePath == "" {
		spec.KubeletServicePath = "/etc/systemd/system/kubelet.service"
	}
	if spec.KubeletBinaryPath == "" {
		spec.KubeletBinaryPath = "/usr/local/bin"
	}
	if spec.CgroupDriver == "" {
		spec.CgroupDriver = "systemd"
	}
	if spec.ClusterDNS == "" {
		spec.ClusterDNS = "10.96.0.10"
	}
	if spec.ClusterDomain == "" {
		spec.ClusterDomain = "cluster.local"
	}
	if spec.ContainerRuntimeEndpoint == "" {
		spec.ContainerRuntimeEndpoint = "unix:///run/containerd/containerd.sock"
	}
	if spec.MaxPods == 0 {
		spec.MaxPods = 110
	}

	return &ConfigureKubeletStep{spec: spec}
}

// Meta returns the step metadata
func (s *ConfigureKubeletStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if kubelet is already configured correctly
func (s *ConfigureKubeletStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if kubelet config file exists
	configExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.KubeletConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if kubelet config exists: %w", err)
	}

	// Check if kubelet service file exists
	serviceExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.KubeletServicePath)
	if err != nil {
		return false, fmt.Errorf("failed to check if kubelet service exists: %w", err)
	}

	if !configExists || !serviceExists {
		ctx.GetLogger().Infof("kubelet configuration or service file missing")
		return false, nil
	}

	// Read existing config and check key settings
	configContent, err := runner.ReadFile(ctx.GoContext(), conn, s.spec.KubeletConfigPath)
	if err != nil {
		ctx.GetLogger().Warnf("failed to read kubelet config: %v", err)
		return false, nil
	}

	// Basic checks for required configuration
	expectedStrings := []string{
		s.spec.ClusterDNS,
		s.spec.ClusterDomain,
		s.spec.CgroupDriver,
		s.spec.ContainerRuntimeEndpoint,
	}

	for _, expected := range expectedStrings {
		if !containsString(configContent, expected) {
			ctx.GetLogger().Infof("kubelet config missing expected setting: %s", expected)
			return false, nil
		}
	}

	ctx.GetLogger().Infof("kubelet configuration appears to be correct")
	return true, nil
}

// Run executes the kubelet configuration
func (s *ConfigureKubeletStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Configuring kubelet on host %s", host.GetName())

	// Ensure kubelet config directory exists
	configDir := filepath.Dir(s.spec.KubeletConfigPath)
	err = runner.Mkdirp(ctx.GoContext(), conn, configDir, "0755", true)
	if err != nil {
		return fmt.Errorf("failed to create kubelet config directory: %w", err)
	}

	// Generate kubelet configuration
	config := s.generateKubeletConfig()

	// Write kubelet configuration file
	logger.Infof("Writing kubelet configuration to %s", s.spec.KubeletConfigPath)
	err = runner.WriteFile(ctx.GoContext(), conn, config, s.spec.KubeletConfigPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write kubelet config: %w", err)
	}

	// Create kubelet systemd service file
	err = s.createKubeletService(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to create kubelet service: %w", err)
	}

	// Create kubelet dropin directory and files
	err = s.createKubeletDropins(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to create kubelet dropins: %w", err)
	}

	// Reload systemd daemon
	_, err = runnerSvc.Run(ctx.GoContext(), conn, "systemctl daemon-reload")
	if err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	// Enable kubelet service
	_, err = runnerSvc.Run(ctx.GoContext(), conn, "systemctl enable kubelet")
	if err != nil {
		return fmt.Errorf("failed to enable kubelet service: %w", err)
	}

	logger.Infof("kubelet configuration completed successfully")
	return nil
}

// Rollback removes the kubelet configuration
func (s *ConfigureKubeletStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back kubelet configuration on host %s", host.GetName())

	// Stop and disable kubelet service
	_, err = runnerSvc.Run(ctx.GoContext(), conn, "systemctl stop kubelet")
	if err != nil {
		logger.Warnf("failed to stop kubelet service: %v", err)
	}

	_, err = runnerSvc.Run(ctx.GoContext(), conn, "systemctl disable kubelet")
	if err != nil {
		logger.Warnf("failed to disable kubelet service: %v", err)
	}

	// Remove service file
	rmCmd := fmt.Sprintf("rm -f %s", s.spec.KubeletServicePath)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
	if err != nil {
		logger.Warnf("failed to remove kubelet service file: %v", err)
	}

	// Remove configuration file
	rmCmd = fmt.Sprintf("rm -f %s", s.spec.KubeletConfigPath)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
	if err != nil {
		logger.Warnf("failed to remove kubelet config file: %v", err)
	}

	// Remove dropin directory
	dropinDir := "/etc/systemd/system/kubelet.service.d"
	rmCmd = fmt.Sprintf("rm -rf %s", dropinDir)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
	if err != nil {
		logger.Warnf("failed to remove kubelet dropin directory: %v", err)
	}

	// Reload systemd daemon
	_, err = runnerSvc.Run(ctx.GoContext(), conn, "systemctl daemon-reload")
	if err != nil {
		logger.Warnf("failed to reload systemd daemon: %v", err)
	}

	logger.Infof("kubelet configuration rollback completed")
	return nil
}

// generateKubeletConfig generates the kubelet configuration file content
func (s *ConfigureKubeletStep) generateKubeletConfig() string {
	config := fmt.Sprintf(`# Generated by kubexm
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
address: 0.0.0.0
port: 10250
readOnlyPort: 10255
cgroupDriver: %s
clusterDNS:
- %s
clusterDomain: %s
containerRuntimeEndpoint: %s
failSwapOn: %t
maxPods: %d
authentication:
  anonymous:
    enabled: false
  webhook:
    enabled: true
    cacheTTL: 2m0s
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
serverTLSBootstrap: true
tlsCertFile: ""
tlsPrivateKeyFile: ""
rotateCertificates: true
staticPodPath: /etc/kubernetes/manifests
`,
		s.spec.CgroupDriver,
		s.spec.ClusterDNS,
		s.spec.ClusterDomain,
		s.spec.ContainerRuntimeEndpoint,
		s.spec.FailSwapOn,
		s.spec.MaxPods,
	)

	// Add resource reservations if specified
	if len(s.spec.KubeReserved) > 0 {
		config += "kubeReserved:\n"
		for resource, value := range s.spec.KubeReserved {
			config += fmt.Sprintf("  %s: %s\n", resource, value)
		}
	}

	if len(s.spec.SystemReserved) > 0 {
		config += "systemReserved:\n"
		for resource, value := range s.spec.SystemReserved {
			config += fmt.Sprintf("  %s: %s\n", resource, value)
		}
	}

	// Add pod CIDR if specified
	if s.spec.PodCIDR != "" {
		config += fmt.Sprintf("podCIDR: %s\n", s.spec.PodCIDR)
	}

	return config
}

// createKubeletService creates the kubelet systemd service file
func (s *ConfigureKubeletStep) createKubeletService(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	kubeletBinary := filepath.Join(s.spec.KubeletBinaryPath, "kubelet")

	serviceContent := fmt.Sprintf(`[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=https://kubernetes.io/docs/
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=%s
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
`, kubeletBinary)

	// Ensure service directory exists
	serviceDir := filepath.Dir(s.spec.KubeletServicePath)
	err := runner.Mkdirp(ctx.GoContext(), conn, serviceDir, "0755", true)
	if err != nil {
		return fmt.Errorf("failed to create service directory: %w", err)
	}

	// Write service file
	err = runner.WriteFile(ctx.GoContext(), conn, serviceContent, s.spec.KubeletServicePath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write kubelet service file: %w", err)
	}

	return nil
}

// createKubeletDropins creates the kubelet systemd dropin files
func (s *ConfigureKubeletStep) createKubeletDropins(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	dropinDir := "/etc/systemd/system/kubelet.service.d"
	
	// Create dropin directory
	err := runner.Mkdirp(ctx.GoContext(), conn, dropinDir, "0755", true)
	if err != nil {
		return fmt.Errorf("failed to create kubelet dropin directory: %w", err)
	}

	// Create 10-kubeadm.conf dropin
	kubeadmDropinPath := filepath.Join(dropinDir, "10-kubeadm.conf")
	kubeadmDropinContent := fmt.Sprintf(`# Note: This dropin only works with kubeadm and kubelet v1.11+
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=%s"
# This is a file that "kubeadm init" and "kubeadm join" generates at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably, the user should use
# the .NodeRegistration.KubeletExtraArgs object in the configuration files instead. KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/default/kubelet
ExecStart=
ExecStart=%s $KUBELET_KUBECONFIG_ARGS $KUBELET_CONFIG_ARGS $KUBELET_KUBEADM_ARGS $KUBELET_EXTRA_ARGS
`, s.spec.KubeletConfigPath, filepath.Join(s.spec.KubeletBinaryPath, "kubelet"))

	err = runner.WriteFile(ctx.GoContext(), conn, kubeadmDropinContent, kubeadmDropinPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write kubeadm dropin: %w", err)
	}

	// Create extra args file if extra args are specified
	if len(s.spec.ExtraArgs) > 0 {
		extraArgsPath := "/etc/default/kubelet"
		extraArgsContent := "# Extra arguments for kubelet\nKUBELET_EXTRA_ARGS="
		
		for key, value := range s.spec.ExtraArgs {
			if value != "" {
				extraArgsContent += fmt.Sprintf(" --%s=%s", key, value)
			} else {
				extraArgsContent += fmt.Sprintf(" --%s", key)
			}
		}
		extraArgsContent += "\n"

		// Ensure directory exists
		extraArgsDir := filepath.Dir(extraArgsPath)
		err = runner.Mkdirp(ctx.GoContext(), conn, extraArgsDir, "0755", true)
		if err != nil {
			return fmt.Errorf("failed to create extra args directory: %w", err)
		}

		err = runner.WriteFile(ctx.GoContext(), conn, extraArgsContent, extraArgsPath, "0644", true)
		if err != nil {
			return fmt.Errorf("failed to write kubelet extra args: %w", err)
		}
	}

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