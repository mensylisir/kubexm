package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	//lint:ignore SA1019 we need to use this for now
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/pkg/errors"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/util"
)

const (
	// DefaultDockerInspectTimeout is the default timeout for docker inspect commands.
	DefaultDockerInspectTimeout = 30 * time.Second
	// DefaultDockerPullTimeout is the default timeout for docker pull commands.
	DefaultDockerPullTimeout = 15 * time.Minute
	// DefaultDockerBuildTimeout is the default timeout for docker build commands.
	DefaultDockerBuildTimeout = 60 * time.Minute
	// DefaultDockerRMTimeout is the default timeout for docker rmi/rm commands.
	DefaultDockerRMTimeout = 5 * time.Minute
	// DefaultDockerCreateTimeout is the default timeout for docker create commands.
	DefaultDockerCreateTimeout = 1 * time.Minute
	// DefaultDockerStartTimeout is the default timeout for docker start commands.
	DefaultDockerStartTimeout = 1 * time.Minute
	// DefaultDockerStopTimeout is the default timeout for docker stop commands.
	// This is the timeout for the 'docker stop' command itself to wait for graceful shutdown.
	DefaultDockerStopGracePeriod = 10 * time.Second
	// DefaultDockerStopExecTimeout is the timeout for the Exec call running 'docker stop'.
	// It should be greater than the grace period.
	DefaultDockerStopExecTimeout = DefaultDockerStopGracePeriod + (30 * time.Second)
	// DefaultDockerRestartTimeout is the default timeout for 'docker restart' command itself to wait for graceful shutdown.
	DefaultDockerRestartGracePeriod = 10 * time.Second
	// DefaultDockerRestartExecTimeout is the timeout for the Exec call running 'docker restart'.
	DefaultDockerRestartExecTimeout = DefaultDockerRestartGracePeriod + (10 * time.Second)
	dockerDaemonConfigPath        = "/etc/docker/daemon.json"
)

