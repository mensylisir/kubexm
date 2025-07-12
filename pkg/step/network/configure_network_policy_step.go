package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ConfigureNetworkPolicyStepSpec defines the configuration for network policy management
type ConfigureNetworkPolicyStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// Provider specifies the network policy provider (calico, cilium, weave, etc.)
	Provider string `json:"provider" yaml:"provider"`

	// Enable indicates whether to enable or disable network policies
	Enable bool `json:"enable" yaml:"enable"`

	// DefaultDeny indicates whether to apply default deny policies
	DefaultDeny bool `json:"defaultDeny,omitempty" yaml:"defaultDeny,omitempty"`

	// NamespaceSelector specifies which namespaces to apply policies to
	NamespaceSelector string `json:"namespaceSelector,omitempty" yaml:"namespaceSelector,omitempty"`

	// CustomPolicies allows defining custom network policies
	CustomPolicies []NetworkPolicy `json:"customPolicies,omitempty" yaml:"customPolicies,omitempty"`

	// KubeconfigPath specifies the path to kubeconfig
	KubeconfigPath string `json:"kubeconfigPath,omitempty" yaml:"kubeconfigPath,omitempty"`

	// ManifestPath specifies where to save policy manifests
	ManifestPath string `json:"manifestPath,omitempty" yaml:"manifestPath,omitempty"`

	// WaitForReady indicates whether to wait for policies to be applied
	WaitForReady bool `json:"waitForReady,omitempty" yaml:"waitForReady,omitempty"`

	// TimeoutSeconds specifies the timeout for waiting
	TimeoutSeconds int `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}

// NetworkPolicy defines a custom network policy
type NetworkPolicy struct {
	Name      string                 `json:"name" yaml:"name"`
	Namespace string                 `json:"namespace" yaml:"namespace"`
	Spec      map[string]interface{} `json:"spec" yaml:"spec"`
}

// ConfigureNetworkPolicyStep implements the Step interface for network policy management
type ConfigureNetworkPolicyStep struct {
	spec ConfigureNetworkPolicyStepSpec
}

// NewConfigureNetworkPolicyStep creates a new ConfigureNetworkPolicyStep
func NewConfigureNetworkPolicyStep(spec ConfigureNetworkPolicyStepSpec) *ConfigureNetworkPolicyStep {
	// Set default values
	if spec.KubeconfigPath == "" {
		spec.KubeconfigPath = "/etc/kubernetes/admin.conf"
	}
	if spec.ManifestPath == "" {
		spec.ManifestPath = "/tmp/network-policies.yaml"
	}
	if spec.TimeoutSeconds == 0 {
		spec.TimeoutSeconds = 120 // 2 minutes
	}
	if spec.NamespaceSelector == "" {
		spec.NamespaceSelector = "default"
	}

	return &ConfigureNetworkPolicyStep{spec: spec}
}

// Meta returns the step metadata
func (s *ConfigureNetworkPolicyStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if network policies are already configured correctly
func (s *ConfigureNetworkPolicyStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if kubectl is available
	kubectlPath := "/usr/local/bin/kubectl"
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, kubectlPath)
	if err != nil || !exists {
		ctx.GetLogger().Infof("kubectl not found, cannot check network policy status")
		return false, nil
	}

	if s.spec.Enable {
		// Check if network policies can be listed (provider supports them)
		checkCmd := fmt.Sprintf("KUBECONFIG=%s %s get networkpolicies --all-namespaces", 
			s.spec.KubeconfigPath, kubectlPath)
		_, err := runnerSvc.Run(ctx.GoContext(), conn, checkCmd)
		if err != nil {
			ctx.GetLogger().Infof("Network policies not supported or provider not ready: %v", err)
			return false, nil
		}

		// Check if default deny policy exists (if required)
		if s.spec.DefaultDeny {
			denyPolicyCmd := fmt.Sprintf("KUBECONFIG=%s %s get networkpolicy default-deny -n %s", 
				s.spec.KubeconfigPath, kubectlPath, s.spec.NamespaceSelector)
			_, err = runnerSvc.Run(ctx.GoContext(), conn, denyPolicyCmd)
			if err != nil {
				ctx.GetLogger().Infof("Default deny policy not found")
				return false, nil
			}
		}

		// Check custom policies
		for _, policy := range s.spec.CustomPolicies {
			policyCmd := fmt.Sprintf("KUBECONFIG=%s %s get networkpolicy %s -n %s", 
				s.spec.KubeconfigPath, kubectlPath, policy.Name, policy.Namespace)
			_, err = runnerSvc.Run(ctx.GoContext(), conn, policyCmd)
			if err != nil {
				ctx.GetLogger().Infof("Custom policy %s not found", policy.Name)
				return false, nil
			}
		}

		ctx.GetLogger().Infof("Network policies appear to be configured correctly")
		return true, nil
	} else {
		// Check if policies are disabled (no policies exist)
		checkCmd := fmt.Sprintf("KUBECONFIG=%s %s get networkpolicies --all-namespaces", 
			s.spec.KubeconfigPath, kubectlPath)
		output, err := runnerSvc.Run(ctx.GoContext(), conn, checkCmd)
		if err == nil && !containsString(output, "default-deny") && len(s.spec.CustomPolicies) == 0 {
			ctx.GetLogger().Infof("Network policies are already disabled")
			return true, nil
		}
	}

	return false, nil
}

// Run executes the network policy configuration
func (s *ConfigureNetworkPolicyStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	if s.spec.Enable {
		logger.Infof("Configuring network policies on host %s", host.GetName())

		// Create default deny policy if requested
		if s.spec.DefaultDeny {
			err = s.createDefaultDenyPolicy(ctx, runner, conn)
			if err != nil {
				return fmt.Errorf("failed to create default deny policy: %w", err)
			}
		}

		// Create custom policies
		err = s.createCustomPolicies(ctx, runner, conn)
		if err != nil {
			return fmt.Errorf("failed to create custom policies: %w", err)
		}
	} else {
		logger.Infof("Disabling network policies on host %s", host.GetName())

		// Remove all network policies
		err = s.removeNetworkPolicies(ctx, runner, conn)
		if err != nil {
			return fmt.Errorf("failed to remove network policies: %w", err)
		}
	}

	// Wait for policies to be applied if requested
	if s.spec.WaitForReady && s.spec.Enable {
		err = s.waitForPoliciesReady(ctx, runner, conn)
		if err != nil {
			return fmt.Errorf("network policies failed to become ready: %w", err)
		}
	}

	logger.Infof("Network policy configuration completed successfully")
	return nil
}

// Rollback attempts to reverse the network policy configuration
func (s *ConfigureNetworkPolicyStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back network policy configuration on host %s", host.GetName())

	if s.spec.Enable {
		// If we enabled policies, remove them
		err = s.removeNetworkPolicies(ctx, runner, conn)
		if err != nil {
			logger.Warnf("failed to remove network policies during rollback: %v", err)
		}
	}
	// If we disabled policies, there's nothing to rollback

	// Remove manifest file
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.ManifestPath)
	if err == nil && exists {
		rmCmd := fmt.Sprintf("rm -f %s", s.spec.ManifestPath)
		_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
		if err != nil {
			logger.Warnf("failed to remove manifest file: %v", err)
		}
	}

	logger.Infof("Network policy rollback completed")
	return nil
}

// createDefaultDenyPolicy creates a default deny all network policy
func (s *ConfigureNetworkPolicyStep) createDefaultDenyPolicy(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()

	defaultDenyPolicy := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: %s
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
---`, s.spec.NamespaceSelector)

	logger.Infof("Creating default deny network policy")

	// Write policy to file
	err := runner.WriteFile(ctx.GoContext(), conn, defaultDenyPolicy, s.spec.ManifestPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write default deny policy: %w", err)
	}

	// Apply policy
	return s.applyManifest(ctx, runner, conn)
}

