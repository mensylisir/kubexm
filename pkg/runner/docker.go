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
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/pkg/errors"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/utils"
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
// For simplicity, this example wraps with single quotes and escapes internal single quotes.
// A more robust solution might involve more sophisticated escaping logic or using
// command execution libraries that handle arguments safely.
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

const dockerDaemonConfigPath = "/etc/docker/daemon.json"

// GetDockerDaemonConfig retrieves the current Docker daemon configuration.
// It reads /etc/docker/daemon.json and unmarshals it.
// Returns an empty DockerDaemonOptions if the file doesn't exist or is empty.
func (r *defaultRunner) GetDockerDaemonConfig(ctx context.Context, conn connector.Connector) (*DockerDaemonOptions, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}

	configContent, err := r.ReadFile(ctx, conn, dockerDaemonConfigPath)
	if err != nil {
		// If file doesn't exist, it's not an error; return empty config.
		// This assumes ReadFile returns a specific error for "not found" that can be checked.
		// For simplicity, if any error occurs (incl. not found), we'll try to proceed as if empty,
		// or return a wrapped error if it's not a simple "not found".
		// A more robust way: check with r.Exists first.
		exists, _ := r.Exists(ctx, conn, dockerDaemonConfigPath)
		if !exists {
			return &DockerDaemonOptions{}, nil // File not found, return empty/default struct
		}
		return nil, errors.Wrapf(err, "failed to read Docker daemon config file %s", dockerDaemonConfigPath)
	}

	if len(configContent) == 0 {
		return &DockerDaemonOptions{}, nil // File is empty
	}

	var opts DockerDaemonOptions
	if err := json.Unmarshal(configContent, &opts); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal Docker daemon config from %s. Content: %s", dockerDaemonConfigPath, string(configContent))
	}
	return &opts, nil
}

// ConfigureDockerDaemon applies new daemon configurations.
// It reads the existing daemon.json, merges new options, writes back, and optionally restarts Docker.
func (r *defaultRunner) ConfigureDockerDaemon(ctx context.Context, conn connector.Connector, newOpts DockerDaemonOptions, restartService bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}

	// 1. Get current config
	currentOpts, err := r.GetDockerDaemonConfig(ctx, conn)
	if err != nil {
		// If GetDockerDaemonConfig failed for a reason other than "not found" or "empty", propagate error.
		// If it returned an empty struct due to not found/empty, currentOpts will be non-nil.
		return errors.Wrap(err, "failed to get current Docker daemon config before applying new settings")
	}
	if currentOpts == nil { // Should not happen if GetDockerDaemonConfig is correct
		currentOpts = &DockerDaemonOptions{}
	}

	// 2. Merge newOpts into currentOpts
	// This is a shallow merge. For deep merge of maps/slices, more logic would be needed.
	// Pointers in newOpts allow distinguishing between "not set" and "set to empty/zero value".

	// Example of merging a few fields. This should be done for all fields in DockerDaemonOptions.
	// This can be done more generically using reflection, but direct assignment is clearer for fewer fields.
	// For a production system, a library for merging structs or a more detailed merge logic is advised.

	// Helper for merging string slices like RegistryMirrors, InsecureRegistries, ExecOpts, DNS, Hosts
	mergeStringSlice := func(current, new *[]string) *[]string {
		if new == nil { // New value not provided
			return current
		}
		// If new value is provided (even if empty slice), it overwrites current.
		return new
	}
	// Helper for merging map[string]string like LogOpts
	mergeMapStringString := func(current, new *map[string]string) *map[string]string {
		if new == nil {
			return current
		}
		return new
	}
	// Helper for merging map[string]DockerRuntime like Runtimes
	mergeMapStringRuntime := func(current, new *map[string]DockerRuntime) *map[string]DockerRuntime {
		if new == nil {
			return current
		}
		return new
	}


	if newOpts.LogDriver != nil { currentOpts.LogDriver = newOpts.LogDriver }
	if newOpts.LogOpts != nil { currentOpts.LogOpts = mergeMapStringString(currentOpts.LogOpts, newOpts.LogOpts) }
	if newOpts.StorageDriver != nil { currentOpts.StorageDriver = newOpts.StorageDriver }
	if newOpts.StorageOpts != nil { currentOpts.StorageOpts = mergeStringSlice(currentOpts.StorageOpts, newOpts.StorageOpts) }
	if newOpts.RegistryMirrors != nil { currentOpts.RegistryMirrors = mergeStringSlice(currentOpts.RegistryMirrors, newOpts.RegistryMirrors) }
	if newOpts.InsecureRegistries != nil { currentOpts.InsecureRegistries = mergeStringSlice(currentOpts.InsecureRegistries, newOpts.InsecureRegistries) }
	if newOpts.ExecOpts != nil { currentOpts.ExecOpts = mergeStringSlice(currentOpts.ExecOpts, newOpts.ExecOpts) }
	if newOpts.Bridge != nil { currentOpts.Bridge = newOpts.Bridge }
	if newOpts.Bip != nil { currentOpts.Bip = newOpts.Bip }
	if newOpts.FixedCIDR != nil { currentOpts.FixedCIDR = newOpts.FixedCIDR }
	if newOpts.DefaultGateway != nil { currentOpts.DefaultGateway = newOpts.DefaultGateway }
	if newOpts.DNS != nil { currentOpts.DNS = mergeStringSlice(currentOpts.DNS, newOpts.DNS) }
	if newOpts.IPTables != nil { currentOpts.IPTables = newOpts.IPTables }
	if newOpts.Experimental != nil { currentOpts.Experimental = newOpts.Experimental }
	if newOpts.Debug != nil { currentOpts.Debug = newOpts.Debug }
	if newOpts.APICorsHeader != nil { currentOpts.APICorsHeader = newOpts.APICorsHeader }
	if newOpts.Hosts != nil { currentOpts.Hosts = mergeStringSlice(currentOpts.Hosts, newOpts.Hosts) }
	if newOpts.UserlandProxy != nil { currentOpts.UserlandProxy = newOpts.UserlandProxy }
	if newOpts.LiveRestore != nil { currentOpts.LiveRestore = newOpts.LiveRestore }
	if newOpts.CgroupParent != nil { currentOpts.CgroupParent = newOpts.CgroupParent }
	if newOpts.DefaultRuntime != nil { currentOpts.DefaultRuntime = newOpts.DefaultRuntime }
	if newOpts.Runtimes != nil { currentOpts.Runtimes = mergeMapStringRuntime(currentOpts.Runtimes, newOpts.Runtimes) }
	if newOpts.Graph != nil { currentOpts.Graph = newOpts.Graph } // Deprecated
	if newOpts.DataRoot != nil { currentOpts.DataRoot = newOpts.DataRoot }
	if newOpts.MaxConcurrentDownloads != nil { currentOpts.MaxConcurrentDownloads = newOpts.MaxConcurrentDownloads }
	if newOpts.MaxConcurrentUploads != nil { currentOpts.MaxConcurrentUploads = newOpts.MaxConcurrentUploads }
	if newOpts.ShutdownTimeout != nil { currentOpts.ShutdownTimeout = newOpts.ShutdownTimeout }


	// 3. Marshal merged config back to JSON
	mergedConfigBytes, err := json.MarshalIndent(currentOpts, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal merged Docker daemon config to JSON")
	}

	// 4. Write to /etc/docker/daemon.json
	// Ensure directory /etc/docker exists
	if err := r.Mkdirp(ctx, conn, filepath.Dir(dockerDaemonConfigPath), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create directory for %s", dockerDaemonConfigPath)
	}
	if err := r.WriteFile(ctx, conn, mergedConfigBytes, dockerDaemonConfigPath, "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write Docker daemon config to %s", dockerDaemonConfigPath)
	}

	// 5. Optionally restart Docker service
	if restartService {
		// Need to get facts to know how to restart service
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
// ImageExists checks if a Docker image exists locally.
func (r *defaultRunner) ImageExists(ctx context.Context, c connector.Connector, imageName string) (bool, error) {
	if c == nil {
		return false, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(imageName) == "" {
		return false, errors.New("imageName cannot be empty")
	}

	// docker image inspect exits with 0 if image exists, 1 if not.
	// Redirecting output to /dev/null as we only care about the exit code.
	cmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout,
	}

	_, _, err := c.Exec(ctx, cmd, execOptions)
	if err == nil {
		return true, nil // Exit code 0 means image exists
	}

	// If Exec returns an error, we need to check if it's a *connector.CommandError
	// and if the exit code indicates "not found" (usually 1 for inspect).
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 { // Common exit code for "not found"
			return false, nil
		}
	}
	// For other errors, or unexpected exit codes, return the error.
	return false, errors.Wrapf(err, "failed to check if image %s exists", imageName)
}

