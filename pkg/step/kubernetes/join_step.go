package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmJoinStepSpec defines the configuration for kubeadm join
type KubeadmJoinStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// JoinCommand is the complete join command (can be retrieved from cache)
	JoinCommand string `json:"joinCommand,omitempty" yaml:"joinCommand,omitempty"`

	// ControlPlaneEndpoint is the load balancer endpoint for the control plane
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`

	// Token is the bootstrap token for joining
	Token string `json:"token,omitempty" yaml:"token,omitempty"`

	// DiscoveryTokenCACertHash is the CA cert hash for discovery
	DiscoveryTokenCACertHash string `json:"discoveryTokenCACertHash,omitempty" yaml:"discoveryTokenCACertHash,omitempty"`

	// CertificateKey is used for joining additional control plane nodes
	CertificateKey string `json:"certificateKey,omitempty" yaml:"certificateKey,omitempty"`

	// ControlPlane indicates if this node should join as a control plane node
	ControlPlane bool `json:"controlPlane,omitempty" yaml:"controlPlane,omitempty"`

	// IgnorePreflightErrors lists preflight errors to ignore
	IgnorePreflightErrors []string `json:"ignorePreflightErrors,omitempty" yaml:"ignorePreflightErrors,omitempty"`

	// DryRun indicates whether to perform a dry run
	DryRun bool `json:"dryRun,omitempty" yaml:"dryRun,omitempty"`

	// KubeadmPath is the path to the kubeadm binary
	KubeadmPath string `json:"kubeadmPath,omitempty" yaml:"kubeadmPath,omitempty"`

	// UseCachedJoinInfo indicates whether to use cached join information
	UseCachedJoinInfo bool `json:"useCachedJoinInfo,omitempty" yaml:"useCachedJoinInfo,omitempty"`
}

// KubeadmJoinStep implements the Step interface for kubeadm join
type KubeadmJoinStep struct {
	spec KubeadmJoinStepSpec
}

// NewKubeadmJoinStep creates a new KubeadmJoinStep
func NewKubeadmJoinStep(spec KubeadmJoinStepSpec) *KubeadmJoinStep {
	// Set default values
	if spec.KubeadmPath == "" {
		spec.KubeadmPath = "/usr/local/bin/kubeadm"
	}

	return &KubeadmJoinStep{spec: spec}
}

// Meta returns the step metadata
func (s *KubeadmJoinStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if the node has already joined the cluster
func (s *KubeadmJoinStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if kubelet is running and healthy
	kubeletRunning, err := s.isKubeletRunning(ctx.GoContext(), runner, conn)
	if err != nil {
		ctx.GetLogger().Debugf("error checking kubelet status: %v", err)
	}

	// Check if the node is already part of a cluster
	nodeInCluster := false
	if kubeletRunning {
		inCluster, err := s.isNodeInCluster(ctx.GoContext(), runner, conn)
		if err != nil {
			ctx.GetLogger().Debugf("error checking cluster membership: %v", err)
		} else {
			nodeInCluster = inCluster
		}
	}

	// If kubelet is running and node is in cluster, join has already been completed
	if kubeletRunning && nodeInCluster {
		ctx.GetLogger().Infof("node appears to have already joined the cluster")
		return true, nil
	}

	return false, nil
}

// Run executes the kubeadm join
func (s *KubeadmJoinStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Running kubeadm join on host %s", host.GetName())

	// Determine join command
	joinCmd, err := s.buildJoinCommand(ctx)
	if err != nil {
		return fmt.Errorf("failed to build join command: %w", err)
	}

	logger.Infof("Executing: %s", joinCmd)

	// Execute kubeadm join
	output, err := runnerSvc.Run(ctx.GoContext(), conn, joinCmd)
	if err != nil {
		return fmt.Errorf("kubeadm join failed: %w\nOutput: %s", err, output)
	}

	logger.Infof("kubeadm join completed successfully")

	// Setup kubectl for control plane nodes if not dry run
	if !s.spec.DryRun && s.spec.ControlPlane {
		err = s.setupKubectl(ctx, runner, conn)
		if err != nil {
			logger.Warnf("failed to setup kubectl: %v", err)
		}
	}

	return nil
}

// Rollback attempts to reset the node and remove it from the cluster
func (s *KubeadmJoinStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back kubeadm join on host %s", host.GetName())

	// Run kubeadm reset
	resetCmd := fmt.Sprintf("%s reset --force", s.spec.KubeadmPath)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, resetCmd)
	if err != nil {
		logger.Warnf("kubeadm reset failed: %v\nOutput: %s", err, output)
	} else {
		logger.Infof("kubeadm reset completed")
	}

	// Clean up additional directories that kubeadm reset might not remove
	cleanupDirs := []string{
		"/etc/kubernetes",
		"/etc/cni/net.d",
	}

	for _, dir := range cleanupDirs {
		rmCmd := fmt.Sprintf("rm -rf %s", dir)
		_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
		if err != nil {
			logger.Warnf("failed to remove %s during rollback: %v", dir, err)
		}
	}

	// Reset iptables rules
	resetIPTablesCmd := `
		iptables -F && iptables -t nat -F && iptables -t mangle -F && iptables -X || true
		ipvsadm -C || true
	`
	_, err = runnerSvc.Run(ctx.GoContext(), conn, resetIPTablesCmd)
	if err != nil {
		logger.Warnf("failed to reset iptables during rollback: %v", err)
	}

	logger.Infof("kubeadm join rollback completed")
	return nil
}

// buildJoinCommand builds the kubeadm join command with all options
func (s *KubeadmJoinStep) buildJoinCommand(ctx step.StepContext) (string, error) {
	// If join command is explicitly provided, use it
	if s.spec.JoinCommand != "" {
		return s.modifyJoinCommand(s.spec.JoinCommand), nil
	}

	// Try to get join command from cache if enabled
	if s.spec.UseCachedJoinInfo {
		cache := ctx.GetStepCache()
		if cachedCmd := cache.Get("kubeadm-join-command"); cachedCmd != "" {
			s.spec.JoinCommand = cachedCmd
			// Get certificate key from cache if this is a control plane join
			if s.spec.ControlPlane {
				if certKey := cache.Get("kubeadm-certificate-key"); certKey != "" {
					s.spec.CertificateKey = certKey
				}
			}
			return s.modifyJoinCommand(s.spec.JoinCommand), nil
		}
	}

	// Build join command manually if no cached command available
	if s.spec.ControlPlaneEndpoint == "" || s.spec.Token == "" || s.spec.DiscoveryTokenCACertHash == "" {
		return "", fmt.Errorf("insufficient information to build join command: need ControlPlaneEndpoint, Token, and DiscoveryTokenCACertHash")
	}

	cmd := []string{
		s.spec.KubeadmPath,
		"join",
		s.spec.ControlPlaneEndpoint,
		"--token", s.spec.Token,
		"--discovery-token-ca-cert-hash", s.spec.DiscoveryTokenCACertHash,
	}

	// Add control plane options if joining as control plane
	if s.spec.ControlPlane {
		cmd = append(cmd, "--control-plane")
		if s.spec.CertificateKey != "" {
			cmd = append(cmd, "--certificate-key", s.spec.CertificateKey)
		}
	}

	// Add ignore preflight errors
	if len(s.spec.IgnorePreflightErrors) > 0 {
		cmd = append(cmd, "--ignore-preflight-errors", strings.Join(s.spec.IgnorePreflightErrors, ","))
	}

	// Add dry run option
	if s.spec.DryRun {
		cmd = append(cmd, "--dry-run")
	}

	// Add verbose output
	cmd = append(cmd, "-v=5")

	return strings.Join(cmd, " "), nil
}

// modifyJoinCommand modifies an existing join command with additional options
func (s *KubeadmJoinStep) modifyJoinCommand(joinCmd string) string {
	cmd := joinCmd

	// Add control plane flag if needed and not already present
	if s.spec.ControlPlane && !strings.Contains(cmd, "--control-plane") {
		cmd += " --control-plane"
		if s.spec.CertificateKey != "" && !strings.Contains(cmd, "--certificate-key") {
			cmd += fmt.Sprintf(" --certificate-key %s", s.spec.CertificateKey)
		}
	}

	// Add ignore preflight errors if specified and not already present
	if len(s.spec.IgnorePreflightErrors) > 0 && !strings.Contains(cmd, "--ignore-preflight-errors") {
		cmd += fmt.Sprintf(" --ignore-preflight-errors %s", strings.Join(s.spec.IgnorePreflightErrors, ","))
	}

	// Add dry run if specified and not already present
	if s.spec.DryRun && !strings.Contains(cmd, "--dry-run") {
		cmd += " --dry-run"
	}

	// Add verbose output if not already present
	if !strings.Contains(cmd, "-v=") {
		cmd += " -v=5"
	}

	return cmd
}

// isKubeletRunning checks if kubelet service is running
func (s *KubeadmJoinStep) isKubeletRunning(ctx context.Context, runner runner.Runner, conn connector.Connector) (bool, error) {
	cmd := "systemctl is-active kubelet"
	output, err := runnerSvc.Run(ctx, conn, cmd)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(output) == "active", nil
}

// isNodeInCluster checks if the node is already part of a Kubernetes cluster
func (s *KubeadmJoinStep) isNodeInCluster(ctx context.Context, runner runner.Runner, conn connector.Connector) (bool, error) {
	// Check if kubelet config exists and has cluster info
	kubeletConfigPath := "/var/lib/kubelet/config.yaml"
	exists, err := runnerSvc.Exists(ctx, conn, kubeletConfigPath)
	if err != nil || !exists {
		return false, nil
	}

	// Read kubelet config to check for cluster configuration
	configContent, err := runner.ReadFile(ctx, conn, kubeletConfigPath)
	if err != nil {
		return false, nil
	}

	// Basic check for cluster configuration
	return strings.Contains(configContent, "clusterDNS") && 
		   strings.Contains(configContent, "clusterDomain"), nil
}

// setupKubectl sets up kubectl for the current user on control plane nodes
func (s *KubeadmJoinStep) setupKubectl(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	kubeconfigPath := "/etc/kubernetes/admin.conf"

	// Check if admin.conf exists
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, kubeconfigPath)
	if err != nil || !exists {
		return fmt.Errorf("admin kubeconfig not found at %s", kubeconfigPath)
	}

	// Create .kube directory for root user
	mkdirCmd := "mkdir -p $HOME/.kube"
	_, err = runnerSvc.Run(ctx.GoContext(), conn, mkdirCmd)
	if err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	// Copy admin.conf to user's .kube/config
	copyCmd := fmt.Sprintf("cp -i %s $HOME/.kube/config", kubeconfigPath)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, copyCmd)
	if err != nil {
		return fmt.Errorf("failed to copy kubeconfig: %w", err)
	}

	// Set proper ownership
	chownCmd := "chown $(id -u):$(id -g) $HOME/.kube/config"
	_, err = runnerSvc.Run(ctx.GoContext(), conn, chownCmd)
	if err != nil {
		return fmt.Errorf("failed to set kubeconfig ownership: %w", err)
	}

	return nil
}