var (
	// sizeRegex helps parse human-readable sizes from Docker commands (e.g., "1.23GB", "500MB").
	sizeRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMGT]?B)$`)
	sizeUnits = map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}
)

// parseDockerSize converts a Docker size string (e.g., "100MB") to bytes.
func parseDockerSize(sizeStr string) (int64, error) {
	matches := sizeRegex.FindStringSubmatch(strings.ToUpper(sizeStr))
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size value '%s': %w", matches[1], err)
	}

	unit, ok := sizeUnits[matches[2]]
	if !ok {
		return 0, fmt.Errorf("unknown size unit '%s'", matches[2])
	}

	return int64(value * float64(unit)), nil
}

// shellEscape ensures that a string is properly quoted for shell execution.
func shellEscape(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// PullImage pulls a Docker image from a registry.
func (r *defaultRunner) PullImage(ctx context.Context, c connector.Connector, imageName string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(imageName) == "" {
		return errors.New("imageName cannot be empty")
	}

	cmd := fmt.Sprintf("docker pull %s", shellEscape(imageName))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerPullTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to pull image %s. Stderr: %s", imageName, string(stderr))
	}
	return nil
}

// GetDockerDaemonConfig retrieves the current Docker daemon configuration.
func (r *defaultRunner) GetDockerDaemonConfig(ctx context.Context, conn connector.Connector) (*DockerDaemonOptions, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}

	configContent, err := r.ReadFile(ctx, conn, dockerDaemonConfigPath)
	if err != nil {
		exists, _ := r.Exists(ctx, conn, dockerDaemonConfigPath)
		if !exists {
			return &DockerDaemonOptions{}, nil
		}
		return nil, errors.Wrapf(err, "failed to read Docker daemon config file %s", dockerDaemonConfigPath)
	}

	if len(configContent) == 0 {
		return &DockerDaemonOptions{}, nil
	}

	var opts DockerDaemonOptions
	if err := json.Unmarshal(configContent, &opts); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal Docker daemon config from %s. Content: %s", dockerDaemonConfigPath, string(configContent))
	}
	return &opts, nil
}

// ConfigureDockerDaemon applies new daemon configurations.
func (r *defaultRunner) ConfigureDockerDaemon(ctx context.Context, conn connector.Connector, newOpts DockerDaemonOptions, restartService bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}

	currentOpts, err := r.GetDockerDaemonConfig(ctx, conn)
	if err != nil {
		return errors.Wrap(err, "failed to get current Docker daemon config before applying new settings")
	}
	if currentOpts == nil {
		currentOpts = &DockerDaemonOptions{}
	}

	// Merge Strategy: newOpts fields overwrite currentOpts fields if newOpts field is not nil.
	if newOpts.LogDriver != nil { currentOpts.LogDriver = newOpts.LogDriver }
	if newOpts.LogOpts != nil { currentOpts.LogOpts = newOpts.LogOpts }
	if newOpts.StorageDriver != nil { currentOpts.StorageDriver = newOpts.StorageDriver }
	if newOpts.StorageOpts != nil { currentOpts.StorageOpts = newOpts.StorageOpts }
	if newOpts.RegistryMirrors != nil { currentOpts.RegistryMirrors = newOpts.RegistryMirrors }
	if newOpts.InsecureRegistries != nil { currentOpts.InsecureRegistries = newOpts.InsecureRegistries }
	if newOpts.ExecOpts != nil { currentOpts.ExecOpts = newOpts.ExecOpts }
	if newOpts.Bridge != nil { currentOpts.Bridge = newOpts.Bridge }
	if newOpts.Bip != nil { currentOpts.Bip = newOpts.Bip }
	if newOpts.FixedCIDR != nil { currentOpts.FixedCIDR = newOpts.FixedCIDR }
	if newOpts.DefaultGateway != nil { currentOpts.DefaultGateway = newOpts.DefaultGateway }
	if newOpts.DNS != nil { currentOpts.DNS = newOpts.DNS }
	if newOpts.IPTables != nil { currentOpts.IPTables = newOpts.IPTables }
	if newOpts.Experimental != nil { currentOpts.Experimental = newOpts.Experimental }
	if newOpts.Debug != nil { currentOpts.Debug = newOpts.Debug }
	if newOpts.APICorsHeader != nil { currentOpts.APICorsHeader = newOpts.APICorsHeader }
	if newOpts.Hosts != nil { currentOpts.Hosts = newOpts.Hosts }
	if newOpts.UserlandProxy != nil { currentOpts.UserlandProxy = newOpts.UserlandProxy }
	if newOpts.LiveRestore != nil { currentOpts.LiveRestore = newOpts.LiveRestore }
	if newOpts.CgroupParent != nil { currentOpts.CgroupParent = newOpts.CgroupParent }
	if newOpts.DefaultRuntime != nil { currentOpts.DefaultRuntime = newOpts.DefaultRuntime }
	if newOpts.Runtimes != nil { currentOpts.Runtimes = newOpts.Runtimes }
	if newOpts.Graph != nil { currentOpts.Graph = newOpts.Graph }
	if newOpts.DataRoot != nil { currentOpts.DataRoot = newOpts.DataRoot }
	if newOpts.MaxConcurrentDownloads != nil { currentOpts.MaxConcurrentDownloads = newOpts.MaxConcurrentDownloads }
	if newOpts.MaxConcurrentUploads != nil { currentOpts.MaxConcurrentUploads = newOpts.MaxConcurrentUploads }
	if newOpts.ShutdownTimeout != nil { currentOpts.ShutdownTimeout = newOpts.ShutdownTimeout }

	mergedConfigBytes, err := json.MarshalIndent(currentOpts, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal merged Docker daemon config to JSON")
	}

	if err := r.Mkdirp(ctx, conn, filepath.Dir(dockerDaemonConfigPath), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create directory for %s", dockerDaemonConfigPath)
	}
	if err := r.WriteFile(ctx, conn, mergedConfigBytes, dockerDaemonConfigPath, "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write Docker daemon config to %s", dockerDaemonConfigPath)
	}

	if restartService {
		facts, errFacts := r.GatherFacts(ctx, conn)
		if errFacts != nil {
			return errors.Wrap(errFacts, "failed to gather facts for restarting Docker service")
		}
		if err := r.RestartService(ctx, conn, facts, "docker"); err != nil {
			return errors.Wrap(err, "failed to restart Docker service after configuration change")
		}
	}
	return nil
}

// EnsureDefaultDockerConfig ensures that a default Docker daemon configuration exists.
func (r *defaultRunner) EnsureDefaultDockerConfig(ctx context.Context, conn connector.Connector, facts *Facts, restartService bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}

	exists, err := r.Exists(ctx, conn, dockerDaemonConfigPath)
	if err != nil {
		return errors.Wrapf(err, "failed to check existence of %s", dockerDaemonConfigPath)
	}

	createDefaults := false
	if !exists {
		createDefaults = true
	} else {
		content, errRead := r.ReadFile(ctx, conn, dockerDaemonConfigPath)
		if errRead != nil {
			return errors.Wrapf(errRead, "failed to read existing %s to check if empty", dockerDaemonConfigPath)
		}
		trimmedContent := strings.TrimSpace(string(content))
		if len(trimmedContent) == 0 || trimmedContent == "{}" {
			createDefaults = true
		}
	}

	if createDefaults {
		logOptsMap := map[string]string{"max-size": "100m"}
		execOptsSlice := []string{"native.cgroupdriver=systemd"}
		storageDriverVal := "overlay2"
		// Example of fact-based adjustment (simplified)
		// if facts != nil && facts.OS != nil && facts.OS.Family == "rhel" && facts.OS.Major < 8 {
		// storageDriverVal = "devicemapper" // Or another appropriate driver
		// }

		defaultOpts := DockerDaemonOptions{
			ExecOpts:      &execOptsSlice,
			LogDriver:     strPtr("json-file"),
			LogOpts:       &logOptsMap,
			StorageDriver: &storageDriverVal,
		}

		if err := r.ConfigureDockerDaemon(ctx, conn, defaultOpts, false); err != nil { // Restart handled below if requested for this specific action
			return errors.Wrap(err, "failed to apply default Docker daemon configuration")
		}

		if restartService {
			var currentFacts *Facts = facts
			if currentFacts == nil {
				currentFacts, err = r.GatherFacts(ctx, conn)
				if err != nil {
					return errors.Wrap(err, "failed to gather facts for restarting Docker service after ensuring default config")
				}
			}
			if err := r.RestartService(ctx, conn, currentFacts, "docker"); err != nil {
				return errors.Wrap(err, "failed to restart Docker service after ensuring default config")
			}
		}
	}
	return nil
}

// ImageExists checks if a Docker image exists locally.
func (r *defaultRunner) ImageExists(ctx context.Context, c connector.Connector, imageName string) (bool, error) {
	if c == nil {
		return false, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(imageName) == "" {
		return false, errors.New("imageName cannot be empty")
	}

	cmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout,
	}

	_, _, err := c.Exec(ctx, cmd, execOptions)
	if err == nil {
		return true, nil
	}

	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 {
			return false, nil
		}
	}
	return false, errors.Wrapf(err, "failed to check if image %s exists", imageName)
}

// ListImages lists Docker images on the host.
func (r *defaultRunner) ListImages(ctx context.Context, c connector.Connector, all bool) ([]image.Summary, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmd := "docker images"
	if all {
		cmd += " --all"
	}
	cmd += " --format {{json .}}"

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list images. Stderr: %s", string(stderr))
	}

	var images []image.Summary
	scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var dockerImage struct {
			ID           string
			Repository   string
			Tag          string
			Digest       string
			CreatedSince string
			CreatedAt    string
			Size         string
		}

		if err := json.Unmarshal([]byte(line), &dockerImage); err != nil {
			return nil, errors.Wrapf(err, "failed to parse image JSON line: %s", line)
		}

		sizeBytes, parseErr := parseDockerSize(dockerImage.Size)
		if parseErr != nil {
			return nil, errors.Wrapf(parseErr, "failed to parse size '%s' for image %s", dockerImage.Size, dockerImage.ID)
		}

		summary := image.Summary{
			ID:          dockerImage.ID,
			RepoDigests: nil,
			Size:        sizeBytes,
			VirtualSize: sizeBytes,
		}
		if dockerImage.Repository != "<none>" && dockerImage.Tag != "<none>" {
			summary.RepoTags = []string{fmt.Sprintf("%s:%s", dockerImage.Repository, dockerImage.Tag)}
		} else if dockerImage.Repository != "<none>" {
			summary.RepoTags = []string{dockerImage.Repository}
		}

		if parsedTime, errTime := time.Parse("2006-01-02 15:04:05 -0700 MST", dockerImage.CreatedAt); errTime == nil {
			summary.Created = parsedTime.Unix()
		} else if parsedTime, errTime := time.Parse("2006-01-02 15:04:05 Z0700 MST", dockerImage.CreatedAt); errTime == nil {
			summary.Created = parsedTime.Unix()
		}

		images = append(images, summary)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker images output")
	}

	return images, nil
}

// RemoveImage removes a Docker image.
func (r *defaultRunner) RemoveImage(ctx context.Context, c connector.Connector, imageName string, force bool) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(imageName) == "" {
		return errors.New("imageName cannot be empty")
	}

	cmd := "docker rmi"
	if force {
		cmd += " -f"
	}
	cmd += " " + shellEscape(imageName)

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerRMTimeout,
	}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to remove image %s. Stderr: %s", imageName, string(stderr))
	}
	return nil
}

// BuildImage builds a Docker image from a Dockerfile.
func (r *defaultRunner) BuildImage(ctx context.Context, c connector.Connector, dockerfilePath, imageNameAndTag, contextPath string, buildArgs map[string]string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(imageNameAndTag) == "" {
		return errors.New("imageNameAndTag cannot be empty")
	}
	if strings.TrimSpace(contextPath) == "" {
		return errors.New("contextPath cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "build")

	if strings.TrimSpace(dockerfilePath) != "" {
		cmdArgs = append(cmdArgs, "-f", shellEscape(dockerfilePath))
	}

	cmdArgs = append(cmdArgs, "-t", shellEscape(imageNameAndTag))

	if buildArgs != nil {
		for key, value := range buildArgs {
			if strings.TrimSpace(key) == "" {
				return errors.New("buildArg key cannot be empty")
			}
			cmdArgs = append(cmdArgs, "--build-arg", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	cmdArgs = append(cmdArgs, shellEscape(contextPath))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerBuildTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to build image %s. Stdout: %s, Stderr: %s", imageNameAndTag, string(stdout), string(stderr))
	}
	return nil
}

// CreateContainer creates a new Docker container.
func (r *defaultRunner) CreateContainer(ctx context.Context, c connector.Connector, options ContainerCreateOptions) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(options.ImageName) == "" {
		return "", errors.New("options.ImageName cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "create")

	if strings.TrimSpace(options.ContainerName) != "" {
		cmdArgs = append(cmdArgs, "--name", shellEscape(options.ContainerName))
	}

	for _, portMapping := range options.Ports {
		var p string
		if strings.TrimSpace(portMapping.HostIP) != "" {
			p += portMapping.HostIP + ":"
		}
		if strings.TrimSpace(portMapping.HostPort) != "" {
			p += portMapping.HostPort + ":"
		}
		p += portMapping.ContainerPort
		if strings.TrimSpace(portMapping.Protocol) != "" {
			p += "/" + portMapping.Protocol
		}
		cmdArgs = append(cmdArgs, "-p", shellEscape(p))
	}

	for _, volumeMount := range options.Volumes {
		var v string
		if strings.TrimSpace(volumeMount.Source) == "" || strings.TrimSpace(volumeMount.Destination) == "" {
			return "", errors.New("volume source and destination cannot be empty")
		}
		v += volumeMount.Source + ":" + volumeMount.Destination
		if strings.TrimSpace(volumeMount.Mode) != "" {
			v += ":" + volumeMount.Mode
		}
		cmdArgs = append(cmdArgs, "-v", shellEscape(v))
	}

	for _, envVar := range options.EnvVars {
		if strings.TrimSpace(envVar) != "" {
			cmdArgs = append(cmdArgs, "-e", shellEscape(envVar))
		}
	}

	if len(options.Entrypoint) > 0 {
		cmdArgs = append(cmdArgs, "--entrypoint", shellEscape(options.Entrypoint[0]))
	}

	if strings.TrimSpace(options.RestartPolicy) != "" {
		cmdArgs = append(cmdArgs, "--restart", shellEscape(options.RestartPolicy))
	}
	if options.Privileged {
		cmdArgs = append(cmdArgs, "--privileged")
	}
	if options.AutoRemove {
		cmdArgs = append(cmdArgs, "--rm")
	}

	cmdArgs = append(cmdArgs, shellEscape(options.ImageName))

	if len(options.Command) > 0 {
		for _, cmdPart := range options.Command {
			cmdArgs = append(cmdArgs, shellEscape(cmdPart))
		}
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerCreateTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create container from image %s. Stderr: %s", options.ImageName, string(stderr))
	}

	containerID := strings.TrimSpace(string(stdout))
	if containerID == "" {
		return "", errors.New("docker create succeeded but returned an empty container ID")
	}
	return containerID, nil
}

// ContainerExists checks if a container (by name or ID) exists.
func (r *defaultRunner) ContainerExists(ctx context.Context, c connector.Connector, containerNameOrID string) (bool, error) {
	if c == nil {
		return false, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return false, errors.New("containerNameOrID cannot be empty")
	}

	cmd := fmt.Sprintf("docker inspect %s > /dev/null 2>&1", shellEscape(containerNameOrID))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout,
	}

	_, _, err := c.Exec(ctx, cmd, execOptions)
	if err == nil {
		return true, nil
	}

	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 {
			return false, nil
		}
	}
	return false, errors.Wrapf(err, "failed to check if container %s exists", containerNameOrID)
}

// StartContainer starts an existing Docker container.
func (r *defaultRunner) StartContainer(ctx context.Context, c connector.Connector, containerNameOrID string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmd := fmt.Sprintf("docker start %s", shellEscape(containerNameOrID))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerStartTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to start container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// StopContainer stops a running Docker container.
func (r *defaultRunner) StopContainer(ctx context.Context, c connector.Connector, containerNameOrID string, timeout *time.Duration) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "stop"}
	execTimeout := DefaultDockerStopExecTimeout

	if timeout != nil {
		gracePeriod := int((*timeout).Seconds())
		if gracePeriod < 0 {
			gracePeriod = 0
		}
		cmdArgs = append(cmdArgs, "-t", strconv.Itoa(gracePeriod))
		execTimeout = (*timeout) + (30 * time.Second)
	}

	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to stop container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// RestartContainer restarts a Docker container.
func (r *defaultRunner) RestartContainer(ctx context.Context, c connector.Connector, containerNameOrID string, timeout *time.Duration) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "restart"}
	execTimeout := DefaultDockerRestartExecTimeout

	if timeout != nil {
		gracePeriod := int((*timeout).Seconds())
		if gracePeriod < 0 {
			gracePeriod = 0
		}
		cmdArgs = append(cmdArgs, "-t", strconv.Itoa(gracePeriod))
		execTimeout = (*timeout) + (10 * time.Second)
	}

	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to restart container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// RemoveContainer removes a Docker container.
func (r *defaultRunner) RemoveContainer(ctx context.Context, c connector.Connector, containerNameOrID string, force, removeVolumes bool) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "rm"}
	if force {
		cmdArgs = append(cmdArgs, "-f")
	}
	if removeVolumes {
		cmdArgs = append(cmdArgs, "-v")
	}
	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerRMTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		if force && strings.Contains(string(stderr), "No such container") {
			return nil
		}
		return errors.Wrapf(err, "failed to remove container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// ListContainers lists Docker containers.
func (r *defaultRunner) ListContainers(ctx context.Context, c connector.Connector, all bool, filters map[string]string) ([]ContainerInfo, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "ps")
	if all {
		cmdArgs = append(cmdArgs, "--all")
	}
	if filters != nil {
		for key, value := range filters {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				return nil, errors.New("filter key and value cannot be empty")
			}
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}
	cmdArgs = append(cmdArgs, "--format", "{{json .}}")
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list containers. Stderr: %s", string(stderr))
	}

	var containers []ContainerInfo
	scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var cliContainer struct {
			ID         string
			Image      string
			Command    string
			CreatedAt  string
			RunningFor string
			Ports      string
			Status     string
			Size       string
			Names      string
			Labels     string
			Mounts     string
			Networks   string
		}

		if err := json.Unmarshal([]byte(line), &cliContainer); err != nil {
			return nil, errors.Wrapf(err, "failed to parse container JSON line: %s", line)
		}

		var createdTimestamp int64
		if parsedTime, errTime := time.Parse("2006-01-02 15:04:05 -0700 MST", cliContainer.CreatedAt); errTime == nil {
			createdTimestamp = parsedTime.Unix()
		} else if parsedTime, errTime := time.Parse("2006-01-02 15:04:05 Z0700 MST", cliContainer.CreatedAt); errTime == nil {
			createdTimestamp = parsedTime.Unix()
		}

		var namesList []string
		if strings.TrimSpace(cliContainer.Names) != "" {
			namesList = strings.Split(cliContainer.Names, ",")
		}

		labelsMap := make(map[string]string)
		if strings.TrimSpace(cliContainer.Labels) != "" {
			pairs := strings.Split(cliContainer.Labels, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					labelsMap[kv[0]] = kv[1]
				}
			}
		}

		var portMappings []ContainerPortMapping
		if strings.TrimSpace(cliContainer.Ports) != "" {
			rawPorts := strings.Split(cliContainer.Ports, ",")
			for _, rp := range rawPorts {
				if strings.TrimSpace(rp) == "" {
					continue
				}
				parts := strings.Split(rp, "->")
				var hostPart, containerPartVal string
				if len(parts) == 2 {
					hostPart = parts[0]
					containerPartVal = parts[1]
				} else {
					containerPartVal = parts[0]
				}

				var hostIP, hostPort, containerPortStr, protocol string
				if strings.Contains(hostPart, ":") {
					hostIPPort := strings.SplitN(hostPart, ":", 2)
					if len(hostIPPort) == 2 {
						hostIP = hostIPPort[0]
						hostPort = hostIPPort[1]
					} else {
						hostPort = hostIPPort[0]
					}
				}

				if strings.Contains(containerPartVal, "/") {
					containerProto := strings.SplitN(containerPartVal, "/", 2)
					containerPortStr = containerProto[0]
					if len(containerProto) == 2 {
						protocol = containerProto[1]
					}
				} else {
					containerPortStr = containerPartVal
				}
				portMappings = append(portMappings, ContainerPortMapping{
					HostIP:        hostIP,
					HostPort:      hostPort,
					ContainerPort: containerPortStr,
					Protocol:      protocol,
				})
			}
		}

		var mountsList []ContainerMount
		if strings.TrimSpace(cliContainer.Mounts) != "" {
			mountSources := strings.Split(cliContainer.Mounts, ",")
			for _, src := range mountSources {
				mountsList = append(mountsList, ContainerMount{Source: strings.TrimSpace(src)})
			}
		}

		var state string
		statusLower := strings.ToLower(cliContainer.Status)
		if strings.HasPrefix(statusLower, "up") {
			state = "running"
		} else if strings.HasPrefix(statusLower, "exited") {
			state = "exited"
		} else if strings.Contains(statusLower, "created") {
			state = "created"
		} else if strings.Contains(statusLower, "restarting") {
			state = "restarting"
		} else if strings.Contains(statusLower, "paused") {
			state = "paused"
		} else {
			state = "unknown"
		}

		containers = append(containers, ContainerInfo{
			ID:        cliContainer.ID,
			Names:     namesList,
			Image:     cliContainer.Image,
			Command:   cliContainer.Command,
			Created:   createdTimestamp,
			State:     state,
			Status:    cliContainer.Status,
			Ports:     portMappings,
			Labels:    labelsMap,
			Mounts:    mountsList,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker ps output")
	}

	return containers, nil
}

// GetContainerLogs retrieves logs from a container.
func (r *defaultRunner) GetContainerLogs(ctx context.Context, c connector.Connector, containerNameOrID string, options ContainerLogOptions) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return "", errors.New("containerNameOrID cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "logs")

	if options.Timestamps {
		cmdArgs = append(cmdArgs, "--timestamps")
	}
	if strings.TrimSpace(options.Since) != "" {
		cmdArgs = append(cmdArgs, "--since", shellEscape(options.Since))
	}
	if strings.TrimSpace(options.Until) != "" {
		cmdArgs = append(cmdArgs, "--until", shellEscape(options.Until))
	}
	if strings.TrimSpace(options.Tail) != "" {
		cmdArgs = append(cmdArgs, "--tail", shellEscape(options.Tail))
	}
	if options.Details {
		cmdArgs = append(cmdArgs, "--details")
	}

	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execTimeout := 2 * time.Minute
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get logs for container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return string(stdout), nil
}

// GetContainerStats retrieves live resource usage statistics for a container.
func (r *defaultRunner) GetContainerStats(ctx context.Context, c connector.Connector, containerNameOrID string, stream bool) (<-chan ContainerStats, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return nil, errors.New("containerNameOrID cannot be empty")
	}

	statsChan := make(chan ContainerStats)

	go func() {
		defer close(statsChan)

		cmdArgs := []string{"docker", "stats"}
		if !stream {
			cmdArgs = append(cmdArgs, "--no-stream")
		}
		cmdArgs = append(cmdArgs, "--format", "{{json .}}", shellEscape(containerNameOrID))
		cmd := strings.Join(cmdArgs, " ")

		execTimeout := 30 * time.Second
		if stream {
			execTimeout = 2 * time.Minute
		}

		execOptions := &connector.ExecOptions{
			Sudo:    true,
			Timeout: execTimeout,
		}

		stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
		if err != nil {
			if ctx.Err() != nil && (errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled)) {
				// Expected termination for streaming
			} else {
				statsChan <- ContainerStats{Error: errors.Wrapf(err, "stats for container %s failed. Stderr: %s", containerNameOrID, string(stderr))}
			}
			return
		}

		scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
		for scanner.Scan() {
			select {
			case <-ctx.Done(): // Check context cancellation at the start of each iteration
				return
			default:
			}

			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			var statData struct {
				Name      string
				ID        string
				CPUPerc   string
				MemUsage  string
				MemPerc   string
				NetIO     string
				BlockIO   string
				PIDs      string
			}
			if err := json.Unmarshal([]byte(line), &statData); err != nil {
				statsChan <- ContainerStats{Error: errors.Wrapf(err, "failed to parse streaming stats line: %s. Line: %s", err, line)}
				continue
			}

			var cs ContainerStats
			cpuPercStr := strings.TrimSuffix(statData.CPUPerc, "%")
			cs.CPUPercentage, _ = strconv.ParseFloat(cpuPercStr, 64)

			memParts := strings.Split(statData.MemUsage, " / ")
			if len(memParts) > 0 {
				cs.MemoryUsageBytes, _ = utils.ParseSizeToBytes(strings.TrimSpace(memParts[0]))
			}
			if len(memParts) > 1 {
				cs.MemoryLimitBytes, _ = utils.ParseSizeToBytes(strings.TrimSpace(memParts[1]))
			}

			netIOParts := strings.Split(statData.NetIO, " / ")
			if len(netIOParts) == 2 {
				cs.NetworkRxBytes, _ = utils.ParseSizeToBytes(strings.TrimSpace(netIOParts[0]))
				cs.NetworkTxBytes, _ = utils.ParseSizeToBytes(strings.TrimSpace(netIOParts[1]))
			}

			blockIOParts := strings.Split(statData.BlockIO, " / ")
			if len(blockIOParts) == 2 {
				cs.BlockReadBytes, _ = utils.ParseSizeToBytes(strings.TrimSpace(blockIOParts[0]))
				cs.BlockWriteBytes, _ = utils.ParseSizeToBytes(strings.TrimSpace(blockIOParts[1]))
			}
			pids, _ := strconv.ParseUint(statData.PIDs, 10, 64)
			cs.PidsCurrent = pids

			select {
			case statsChan <- cs:
			case <-ctx.Done():
				return
			}
			if !stream { // If not streaming, send one stat and finish
				return
			}
		}
		if err := scanner.Err(); err != nil {
			statsChan <- ContainerStats{Error: errors.Wrap(err, "error reading streaming stats output")}
		}
	}()
	return statsChan, nil
}

// InspectContainer returns low-level information on a Docker container.
func (r *defaultRunner) InspectContainer(ctx context.Context, c connector.Connector, containerNameOrID string) (*ContainerDetails, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return nil, errors.New("containerNameOrID cannot be empty")
	}

	cmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) && cmdErr.ExitCode == 1 {
			if strings.Contains(strings.ToLower(string(stderr)), "no such object") ||
				strings.Contains(strings.ToLower(string(stderr)), "is not a docker command") {
				return nil, nil
			}
		}
		return nil, errors.Wrapf(err, "failed to inspect container %s. Stderr: %s", containerNameOrID, string(stderr))
	}

	outputStr := strings.TrimSpace(string(stdout))
	if outputStr == "" {
		return nil, errors.New("docker inspect returned empty output")
	}

	var details []ContainerDetails
	if err := json.Unmarshal([]byte(outputStr), &details); err != nil {
		var singleDetail ContainerDetails
		if errSingle := json.Unmarshal([]byte(outputStr), &singleDetail); errSingle != nil {
			return nil, errors.Wrapf(err, "failed to parse container inspect JSON (tried array and object): %s. Output: %s", errSingle, outputStr)
		}
		return &singleDetail, nil
	}

	if len(details) == 0 {
		return nil, nil
	}

	return &details[0], nil
}

// PauseContainer pauses all processes within a running container.
func (r *defaultRunner) PauseContainer(ctx context.Context, c connector.Connector, containerNameOrID string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmd := fmt.Sprintf("docker pause %s", shellEscape(containerNameOrID))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerStartTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to pause container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// UnpauseContainer unpauses all processes within a paused container.
func (r *defaultRunner) UnpauseContainer(ctx context.Context, c connector.Connector, containerNameOrID string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmd := fmt.Sprintf("docker unpause %s", shellEscape(containerNameOrID))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerStartTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to unpause container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// ExecInContainer executes a command inside a running container.
func (r *defaultRunner) ExecInContainer(ctx context.Context, c connector.Connector, containerNameOrID string, cmdArgsToExec []string, user, workDir string, tty bool) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return "", errors.New("containerNameOrID cannot be empty")
	}
	if len(cmdArgsToExec) == 0 {
		return "", errors.New("command to execute cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "exec")

	if tty {
		cmdArgs = append(cmdArgs, "-t")
	}
	if strings.TrimSpace(user) != "" {
		cmdArgs = append(cmdArgs, "--user", shellEscape(user))
	}
	if strings.TrimSpace(workDir) != "" {
		cmdArgs = append(cmdArgs, "--workdir", shellEscape(workDir))
	}

	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	for _, arg := range cmdArgsToExec {
		cmdArgs = append(cmdArgs, shellEscape(arg))
	}
	cmd := strings.Join(cmdArgs, " ")

	execTimeout := 5 * time.Minute
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		output := string(stdout) + string(stderr)
		return output, errors.Wrapf(err, "failed to exec in container %s (cmd: %s). Combined output: %s", containerNameOrID, strings.Join(cmdArgsToExec, " "), output)
	}
	return string(stdout) + string(stderr), nil
}

// --- Docker Network Methods ---

// CreateDockerNetwork creates a new Docker network.
func (r *defaultRunner) CreateDockerNetwork(ctx context.Context, c connector.Connector, name, driver, subnet, gateway string, options map[string]string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("network name cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "network", "create")

	if strings.TrimSpace(driver) != "" {
		cmdArgs = append(cmdArgs, "--driver", shellEscape(driver))
	}
	if strings.TrimSpace(subnet) != "" {
		cmdArgs = append(cmdArgs, "--subnet", shellEscape(subnet))
	}
	if strings.TrimSpace(gateway) != "" {
		cmdArgs = append(cmdArgs, "--gateway", shellEscape(gateway))
	}
	if options != nil {
		for k, v := range options {
			cmdArgs = append(cmdArgs, "--opt", shellEscape(fmt.Sprintf("%s=%s", k, v)))
		}
	}

	cmdArgs = append(cmdArgs, shellEscape(name))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to create docker network %s. Stderr: %s", name, string(stderr))
	}
	return nil
}

// RemoveDockerNetwork removes a Docker network.
func (r *defaultRunner) RemoveDockerNetwork(ctx context.Context, c connector.Connector, networkNameOrID string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(networkNameOrID) == "" {
		return errors.New("networkNameOrID cannot be empty")
	}

	cmd := fmt.Sprintf("docker network rm %s", shellEscape(networkNameOrID))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		if strings.Contains(string(stderr), "No such network") || strings.Contains(string(stderr), "not found") {
			return nil
		}
		return errors.Wrapf(err, "failed to remove docker network %s. Stderr: %s", networkNameOrID, string(stderr))
	}
	return nil
}

// ListDockerNetworks lists Docker networks.
func (r *defaultRunner) ListDockerNetworks(ctx context.Context, c connector.Connector, filters map[string]string) ([]DockerNetworkInfo, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "network", "ls")
	if filters != nil {
		for key, value := range filters {
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}
	cmdArgs = append(cmdArgs, "--format", "{{json .}}")
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list docker networks. Stderr: %s", string(stderr))
	}

	var networks []DockerNetworkInfo
	scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var netInfo struct {
			ID         string
			Name       string
			Driver     string
			Scope      string
			IPv6       string
			Internal   string
			Attachable string
			Ingress    string
		}
		if err := json.Unmarshal([]byte(line), &netInfo); err != nil {
			return nil, errors.Wrapf(err, "failed to parse network JSON line: %s", line)
		}

		enableIPv6, _ := strconv.ParseBool(netInfo.IPv6)
		networks = append(networks, DockerNetworkInfo{
			ID:         netInfo.ID,
			Name:       netInfo.Name,
			Driver:     netInfo.Driver,
			Scope:      netInfo.Scope,
			EnableIPv6: enableIPv6,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker networks ls output")
	}
	return networks, nil
}

// ConnectContainerToNetwork connects a container to a Docker network.
func (r *defaultRunner) ConnectContainerToNetwork(ctx context.Context, c connector.Connector, containerNameOrID string, networkNameOrID string, ipAddress string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}
	if strings.TrimSpace(networkNameOrID) == "" {
		return errors.New("networkNameOrID cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "network", "connect")
	if strings.TrimSpace(ipAddress) != "" {
		cmdArgs = append(cmdArgs, "--ip", shellEscape(ipAddress))
	}
	cmdArgs = append(cmdArgs, shellEscape(networkNameOrID), shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		if strings.Contains(string(stderr), "is already connected to network") {
			return nil
		}
		return errors.Wrapf(err, "failed to connect container %s to network %s. Stderr: %s", containerNameOrID, networkNameOrID, string(stderr))
	}
	return nil
}

// DisconnectContainerFromNetwork disconnects a container from a Docker network.
func (r *defaultRunner) DisconnectContainerFromNetwork(ctx context.Context, c connector.Connector, containerNameOrID string, networkNameOrID string, force bool) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}
	if strings.TrimSpace(networkNameOrID) == "" {
		return errors.New("networkNameOrID cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "network", "disconnect")
	if force {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmdArgs = append(cmdArgs, shellEscape(networkNameOrID), shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		if strings.Contains(string(stderr), "is not connected to network") {
			return nil
		}
		return errors.Wrapf(err, "failed to disconnect container %s from network %s. Stderr: %s", containerNameOrID, networkNameOrID, string(stderr))
	}
	return nil
}

// --- Docker Volume Methods ---

// CreateDockerVolume creates a new Docker volume.
func (r *defaultRunner) CreateDockerVolume(ctx context.Context, c connector.Connector, name string, driver string, driverOpts map[string]string, labels map[string]string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "volume", "create")
	if strings.TrimSpace(driver) != "" {
		cmdArgs = append(cmdArgs, "--driver", shellEscape(driver))
	}
	if driverOpts != nil {
		for k, v := range driverOpts {
			cmdArgs = append(cmdArgs, "--opt", shellEscape(fmt.Sprintf("%s=%s", k, v)))
		}
	}
	if labels != nil {
		for k, v := range labels {
			cmdArgs = append(cmdArgs, "--label", shellEscape(fmt.Sprintf("%s=%s", k, v)))
		}
	}
	if strings.TrimSpace(name) != "" {
		cmdArgs = append(cmdArgs, shellEscape(name))
	}
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		if strings.TrimSpace(name) != "" && strings.Contains(string(stderr), "already exists") && strings.Contains(string(stderr), name) {
			return nil
		}
		return errors.Wrapf(err, "failed to create docker volume %s. Stderr: %s, Stdout: %s", name, string(stderr), string(stdout))
	}
	return nil
}

// RemoveDockerVolume removes a Docker volume.
func (r *defaultRunner) RemoveDockerVolume(ctx context.Context, c connector.Connector, volumeName string, force bool) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(volumeName) == "" {
		return errors.New("volumeName cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "volume", "rm")
	if force {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmdArgs = append(cmdArgs, shellEscape(volumeName))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		if strings.Contains(string(stderr), "No such volume") && strings.Contains(string(stderr), volumeName) {
			return nil
		}
		return errors.Wrapf(err, "failed to remove docker volume %s. Stderr: %s", volumeName, string(stderr))
	}
	return nil
}

// ListDockerVolumes lists Docker volumes.
func (r *defaultRunner) ListDockerVolumes(ctx context.Context, c connector.Connector, filters map[string]string) ([]DockerVolumeInfo, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "volume", "ls")
	if filters != nil {
		for key, value := range filters {
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}
	cmdArgs = append(cmdArgs, "--format", "{{json .}}")
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list docker volumes. Stderr: %s", string(stderr))
	}

	var volumes []DockerVolumeInfo
	scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var volData struct {
			Driver     string
			Labels     string
			Links      string
			Mountpoint string
			Name       string
			Scope      string
			Size       string
		}
		if err := json.Unmarshal([]byte(line), &volData); err != nil {
			return nil, errors.Wrapf(err, "failed to parse volume JSON line: %s", line)
		}

		labelsMap := make(map[string]string)
		if strings.TrimSpace(volData.Labels) != "" {
			pairs := strings.Split(volData.Labels, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					labelsMap[kv[0]] = kv[1]
				} else {
					labelsMap[kv[0]] = ""
				}
			}
		}

		volumes = append(volumes, DockerVolumeInfo{
			Name:       volData.Name,
			Driver:     volData.Driver,
			Mountpoint: volData.Mountpoint,
			Labels:     labelsMap,
			Scope:      volData.Scope,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker volumes ls output")
	}
	return volumes, nil
}

// InspectDockerVolume returns information about a Docker volume.
func (r *defaultRunner) InspectDockerVolume(ctx context.Context, c connector.Connector, volumeName string) (*DockerVolumeDetails, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(volumeName) == "" {
		return nil, errors.New("volumeName cannot be empty")
	}

	cmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) && cmdErr.ExitCode == 1 {
			if strings.Contains(strings.ToLower(string(stderr)), "no such volume") {
				return nil, nil
			}
		}
		return nil, errors.Wrapf(err, "failed to inspect volume %s. Stderr: %s", volumeName, string(stderr))
	}

	outputStr := strings.TrimSpace(string(stdout))
	if outputStr == "" {
		return nil, errors.New("docker volume inspect returned empty output")
	}

	var detailsList []DockerVolumeDetails
	if err := json.Unmarshal([]byte(outputStr), &detailsList); err == nil && len(detailsList) > 0 {
		return &detailsList[0], nil
	}

	var detail DockerVolumeDetails
	if err := json.Unmarshal([]byte(outputStr), &detail); err != nil {
		return nil, errors.Wrapf(err, "failed to parse volume inspect JSON: %s. Output: %s", err, outputStr)
	}
	return &detail, nil
}

// --- Docker System Methods ---

// DockerInfo displays system-wide information about Docker.
func (r *defaultRunner) DockerInfo(ctx context.Context, c connector.Connector) (*DockerSystemInfo, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmd := "docker info --format \"{{json .}}\""
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get docker info. Stderr: %s", string(stderr))
	}

	var info DockerSystemInfo
	if err := json.Unmarshal(stdout, &info); err != nil {
		return nil, errors.Wrapf(err, "failed to parse docker info JSON: %s. Output: %s", err, string(stdout))
	}
	return &info, nil
}

// DockerPrune removes unused Docker data.
func (r *defaultRunner) DockerPrune(ctx context.Context, c connector.Connector, pruneType string, filters map[string]string, all bool) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(pruneType) == "" {
		pruneType = "system"
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker")

	validPruneTypes := map[string]bool{
		"system":    true, "builder": true, "container": true,
		"image":     true, "network": true, "volume": true,
	}
	if !validPruneTypes[pruneType] {
		return "", errors.Errorf("invalid pruneType: %s", pruneType)
	}

	if pruneType == "system" {
		cmdArgs = append(cmdArgs, "system", "prune", "-f")
	} else {
		cmdArgs = append(cmdArgs, pruneType, "prune", "-f")
	}

	if all && (pruneType == "system" || pruneType == "image" || pruneType == "builder") {
		cmdArgs = append(cmdArgs, "--all")
	}
	if filters != nil {
		for key, value := range filters {
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Minute}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to prune docker %s. Stderr: %s, Stdout: %s", pruneType, string(stderr), string(stdout))
	}
	return string(stdout), nil
}

// GetDockerServerVersion returns the version of the Docker server.
func (r *defaultRunner) GetDockerServerVersion(ctx context.Context, c connector.Connector) (*semver.Version, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	cmd := "docker version --format '{{.Server.Version}}'"
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get Docker server version. Stderr: %s", string(stderr))
	}
	versionStr := strings.TrimSpace(string(stdout))
	if versionStr == "" {
		return nil, errors.New("failed to parse Docker server version: output is empty")
	}
	v, err := semver.NewVersion(versionStr)
	if err != nil {
		re := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
		matches := re.FindStringSubmatch(versionStr)
		if len(matches) > 1 {
			cleanedVersionStr := matches[1]
			v, err = semver.NewVersion(cleanedVersionStr)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse cleaned Docker server version '%s' (original: '%s')", cleanedVersionStr, versionStr)
			}
		} else {
			return nil, errors.Wrapf(err, "failed to parse Docker server version '%s'", versionStr)
		}
	}
	return v, nil
}

// CheckDockerInstalled checks if Docker is installed and accessible.
func (r *defaultRunner) CheckDockerInstalled(ctx context.Context, c connector.Connector) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	cmd := "docker version"
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "docker not installed or not accessible. Stderr: %s", string(stderr))
	}
	return nil
}

// EnsureDockerService ensures the Docker service is running and enabled.
func (r *defaultRunner) EnsureDockerService(ctx context.Context, c connector.Connector) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	facts, err := r.GatherFacts(ctx, c) // Gather facts to determine init system
	if err != nil {
		return errors.Wrap(err, "failed to gather facts to ensure docker service")
	}
	if facts.InitSystem == nil || facts.InitSystem.Type == InitSystemUnknown {
		return errors.New("unknown init system, cannot ensure docker service state")
	}


	isActiveCmd := fmt.Sprintf("%s %s", facts.InitSystem.IsActiveCmd, "docker")
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second}
	stdoutActive, _, errActive := c.Exec(ctx, isActiveCmd, execOptions)

	if errActive == nil && strings.TrimSpace(string(stdoutActive)) == "active" { // systemctl is-active returns "active"
		// Check if enabled
		isEnabledCmd := fmt.Sprintf("%s %s", facts.InitSystem.EnableCmd, "docker") // This is not is-enabled, but enable itself
		// For systemd, `systemctl is-enabled docker` is better.
		// This part needs to be adapted based on actual ServiceInfo content.
		// Assuming EnableCmd is idempotent or we have IsEnabledCmd.
		// For now, let's assume if it's active, we try to enable it to be sure.
		if facts.InitSystem.Type == InitSystemSystemd {
			isEnabledCmd = fmt.Sprintf("systemctl is-enabled docker")
			stdoutEnabled, _, errEnabled := c.Exec(ctx, isEnabledCmd, execOptions)
			if errEnabled == nil && (strings.TrimSpace(string(stdoutEnabled)) == "enabled" || strings.TrimSpace(string(stdoutEnabled)) == "static") {
				return nil // Active and enabled.
			}
		}
		// If not systemd or not enabled, try to enable
		enableCmd := fmt.Sprintf("%s %s", facts.InitSystem.EnableCmd, "docker")
		_, stderrEnable, errEnableCmd := c.Exec(ctx, enableCmd, execOptions)
		if errEnableCmd != nil {
			return errors.Wrapf(errEnableCmd, "failed to enable docker service. Stderr: %s", string(stderrEnable))
		}
		return nil
	}

	// Not active or check failed, try to start
	startCmd := fmt.Sprintf("%s %s", facts.InitSystem.StartCmd, "docker")
	_, stderrStart, errStart := c.Exec(ctx, startCmd, execOptions)
	if errStart != nil {
		if installErr := r.CheckDockerInstalled(ctx, c); installErr != nil {
			return errors.Wrap(installErr, "docker service failed to start and docker is not installed or accessible")
		}
		return errors.Wrapf(errStart, "failed to start docker service. Stderr: %s", string(stderrStart))
	}

	// Started, now enable
	enableCmd := fmt.Sprintf("%s %s", facts.InitSystem.EnableCmd, "docker")
	_, stderrEnable, errEnable := c.Exec(ctx, enableCmd, execOptions)
	if errEnable != nil {
		return errors.Wrapf(errEnable, "docker service started but failed to enable it. Stderr: %s", string(stderrEnable))
	}
	return nil
}


// ResolveDockerImage resolves a Docker image name to its digest.
func (r *defaultRunner) ResolveDockerImage(ctx context.Context, c connector.Connector, imageName string) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(imageName) == "" {
		return "", errors.New("imageName cannot be empty")
	}

	cmd := fmt.Sprintf("docker image inspect --format '{{range .RepoDigests}}{{.}}{{println}}{{end}}' %s", shellEscape(imageName))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to inspect image %s to resolve digest. Stderr: %s", imageName, string(stderr))
	}

	output := strings.TrimSpace(string(stdout))
	if output == "" {
		cmdID := fmt.Sprintf("docker image inspect --format '{{.ID}}' %s", shellEscape(imageName))
		stdoutID, stderrID, errID := c.Exec(ctx, cmdID, execOptions)
		if errID != nil {
			return "", errors.Wrapf(errID, "failed to get ImageID for %s. Stderr: %s", imageName, string(stderrID))
		}
		imageID := strings.TrimSpace(string(stdoutID))
		if imageID == "" {
			return "", errors.Errorf("could not resolve image %s to a RepoDigest or ImageID", imageName)
		}
		return imageID, nil
	}

	digests := strings.Split(output, "\n")
	if len(digests) > 0 && strings.TrimSpace(digests[0]) != "" {
		return strings.TrimSpace(digests[0]), nil
	}

	return "", errors.Errorf("could not resolve image %s to a RepoDigest (output was: '%s')", imageName, output)
}

// DockerSave saves one or more images to a tar archive.
func (r *defaultRunner) DockerSave(ctx context.Context, c connector.Connector, outputFilePath string, imageNames []string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(outputFilePath) == "" {
		return errors.New("outputFilePath cannot be empty")
	}
	if len(imageNames) == 0 {
		return errors.New("imageNames cannot be empty")
	}

	escapedImageNames := make([]string, len(imageNames))
	for i, name := range imageNames {
		if strings.TrimSpace(name) == "" {
			return errors.Errorf("image name at index %d cannot be empty", i)
		}
		escapedImageNames[i] = shellEscape(name)
	}

	cmd := fmt.Sprintf("docker save -o %s %s", shellEscape(outputFilePath), strings.Join(escapedImageNames, " "))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerBuildTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to save images to %s. Stderr: %s", outputFilePath, string(stderr))
	}
	return nil
}

// DockerLoad loads an image or repository from a tar archive.
func (r *defaultRunner) DockerLoad(ctx context.Context, c connector.Connector, inputFilePath string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(inputFilePath) == "" {
		return errors.New("inputFilePath cannot be empty")
	}

	cmd := fmt.Sprintf("docker load -i %s", shellEscape(inputFilePath))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerBuildTimeout,
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to load image(s) from %s. Stderr: %s", inputFilePath, string(stderr))
	}
	return nil
}

// strPtr returns a pointer to the string value s.
func strPtr(s string) *string {
	return &s
}

// boolPtr returns a pointer to the bool value b.
func boolPtr(b bool) *bool {
	return &b
}

// PruneDockerBuildCache prunes the Docker build cache.
func (r *defaultRunner) PruneDockerBuildCache(ctx context.Context, c connector.Connector) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	cmd := "docker builder prune -a -f"
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to prune Docker build cache. Stderr: %s", string(stderr))
	}
	return nil
}

// GetHostArchitecture retrieves the host architecture using Docker.
func (r *defaultRunner) GetHostArchitecture(ctx context.Context, c connector.Connector) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	cmd := "docker version --format '{{.Server.Architecture}}'"
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get host architecture via Docker. Stderr: %s", string(stderr))
	}
	arch := strings.TrimSpace(string(stdout))
	if arch == "" {
		return "", errors.New("failed to parse host architecture: output is empty")
	}
	return arch, nil
}

// CheckDockerRequirement checks if the Docker version meets a minimum requirement.
func (r *defaultRunner) CheckDockerRequirement(ctx context.Context, c connector.Connector, versionConstraint string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(versionConstraint) == "" {
		return errors.New("versionConstraint cannot be empty")
	}

	serverVersion, err := r.GetDockerServerVersion(ctx, c)
	if err != nil {
		return errors.Wrap(err, "could not get Docker server version to check requirement")
	}

	constraint, err := semver.NewConstraint(versionConstraint)
	if err != nil {
		return errors.Wrapf(err, "invalid version constraint format: %s", versionConstraint)
	}

	if !constraint.Check(serverVersion) {
		return fmt.Errorf("docker version %s does not meet requirement %s", serverVersion.String(), versionConstraint)
	}
	return nil
}
