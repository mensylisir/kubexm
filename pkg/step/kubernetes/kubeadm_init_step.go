package kubernetes

import (
	"fmt"
	"regexp"
	"strings"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	KubeadmInitOutputCacheKey      = "KubeadmInitOutput"
	KubeadmJoinCommandCacheKey     = "KubeadmJoinCommand"
	KubeadmTokenCacheKey           = "KubeadmToken"
	KubeadmCertificateKeyCacheKey  = "KubeadmCertificateKey" // For control-plane join
	KubeadmDiscoveryHashCacheKey = "KubeadmDiscoveryHash"  // For worker join
)

// KubeadmInitStep executes 'kubeadm init' and captures its output.
type KubeadmInitStep struct {
	meta                spec.StepMeta
	ConfigPathOnHost    string // Path to the kubeadm config file on the target host
	Sudo                bool
	IgnorePreflightErrors string // Comma-separated list of preflight errors to ignore, or "all"
}

// NewKubeadmInitStep creates a new KubeadmInitStep.
func NewKubeadmInitStep(instanceName, configPathOnHost, ignorePreflightErrors string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "KubeadmInit"
	}
	return &KubeadmInitStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Initializes Kubernetes control plane using kubeadm with config %s", configPathOnHost),
		},
		ConfigPathOnHost:    configPathOnHost,
		Sudo:                sudo,
		IgnorePreflightErrors: ignorePreflightErrors,
	}
}

func (s *KubeadmInitStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubeadmInitStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	// Kubeadm init is generally not idempotent in a simple way by checking file existence.
	// It might be considered "done" if the control plane is already up and running.
	// This would involve more complex checks (e.g., API server health, etcd health).
	// For now, assume it always needs to run if scheduled, or a more specific pre-task handles this.
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	logger.Info("KubeadmInitStep Precheck: Assuming run is required if this step is reached.")
	return false, nil
}

