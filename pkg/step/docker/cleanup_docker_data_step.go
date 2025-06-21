package docker

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CleanupDockerDataStep removes the Docker data directory (typically /var/lib/docker).
// This is a destructive operation.
type CleanupDockerDataStep struct {
	meta    spec.StepMeta
	DataDir string // Path to the Docker data directory
	Sudo    bool
}

// NewCleanupDockerDataStep creates a new CleanupDockerDataStep.
func NewCleanupDockerDataStep(instanceName, dataDir string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "CleanupDockerDataDirectory"
	}
	dd := dataDir
	if dd == "" {
		dd = "/var/lib/docker" // Default data directory for Docker
	}
	return &CleanupDockerDataStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes the Docker data directory %s. This is destructive.", dd),
		},
		DataDir: dd,
		Sudo:    true,
	}
}

func (s *CleanupDockerDataStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CleanupDockerDataStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	if s.DataDir == "" {
		err := fmt.Errorf("DataDir is not specified")
		logger.Error("Precheck failed: DataDir is empty.", "error", err)
		return false, err
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.DataDir)
	if err != nil {
		logger.Warn("Failed to check for existence of Docker data directory, assuming it might exist.", "path", s.DataDir, "error", err)
		return false, nil
	}

	if !exists {
		logger.Info("Docker data directory already removed or does not exist.", "path", s.DataDir)
		return true, nil
	}
	logger.Info("Docker data directory exists and needs removal.", "path", s.DataDir)
	return false, nil
}

func (s *CleanupDockerDataStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.DataDir == "" {
		return fmt.Errorf("DataDir is not specified for host %s", host.GetName())
	}
	if s.DataDir == "/" || s.DataDir == "/var" || s.DataDir == "/var/lib" || s.DataDir == "/etc" {
		return fmt.Errorf("refusing to remove potentially critical directory: %s", s.DataDir)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	// Ensure Docker service is stopped before removing data dir.
	isActive, _ := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, "docker")
	if isActive {
		logger.Warn("Docker service is active. It should be stopped before removing data directory. Attempting to stop now...")
		stopCmd := "systemctl stop docker" // Or use ManageDockerServiceStep(ActionStop) if available to runner
		_, _, stopErr := runnerSvc.Run(ctx.GoContext(),conn, stopCmd, s.Sudo)
		if stopErr != nil {
			logger.Error("Failed to stop Docker service before data dir removal. Aborting data removal.", "error", stopErr)
			return fmt.Errorf("failed to stop Docker before data dir removal: %w", stopErr)
		}
		logger.Info("Docker service stopped.")
	}
	// Also stop cri-dockerd if it's likely running and using this data dir indirectly
	isCriDockerdActive, _ := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, "cri-dockerd")
	if isCriDockerdActive {
		logger.Warn("cri-dockerd service is active. Attempting to stop now...")
		stopCriDockerdCmd := "systemctl stop cri-dockerd"
		_, _, stopCriDockerdErr := runnerSvc.Run(ctx.GoContext(), conn, stopCriDockerdCmd, s.Sudo)
		if stopCriDockerdErr != nil {
			logger.Error("Failed to stop cri-dockerd service. Continuing data removal with caution.", "error", stopCriDockerdErr)
			// Not necessarily fatal for data removal itself, but important context.
		} else {
			logger.Info("cri-dockerd service stopped.")
		}
	}


	logger.Info("Removing Docker data directory.", "path", s.DataDir)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.DataDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to remove Docker data directory %s: %w", s.DataDir, err)
	}

	logger.Info("Docker data directory removed successfully.")
	return nil
}

func (s *CleanupDockerDataStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for CleanupDockerDataStep is not applicable.")
	return nil
}

var _ step.Step = (*CleanupDockerDataStep)(nil)
