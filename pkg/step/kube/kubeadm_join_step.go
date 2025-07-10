package kube

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	kubernikus "github.com/mensylisir/kubexm/pkg/step/kubernetes" // For cache keys
)

// KubeadmJoinStep executes 'kubeadm join' on a node.
type KubeadmJoinStep struct {
	meta                  spec.StepMeta
	IsControlPlane        bool
	ControlPlaneAddress   string // e.g., "apiserver.example.com:6443" or "192.168.1.100:6443"
	Token                 string // If not using full JoinCommand from cache
	DiscoveryTokenCACertHash string // If not using full JoinCommand from cache
	CertificateKey        string // Required if IsControlPlane is true, if not using full JoinCommand
	JoinCommandFromCache  bool   // If true, attempts to use full join command from KubeadmInitOutputCacheKey or KubeadmJoinCommandCacheKey
	IgnorePreflightErrors string
	CriSocket             string // Optional: path to CRI socket
	Sudo                  bool
}

// NewKubeadmJoinStep creates a new KubeadmJoinStep.
// If joinCommandFromCache is true, other specific params like Token might be ignored if command is found in cache.
func NewKubeadmJoinStep(instanceName string, isControlPlane bool, controlPlaneAddress, token, discoveryHash, certKey, ignoreErrors, criSocket string, sudo, joinCommandFromCache bool) step.Step {
	name := instanceName
	if name == "" {
		name = "KubeadmJoin"
		if isControlPlane {
			name += "ControlPlane"
		} else {
			name += "Worker"
		}
	}
	return &KubeadmJoinStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Joins node to Kubernetes cluster (ControlPlane: %v)", isControlPlane),
		},
		IsControlPlane:        isControlPlane,
		ControlPlaneAddress:   controlPlaneAddress,
		Token:                 token,
		DiscoveryTokenCACertHash: discoveryHash,
		CertificateKey:        certKey,
		JoinCommandFromCache:  joinCommandFromCache,
		IgnorePreflightErrors: ignoreErrors,
		CriSocket:             criSocket,
		Sudo:                  true, // kubeadm join usually requires sudo
	}
}

func (s *KubeadmJoinStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubeadmJoinStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	// A simple precheck: check if /etc/kubernetes/kubelet.conf exists.
	// If it exists, the node has likely already joined.
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("Precheck: failed to get connector for host %s: %w", host.GetName(), err)
	}
	kubeletConfPath := "/etc/kubernetes/kubelet.conf"
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, kubeletConfPath)
	if err != nil {
		logger.Warn("Failed to check for kubelet.conf, assuming node has not joined.", "path", kubeletConfPath, "error", err)
		return false, nil // Proceed with Run
	}
	if exists {
		logger.Info("Node already has kubelet.conf, assuming it has joined the cluster.", "path", kubeletConfPath)
		return true, nil // Already joined
	}
	logger.Info("Node does not have kubelet.conf, attempting to join.", "path", kubeletConfPath)
	return false, nil
}

func (s *KubeadmJoinStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmd string

	if s.JoinCommandFromCache {
		cachedCmd, found := ctx.GetTaskCache().Get(kubernikus.KubeadmJoinCommandCacheKey) // Assuming KubeadmJoinCommandCacheKey holds worker join
		if s.IsControlPlane {
			// For control plane, a specific join command might be different or might need certificate key.
			// Kubeadm init output often provides a specific join command for control planes.
			// Let's assume for now that if IsControlPlane is true, we construct it, or a specific
			// control plane join command was cached by KubeadmInitStep.
			// For now, error if specific CP join command isn't found and JoinCommandFromCache is true.
			// This part needs careful coordination with KubeadmInitStep's caching.
			// Let's assume KubeadmInitStep might cache "KubeadmControlPlaneJoinCommand"
			cachedCpCmd, cpFound := ctx.GetTaskCache().Get("KubeadmControlPlaneJoinCommand") // Example key
			if cpFound {
				cmd = cachedCpCmd.(string)
				logger.Info("Using cached full control-plane join command.")
			} else if found && !s.IsControlPlane { // Fallback to worker join if CP not found and it's not a CP join
				cmd = cachedCmd.(string)
				logger.Info("Using cached full worker join command.")
			} else {
				// If JoinCommandFromCache is true, but no suitable command found, fallback to constructing.
				logger.Warn("JoinCommandFromCache was true, but no suitable command found in cache. Will attempt to construct.")
				// Fall through to construct command manually
			}
		} else if found { // Worker node and worker join command found
			cmd = cachedCmd.(string)
			logger.Info("Using cached full worker join command.")
		} else {
			logger.Warn("JoinCommandFromCache was true, but no join command found in cache. Will attempt to construct.")
			// Fall through
		}
	}

	if cmd == "" { // If not using cached command or cache miss
		logger.Info("Constructing kubeadm join command manually.")
		token := s.Token
		discoveryHash := s.DiscoveryTokenCACertHash
		certKey := s.CertificateKey

		if token == "" {
			cachedToken, found := ctx.GetTaskCache().Get(kubernikus.KubeadmTokenCacheKey)
			if !found {
				return fmt.Errorf("kubeadm token not provided and not found in cache for step %s", s.meta.Name)
			}
			token = cachedToken.(string)
		}
		if discoveryHash == "" {
			cachedHash, found := ctx.GetTaskCache().Get(kubernikus.KubeadmDiscoveryHashCacheKey)
			if !found {
				return fmt.Errorf("kubeadm discovery token CA cert hash not provided and not found in cache for step %s", s.meta.Name)
			}
			discoveryHash = cachedHash.(string)
		}

		cmdArgs := []string{"kubeadm", "join", s.ControlPlaneAddress}
		cmdArgs = append(cmdArgs, "--token", token)
		cmdArgs = append(cmdArgs, "--discovery-token-ca-cert-hash", "sha256:"+discoveryHash)

		if s.IsControlPlane {
			cmdArgs = append(cmdArgs, "--control-plane")
			if certKey == "" {
				cachedCertKey, found := ctx.GetTaskCache().Get(kubernikus.KubeadmCertificateKeyCacheKey)
				if !found {
					return fmt.Errorf("certificate key not provided and not found in cache for control-plane join step %s", s.meta.Name)
				}
				certKey = cachedCertKey.(string)
			}
			cmdArgs = append(cmdArgs, "--certificate-key", certKey)
		}
		if s.IgnorePreflightErrors != "" {
			cmdArgs = append(cmdArgs, "--ignore-preflight-errors="+s.IgnorePreflightErrors)
		}
		if s.CriSocket != "" {
			cmdArgs = append(cmdArgs, "--cri-socket", s.CriSocket)
		}
		cmd = strings.Join(cmdArgs, " ")
	}


	logger.Info("Running kubeadm join command.", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Error("kubeadm join failed.", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("kubeadm join failed for step %s: %w. Stdout: %s, Stderr: %s", s.meta.Name, err, string(stdout), string(stderr))
	}

	logger.Info("kubeadm join completed successfully.", "stdout", string(stdout))
	return nil
}

func (s *KubeadmJoinStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback for join is typically a reset.
	resetStep := NewKubeadmResetStep("RollbackJoin:"+s.meta.Name, true /*force*/, "all", "", s.CriSocket, s.Sudo)
	logger.Info("Attempting kubeadm reset as rollback for join.")
	return resetStep.Run(ctx, host) // Run the reset step's logic directly
}

var _ step.Step = (*KubeadmJoinStep)(nil)
```