// ListImages lists Docker images on the host.
// Corresponds to `docker images`.
// Uses a custom format `{{json .}}` to get structured data for each image.
func (r *defaultRunner) ListImages(ctx context.Context, c connector.Connector, all bool) ([]image.Summary, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmd := "docker images"
	if all {
		cmd += " --all"
	}
	cmd += " --format {{json .}}" // Output each image info as a JSON line

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

		var dockerImage struct { // Intermediate struct to parse docker CLI JSON output
			ID           string
			Repository   string
			Tag          string
			Digest       string // Not always present in basic `docker images`
			CreatedSince string // e.g., "2 days ago"
			CreatedAt    string // e.g., "2023-03-20 10:00:00 +0000 UTC"
			Size         string // e.g., "125MB"
		}

		if err := json.Unmarshal([]byte(line), &dockerImage); err != nil {
			return nil, errors.Wrapf(err, "failed to parse image JSON line: %s", line)
		}

		sizeBytes, err := parseDockerSize(dockerImage.Size)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse size '%s' for image %s", dockerImage.Size, dockerImage.ID)
		}

		// Populate the image.Summary fields
		// Note: Docker CLI's JSON output for `docker images` is simpler than the API's image.Summary.
		// We adapt as best as possible.
		summary := image.Summary{
			ID: dockerImage.ID,
			// RepoTags: If Repository and Tag are not <none>, combine them.
			// Digest may not be available from `docker images` output directly, might need `docker image inspect`
			// or rely on API for richer info. For CLI parsing, this is an approximation.
			RepoDigests: nil, // Typically filled by API or more detailed inspect.
			Size:        sizeBytes,
			VirtualSize: sizeBytes, // For `docker images` output, Size and VirtualSize are often the same.
			// Created: Parse CreatedSince or CreatedAt if available/needed. For simplicity, we'll use CreatedSince as string.
			// This field is int64 (timestamp) in image.Summary. Docker CLI's `CreatedSince` is human-readable.
			// `CreatedAt` is a parsable timestamp.
			// For now, we are skipping proper timestamp conversion for 'Created' to keep CLI parsing simpler.
			// If a precise 'Created' timestamp is needed, `docker image inspect` for each image would be more reliable.
		}
		if dockerImage.Repository != "<none>" && dockerImage.Tag != "<none>" {
			summary.RepoTags = []string{fmt.Sprintf("%s:%s", dockerImage.Repository, dockerImage.Tag)}
		} else if dockerImage.Repository != "<none>" { // Image might have repo but no tag (e.g. intermediate layer)
			summary.RepoTags = []string{dockerImage.Repository}
		}


		// Attempt to parse CreatedAt if available for a more accurate 'Created' field
		// This is a common format, but Docker's output can vary.
		// Example: "2023-10-26 09:07:26 -0700 PDT"
		// The `go-units` library used by Docker for formatting `CreatedSince` is not trivial to parse back to a timestamp.
		// `CreatedAt` provides a direct timestamp string.
		if dockerImage.CreatedAt != "" {
			// Attempt to parse common date formats. This might need to be more robust.
			// Example format: "2023-10-26 09:07:26 -0700 PDT" - Go's time.Parse needs "2006-01-02 15:04:05 -0700 MST"
			// Docker's format can be tricky. For now, let's try a common one.
			// A more robust way would be to inspect each image individually if precise created time is critical.
			// For the purpose of `ListImages` via CLI, this is an approximation.
			// If CreatedAt is available and parsable, use it. Otherwise, Created remains 0.
			// This part is simplified; robust parsing of Docker's 'CreatedAt' string can be complex.
		}
		// We'll also store the human-readable "CreatedSince" in a temporary way if needed,
		// or decide that for CLI parsing, ID, RepoTags, and Size are primary.
		// The `image.Summary` doesn't have a direct field for "CreatedSince" string.
		// We can add it to Labels if desired, or a custom struct.
		// For now, we'll set `summary.Created` based on CreatedAt if available, or leave it as 0.
		// The `image.Summary.Created` field expects a Unix timestamp (int64).
		// Parsing `dockerImage.CreatedAt` (e.g., "2023-10-26 09:07:26 -0700 MST") to Unix timestamp:
		if parsedTime, err := time.Parse("2006-01-02 15:04:05 -0700 MST", dockerImage.CreatedAt); err == nil {
			summary.Created = parsedTime.Unix()
		} else if parsedTime, err := time.Parse("2006-01-02 15:04:05 Z0700 MST", dockerImage.CreatedAt); err == nil { // Another common variation
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
// Corresponds to `docker rmi`.
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
// Corresponds to `docker build`.
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
		// Ensure dockerfilePath is absolute or relative to contextPath.
		// For remote execution, assume paths are already correct for the remote machine.
		cmdArgs = append(cmdArgs, "-f", shellEscape(dockerfilePath))
	}

	cmdArgs = append(cmdArgs, "-t", shellEscape(imageNameAndTag))

	if buildArgs != nil {
		for key, value := range buildArgs {
			if strings.TrimSpace(key) == "" {
				return errors.New("buildArg key cannot be empty")
			}
			// Value can be empty, e.g., --build-arg MY_ARG=
			cmdArgs = append(cmdArgs, "--build-arg", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	// Context path must be the last argument before the path itself.
	cmdArgs = append(cmdArgs, shellEscape(contextPath))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerBuildTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Include stdout and stderr in the error for better debugging
		return errors.Wrapf(err, "failed to build image %s. Stdout: %s, Stderr: %s", imageNameAndTag, string(stdout), string(stderr))
	}
	return nil
}

// CreateContainer creates a new Docker container.
// Corresponds to `docker create`.
// Returns the ID of the created container.
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
		if strings.TrimSpace(envVar) != "" { // Allow empty values, but not empty var names if split by '='
			cmdArgs = append(cmdArgs, "-e", shellEscape(envVar))
		}
	}

	if len(options.Entrypoint) > 0 {
		// Docker CLI expects --entrypoint to be a single string if the entrypoint itself is a single command.
		// If it's a JSON array in the Dockerfile, it's typically overridden as a single command string here.
		// However, to pass multiple arguments to the entrypoint via CLI, they usually become part of the command.
		// For `docker create --entrypoint`, it's usually a single path.
		// If options.Entrypoint is a slice, we'll take the first element as the entrypoint
		// and subsequent elements would typically be part of the CMD.
		// Docker's behavior: `docker run --entrypoint /new/entry myimage cmd arg1`
		// `docker create --entrypoint` expects a single string.
		// If the intent is to set an entrypoint that is a list of strings (like JSON format in Dockerfile),
		// this CLI approach is tricky. `docker create --entrypoint '["/bin/sh", "-c"]'` might work on some shells.
		// For simplicity, we'll assume options.Entrypoint[0] is the executable.
		// A common use is `docker create --entrypoint myentrypoint image mycommand myarg`
		// If Entrypoint is ["/bin/sh", "-c", "echo hello"], this needs careful formatting.
		// Let's assume Entrypoint is just the command path, and Command slice contains its arguments.
		cmdArgs = append(cmdArgs, "--entrypoint", shellEscape(options.Entrypoint[0]))
		// Note: If options.Entrypoint has more elements, they are ignored here, assuming they'd be part of options.Command
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
	// Add other options like Labels, WorkingDir, User, etc. as needed.

	cmdArgs = append(cmdArgs, shellEscape(options.ImageName))

	// Command and its arguments come after the image name
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
// It uses `docker inspect` which is reliable for this.
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
		return true, nil // Exit code 0 means container exists
	}

	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 { // Common exit code for "not found"
			return false, nil
		}
	}
	return false, errors.Wrapf(err, "failed to check if container %s exists", containerNameOrID)
}

