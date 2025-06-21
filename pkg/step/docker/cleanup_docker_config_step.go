package docker

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CleanupDockerConfigStep removes Docker configuration files and directories.
type CleanupDockerConfigStep struct {
	meta             spec.StepMeta
	DaemonJSONPath   string // e.g., /etc/docker/daemon.json
	DockerRootDir    string // e.g., /etc/docker (to remove the whole dir if empty or specified)
	ServiceFilePath  string // e.g., /etc/systemd/system/docker.service (if installed manually, not via package)
	SocketFilePath   string // e.g., /etc/systemd/system/docker.socket (if installed manually)
	Sudo             bool
}

// NewCleanupDockerConfigStep creates a new CleanupDockerConfigStep.
func NewCleanupDockerConfigStep(instanceName, daemonJSONPath, dockerRootDir, serviceFilePath, socketFilePath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "CleanupDockerConfiguration"
	}
	djp := daemonJSONPath
	if djp == "" {
		djp = DefaultDockerDaemonJSONPath // From generate_docker_daemon_json_step.go
	}
	drd := dockerRootDir // Optional, if not set, only daemon.json might be removed.

	// serviceFilePath and socketFilePath are often managed by package uninstall.
	// Only specify them if they were manually created or need explicit removal beyond package manager.

	return &CleanupDockerConfigStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes Docker configuration items (daemon.json: %s, root config dir: %s, service: %s, socket: %s).", djp, drd, serviceFilePath, socketFilePath),
		},
		DaemonJSONPath:  djp,
		DockerRootDir:   drd,
		ServiceFilePath: serviceFilePath,
		SocketFilePath:  socketFilePath,
		Sudo:            true,
	}
}

func (s *CleanupDockerConfigStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CleanupDockerConfigStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	pathsToCheck := []string{s.DaemonJSONPath}
	if s.DockerRootDir != "" { // Only check root dir if specified for removal
		pathsToCheck = append(pathsToCheck, s.DockerRootDir)
	}
	if s.ServiceFilePath != "" {
		pathsToCheck = append(pathsToCheck, s.ServiceFilePath)
	}
	if s.SocketFilePath != "" {
		pathsToCheck = append(pathsToCheck, s.SocketFilePath)
	}

	allMissing := true
	for _, p := range pathsToCheck {
		if p == "" { continue }
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, p)
		if err != nil {
			logger.Warn("Failed to check existence, assuming it might exist.", "path", p, "error", err)
			return false, nil
		}
		if exists {
			logger.Info("Docker configuration item still exists.", "path", p)
			allMissing = false
		}
	}

	if allMissing {
		logger.Info("All specified Docker configuration items already removed.")
		return true, nil
	}
	return false, nil
}

func (s *CleanupDockerConfigStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	var lastErr error

	itemsToRemove := []string{s.DaemonJSONPath}
	if s.ServiceFilePath != "" {
		itemsToRemove = append(itemsToRemove, s.ServiceFilePath)
	}
	if s.SocketFilePath != "" {
		itemsToRemove = append(itemsToRemove, s.SocketFilePath)
	}
	// DockerRootDir is removed last if specified, as it might contain daemon.json
	if s.DockerRootDir != "" && s.DockerRootDir != "/" && s.DockerRootDir != "/etc" { // Safety check
		itemsToRemove = append(itemsToRemove, s.DockerRootDir)
	}


	for _, itemPath := range itemsToRemove {
		if itemPath == "" { continue }
		logger.Info("Removing Docker configuration item.", "path", itemPath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, itemPath, s.Sudo); err != nil {
			// If removing a directory that contained daemon.json already removed, it might error.
			// Runner's Remove should ideally have an IgnoreNotExist option.
			logger.Error("Failed to remove item (best effort).", "path", itemPath, "error", err)
			lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", itemPath, err, lastErr)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("one or more errors occurred during Docker config cleanup: %w", lastErr)
	}
	logger.Info("Docker configuration cleanup successful.")
	return nil
}

func (s *CleanupDockerConfigStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for CleanupDockerConfigStep is not applicable.")
	return nil
}

var _ step.Step = (*CleanupDockerConfigStep)(nil)