// createCustomPolicies creates custom network policies
func (s *ConfigureNetworkPolicyStep) createCustomPolicies(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	if len(s.spec.CustomPolicies) == 0 {
		return nil
	}

	logger := ctx.GetLogger()
	logger.Infof("Creating custom network policies")

	policiesYAML := ""
	for _, policy := range s.spec.CustomPolicies {
		policyYAML := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %s
  namespace: %s
spec:
`, policy.Name, policy.Namespace)

		// Convert spec to YAML (simplified approach)
		for key, value := range policy.Spec {
			policyYAML += fmt.Sprintf("  %s: %v\n", key, value)
		}
		policyYAML += "---\n"
		policiesYAML += policyYAML
	}

	// Append to manifest file
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.ManifestPath)
	if err != nil {
		return fmt.Errorf("failed to check manifest file: %w", err)
	}

	if exists {
		// Append to existing file
		content, err := runner.ReadFile(ctx.GoContext(), conn, s.spec.ManifestPath)
		if err != nil {
			return fmt.Errorf("failed to read existing manifest: %w", err)
		}
		policiesYAML = content + "\n" + policiesYAML
	}

	err = runner.WriteFile(ctx.GoContext(), conn, policiesYAML, s.spec.ManifestPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write custom policies: %w", err)
	}

	// Apply policies
	return s.applyManifest(ctx, runner, conn)
}

// removeNetworkPolicies removes all network policies
func (s *ConfigureNetworkPolicyStep) removeNetworkPolicies(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	kubectlPath := "/usr/local/bin/kubectl"

	// Remove default deny policy if it exists
	if s.spec.DefaultDeny {
		deleteCmd := fmt.Sprintf("KUBECONFIG=%s %s delete networkpolicy default-deny -n %s", 
			s.spec.KubeconfigPath, kubectlPath, s.spec.NamespaceSelector)
		_, err := runnerSvc.Run(ctx.GoContext(), conn, deleteCmd)
		if err != nil {
			logger.Debugf("default deny policy not found or already removed: %v", err)
		}
	}

	// Remove custom policies
	for _, policy := range s.spec.CustomPolicies {
		deleteCmd := fmt.Sprintf("KUBECONFIG=%s %s delete networkpolicy %s -n %s", 
			s.spec.KubeconfigPath, kubectlPath, policy.Name, policy.Namespace)
		_, err := runnerSvc.Run(ctx.GoContext(), conn, deleteCmd)
		if err != nil {
			logger.Debugf("custom policy %s not found or already removed: %v", policy.Name, err)
		}
	}

	logger.Infof("Network policies removed")
	return nil
}

// applyManifest applies the network policy manifest
func (s *ConfigureNetworkPolicyStep) applyManifest(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	kubectlPath := "/usr/local/bin/kubectl"
	
	applyCmd := fmt.Sprintf("KUBECONFIG=%s %s apply -f %s", 
		s.spec.KubeconfigPath, kubectlPath, s.spec.ManifestPath)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, applyCmd)
	if err != nil {
		return fmt.Errorf("failed to apply network policies: %w\nOutput: %s", err, output)
	}

	return nil
}

// waitForPoliciesReady waits for network policies to be applied
func (s *ConfigureNetworkPolicyStep) waitForPoliciesReady(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	kubectlPath := "/usr/local/bin/kubectl"
	
	logger.Infof("Waiting for network policies to be ready...")

	// Simple check - just verify policies exist
	if s.spec.DefaultDeny {
		checkCmd := fmt.Sprintf("KUBECONFIG=%s %s get networkpolicy default-deny -n %s", 
			s.spec.KubeconfigPath, kubectlPath, s.spec.NamespaceSelector)
		_, err := runnerSvc.Run(ctx.GoContext(), conn, checkCmd)
		if err != nil {
			return fmt.Errorf("default deny policy not found after creation: %w", err)
		}
	}

	for _, policy := range s.spec.CustomPolicies {
		checkCmd := fmt.Sprintf("KUBECONFIG=%s %s get networkpolicy %s -n %s", 
			s.spec.KubeconfigPath, kubectlPath, policy.Name, policy.Namespace)
		_, err := runnerSvc.Run(ctx.GoContext(), conn, checkCmd)
		if err != nil {
			return fmt.Errorf("custom policy %s not found after creation: %w", policy.Name, err)
		}
	}

	logger.Infof("Network policies are ready")
	return nil
}