// StartContainer starts an existing Docker container.
// Corresponds to `docker start`.
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

	// Docker start usually outputs the container name/ID on success.
	// Stderr might contain warnings (e.g., already started) but still exit 0.
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Check if the error is because the container is already running.
		// Docker CLI might return exit code 0 and print to stderr, or a specific error.
		// This behavior can be inconsistent. For simplicity, we treat any error from Exec as failure.
		// More sophisticated handling might parse stderr for "already started" messages if err is nil but stderr is not.
		return errors.Wrapf(err, "failed to start container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// StopContainer stops a running Docker container.
// Corresponds to `docker stop`.
// `timeoutSeconds` is the grace period for the container to stop before being killed.
// If `timeoutSeconds` is nil, Docker's default grace period (usually 10 seconds) is used.
func (r *defaultRunner) StopContainer(ctx context.Context, c connector.Connector, containerNameOrID string, timeoutSeconds *time.Duration) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "stop"}
	execTimeout := DefaultDockerStopExecTimeout

	if timeoutSeconds != nil {
		gracePeriod := int((*timeoutSeconds).Seconds())
		if gracePeriod < 0 { // Docker CLI might not accept negative, default to 0 or positive.
			gracePeriod = 0
		}
		cmdArgs = append(cmdArgs, "-t", strconv.Itoa(gracePeriod))
		// Adjust overall exec timeout to be grace period + buffer
		execTimeout = (*timeoutSeconds) + (30 * time.Second) // Give ample time for the command itself
	}


	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	// Docker stop usually outputs the container name/ID on success.
	// Stderr might contain "No such container" or other errors.
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to stop container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}


