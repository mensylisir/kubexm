package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RemoveContainerdBinariesStep removes containerd related binaries from system paths.
type RemoveContainerdBinariesStep struct {
	meta         spec.StepMeta
	BinaryPaths  []string // List of absolute paths to binaries to remove, e.g., /usr/local/bin/containerd
	Sudo         bool
}

// NewRemoveContainerdBinariesStep creates a new RemoveContainerdBinariesStep.
// If binaryPaths is empty, it uses a default list.
func NewRemoveContainerdBinariesStep(instanceName string, binaryPaths []string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "RemoveContainerdBinaries"
	}
	paths := binaryPaths
	if len(paths) == 0 {
		paths = []string{
			"/usr/local/bin/containerd",
			"/usr/local/bin/containerd-shim",
			"/usr/local/bin/containerd-shim-runc-v1",
			"/usr/local/bin/containerd-shim-runc-v2",
			"/usr/local/bin/ctr",
			// "/usr/local/sbin/runc", // Runc removal should be its own step if installed separately
			// Also consider /opt/cni/bin for CNI plugins
		}
	}

	return &RemoveContainerdBinariesStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes containerd related binaries: %v", paths),
		},
		BinaryPaths:  paths,
		Sudo:         true, // Removing from system paths usually requires sudo
	}
}

func (s *RemoveContainerdBinariesStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *RemoveContainerdBinariesStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	allMissing := true
	for _, binPath := range s.BinaryPaths {
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, binPath)
		if err != nil {
			logger.Warn("Failed to check for binary existence, assuming it might exist.", "path", binPath, "error", err)
			return false, nil
		}
		if exists {
			logger.Info("Containerd-related binary still exists.", "path", binPath)
			allMissing = false
		}
	}

	if allMissing {
		logger.Info("All specified containerd-related binaries already removed.")
		return true, nil
	}
	return false, nil
}

func (s *RemoveContainerdBinariesStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	var lastErr error
	for _, binPath := range s.BinaryPaths {
		logger.Info("Removing binary.", "path", binPath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, binPath, s.Sudo); err != nil {
			logger.Error("Failed to remove binary (best effort).", "path", binPath, "error", err)
			lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", binPath, err, lastErr)
		}
	}

	// Removing CNI plugins directory /opt/cni/bin
	// This should be conditional or a separate step if /opt/cni/bin is shared.
	// For a full containerd cleanup, it's often included.
	cniBinDir := "/opt/cni/bin"
	logger.Info("Removing CNI plugin directory.", "path", cniBinDir)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, cniBinDir, s.Sudo); err != nil {
		logger.Error("Failed to remove CNI plugin directory (best effort).", "path", cniBinDir, "error", err)
		lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", cniBinDir, err, lastErr)
	}


	if lastErr != nil {
		return fmt.Errorf("one or more errors occurred while removing containerd binaries/CNI: %w", lastErr)
	}
	logger.Info("Containerd-related binaries and CNI plugins removed successfully.")
	return nil
}

func (s *RemoveContainerdBinariesStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for RemoveContainerdBinariesStep is not applicable.")
	return nil
}

var _ step.Step = (*RemoveContainerdBinariesStep)(nil)
