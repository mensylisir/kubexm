package kubernetes

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CreateKubeadmConfigStepSpec defines the configuration for creating kubeadm config
type CreateKubeadmConfigStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// ConfigPath is where to place the kubeadm configuration file
	ConfigPath string `json:"configPath,omitempty" yaml:"configPath,omitempty"`

	// KubernetesVersion specifies the Kubernetes version
	KubernetesVersion string `json:"kubernetesVersion" yaml:"kubernetesVersion"`

	// ControlPlaneEndpoint specifies the load balancer endpoint
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`

	// PodSubnet specifies the pod network CIDR
	PodSubnet string `json:"podSubnet,omitempty" yaml:"podSubnet,omitempty"`

	// ServiceSubnet specifies the service network CIDR
	ServiceSubnet string `json:"serviceSubnet,omitempty" yaml:"serviceSubnet,omitempty"`

	// ClusterName specifies the cluster name
	ClusterName string `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`

	// NodeName specifies the node name
	NodeName string `json:"nodeName,omitempty" yaml:"nodeName,omitempty"`

	// AdvertiseAddress specifies the API server advertise address
	AdvertiseAddress string `json:"advertiseAddress,omitempty" yaml:"advertiseAddress,omitempty"`

	// BindPort specifies the API server bind port
	BindPort int `json:"bindPort,omitempty" yaml:"bindPort,omitempty"`

	// CertSANs specifies additional certificate SANs
	CertSANs []string `json:"certSANs,omitempty" yaml:"certSANs,omitempty"`

	// EtcdEndpoints specifies external etcd endpoints
	EtcdEndpoints []string `json:"etcdEndpoints,omitempty" yaml:"etcdEndpoints,omitempty"`

	// EtcdCAFile specifies the etcd CA file path
	EtcdCAFile string `json:"etcdCAFile,omitempty" yaml:"etcdCAFile,omitempty"`

	// EtcdCertFile specifies the etcd cert file path
	EtcdCertFile string `json:"etcdCertFile,omitempty" yaml:"etcdCertFile,omitempty"`

	// EtcdKeyFile specifies the etcd key file path
	EtcdKeyFile string `json:"etcdKeyFile,omitempty" yaml:"etcdKeyFile,omitempty"`

	// ContainerRuntimeEndpoint specifies the container runtime endpoint
	ContainerRuntimeEndpoint string `json:"containerRuntimeEndpoint,omitempty" yaml:"containerRuntimeEndpoint,omitempty"`

	// CgroupDriver specifies the cgroup driver
	CgroupDriver string `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`

	// ImageRepository specifies the image repository
	ImageRepository string `json:"imageRepository,omitempty" yaml:"imageRepository,omitempty"`

	// DNSDomain specifies the cluster DNS domain
	DNSDomain string `json:"dnsDomain,omitempty" yaml:"dnsDomain,omitempty"`

	// ProxyMode specifies the kube-proxy mode
	ProxyMode string `json:"proxyMode,omitempty" yaml:"proxyMode,omitempty"`

	// FeatureGates specifies feature gates to enable
	FeatureGates map[string]bool `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`

	// ExtraArgs allows adding custom arguments to components
	ExtraArgs map[string]map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`

	// Patches allows adding patches to components
	Patches map[string][]string `json:"patches,omitempty" yaml:"patches,omitempty"`
}

// CreateKubeadmConfigStep implements the Step interface for creating kubeadm config
type CreateKubeadmConfigStep struct {
	spec CreateKubeadmConfigStepSpec
}