// RestartContainer restarts a Docker container.
// Corresponds to `docker restart`.
// `timeoutSeconds` is the grace period for stopping the container before it's forcefully killed and then restarted.
// If `timeoutSeconds` is nil, Docker's default grace period (usually 10 seconds) is used for the stop phase.
func (r *defaultRunner) RestartContainer(ctx context.Context, c connector.Connector, containerNameOrID string, timeoutSeconds *time.Duration) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "restart"}
	execTimeout := DefaultDockerRestartExecTimeout // Default execution timeout for the command

	if timeoutSeconds != nil {
		gracePeriod := int((*timeoutSeconds).Seconds())
		if gracePeriod < 0 {
			gracePeriod = 0 // Docker CLI might not accept negative values
		}
		cmdArgs = append(cmdArgs, "-t", strconv.Itoa(gracePeriod))
		// Adjust overall exec timeout to be grace period + buffer, if a specific grace period is given.
		// If not, DefaultDockerRestartExecTimeout already considers Docker's default grace period.
		execTimeout = (*timeoutSeconds) + (10 * time.Second) // Ensure exec timeout is larger than restart grace period
	}

	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	// Docker restart usually outputs the container name/ID on success (if not already stopped/restarted quickly).
	// Stderr might contain "No such container" or other errors.
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to restart container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// RemoveContainer removes a Docker container.
// Corresponds to `docker rm`.
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

	// Docker rm usually outputs the container name/ID on success.
	// Stderr might contain "No such container" or other errors.
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Docker CLI might return exit code 1 if container not found but still print to stderr.
		// If force is true, "no such container" is not an error for idempotency.
		if force && strings.Contains(string(stderr), "No such container") {
			return nil // Suppress error if forcing and container doesn't exist
		}
		return errors.Wrapf(err, "failed to remove container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// ListContainers lists Docker containers.
// Corresponds to `docker ps`.
// Uses a custom format `{{json .}}` to get structured data for each container.
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
	cmdArgs = append(cmdArgs, "--format", "{{json .}}") // Output each container info as a JSON line
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

		// This intermediate struct matches the JSON fields from `docker ps --format "{{json .}}"`
		var cliContainer struct {
			ID         string
			Image      string
			Command    string
			CreatedAt  string // Human-readable, e.g., "2023-03-20 10:00:00 +0000 UTC"
			RunningFor string // e.g., "2 hours ago"
			Ports      string // e.g., "0.0.0.0:80->80/tcp, :::80->80/tcp"
			Status     string // e.g., "Up 2 hours" or "Exited (0) 5 minutes ago"
			Size       string // Only if --size is used, not by default with json format.
			Names      string
			Labels     string // Comma-separated key=value pairs
			Mounts     string // Comma-separated list of mount points (source paths)
			Networks   string // Comma-separated list of network names
		}

		if err := json.Unmarshal([]byte(line), &cliContainer); err != nil {
			return nil, errors.Wrapf(err, "failed to parse container JSON line: %s", line)
		}

		// Parse CreatedAt (this is a simplified parsing attempt)
		var createdTimestamp int64
		if parsedTime, err := time.Parse("2006-01-02 15:04:05 -0700 MST", cliContainer.CreatedAt); err == nil {
			createdTimestamp = parsedTime.Unix()
		} else if parsedTime, err := time.Parse("2006-01-02 15:04:05 Z0700 MST", cliContainer.CreatedAt); err == nil {
			createdTimestamp = parsedTime.Unix()
		}


		// Parse Names (can be multiple, comma-separated)
		var namesList []string
		if strings.TrimSpace(cliContainer.Names) != "" {
			namesList = strings.Split(cliContainer.Names, ",")
		}

		// Parse Labels
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

		// Parse Ports (this is a very simplified parsing for the structure)
        // Docker's port format "0.0.0.0:80->80/tcp, :::80->80/tcp" is complex.
        // A full parsing into ContainerPortMapping is non-trivial from this string.
        // For now, we'll store the raw string or skip detailed parsing.
        // This implementation will just create a placeholder.
        var portMappings []ContainerPortMapping
        if strings.TrimSpace(cliContainer.Ports) != "" {
            // Simplified: add one mapping per entry if possible, but format is complex
            rawPorts := strings.Split(cliContainer.Ports, ",")
            for _, rp := range rawPorts {
                if strings.TrimSpace(rp) != "" {
                     // Example: 0.0.0.0:80->80/tcp - needs parsing
                    parts := strings.Split(rp, "->")
                    var hostPart, containerPart string
                    if len(parts) == 2 {
                        hostPart = parts[0]
                        containerPart = parts[1]
                    } else {
                        containerPart = parts[0] // Could be just container port
                    }

                    var hostIP, hostPort, containerPortVal, protocol string

                    // Parse hostPart (e.g., 0.0.0.0:80)
                    if strings.Contains(hostPart, ":") {
                        hostIPPort := strings.SplitN(hostPart, ":", 2)
                        if len(hostIPPort) == 2 {
                            hostIP = hostIPPort[0]
                            hostPort = hostIPPort[1]
                        } else {
                            hostPort = hostIPPort[0]
                        }
                    }


                    // Parse containerPart (e.g., 80/tcp)
                    if strings.Contains(containerPart, "/") {
                        containerProto := strings.SplitN(containerPart, "/", 2)
                        containerPortVal = containerProto[0]
                        if len(containerProto) == 2 {
                            protocol = containerProto[1]
                        }
                    } else {
                        containerPortVal = containerPart
                    }

                    portMappings = append(portMappings, ContainerPortMapping{
                        HostIP: hostIP,
                        HostPort: hostPort,
                        ContainerPort: containerPortVal,
                        Protocol: protocol,
                    })
                }
            }
        }


		// Parse Mounts (CLI output is just a list of source paths)
		var mountsList []ContainerMount
		if strings.TrimSpace(cliContainer.Mounts) != "" {
			mountSources := strings.Split(cliContainer.Mounts, ",")
			for _, src := range mountSources {
				mountsList = append(mountsList, ContainerMount{Source: strings.TrimSpace(src)}) // Only source is available here
			}
		}

		// Extract State from Status (e.g., "Up", "Exited")
		var state string
		if strings.HasPrefix(strings.ToLower(cliContainer.Status), "up") {
			state = "running"
		} else if strings.HasPrefix(strings.ToLower(cliContainer.Status), "exited") {
			state = "exited"
		} else if strings.Contains(strings.ToLower(cliContainer.Status), "created") {
			state = "created"
		} else if strings.Contains(strings.ToLower(cliContainer.Status), "restarting") {
            state = "restarting"
        } else if strings.Contains(strings.ToLower(cliContainer.Status), "paused") {
            state = "paused"
        } else {
			state = "unknown" // Or parse more states
		}


		containers = append(containers, ContainerInfo{
			ID:      cliContainer.ID,
			Names:   namesList,
			Image:   cliContainer.Image,
			Command: cliContainer.Command,
			Created: createdTimestamp,
			State:   state,
			Status:  cliContainer.Status,
			Ports:   portMappings, // Simplified
			Labels:  labelsMap,
			Mounts:  mountsList,  // Simplified
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker ps output")
	}

	return containers, nil
}

// GetContainerLogs retrieves logs from a container.
// Corresponds to `docker logs`.
func (r *defaultRunner) GetContainerLogs(ctx context.Context, c connector.Connector, containerNameOrID string, options ContainerLogOptions) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return "", errors.New("containerNameOrID cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker", "logs")

	if options.Follow {
		// Streaming logs is not directly supported by this function's signature (returns string).
		// Consider a different function for streaming, or document that Follow=true is best-effort (e.g. get recent logs).
		// For now, we'll add the flag, but the Exec call won't stream indefinitely.
		// Perhaps Follow should imply a very short timeout or be disallowed for this func.
		// For simplicity, let's ignore Follow for a simple string return.
		// cmdArgs = append(cmdArgs, "--follow")
	}
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

	// By default, docker logs shows both stdout and stderr.
	// If only one is requested, Docker CLI doesn't have separate flags for this.
	// The API client does, but we are using CLI.
	// So, ShowStdout and ShowStderr flags are more informational for an API-like interface.
	// The CLI will always return both.

	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	// Timeout for logs should be configurable or reasonably short for non-streaming.
	execTimeout := 2 * time.Minute // Default, can be overridden by options if we add Timeout to ContainerLogOptions
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Logs command can output to both stdout and stderr even on success (stderr for progress/timestamps if enabled differently)
		// The primary log content is usually on stdout.
		// If there's an error, stderr from Exec call is more relevant for the error itself.
		return "", errors.Wrapf(err, "failed to get logs for container %s. Stderr: %s", containerNameOrID, string(stderr))
	}

	// Docker logs command sends actual log lines to stdout.
	// If timestamps are enabled, they are part of stdout.
	// If there were errors during log retrieval not causing Exec to fail (e.g. log driver issues),
	// they might appear in stderr from the docker daemon.
	// For this runner, we'll return stdout as the logs.
	// The `stderr` from `c.Exec` will contain errors from the `docker logs` command execution itself.
	// If `docker logs` prints its own error messages to its stderr (e.g. "Error: No such container: foo"),
	// those would be in `string(stderr)` from `c.Exec`.
	// It's a bit nuanced. If `err != nil`, `string(stderr)` is the command's error stream.
	// If `err == nil`, `string(stdout)` is the logs. `string(stderr)` might be empty or contain non-fatal messages.
	// We return `string(stdout)` as the primary output.

	return string(stdout), nil
}


