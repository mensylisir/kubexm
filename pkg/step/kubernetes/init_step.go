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

// KubeadmInitStepSpec defines the configuration for kubeadm init
type KubeadmInitStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// ConfigPath is the path to the kubeadm configuration file
	ConfigPath string `json:"configPath,omitempty" yaml:"configPath,omitempty"`

	// KubeconfigPath is where to save the admin kubeconfig
	KubeconfigPath string `json:"kubeconfigPath,omitempty" yaml:"kubeconfigPath,omitempty"`

	// UploadCerts indicates whether to upload control plane certificates
	UploadCerts bool `json:"uploadCerts,omitempty" yaml:"uploadCerts,omitempty"`

	// CertificateKey is the key used to encrypt uploaded certificates
	CertificateKey string `json:"certificateKey,omitempty" yaml:"certificateKey,omitempty"`

	// SkipPhases lists kubeadm phases to skip
	SkipPhases []string `json:"skipPhases,omitempty" yaml:"skipPhases,omitempty"`

	// IgnorePreflightErrors lists preflight errors to ignore
	IgnorePreflightErrors []string `json:"ignorePreflightErrors,omitempty" yaml:"ignorePreflightErrors,omitempty"`

	// DryRun indicates whether to perform a dry run
	DryRun bool `json:"dryRun,omitempty" yaml:"dryRun,omitempty"`

	// CacheJoinInfo indicates whether to cache join information
	CacheJoinInfo bool `json:"cacheJoinInfo,omitempty" yaml:"cacheJoinInfo,omitempty"`

	// KubeadmPath is the path to the kubeadm binary
	KubeadmPath string `json:"kubeadmPath,omitempty" yaml:"kubeadmPath,omitempty"`
}

// KubeadmInitStep implements the Step interface for kubeadm init
type KubeadmInitStep struct {
	spec KubeadmInitStepSpec
}

// NewKubeadmInitStep creates a new KubeadmInitStep
func NewKubeadmInitStep(spec KubeadmInitStepSpec) *KubeadmInitStep {
	// Set default values
	if spec.ConfigPath == "" {
		spec.ConfigPath = "/etc/kubernetes/kubeadm-config.yaml"
	}
	if spec.KubeconfigPath == "" {
		spec.KubeconfigPath = "/etc/kubernetes/admin.conf"
	}
	if spec.KubeadmPath == "" {
		spec.KubeadmPath = "/usr/local/bin/kubeadm"
	}

	return &KubeadmInitStep{spec: spec}
}

// Meta returns the step metadata
func (s *KubeadmInitStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if kubeadm init has already been run
func (s *KubeadmInitStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
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

	// Check if kubeconfig exists
	kubeconfigExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.KubeconfigPath)
	if err != nil {
		ctx.GetLogger().Debugf("error checking kubeconfig: %v", err)
		kubeconfigExists = false
	}

	// Check if API server is responding
	apiServerHealthy := false
	if kubeconfigExists {
		healthy, err := s.isAPIServerHealthy(ctx.GoContext(), runner, conn)
		if err != nil {
			ctx.GetLogger().Debugf("error checking API server health: %v", err)
		} else {
			apiServerHealthy = healthy
		}
	}

	// If all checks pass, kubeadm init has already been completed
	if kubeletRunning && kubeconfigExists && apiServerHealthy {
		ctx.GetLogger().Infof("kubeadm init appears to have already been completed")
		return true, nil
	}

	return false, nil
}

// Run executes the kubeadm init
func (s *KubeadmInitStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Running kubeadm init on host %s", host.GetName())

	// Build kubeadm init command
	cmd := s.buildKubeadmInitCommand()
	
	logger.Infof("Executing: %s", cmd)

	// Execute kubeadm init
	output, err := runnerSvc.Run(ctx.GoContext(), conn, cmd)
	if err != nil {
		return fmt.Errorf("kubeadm init failed: %w\nOutput: %s", err, output)
	}

	logger.Infof("kubeadm init completed successfully")

	// Cache join information if requested
	if s.spec.CacheJoinInfo {
		err = s.cacheJoinInformation(ctx, runner, conn, output)
		if err != nil {
			logger.Warnf("failed to cache join information: %v", err)
		}
	}

	// Setup kubectl for the user if not dry run
	if !s.spec.DryRun {
		err = s.setupKubectl(ctx, runner, conn)
		if err != nil {
			logger.Warnf("failed to setup kubectl: %v", err)
		}
	}

	return nil
}