func (s *KubeadmInitStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmd := fmt.Sprintf("kubeadm init --config %s --upload-certs", s.ConfigPathOnHost)
	if s.IgnorePreflightErrors != "" {
		cmd += fmt.Sprintf(" --ignore-preflight-errors=%s", s.IgnorePreflightErrors)
	}

	logger.Info("Running kubeadm init command", "command", cmd)
	// Kubeadm init can take a while. A timeout might be needed from ExecOptions.
	// For now, using default timeout from runner.
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})

	// Store full output in cache for debugging or other purposes
	ctx.TaskCache().Set(KubeadmInitOutputCacheKey, string(stdout)+"\n"+string(stderr))

	if err != nil {
		logger.Error("kubeadm init failed", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("kubeadm init failed: %w. Stdout: %s, Stderr: %s", err, string(stdout), string(stderr))
	}

	logger.Info("kubeadm init completed successfully.", "stdout", string(stdout))

	// TODO: Parse stdout for join command, token, cert hash and store them in TaskCache
	// Example (simplified parsing):
	// outputStr := string(stdout)
	// if joinCmdLine := findJoinCommand(outputStr); joinCmdLine != "" {
	//    ctx.TaskCache().Set(KubeadmJoinCommandCacheKey, joinCmdLine)
	//    logger.Info("Found and cached kubeadm join command.")
	// }
	// Similar for token, cert key, discovery hash.

	// Parse output for critical information
	outputStr := string(stdout)

	// Regex for worker join command
	// Example: kubeadm join <control-plane-host>:<control-plane-port> --token <token> --discovery-token-ca-cert-hash sha256:<hash>
	workerJoinRegex := regexp.MustCompile(`kubeadm join .* --token\s*([^\s]+)\s*--discovery-token-ca-cert-hash\s*sha256:([a-f0-9]{64})`)
	// Regex for control-plane join command (includes --certificate-key)
	// Example: kubeadm join <control-plane-host>:<control-plane-port> --token <token> --discovery-token-ca-cert-hash sha256:<hash> --control-plane --certificate-key <key>
	controlPlaneJoinRegex := regexp.MustCompile(`kubeadm join .* --token\s*([^\s]+)\s*--discovery-token-ca-cert-hash\s*sha256:([a-f0-9]{64})\s*--control-plane\s*--certificate-key\s*([a-f0-9]{64})`)

	// Extract worker join info
	workerMatches := workerJoinRegex.FindStringSubmatch(outputStr)
	if len(workerMatches) == 3 {
		token := workerMatches[1]
		discoveryHash := workerMatches[2]
		ctx.TaskCache().Set(KubeadmTokenCacheKey, token)
		ctx.TaskCache().Set(KubeadmDiscoveryHashCacheKey, discoveryHash)
		// Try to find the full join command line for workers
		for _, line := range strings.Split(outputStr, "\n") {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "kubeadm join") && !strings.Contains(trimmedLine, "--control-plane") {
				ctx.TaskCache().Set(KubeadmJoinCommandCacheKey, trimmedLine) // Cache the first worker join command found
				logger.Info("Found and cached kubeadm worker join command.", "command", trimmedLine)
				break
			}
		}
		logger.Info("Cached kubeadm token and discovery hash.", "token", token, "hash", discoveryHash)
	} else {
		logger.Warn("Could not parse worker join token and discovery hash from kubeadm init output.")
	}

	// Extract control-plane join info (certificate key)
	// This often appears separately or as part of a longer join command for control plane nodes.
	// The regex above tries to capture it if it's in a full command.
	// Kubeadm also prints it like: --certificate-key <key>
	cpMatches := controlPlaneJoinRegex.FindStringSubmatch(outputStr)
	if len(cpMatches) == 4 {
		// Token and hash might be redundant if already captured by worker regex, but good to have if format differs.
		// cpToken := cpMatches[1]
		// cpDiscoveryHash := cpMatches[2]
		certificateKey := cpMatches[3]
		ctx.TaskCache().Set(KubeadmCertificateKeyCacheKey, certificateKey)
		logger.Info("Cached kubeadm certificate key for control-plane join.", "key", certificateKey)

		// Try to find the full control-plane join command line
		for _, line := range strings.Split(outputStr, "\n") {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "kubeadm join") && strings.Contains(trimmedLine, "--control-plane") {
				// Cache the full control-plane join command if needed by other steps.
				// For now, just caching the key is often sufficient as other parts are known or in cache.
				// Example: ctx.TaskCache().Set("KubeadmControlPlaneJoinCommand", trimmedLine)
				logger.Info("Found control-plane join command line.", "command", trimmedLine)
				break
			}
		}
	} else {
		// Fallback: search for certificate key if not in a full join command line in the output
		certKeyRegex := regexp.MustCompile(`--certificate-key\s*([a-f0-9]{64})`)
		certKeyMatch := certKeyRegex.FindStringSubmatch(outputStr)
		if len(certKeyMatch) == 2 {
			certificateKey := certKeyMatch[1]
			ctx.TaskCache().Set(KubeadmCertificateKeyCacheKey, certificateKey)
			logger.Info("Cached kubeadm certificate key (found standalone).", "key", certificateKey)
		} else {
			logger.Warn("Could not parse certificate key from kubeadm init output.")
		}
	}

	// It's also common for kubeadm to output the token and hash separately.
	// Example: "Your Kubernetes control-plane has initialized successfully!"
	// "To start using your cluster, you need to run the following as a regular user:"
	//   mkdir -p $HOME/.kube
	//   sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
	//   sudo chown $(id -u):$(id -g) $HOME/.kube/config
	//
	// "Alternatively, if you are the root user, you can run:"
	//   export KUBECONFIG=/etc/kubernetes/admin.conf
	//
	// "You should now deploy a pod network to the cluster."
	// "Run \"kubectl apply -f [podnetwork].yaml\" with one of the options listed at:"
	//   https://kubernetes.io/docs/concepts/cluster-administration/addons/
	//
	// "Then you can join any number of worker nodes by running the following on each as root:"
	// kubeadm join 10.0.2.15:6443 --token abcdef.0123456789abcdef \
	//	--discovery-token-ca-cert-hash sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
	//
	// "Then you can join any number of control-plane nodes by running the following on each as root:"
	// kubeadm join 10.0.2.15:6443 --token abcdef.0123456789abcdef \
	//	--discovery-token-ca-cert-hash sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
	//	--control-plane --certificate-key 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

	return nil
}

func (s *KubeadmInitStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback", "error", err)
		return nil // Best effort
	}

	cmd := "kubeadm reset -f" // Force reset
	logger.Info("Attempting kubeadm reset for rollback", "command", cmd)
	_, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Warn("kubeadm reset command failed during rollback (best effort).", "error", err, "stderr", string(stderr))
	} else {
		logger.Info("kubeadm reset executed successfully for rollback.")
	}
	return nil
}

var _ step.Step = (*KubeadmInitStep)(nil)

[end of pkg/step/kubernetes/kubeadm_init_step.go]