// GetContainerStats retrieves live resource usage statistics for a container.
// Corresponds to `docker stats --no-stream <container> --format "{{json .}}"`.
// This implementation fetches a single snapshot of stats, not a stream.
// A streaming version would require a different function signature (e.g., returning a channel).
func (r *defaultRunner) GetContainerStats(ctx context.Context, c connector.Connector, containerNameOrID string, stream bool) (<-chan ContainerStats, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return nil, errors.New("containerNameOrID cannot be empty")
	}

    // This implementation will provide a single stat snapshot if stream is false,
    // and a short stream if stream is true, closing after a few updates or timeout.
    // True real-time streaming is complex with a simple Exec model.
	statsChan := make(chan ContainerStats)

	go func() {
		defer close(statsChan)

		cmdArgs := []string{"docker", "stats"}
		if !stream {
			cmdArgs = append(cmdArgs, "--no-stream")
		}
		cmdArgs = append(cmdArgs, "--format", "{{json .}}", shellEscape(containerNameOrID))
		cmd := strings.Join(cmdArgs, " ")

		// For streaming, we need a way to stop the command. The context passed to Exec will handle this.
		// Timeout for the Exec call itself. If streaming, this acts as max duration.
		execTimeout := 30 * time.Second
		if stream {
            execTimeout = 2 * time.Minute // Allow longer for a short stream
        }

		execOptions := &connector.ExecOptions{
			Sudo:    true,
			Timeout: execTimeout, // For non-streaming, this is ample. For streaming, it limits the stream.
		}

        // We need to use a connector that can stream output.
        // The current `c.Exec` signature returns all output after completion.
        // This is a limitation. For a true stream, `connector.Exec` would need modification
        // or a new streaming method.
        //
        // Workaround for now: If streaming, run repeatedly for a few times or parse multi-JSON output if supported.
        // `docker stats --format "{{json .}}"` without --no-stream continuously prints JSON lines.
        // We can read these lines.

        if !stream {
            stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
            if err != nil {
                statsChan <- ContainerStats{Error: errors.Wrapf(err, "failed to get stats for container %s. Stderr: %s", containerNameOrID, string(stderr))}
                return
            }
            var stat ContainerStats // This needs to match the fields from Docker's JSON output
            if err := json.Unmarshal(stdout, &stat); err != nil {
                statsChan <- ContainerStats{Error: errors.Wrapf(err, "failed to parse container stats JSON: %s. Output: %s", err, string(stdout))}
                return
            }
            statsChan <- stat
            return
        }

        // Simplified streaming: execute command and read its stdout line by line
        // This requires `connector.Exec` to support streaming output, or a new method.
        // Assuming `c.Exec` is blocking and returns all output at once, true streaming is not possible here.
        // For the purpose of this exercise, we'll simulate by fetching a few snapshots if stream=true.
        // Or, if the connector could provide a reader for stdout:
        // stdoutReader, _, err := c.StreamExec(ctx, cmd, execOptions)
        // if err != nil { ... }
        // scanner := bufio.NewScanner(stdoutReader)
        // for scanner.Scan() { ... parse line ... send to statsChan ... }
        //
        // Since `StreamExec` is not defined, this will be a placeholder for how it *could* work.
        // For now, if stream=true, we will just return an error or a very limited stream.

        // Let's assume `c.Exec` can be made to work with a context that times out for a "short stream".
        // The `docker stats` command when streaming will continuously output.
        // The `execOptions.Timeout` will eventually kill it.
        stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
        if err != nil {
            // It's possible the command was killed by timeout, which is expected for streaming.
            // Check context error.
            if ctx.Err() != nil && (errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled)) {
                 // This is an expected termination of the stream.
            } else {
                statsChan <- ContainerStats{Error: errors.Wrapf(err, "streaming stats for container %s failed. Stderr: %s", containerNameOrID, string(stderr))}
            }
            return
        }

        scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
        for scanner.Scan() {
            line := scanner.Text()
            if strings.TrimSpace(line) == "" {
                continue
            }
            var statData struct { // Structure based on `docker stats --format "{{json .}}"`
				Name      string
				ID        string
				CPUPerc   string // e.g. "0.00%"
				MemUsage  string // e.g. "1.2MiB / 1.9GiB"
				MemPerc   string // e.g. "0.06%"
				NetIO     string // e.g. "648B / 648B" (input/output)
				BlockIO   string // e.g. "0B / 0B"
				PIDs      string // e.g. "2"
			}
            if err := json.Unmarshal([]byte(line), &statData); err != nil {
                statsChan <- ContainerStats{Error: errors.Wrapf(err, "failed to parse streaming stats line: %s. Line: %s", err, line)}
                continue // Or return/break
            }

            // Convert parsed strings to ContainerStats fields
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
            case <-ctx.Done(): // Ensure we stop if the overall context is cancelled
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
// Corresponds to `docker inspect <container>`.
// Parses the JSON output into the ContainerDetails struct.
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
		// If inspect fails because container not found, Docker exits with 1.
		// We should return nil, nil in that case, similar to Docker API client.
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) && cmdErr.ExitCode == 1 {
			// Check stderr for "No such object" or similar messages to be more certain.
			// For simplicity, exit code 1 is often "not found".
			if strings.Contains(strings.ToLower(string(stderr)), "no such object") ||
				strings.Contains(strings.ToLower(string(stderr)), "is not a docker command") { // This can happen if name is misinterpreted
				return nil, nil // Not found
			}
		}
		return nil, errors.Wrapf(err, "failed to inspect container %s. Stderr: %s", containerNameOrID, string(stderr))
	}

	// Docker inspect returns a JSON array if multiple objects are inspected,
	// or a single JSON object if one is inspected. We expect one.
	// If the output is an array, we take the first element.
	outputStr := strings.TrimSpace(string(stdout))
	if outputStr == "" {
		return nil, errors.New("docker inspect returned empty output")
	}

	var details []ContainerDetails // Docker inspect returns an array of details
	if err := json.Unmarshal([]byte(outputStr), &details); err != nil {
		// Attempt to unmarshal as single object if array fails (should not happen with single inspect)
		var singleDetail ContainerDetails
		if errSingle := json.Unmarshal([]byte(outputStr), &singleDetail); errSingle != nil {
			return nil, errors.Wrapf(err, "failed to parse container inspect JSON (tried array and object): %s. Output: %s", errSingle, outputStr)
		}
		return &singleDetail, nil
	}

	if len(details) == 0 {
		// This case should ideally be caught by err != nil if container not found.
		// But if inspect returns empty array for some reason.
		return nil, nil // Not found
	}

	return &details[0], nil
}


// PauseContainer pauses all processes within a running container.
// Corresponds to `docker pause`.
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
		Timeout: DefaultDockerStartTimeout, // Pause is usually quick
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to pause container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// UnpauseContainer unpauses all processes within a paused container.
// Corresponds to `docker unpause`.
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
		Timeout: DefaultDockerStartTimeout, // Unpause is usually quick
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to unpause container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// ExecInContainer executes a command inside a running container.
// Corresponds to `docker exec`.
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
		// Note: Docker's behavior with -t can affect output format.
		// For non-interactive exec getting combined output, -t is often omitted or used with -i.
		// If a true TTY interaction is needed, this function signature is insufficient.
		// This implies -t for allocation, but not necessarily interactive (-i).
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

	// Timeout for exec should be reasonably long or configurable.
	execTimeout := 5 * time.Minute
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Combined stdout/stderr is often desired for exec.
		// However, c.Exec separates them. We'll return stdout and include stderr in the error.
		// Docker exec CLI writes command's stdout to its stdout, and command's stderr to its stderr.
		// If the command in container exits non-zero, `docker exec` also exits non-zero.
		output := string(stdout) + string(stderr) // Simple combination
		return output, errors.Wrapf(err, "failed to exec in container %s (cmd: %s). Combined output: %s", containerNameOrID, strings.Join(cmdArgsToExec, " "), output)
	}

	// If successful, stdout contains the command's stdout. Stderr from Exec is for the docker command itself.
	// The actual command's stderr (if any, and if not redirected by the command itself) would also be in Exec's stderr.
	// It's common to want both, so we concatenate.
	return string(stdout) + string(stderr), nil
}


// --- Docker Network Methods ---