// Rollback attempts to reset the kubeadm init
func (s *KubeadmInitStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back kubeadm init on host %s", host.GetName())

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
		"/var/lib/etcd",
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

	logger.Infof("kubeadm init rollback completed")
	return nil
}

// buildKubeadmInitCommand builds the kubeadm init command with all options
func (s *KubeadmInitStep) buildKubeadmInitCommand() string {
	cmd := []string{s.spec.KubeadmPath, "init"}

	// Add config file if specified
	if s.spec.ConfigPath != "" {
		cmd = append(cmd, "--config", s.spec.ConfigPath)
	}

	// Add upload certs option
	if s.spec.UploadCerts {
		cmd = append(cmd, "--upload-certs")
		if s.spec.CertificateKey != "" {
			cmd = append(cmd, "--certificate-key", s.spec.CertificateKey)
		}
	}

	// Add skip phases
	if len(s.spec.SkipPhases) > 0 {
		for _, phase := range s.spec.SkipPhases {
			cmd = append(cmd, "--skip-phases", phase)
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

	return strings.Join(cmd, " ")
}

// isKubeletRunning checks if kubelet service is running
func (s *KubeadmInitStep) isKubeletRunning(ctx context.Context, runner runner.Runner, conn connector.Connector) (bool, error) {
	cmd := "systemctl is-active kubelet"
	output, err := runnerSvc.Run(ctx, conn, cmd)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(output) == "active", nil
}

// isAPIServerHealthy checks if the API server is responding
func (s *KubeadmInitStep) isAPIServerHealthy(ctx context.Context, runner runner.Runner, conn connector.Connector) (bool, error) {
	cmd := fmt.Sprintf("kubectl --kubeconfig=%s get nodes", s.spec.KubeconfigPath)
	_, err := runnerSvc.Run(ctx, conn, cmd)
	return err == nil, err
}

// cacheJoinInformation extracts and caches join information from kubeadm init output
func (s *KubeadmInitStep) cacheJoinInformation(ctx step.StepContext, runner runner.Runner, conn connector.Connector, output string) error {
	logger := ctx.GetLogger()
	
	// Extract join command from output
	joinCmd := s.extractJoinCommand(output)
	if joinCmd == "" {
		// Generate join command manually
		tokenCmd := fmt.Sprintf("%s token create --print-join-command", s.spec.KubeadmPath)
		joinCmdOutput, err := runnerSvc.Run(ctx.GoContext(), conn, tokenCmd)
		if err != nil {
			return fmt.Errorf("failed to generate join command: %w", err)
		}
		joinCmd = strings.TrimSpace(joinCmdOutput)
	}

	if joinCmd != "" {
		// Cache the join command for other nodes to use
		cache := ctx.GetStepCache()
		cache.Set("kubeadm-join-command", joinCmd)
		logger.Infof("cached kubeadm join command")
	}

	// Extract certificate key if upload-certs was used
	certKey := s.extractCertificateKey(output)
	if certKey != "" {
		cache := ctx.GetStepCache()
		cache.Set("kubeadm-certificate-key", certKey)
		logger.Infof("cached kubeadm certificate key")
	}

	return nil
}

// extractJoinCommand extracts the join command from kubeadm init output
func (s *KubeadmInitStep) extractJoinCommand(output string) string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "kubeadm join") {
			// The join command might span multiple lines
			joinCmd := strings.TrimSpace(line)
			// Check next few lines for continuation
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				nextLine := strings.TrimSpace(lines[j])
				if nextLine != "" && !strings.HasPrefix(nextLine, "Then you can join") {
					if strings.HasPrefix(nextLine, "--") {
						joinCmd += " " + nextLine
					} else {
						break
					}
				}
			}
			return joinCmd
		}
	}
	return ""
}

// extractCertificateKey extracts the certificate key from kubeadm init output
func (s *KubeadmInitStep) extractCertificateKey(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "--certificate-key") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "--certificate-key" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return ""
}

// setupKubectl sets up kubectl for the current user
func (s *KubeadmInitStep) setupKubectl(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	// Create .kube directory for root user
	mkdirCmd := "mkdir -p $HOME/.kube"
	_, err := runnerSvc.Run(ctx.GoContext(), conn, mkdirCmd)
	if err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	// Copy admin.conf to user's .kube/config
	copyCmd := fmt.Sprintf("cp -i %s $HOME/.kube/config", s.spec.KubeconfigPath)
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