// NewCreateKubeadmConfigStep creates a new CreateKubeadmConfigStep
func NewCreateKubeadmConfigStep(spec CreateKubeadmConfigStepSpec) *CreateKubeadmConfigStep {
	// Set default values
	if spec.ConfigPath == "" {
		spec.ConfigPath = "/etc/kubernetes/kubeadm-config.yaml"
	}
	if spec.PodSubnet == "" {
		spec.PodSubnet = "10.244.0.0/16"
	}
	if spec.ServiceSubnet == "" {
		spec.ServiceSubnet = "10.96.0.0/12"
	}
	if spec.ClusterName == "" {
		spec.ClusterName = "kubernetes"
	}
	if spec.BindPort == 0 {
		spec.BindPort = 6443
	}
	if spec.ContainerRuntimeEndpoint == "" {
		spec.ContainerRuntimeEndpoint = "unix:///run/containerd/containerd.sock"
	}
	if spec.CgroupDriver == "" {
		spec.CgroupDriver = "systemd"
	}
	if spec.ImageRepository == "" {
		spec.ImageRepository = "registry.k8s.io"
	}
	if spec.DNSDomain == "" {
		spec.DNSDomain = "cluster.local"
	}
	if spec.ProxyMode == "" {
		spec.ProxyMode = "ipvs"
	}

	return &CreateKubeadmConfigStep{spec: spec}
}

// Meta returns the step metadata
func (s *CreateKubeadmConfigStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if kubeadm config already exists and is correct
func (s *CreateKubeadmConfigStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if config file exists
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.ConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check if config file exists: %w", err)
	}

	if !exists {
		ctx.GetLogger().Infof("kubeadm config file %s does not exist", s.spec.ConfigPath)
		return false, nil
	}

	// Read existing config and check key settings
	configContent, err := runner.ReadFile(ctx.GoContext(), conn, s.spec.ConfigPath)
	if err != nil {
		ctx.GetLogger().Warnf("failed to read kubeadm config: %v", err)
		return false, nil
	}

	// Basic checks for required configuration
	expectedStrings := []string{
		s.spec.KubernetesVersion,
		s.spec.PodSubnet,
		s.spec.ServiceSubnet,
	}

	for _, expected := range expectedStrings {
		if !containsString(configContent, expected) {
			ctx.GetLogger().Infof("kubeadm config missing expected setting: %s", expected)
			return false, nil
		}
	}

	ctx.GetLogger().Infof("kubeadm configuration appears to be correct")
	return true, nil
}

// Run executes the kubeadm config creation
func (s *CreateKubeadmConfigStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Creating kubeadm configuration on host %s", host.GetName())

	// Ensure config directory exists
	configDir := filepath.Dir(s.spec.ConfigPath)
	err = runner.Mkdirp(ctx.GoContext(), conn, configDir, "0755", true)
	if err != nil {
		return fmt.Errorf("failed to create kubeadm config directory: %w", err)
	}

	// Generate kubeadm configuration
	config := s.generateKubeadmConfig()

	// Write configuration file
	logger.Infof("Writing kubeadm configuration to %s", s.spec.ConfigPath)
	err = runner.WriteFile(ctx.GoContext(), conn, config, s.spec.ConfigPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write kubeadm config: %w", err)
	}

	logger.Infof("kubeadm configuration created successfully")
	return nil
}

// Rollback removes the kubeadm configuration
func (s *CreateKubeadmConfigStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back kubeadm configuration on host %s", host.GetName())

	// Remove configuration file
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.ConfigPath)
	if err != nil {
		logger.Warnf("failed to check if config file exists during rollback: %v", err)
		return nil
	}

	if exists {
		rmCmd := fmt.Sprintf("rm -f %s", s.spec.ConfigPath)
		_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
		if err != nil {
			logger.Warnf("failed to remove config file during rollback: %v", err)
		}
	}

	logger.Infof("kubeadm configuration rollback completed")
	return nil
}