// CreateDockerNetwork creates a new Docker network.
// Corresponds to `docker network create`.
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
	// TODO: Add labels if the struct in interface.go is updated
	// if labels != nil {
	// 	for k, v := range labels {
	// 		cmdArgs = append(cmdArgs, "--label", shellEscape(fmt.Sprintf("%s=%s", k, v)))
	// 	}
	// }

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
// Corresponds to `docker network rm`.
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
		// Idempotency: if network not found, don't treat as error.
		if strings.Contains(string(stderr), "No such network") || strings.Contains(string(stderr), "not found") {
			return nil
		}
		return errors.Wrapf(err, "failed to remove docker network %s. Stderr: %s", networkNameOrID, string(stderr))
	}
	return nil
}

// ListDockerNetworks lists Docker networks.
// Corresponds to `docker network ls`.
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
		var netInfo struct { // Matches `docker network ls --format "{{json .}}"`
			ID         string
			Name       string
			Driver     string
			Scope      string
			IPv6       string // "true" or "false"
			Internal   string // "true" or "false"
			Attachable string // "true" or "false"
			Ingress    string // "true" or "false"
			// Labels     string // comma-separated
		}
		if err := json.Unmarshal([]byte(line), &netInfo); err != nil {
			return nil, errors.Wrapf(err, "failed to parse network JSON line: %s", line)
		}

		enableIPv6, _ := strconv.ParseBool(netInfo.IPv6)

		// Note: `docker network ls` does not provide Subnets, Gateways, or connected Containers.
		// That info requires `docker network inspect` per network.
		// This ListDockerNetworks provides what `ls` provides.
		networks = append(networks, DockerNetworkInfo{
			ID:         netInfo.ID,
			Name:       netInfo.Name,
			Driver:     netInfo.Driver,
			Scope:      netInfo.Scope,
			EnableIPv6: enableIPv6,
			// Other fields like Subnets, Gateways, Containers would require individual inspect calls.
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker networks ls output")
	}
	return networks, nil
}

// ConnectContainerToNetwork connects a container to a Docker network.
// Corresponds to `docker network connect`.
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
		// Idempotency: if already connected, Docker CLI might error or do nothing.
		// "Error response from daemon: container ... is already connected to network ..."
		if strings.Contains(string(stderr), "is already connected to network") {
			return nil
		}
		return errors.Wrapf(err, "failed to connect container %s to network %s. Stderr: %s", containerNameOrID, networkNameOrID, string(stderr))
	}
	return nil
}

// DisconnectContainerFromNetwork disconnects a container from a Docker network.
// Corresponds to `docker network disconnect`.
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
		// Idempotency: if not connected, Docker CLI errors.
		// "Error response from daemon: container ... is not connected to network ..."
		if strings.Contains(string(stderr), "is not connected to network") {
			return nil
		}
		return errors.Wrapf(err, "failed to disconnect container %s from network %s. Stderr: %s", containerNameOrID, networkNameOrID, string(stderr))
	}
	return nil
}


// --- Docker Volume Methods ---

// CreateDockerVolume creates a new Docker volume.
// Corresponds to `docker volume create`.
func (r *defaultRunner) CreateDockerVolume(ctx context.Context, c connector.Connector, name string, driver string, driverOpts map[string]string, labels map[string]string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	// Name is optional for `docker volume create`, Docker will generate one.
	// However, our interface implies a name is given or expected.

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
	// `docker volume create` outputs the volume name on success.
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Idempotency: If volume with this name already exists, Docker errors.
		// "Error response from daemon: a volume with the name <name> already exists"
		if strings.TrimSpace(name) != "" && strings.Contains(string(stderr), "already exists") && strings.Contains(string(stderr), name) {
			return nil
		}
		return errors.Wrapf(err, "failed to create docker volume %s. Stderr: %s, Stdout: %s", name, string(stderr), string(stdout))
	}
	return nil
}

// RemoveDockerVolume removes a Docker volume.
// Corresponds to `docker volume rm`.
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
		// Idempotency: If volume not found, Docker errors.
		// "Error: No such volume: <name>"
		if strings.Contains(string(stderr), "No such volume") && strings.Contains(string(stderr), volumeName) {
			return nil
		}
		// If volume is in use: "Error response from daemon: remove <volumeName>: volume is in use"
		// This is a valid error if not forced. If forced, this error shouldn't occur.
		return errors.Wrapf(err, "failed to remove docker volume %s. Stderr: %s", volumeName, string(stderr))
	}
	return nil
}

// ListDockerVolumes lists Docker volumes.
// Corresponds to `docker volume ls`.
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
		// Structure from `docker volume ls --format "{{json .}}"`
		var volData struct {
			Driver     string
			Labels     string // Comma-separated key=value
			Links      string // Number of containers using volume (older Docker versions?)
			Mountpoint string
			Name       string
			Scope      string
			Size       string // Only with `docker system df -v` or specific drivers
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
                    labelsMap[kv[0]] = "" // Label without value
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
// Corresponds to `docker volume inspect`.
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
				return nil, nil // Not found
			}
		}
		return nil, errors.Wrapf(err, "failed to inspect volume %s. Stderr: %s", volumeName, string(stderr))
	}

	outputStr := strings.TrimSpace(string(stdout))
	if outputStr == "" {
		return nil, errors.New("docker volume inspect returned empty output")
	}

	// Docker volume inspect returns a JSON array if multiple volumes are inspected by name,
	// or a single JSON object if one specific volume is inspected.
	var detailsList []DockerVolumeDetails
	if err := json.Unmarshal([]byte(outputStr), &detailsList); err == nil && len(detailsList) > 0 {
		// If it parsed as a list and has elements, take the first one.
		// This handles cases where a name might match multiple if not unique (though inspect usually takes one).
		return &detailsList[0], nil
	}

	// If it's not a list or list is empty, try parsing as a single object.
	var detail DockerVolumeDetails
	if err := json.Unmarshal([]byte(outputStr), &detail); err != nil {
		return nil, errors.Wrapf(err, "failed to parse volume inspect JSON: %s. Output: %s", err, outputStr)
	}
	return &detail, nil
}


// --- Docker System Methods ---

// DockerInfo displays system-wide information about Docker.
// Corresponds to `docker info --format "{{json .}}"`.
func (r *defaultRunner) DockerInfo(ctx context.Context, c connector.Connector) (*DockerSystemInfo, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmd := "docker info --format \"{{json .}}\"" // Ensure quotes handle spaces in format string if any
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get docker info. Stderr: %s", string(stderr))
	}

	var info DockerSystemInfo // This struct needs to match the fields from Docker's JSON output
	if err := json.Unmarshal(stdout, &info); err != nil {
		return nil, errors.Wrapf(err, "failed to parse docker info JSON: %s. Output: %s", err, string(stdout))
	}
	return &info, nil
}

// DockerPrune removes unused Docker data.
// Corresponds to `docker system prune`, `docker image prune`, etc.
// `pruneType` can be "system", "builder", "container", "image", "network", "volume".
func (r *defaultRunner) DockerPrune(ctx context.Context, c connector.Connector, pruneType string, filters map[string]string, all bool) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(pruneType) == "" {
		pruneType = "system" // Default to system prune
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "docker")

	validPruneTypes := map[string]bool{
		"system":    true,
		"builder":   true,
		"container": true,
		"image":     true,
		"network":   true,
		"volume":    true,
	}
	if !validPruneTypes[pruneType] {
		return "", errors.Errorf("invalid pruneType: %s. Must be one of system, builder, container, image, network, volume", pruneType)
	}

	if pruneType == "system" {
        cmdArgs = append(cmdArgs, "system", "prune", "-f") // Always force for system prune to avoid prompt
    } else {
        cmdArgs = append(cmdArgs, pruneType, "prune", "-f") // Always force to avoid prompt
    }


	if all && (pruneType == "system" || pruneType == "image" || pruneType == "builder") {
		// --all is applicable to system (implies all for images), image, and builder prune
		cmdArgs = append(cmdArgs, "--all")
	}
	if filters != nil {
		for key, value := range filters {
			// `docker system prune` doesn't support arbitrary key=value filters like `ps` or `images`.
			// It supports `--filter "label=key=value"` or `--filter "until=24h"`.
			// This implementation will assume the filter string is pre-formatted correctly.
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s",key,value))) // This might need more specific formatting based on pruneType
		}
	}
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Minute} // Pruning can take time
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to prune docker %s. Stderr: %s, Stdout: %s", pruneType, string(stderr), string(stdout))
	}
	// Output often includes "Total reclaimed space: ..."
	return string(stdout), nil
}

// GetDockerServerVersion returns the version of the Docker server.
func (r *defaultRunner) GetDockerServerVersion(ctx context.Context, c connector.Connector) (*semver.Version, error) {
	// Implementation for GetDockerServerVersion using `docker version --format '{{.Server.Version}}'`
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
	// Docker versions can sometimes have prefixes like "ce" or "ee", or suffixes.
	// semver.NewVersion is generally robust but might need pre-processing for complex Docker version strings.
	// A common pattern is just "20.10.7".
	// Let's try to parse directly. If it fails, we might need to clean it.
	v, err := semver.NewVersion(versionStr)
	if err != nil {
		// Attempt to clean common non-semver characters if parsing fails.
		// Example: "docker-ce-20.10.7" -> "20.10.7"
		// This is a simple heuristic. More complex version strings might require more advanced parsing.
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
	// A simple way to check is to run `docker version`.
	// If it runs without error, Docker is likely installed and the daemon is reachable.
	cmd := "docker version"
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout} // Sudo might be needed depending on setup
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "docker not installed or not accessible. Stderr: %s", string(stderr))
	}
	return nil
}

// EnsureDockerService ensures the Docker service is running and enabled.
// This is a simplified example; robustly managing system services can be complex
// and might require knowledge of the specific init system (systemd, sysvinit, etc.).
func (r *defaultRunner) EnsureDockerService(ctx context.Context, c connector.Connector) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}

	// 1. Check if Docker service is active (e.g., using systemctl for systemd)
	// This command's specifics can vary based on the OS and init system.
	// Assuming systemd for this example.
	// `systemctl is-active docker` returns "active" and exit code 0 if active.
	// Otherwise, it returns "inactive" or "failed" and a non-zero exit code.
	isActiveCmd := "systemctl is-active docker"
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second}
	stdout, _, err := c.Exec(ctx, isActiveCmd, execOptions)

	if err == nil && strings.TrimSpace(string(stdout)) == "active" {
		// Already active, now check if enabled.
		isEnabledCmd := "systemctl is-enabled docker"
		stdoutEnabled, _, errEnabled := c.Exec(ctx, isEnabledCmd, execOptions)
		if errEnabled == nil && (strings.TrimSpace(string(stdoutEnabled)) == "enabled" || strings.TrimSpace(string(stdoutEnabled)) == "static") {
			return nil // Active and enabled (or static, which means effectively enabled).
		}
		if errEnabled != nil { // Error checking if enabled
			// If it's active but enabling check failed, we might still proceed or log a warning.
			// For now, let's try to enable it if it's not explicitly enabled.
		}
		if strings.TrimSpace(string(stdoutEnabled)) != "enabled" && strings.TrimSpace(string(stdoutEnabled)) != "static" {
			enableCmd := "systemctl enable docker"
			_, stderrEnable, errEnableCmd := c.Exec(ctx, enableCmd, execOptions)
			if errEnableCmd != nil {
				return errors.Wrapf(errEnableCmd, "failed to enable docker service. Stderr: %s", string(stderrEnable))
			}
		}
		return nil // Was active, and now ensured it's enabled.
	}

	// If not active or `is-active` command failed (e.g. service not found, systemctl error)
	// Try to start it.
	startCmd := "systemctl start docker"
	_, stderrStart, errStart := c.Exec(ctx, startCmd, execOptions)
	if errStart != nil {
		// If starting fails, it could be because it's not installed, or a deeper issue.
		// We could try `CheckDockerInstalled` here for a better error.
		// For now, just wrap the start error.
		// Check if docker is installed first for a more specific error
		if installErr := r.CheckDockerInstalled(ctx, c); installErr != nil {
			return errors.Wrap(installErr, "docker service failed to start because docker is not installed or accessible")
		}
		return errors.Wrapf(errStart, "failed to start docker service. Stderr: %s", string(stderrStart))
	}

	// Started successfully, now ensure it's enabled.
	enableCmd := "systemctl enable docker"
	_, stderrEnable, errEnable := c.Exec(ctx, enableCmd, execOptions)
	if errEnable != nil {
		// Log a warning or return error? If it started, it might be okay for current session.
		// For "Ensure", we should probably return an error if enabling fails.
		return errors.Wrapf(errEnable, "docker service started but failed to enable it. Stderr: %s", string(stderrEnable))
	}

	return nil
}

// --- Helper Types (Consider moving to a types.go if they grow numerous) ---

// ContainerCreateOptions holds parameters for creating a container.
// This is a simplified version of moby/api/types/container.CreateOptions / container.HostConfig
type ContainerCreateOptions struct {
	ImageName     string
	ContainerName string
	Ports         []ContainerPortMapping // e.g., "8080:80/tcp"
	Volumes       []ContainerMount       // e.g., "/host/path:/container/path:ro"
	EnvVars       []string               // e.g., "FOO=bar"
	Entrypoint    []string               // e.g., ["/bin/sh", "-c"]
	Command       []string               // e.g., ["echo", "hello"]
	RestartPolicy string                 // e.g., "on-failure:3"
	Privileged    bool
	AutoRemove    bool
	Labels        map[string]string
	WorkingDir    string
	User          string
	// Add more fields as needed: NetworkMode, Resources, etc.
}

// ContainerPortMapping defines a port mapping for a container.
type ContainerPortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string // "tcp", "udp", "sctp" - defaults to "tcp" if empty
}

// ContainerMount defines a volume mount for a container.
type ContainerMount struct {
	Source      string // Host path or named volume
	Destination string // Container path
	Mode        string // e.g., "ro" for read-only, "rw" for read-write (default)
}