// generateKubeadmConfig generates the kubeadm configuration file content
func (s *CreateKubeadmConfigStep) generateKubeadmConfig() string {
	config := fmt.Sprintf(`# Generated by kubexm
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: %s
  bindPort: %d
nodeRegistration:
  criSocket: %s
  imagePullPolicy: IfNotPresent
  taints: null
`,
		s.spec.AdvertiseAddress,
		s.spec.BindPort,
		s.spec.ContainerRuntimeEndpoint,
	)

	// Add node name if specified
	if s.spec.NodeName != "" {
		config += fmt.Sprintf("  name: %s\n", s.spec.NodeName)
	}

	// Add kubelet configuration
	config += fmt.Sprintf(`  kubeletExtraArgs:
    cgroup-driver: %s
`,
		s.spec.CgroupDriver,
	)

	config += `---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
`

	config += fmt.Sprintf(`kubernetesVersion: %s
clusterName: %s
controlPlaneEndpoint: %s
imageRepository: %s
`,
		s.spec.KubernetesVersion,
		s.spec.ClusterName,
		s.spec.ControlPlaneEndpoint,
		s.spec.ImageRepository,
	)

	// Add certificate SANs if specified
	if len(s.spec.CertSANs) > 0 {
		config += "apiServer:\n  certSANs:\n"
		for _, san := range s.spec.CertSANs {
			config += fmt.Sprintf("  - %s\n", san)
		}
	}

	// Add extra args if specified
	if extraArgs, exists := s.spec.ExtraArgs["apiServer"]; exists && len(extraArgs) > 0 {
		if !containsString(config, "apiServer:") {
			config += "apiServer:\n"
		}
		config += "  extraArgs:\n"
		for key, value := range extraArgs {
			config += fmt.Sprintf("    %s: %s\n", key, value)
		}
	}

	// Add etcd configuration
	if len(s.spec.EtcdEndpoints) > 0 {
		config += "etcd:\n  external:\n    endpoints:\n"
		for _, endpoint := range s.spec.EtcdEndpoints {
			config += fmt.Sprintf("    - %s\n", endpoint)
		}
		if s.spec.EtcdCAFile != "" {
			config += fmt.Sprintf("    caFile: %s\n", s.spec.EtcdCAFile)
		}
		if s.spec.EtcdCertFile != "" {
			config += fmt.Sprintf("    certFile: %s\n", s.spec.EtcdCertFile)
		}
		if s.spec.EtcdKeyFile != "" {
			config += fmt.Sprintf("    keyFile: %s\n", s.spec.EtcdKeyFile)
		}
	}

	// Add networking configuration
	config += fmt.Sprintf(`networking:
  serviceSubnet: %s
  podSubnet: %s
  dnsDomain: %s
`,
		s.spec.ServiceSubnet,
		s.spec.PodSubnet,
		s.spec.DNSDomain,
	)

	// Add controller manager extra args
	if extraArgs, exists := s.spec.ExtraArgs["controllerManager"]; exists && len(extraArgs) > 0 {
		config += "controllerManager:\n  extraArgs:\n"
		for key, value := range extraArgs {
			config += fmt.Sprintf("    %s: %s\n", key, value)
		}
	}

	// Add scheduler extra args
	if extraArgs, exists := s.spec.ExtraArgs["scheduler"]; exists && len(extraArgs) > 0 {
		config += "scheduler:\n  extraArgs:\n"
		for key, value := range extraArgs {
			config += fmt.Sprintf("    %s: %s\n", key, value)
		}
	}

	// Add feature gates if specified
	if len(s.spec.FeatureGates) > 0 {
		config += "featureGates:\n"
		for gate, enabled := range s.spec.FeatureGates {
			config += fmt.Sprintf("  %s: %t\n", gate, enabled)
		}
	}

	// Add kubelet configuration
	config += fmt.Sprintf(`---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: %s
`,
		s.spec.CgroupDriver,
	)

	// Add kube-proxy configuration
	config += fmt.Sprintf(`---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: %s
`,
		s.spec.ProxyMode,
	)

	// Add IPVS configuration if using IPVS mode
	if s.spec.ProxyMode == "ipvs" {
		config += `ipvs:
  strictARP: true
`
	}

	return config
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