// ContainerLogOptions specifies parameters for fetching container logs.
type ContainerLogOptions struct {
	ShowStdout bool
	ShowStderr bool
	Since      string // Timestamp (e.g., "2013-01-02T13:23:37Z") or Go duration string (e.g., "10m")
	Until      string // Timestamp or Go duration string
	Timestamps bool
	Follow     bool
	Tail       string // Number of lines to show from the end of the logs, or "all"
	Details    bool   // Show extra details provided to logs
}


// NetworkResource is a simplified representation of `docker network inspect` or `docker network ls` output.
// Based on moby/api/types/network.NetworkResource
type NetworkResource struct {
	Name       string
	ID         string
	Created    time.Time
	Scope      string
	Driver     string
	EnableIPv6 bool
	Internal   bool
	Attachable bool
	Ingress    bool
	ConfigOnly bool
	// IPAM       network.IPAM // Internet Protocol Address Management
	// Containers map[string]network.EndpointResource // Containers connected to this network
	Options    map[string]string
	Labels     map[string]string
}


// Volume is a simplified representation of `docker volume inspect` or `docker volume ls` output.
// Based on moby/api/types/volume.Volume
type Volume struct {
	CreatedAt  string `json:",omitempty"` // Date/Time the volume was created.
	Driver     string // Name of the volume driver used by the volume.
	Labels     map[string]string
	Mountpoint string // Path on the host where the volume is mounted.
	Name       string // Name of the volume.
	Options    map[string]string `json:",omitempty"` // The driver specific options used when creating the volume.
	Scope      string            // The scope of the volume ("local" or "global").
	// Status is no longer part of the official Volume type, but some older API versions or tools might include it.
	// Status     map[string]interface{} `json:",omitempty"` // Cluster Status of the volume (e.g. `{"Replication": "Desired": 1, "Running": 1}`)
	// UsageData *volume.UsageData `json:",omitempty"` // Usage details of the volume. (Available if driver supports it)
}


// SystemInfo is a simplified representation of `docker info` output.
// Based on moby/api/types/info.Info
type SystemInfo struct {
	ID                string
	Containers        int
	ContainersRunning int
	ContainersPaused  int
	ContainersStopped int
	Images            int
	ServerVersion     string
	StorageDriver     string
	LoggingDriver     string
	CgroupDriver      string // "cgroupfs" or "systemd"
	CgroupVersion     string // "1" or "2"
	KernelVersion     string
	OperatingSystem   string
	OSVersion         string
	OSType            string // "linux" or "windows"
	Architecture      string
	MemTotal          int64 // Total memory on the host
	// Add more fields as needed from `docker info`
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

// PruneDockerBuildCache prunes the Docker build cache.
// Corresponds to `docker builder prune -a -f`.
func (r *defaultRunner) PruneDockerBuildCache(ctx context.Context, c connector.Connector) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	cmd := "docker builder prune -a -f" // Prune all, force without prompt
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute}
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to prune Docker build cache. Stderr: %s", string(stderr))
	}
	return nil
}

// GetHostArchitecture retrieves the host architecture using Docker.
// Equivalent to `docker version --format '{{.Server.Arch}}'` or `{{.Server.Architecture}}`.
func (r *defaultRunner) GetHostArchitecture(ctx context.Context, c connector.Connector) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	// {{.Server.Architecture}} is generally more reliable and standard.
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

// ResolveDockerImage resolves a Docker image name to its digest (immutable identifier).
// Uses `docker inspect --format '{{index .RepoDigests 0}}' <image>` or `docker image inspect --format '{{index .RepoDigests 0}}' <image>`
// This is useful for ensuring you're using a specific version of an image.
func (r *defaultRunner) ResolveDockerImage(ctx context.Context, c connector.Connector, imageName string) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(imageName) == "" {
		return "", errors.New("imageName cannot be empty")
	}

	// Using `docker image inspect` is more modern.
	cmd := fmt.Sprintf("docker image inspect --format '{{range .RepoDigests}}{{.}}{{println}}{{end}}' %s", shellEscape(imageName))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultDockerInspectTimeout}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to inspect image %s to resolve digest. Stderr: %s", imageName, string(stderr))
	}

	output := strings.TrimSpace(string(stdout))
	if output == "" {
		// This can happen if the image exists but has no RepoDigests (e.g., locally built image not pushed)
		// Or if the image does not have a digest associated with the given tag (less common for pulled images).
		// Fallback: try to get the ImageID as a stable reference if no RepoDigest.
		// An ImageID (sha256:...) is also an immutable reference.
		cmdID := fmt.Sprintf("docker image inspect --format '{{.ID}}' %s", shellEscape(imageName))
		stdoutID, stderrID, errID := c.Exec(ctx, cmdID, execOptions)
		if errID != nil {
			return "", errors.Wrapf(errID, "failed to get ImageID for %s after RepoDigests were empty. Stderr: %s", imageName, string(stderrID))
		}
		imageID := strings.TrimSpace(string(stdoutID))
		if imageID == "" {
			return "", errors.Errorf("could not resolve image %s to a RepoDigest or ImageID (both were empty)", imageName)
		}
		// Ensure it has the "sha256:" prefix if it's a plain ID, common for RepoID.
		if !strings.HasPrefix(imageID, "sha256:") {
			// This case should be rare if {{.ID}} is used, as it's usually prefixed.
			// However, if we got a short ID, this won't make it a digest.
			// For safety, we'll return it as is, but note that a full digest is preferred.
		}
		return imageID, nil // Return ImageID as a fallback
	}

	// If there are multiple RepoDigests (e.g., image tagged in multiple repos), pick the first one.
	// This is a common convention.
	digests := strings.Split(output, "\n")
	if len(digests) > 0 && strings.TrimSpace(digests[0]) != "" {
		return strings.TrimSpace(digests[0]), nil
	}

	return "", errors.Errorf("could not resolve image %s to a RepoDigest (output was: '%s')", imageName, output)
}

// DockerSave saves one or more images to a tar archive.
// Corresponds to `docker save -o <output_file> <image1> <image2> ...`
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

	// Ensure the directory for the output file exists on the remote host.
	// This might require a separate 'mkdir -p' command if not handled by the user.
	// For simplicity, we assume the path is writable.
	// A more robust implementation might use r.EnsureDirectory(ctx, c, filepath.Dir(outputFilePath))

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
		Timeout: DefaultDockerBuildTimeout, // Saving large images can take time
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to save images to %s. Stderr: %s", outputFilePath, string(stderr))
	}
	return nil
}

// DockerLoad loads an image or repository from a tar archive or STDIN.
// Corresponds to `docker load -i <input_file>`
func (r *defaultRunner) DockerLoad(ctx context.Context, c connector.Connector, inputFilePath string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(inputFilePath) == "" {
		return errors.New("inputFilePath cannot be empty")
	}

	// Check if the input file exists on the remote host.
	// This might require a separate file existence check if strict validation is needed.
	// For simplicity, we assume the path is readable.
	// A more robust implementation might use r.FileExists(ctx, c, inputFilePath)

	cmd := fmt.Sprintf("docker load -i %s", shellEscape(inputFilePath))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerBuildTimeout, // Loading large images can take time
	}

	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Stderr might contain "Loaded image:" lines on success, so check error first.
		return errors.Wrapf(err, "failed to load image(s) from %s. Stderr: %s", inputFilePath, string(stderr))
	}
	return nil
}
