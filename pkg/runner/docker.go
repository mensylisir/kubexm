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

// parseDockerPorts parses the complex port string from `docker ps` into a slice of container.Port.
// Example input: "0.0.0.0:80->80/tcp, :::80->80/tcp, 0.0.0.0:443->443/tcp"
// This is a simplified parser. Docker's port string can be quite complex.
func parseDockerPorts(portsStr string) ([]container.Port, error) {
	if strings.TrimSpace(portsStr) == "" {
		return nil, nil
	}

	var parsedPorts []container.Port
	portParts := strings.Split(portsStr, ",")

	for _, part := range portParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Format: [hostIP:]hostPort->containerPort[/protocol]
		// Examples:
		// "0.0.0.0:49153->80/tcp"
		// "80/tcp" (host port and IP are dynamic/unspecified)
		// ":::80->80/tcp" (IPv6)

		var publicPort, privatePort uint16
		var ip, portType string

		arrowParts := strings.SplitN(part, "->", 2)
		if len(arrowParts) != 2 {
			// Could be a case like "80/tcp" where only private port and type are listed
			// This means host port is dynamically assigned if published at all.
			protoParts := strings.SplitN(arrowParts[0], "/", 2)
			if len(protoParts) == 2 {
				portType = protoParts[1]
			} else {
				portType = "tcp" // Default
			}
			privPortInt, err := strconv.ParseUint(protoParts[0], 10, 16)
			if err != nil {
				// Cannot parse, skip or log error
				// return nil, errors.Wrapf(err, "parsing private port from '%s'", protoParts[0])
				continue
			}
			privatePort = uint16(privPortInt)
			// PublicPort would be 0, IP empty, indicating not explicitly mapped or dynamic.
		} else {
			// Typical case: hostInfo->containerInfo
			hostInfo := arrowParts[0]
			containerInfo := arrowParts[1]

			// Parse containerInfo: port[/protocol]
			protoParts := strings.SplitN(containerInfo, "/", 2)
			if len(protoParts) == 2 {
				portType = protoParts[1]
			} else {
				portType = "tcp" // Default
			}
			privPortInt, err := strconv.ParseUint(protoParts[0], 10, 16)
			if err != nil {
				// return nil, errors.Wrapf(err, "parsing private port from '%s'", protoParts[0])
				continue
			}
			privatePort = uint16(privPortInt)

			// Parse hostInfo: [ip:]port
			lastColon := strings.LastIndex(hostInfo, ":")
			if lastColon == -1 { // No IP, just host port
				pubPortInt, err := strconv.ParseUint(hostInfo, 10, 16)
				if err != nil {
					// return nil, errors.Wrapf(err, "parsing public port from '%s'", hostInfo)
					continue
				}
				publicPort = uint16(pubPortInt)
				ip = "0.0.0.0" // Default if not specified
			} else {
				ip = hostInfo[:lastColon]
				if ip == "" || ip == "::" { // Docker uses "::" for IPv6 any-address
					ip = "::" // Normalize if needed, or keep as is. The Port struct takes string.
				}
				pubPortStr := hostInfo[lastColon+1:]
				pubPortInt, err := strconv.ParseUint(pubPortStr, 10, 16)
				if err != nil {
					// return nil, errors.Wrapf(err, "parsing public port from '%s'", pubPortStr)
					continue
				}
				publicPort = uint16(pubPortInt)
			}
		}

		parsedPorts = append(parsedPorts, container.Port{
			IP:          ip,
			PrivatePort: privatePort,
			PublicPort:  publicPort,
			Type:        portType,
		})
	}
	return parsedPorts, nil
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
		return errors.Wrapf(err, "failed to remove container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// ListContainers lists Docker containers.
// Corresponds to `docker ps`.
func (r *defaultRunner) ListContainers(ctx context.Context, c connector.Connector, all bool, filters map[string]string) ([]container.Container, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmdArgs := []string{"docker", "ps"}
	if all {
		cmdArgs = append(cmdArgs, "-a")
	}

	// --format "{{json .}}" is crucial for structured output.
	// Note: `docker ps --format json` is a simplified JSON.
	// The full `container.Container` type from moby/moby has more fields than `docker ps` typically outputs in its default JSON.
	// We will parse what's available and populate `container.Container` as best as possible.
	// Fields like NetworkSettings might be summaries or absent.
	cmdArgs = append(cmdArgs, "--format", "{{json .}}")

	if filters != nil {
		for key, value := range filters {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				return nil, errors.New("filter key and value cannot be empty")
			}
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout, // Listing containers should be quick
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list containers. Stderr: %s", string(stderr))
	}

	var containers []container.Container
	output := string(stdout)
	if strings.TrimSpace(output) == "" {
		return containers, nil // No containers found or empty output
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// The JSON output from `docker ps --format "{{json .}}"` is a custom format.
		// It's NOT directly unmarshallable into `moby/api/types/container.Container`.
		// We need an intermediate struct that matches the fields provided by the CLI format.
		var cliContainer struct {
			ID         string
			Image      string
			Command    string
			CreatedAt  string // Human-readable, e.g., "2 hours ago"
			RunningFor string // Human-readable, e.g., "2 hours" (present if running)
			Ports      string // e.g., "0.0.0.0:80->80/tcp, :::80->80/tcp"
			Status     string // e.g., "Up 2 hours" or "Exited (0) 5 minutes ago"
			Size       string // Only with --size, not default with json format typically
			Names      string
			Labels     string // Comma-separated key=value
			Mounts     string // Comma-separated mount sources
			Networks   string // Comma-separated network names
		}

		if err := json.Unmarshal([]byte(line), &cliContainer); err != nil {
			return nil, errors.Wrapf(err, "failed to parse container JSON line: %s", line)
		}

		// Now, map `cliContainer` to `moby/api/types/container.Container`. This will be an approximation.
		cont := container.Container{
			ID:      cliContainer.ID,
			Image:   cliContainer.Image,
			Command: cliContainer.Command,
			Status:  cliContainer.Status,
			Names:   strings.Split(cliContainer.Names, ","), // Names can be multiple for a container
			// Created: Needs parsing from CreatedAt string. `docker ps` CreatedAt is human-readable.
			//          `docker inspect` provides a proper timestamp. For `ps`, this is harder.
			// Ports: Needs parsing from string to []Port.
			// Labels: Needs parsing from string "k1=v1,k2=v2" to map[string]string.
			// Mounts: Needs parsing from string to []MountPoint.
			// NetworkSettings: Partially inferable from Networks and Ports.
		}

		// Parse Labels "label1=value1,label2=value2"
		if cliContainer.Labels != "" {
			cont.Labels = make(map[string]string)
			pairs := strings.Split(cliContainer.Labels, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					cont.Labels[kv[0]] = kv[1]
				}
			}
		}

		// Parse created time (this is a best effort from human-readable string)
		// `docker ps` output like "2 weeks ago" is not easily parsed into a timestamp.
		// `docker inspect <id> --format '{{.Created}}'` gives ISO8601.
		// For `ListContainers` via CLI, precise `Created` timestamp is hard. We'll leave it 0.
		// If `cliContainer.CreatedAt` was a parsable timestamp, we'd use it.
		// Example: `docker ps --format '{{.CreatedAt}}'` might give `2023-10-27 10:20:30 -0700 PDT`
		// but this format string is not standard across all Docker versions for `ps`.
		// The default JSON often has human-readable "X days ago".


		// Parse Ports string e.g., "0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp"
		// The `container.Port` type has IP, PrivatePort, PublicPort, Type.
		if cliContainer.Ports != "" {
			parsedPorts, err := parseDockerPorts(cliContainer.Ports)
			if err != nil {
				// Log or handle error if port parsing is critical
				// For now, we'll skip adding ports if parsing fails for a container
			}
			cont.Ports = parsedPorts
		}

		// HostConfig is usually minimal from `ps`
		cont.HostConfig = struct {
			NetworkMode string `json:",omitempty"`
		}{}
		if cliContainer.Networks != "" {
			// `docker ps` usually just gives network names, not full NetworkMode like "bridge"
			// This is an approximation.
			// cont.HostConfig.NetworkMode = cliContainer.Networks // This might be a comma-separated list
		}

		// NetworkSettings - very simplified from `ps` output
		if cliContainer.Networks != "" {
			cont.NetworkSettings = &container.SummaryNetworkSettings{
				Networks: make(map[string]*network.EndpointSettings),
			}
			networkNames := strings.Split(cliContainer.Networks, ",")
			for _, netName := range networkNames {
				if strings.TrimSpace(netName) != "" {
					cont.NetworkSettings.Networks[strings.TrimSpace(netName)] = &network.EndpointSettings{
						// IPAddress, Gateway etc. are not available from basic `ps` output.
					}
				}
			}
		}

		// Mounts - `docker ps --format "{{json .}}"` includes "Mounts" as a comma-separated list of source paths.
		// The `container.MountPoint` struct is richer (Destination, Mode, RW, etc.).
		// This is a simplified mapping.
		if cliContainer.Mounts != "" {
			mountSources := strings.Split(cliContainer.Mounts, ",")
			cont.Mounts = make([]mount.MountPoint, len(mountSources))
			for i, src := range mountSources {
				if strings.TrimSpace(src) != "" {
					cont.Mounts[i] = mount.MountPoint{
						Source: strings.TrimSpace(src),
						// Other fields like Destination, Driver, Mode, RW are not directly available from this `ps` output.
					}
				}
			}
		}


		containers = append(containers, cont)
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

	cmdArgs := []string{"docker", "logs"}

	if options.ShowStdout {
		// `docker logs` outputs stdout by default. This flag is more for API consistency.
		// No specific CLI flag for *only* stdout, but absence of --stderr implies it.
	}
	if options.ShowStderr {
		// `docker logs` outputs stderr by default. This flag is more for API consistency.
		// No specific CLI flag for *only* stderr. Both are usually multiplexed.
	}
	if options.Timestamps {
		cmdArgs = append(cmdArgs, "-t") // or --timestamps
	}
	if options.Follow {
		cmdArgs = append(cmdArgs, "-f") // or --follow
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

	// Timeout for logs can be tricky, especially with --follow.
	// For non-following logs, a reasonable timeout is good.
	// If --follow is used, the timeout essentially becomes a timeout for establishing the stream.
	// The caller would need to handle context cancellation to stop following.
	// For this CLI wrapper, if --follow is true, we might need a very long or configurable timeout.
	// Defaulting to a moderate timeout. User should use context for long-running `follow`.
	timeout := 1 * time.Minute
	if options.Follow {
		timeout = 24 * time.Hour // A very long timeout for follow, rely on ctx cancellation
	}


	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: timeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// `docker logs` can output to both stdout and stderr even on success (e.g. if container writes to stderr).
		// However, `connector.Exec` separates these. If `err` is not nil, it's a command execution error.
		// Stderr from the command itself (not container's stderr) will be in `stderr` from Exec.
		return "", errors.Wrapf(err, "failed to get logs for container %s. Stderr: %s", containerNameOrID, string(stderr))
	}

	// `docker logs` multiplexes stdout and stderr of the container into its own stdout stream.
	// If `connector.Exec`'s `stderr` is populated here with a nil error, it implies the `docker logs` command
	// itself wrote to stderr (e.g., warnings), not the container's stderr stream (which is in `stdout`).
	// This implementation returns the combined output from `docker logs` stdout.
	return string(stdout), nil
}

// GetContainerStats streams live resource usage statistics for a container.
// Corresponds to `docker stats`.
// Returns a channel that will stream stat objects. The caller must close the channel.
// The implementation would involve setting up a persistent command execution and parsing its output stream.
func (r *defaultRunner) GetContainerStats(ctx context.Context, c connector.Connector, containerNameOrID string, stream bool) (<-chan *container.StatsResponse, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return nil, errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "stats"}
	if !stream {
		cmdArgs = append(cmdArgs, "--no-stream")
	}
	cmdArgs = append(cmdArgs, "--format", "{{json .}}") // Output each stat as a JSON line
	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execTimeout := 15 * time.Second // Default for non-streaming
	if stream {
		// For streaming, the command runs until context is canceled or container stops.
		// The Exec timeout should be very long. Actual termination is via context.
		execTimeout = 24 * time.Hour // Effectively "infinite" for the command execution itself
	}

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: execTimeout,
		// IMPORTANT: This implementation assumes `c.Exec` can handle long-running commands
		// and that its output (stdout) can be processed line by line as it arrives.
		// If `c.Exec` buffers the entire output until the command finishes (or times out),
		// true streaming will not work. The channel would only receive data at the very end
		// or when the buffer flushes. A dedicated `ExecStream` method on the connector
		// that provides an io.ReadCloser for stdout would be ideal for true streaming.
	}

	statsChan := make(chan *container.StatsResponse)

	go func() {
		// It's crucial to close the channel when this goroutine exits, whether normally or due to error/cancellation.
		defer close(statsChan)

		// Execute the command.
		stdoutBytes, stderrBytes, err := c.Exec(ctx, cmd, execOptions)

		// Check for context cancellation first. This is the expected way to stop a stream.
		if ctx.Err() != nil {
			// Context was canceled (e.g., by the caller of GetContainerStats).
			// This is a normal termination for a stream, so no error is reported to the user of statsChan here.
			return
		}

		if err != nil {
			// An actual error occurred executing the command (not just context cancellation).
			// Log this error. Depending on desired behavior, one might send an error object
			// through a separate error channel if the API supported it, or simply log and close statsChan.
			// For this example, we'll assume logging is sufficient and statsChan will be closed by defer.
			// Consider using a structured logger if available.
			// fmt.Fprintf(os.Stderr, "Error executing docker stats for %s: %v. Stderr: %s\n", containerNameOrID, err, string(stderrBytes))
			return
		}

		if len(stderrBytes) > 0 {
			// The `docker stats` command itself might have written to its own stderr (e.g., warnings).
			// This doesn't necessarily mean the stream failed.
			// fmt.Fprintf(os.Stderr, "Docker stats command stderr for %s: %s\n", containerNameOrID, string(stderrBytes))
		}

		scanner := bufio.NewScanner(strings.NewReader(string(stdoutBytes)))
		for scanner.Scan() {
			// Periodically check context cancellation within the loop, especially if processing each line is quick.
			if ctx.Err() != nil {
				return
			}

			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			var cliStat struct {
				Name    string `json:"Name"`
				ID      string `json:"ID"`
				CPUPerc string `json:"CPUPerc"`
				MemUsage string `json:"MemUsage"`
				MemPerc string `json:"MemPerc"`
				NetIO   string `json:"NetIO"`
				BlockIO string `json:"BlockIO"`
				PIDs    string `json:"PIDs"`
			}

			if errUnmarshal := json.Unmarshal([]byte(line), &cliStat); errUnmarshal != nil {
				// Log error and skip this line
				// fmt.Fprintf(os.Stderr, "Failed to parse stats JSON line for %s: '%s', error: %v\n", containerNameOrID, line, errUnmarshal)
				continue
			}

			statsJSON := types.StatsJSON{
				Name: cliStat.Name,
				ID:   cliStat.ID,
				Read: time.Now(), // Timestamp of when the stat was processed.
				// PreRead is ideally the timestamp of the *previous* stat reading for accurate CPU delta calculation.
				// The Docker API client often manages this. For CLI parsing, it's harder without maintaining state
				// between calls to this function or within the stream. For simplicity, we can set PreRead to Read
				// or a zero value. If set to Read, CPU delta calculations might be incorrect or require assumptions.
				PreRead: time.Time{}, // Or statsJSON.Read if a placeholder is acceptable.
			}

			// --- Populate MemoryStats ---
			memParts := strings.Split(cliStat.MemUsage, " / ")
			if len(memParts) > 0 {
				usageStr := strings.TrimSpace(memParts[0])
				if val, errParseSize := parseDockerSize(usageStr); errParseSize == nil {
					statsJSON.MemoryStats.Usage = uint64(val)
				}
			}
			if len(memParts) > 1 {
				limitStr := strings.TrimSpace(memParts[1])
				if val, errParseSize := parseDockerSize(limitStr); errParseSize == nil {
					statsJSON.MemoryStats.Limit = uint64(val)
				}
			}
			// Example for MemPerc (Note: types.MemoryStats doesn't have a direct percent field)
			// memPercStr := strings.TrimSuffix(cliStat.MemPerc, "%")
			// if val, errParseFloat := strconv.ParseFloat(memPercStr, 64); errParseFloat == nil {
			//    // Store it in a custom field or calculate if needed: statsJSON.MemoryStats.Usage / statsJSON.MemoryStats.Limit
			// }


			// --- Populate CPUStats (Highly Simplified) ---
			// CPUPerc is a pre-calculated percentage from `docker stats`.
			// `types.CPUStats` expects raw usage counters (TotalUsage, SystemCpuUsage).
			// Directly mapping a percentage here is an approximation and not what the Docker API provides.
			// This part would need significant rework if strict API compatibility for CPU stats is required.
			cpuPercStr := strings.TrimSuffix(cliStat.CPUPerc, "%")
			if val, errParseFloat := strconv.ParseFloat(cpuPercStr, 64); errParseFloat == nil {
				// This is NOT how TotalUsage is meant to be used. It's a counter.
				// This is a placeholder to store the percentage value somewhere.
				statsJSON.CPUStats.CPUUsage.TotalUsage = uint64(val * 1e7) // Example scaling of percentage
			}
			// statsJSON.CPUStats.OnlineCPUs would ideally come from system info if needed for calculations.


			// --- Populate PidsStats ---
			if pids, errParseUint := strconv.ParseUint(cliStat.PIDs, 10, 64); errParseUint == nil {
				statsJSON.PidsStats.Current = pids
			}

			// --- BlkioStats and NetworkStats parsing would be similarly complex ---
			// BlkIO: "read / write" -> types.BlkioStats (array of BlkioStatEntry)
			// NetIO: "rx / tx" -> types.NetworkStats (map of interface name to NetworkStatsEntry)
			// These require more detailed parsing routines not included here for brevity.

			statsResp := &container.StatsResponse{
				Body:      nil, // Not used when parsing JSON lines directly
				OSType:    "linux", // Default assumption, could be "windows"
				StatsJSON: statsJSON,
			}
			// Ensure Read and PreRead are set on the embedded StatsJSON
			statsResp.StatsJSON.Read = statsJSON.Read // Already set above
			statsResp.StatsJSON.PreRead = statsJSON.PreRead // Already set above

			// Send the parsed stat to the channel.
			// This will block if the channel buffer is full and no receiver is ready.
			// It also respects context cancellation.
			select {
			case <-ctx.Done(): // Context was canceled by the caller.
				return // Exit goroutine.
			case statsChan <- statsResp:
				// Successfully sent.
			}

			// If not in streaming mode, we've sent one stat object, so exit the goroutine.
			if !stream {
				return
			}
		}

		if scanErr := scanner.Err(); scanErr != nil {
			// Error occurred during scanning (e.g., I/O error from the underlying reader if it were a true stream).
			// fmt.Fprintf(os.Stderr, "Error scanning docker stats output for %s: %v\n", containerNameOrID, scanErr)
		}
	}()

	return statsChan, nil
}

// InspectContainer returns low-level information on Docker objects (container, image, etc.)
// Corresponds to `docker inspect`.
// For now, this focuses on container inspect. A more generic version could inspect other types.
func (r *defaultRunner) InspectContainer(ctx context.Context, c connector.Connector, containerNameOrID string) (*container.InspectResponse, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return nil, errors.New("containerNameOrID cannot be empty")
	}

	// `docker inspect` outputs a JSON array if multiple objects are inspected,
	// or a single JSON object if one object is inspected.
	// We are inspecting one, so we expect a single JSON object, not an array.
	cmd := fmt.Sprintf("docker inspect %s", shellEscape(containerNameOrID))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// If stderr contains "No such object", it's a specific error we can check.
		if strings.Contains(string(stderr), "No such object") || strings.Contains(string(stderr), "no such container") {
			return nil, errors.Wrapf(connector.ErrNotFound, "container %s not found. Stderr: %s", containerNameOrID, string(stderr))
		}
		return nil, errors.Wrapf(err, "failed to inspect container %s. Stderr: %s", containerNameOrID, string(stderr))
	}

	output := string(stdout)
	if strings.TrimSpace(output) == "" {
		return nil, errors.New("docker inspect returned empty output")
	}

	// `docker inspect` returns a JSON array. We need to unmarshal into []container.InspectResponse
	// and then take the first element.
	var inspectResponses []container.InspectResponse // Note: container.InspectResponse is types.ContainerJSON
	if err := json.Unmarshal([]byte(output), &inspectResponses); err != nil {
		// Attempt to unmarshal as a single object if array unmarshal fails.
		// This can happen if Docker's output format changes or for very old versions.
		var singleResponse container.InspectResponse
		if errSingle := json.Unmarshal([]byte(output), &singleResponse); errSingle != nil {
			return nil, errors.Wrapf(err, "failed to parse docker inspect output as JSON array (first attempt) or object (second attempt: %v) for %s. Output: %s", errSingle, containerNameOrID, output)
		}
		// If single object unmarshal succeeded, use it.
		if singleResponse.ID == "" && singleResponse.Name == "" { // Basic check if it's a valid-looking response
			return nil, errors.Wrapf(err, "failed to parse docker inspect output for %s (parsed as single object but seems invalid). Output: %s", containerNameOrID, output)
		}
		return &singleResponse, nil
	}

	if len(inspectResponses) == 0 {
		return nil, errors.Errorf("docker inspect for %s returned an empty JSON array", containerNameOrID)
	}
	if len(inspectResponses) > 1 {
		// This shouldn't happen if we inspect by specific ID/name, but good to be aware.
		// log.Printf("Warning: docker inspect for %s returned multiple results (%d), using the first one.", containerNameOrID, len(inspectResponses))
	}

	return &inspectResponses[0], nil
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
		Timeout: DefaultDockerStartTimeout, // Pausing should be quick
	}

	// `docker pause` outputs the container name/ID on success.
	// Stderr might contain errors like "container already paused" or "No such container".
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Check if already paused: Docker CLI might return error or specific stderr message.
		// e.g., "Error response from daemon: Container <id> is already paused"
		// For simplicity, we treat any error from Exec as a failure to pause,
		// but specific error parsing could be added.
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
		Timeout: DefaultDockerStartTimeout, // Unpausing should be quick
	}

	// `docker unpause` outputs the container name/ID on success.
	// Stderr might contain errors like "container not paused" or "No such container".
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Similar to pause, specific error checking for "not paused" could be added.
		return errors.Wrapf(err, "failed to unpause container %s. Stderr: %s", containerNameOrID, string(stderr))
	}
	return nil
}

// ExecInContainer executes a command inside a running container.
// Corresponds to `docker exec`.
func (r *defaultRunner) ExecInContainer(ctx context.Context, c connector.Connector, containerNameOrID string, cmdToExec []string, user, workDir string, tty bool) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return "", errors.New("containerNameOrID cannot be empty")
	}
	if len(cmdToExec) == 0 {
		return "", errors.New("command to execute cannot be empty")
	}

	cmdArgs := []string{"docker", "exec"}

	// Options for docker exec:
	if tty {
		// Note: `docker exec -t` (TTY) often implies `-i` (interactive) as well for useful terminal interaction.
		// However, `-i` with a non-interactive connector might hang if the command expects input.
		// For a simple exec returning output, `-t` might affect output formatting (e.g. line endings).
		// If true interactivity is needed, the connector itself would need to support PTY.
		// For now, we'll add `-t` if requested, but be mindful of its implications.
		// If the executed command doesn't behave well without a "real" TTY, this might be problematic.
		// Many commands run fine without a TTY.
		cmdArgs = append(cmdArgs, "-t") // or "--tty"
	}
	if strings.TrimSpace(user) != "" {
		cmdArgs = append(cmdArgs, "-u", shellEscape(user)) // or "--user"
	}
	if strings.TrimSpace(workDir) != "" {
		cmdArgs = append(cmdArgs, "-w", shellEscape(workDir)) // or "--workdir"
	}
	// TODO: Add support for environment variables (-e or --env) if needed.

	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	for _, part := range cmdToExec {
		cmdArgs = append(cmdArgs, shellEscape(part))
	}
	cmd := strings.Join(cmdArgs, " ")

	// Timeout for exec can vary greatly depending on the command.
	// Using a generic moderate timeout. Caller should use context for long-running execs.
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 5 * time.Minute, // Adjust as necessary, or make configurable
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// stderr from c.Exec will contain stderr from the `docker exec` command itself,
		// or from the executed command within the container.
		return string(stdout), errors.Wrapf(err, "failed to execute command in container %s. Stderr: %s", containerNameOrID, string(stderr))
	}

	// `docker exec` output: stdout contains the stdout of the executed command.
	// stderr from `c.Exec` contains the stderr of the executed command.
	// If `docker exec` itself fails (e.g. container not found), `err` will be non-nil.
	// If the command in the container exits non-zero, `docker exec` also exits non-zero, so `err` will be non-nil.
	// It's useful to return both stdout and stderr from the command.
	// The current function signature only returns one string (intended for stdout).
	// We might need to reconsider how to return stderr of the executed command if it's important.
	// For now, stdout is returned on success. If err is nil but stderr is populated, it means the command ran
	// successfully (exit 0) but produced stderr output. This is common.
	// We should combine stdout and stderr if error is nil but stderr is present.
	// However, the current `connector.Exec` contract is that `err` is non-nil if the command had non-zero exit.
	// So, if `err` is nil, `stderr` from `c.Exec` should ideally be empty or only contain benign warnings from `docker` itself.
	// The primary output of the command is `stdout`.

	// If there was stderr from the command execution (even with exit code 0),
	// it might be desirable to include it in the error or log it.
	// For now, just return stdout. If `err` is non-nil, stderr is already part of the error message.
	return string(stdout), nil
}


// --- Docker Network Methods ---

// CreateDockerNetwork creates a new Docker network.
// Corresponds to `docker network create`.
func (r *defaultRunner) CreateDockerNetwork(ctx context.Context, c connector.Connector, name, driver, subnet, gateway string, labels map[string]string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("network name cannot be empty")
	}

	cmdArgs := []string{"docker", "network", "create"}

	if strings.TrimSpace(driver) != "" {
		cmdArgs = append(cmdArgs, "--driver", shellEscape(driver))
	}
	if strings.TrimSpace(subnet) != "" {
		cmdArgs = append(cmdArgs, "--subnet", shellEscape(subnet))
	}
	if strings.TrimSpace(gateway) != "" {
		cmdArgs = append(cmdArgs, "--gateway", shellEscape(gateway))
	}

	if labels != nil {
		for key, value := range labels {
			if strings.TrimSpace(key) == "" { // Label key cannot be empty
				return errors.New("label key cannot be empty")
			}
			cmdArgs = append(cmdArgs, "--label", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	cmdArgs = append(cmdArgs, shellEscape(name))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 1 * time.Minute, // Network creation should be relatively quick
	}

	// `docker network create` outputs the network ID (or name if ID is not easily retrieved by CLI) on success.
	// Stderr might contain errors like "network already exists".
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Example: "Error response from daemon: network with name my-net already exists"
		if strings.Contains(string(stderr), "already exists") {
			// Consider if this should be an error or handled as success (idempotency).
			// For strict "create", it's an error if it exists.
			// If idempotent behavior is desired, check existence first or parse this error.
		}
		return errors.Wrapf(err, "failed to create docker network %s. Stderr: %s", name, string(stderr))
	}
	return nil
}

// RemoveDockerNetwork removes a Docker network.
// Corresponds to `docker network rm`.
func (r *defaultRunner) RemoveDockerNetwork(ctx context.Context, c connector.Connector, networkIDOrName string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(networkIDOrName) == "" {
		return errors.New("networkIDOrName cannot be empty")
	}

	cmd := fmt.Sprintf("docker network rm %s", shellEscape(networkIDOrName))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 30 * time.Second, // Network removal should be quick
	}

	// `docker network rm` outputs the network name/ID on success.
	// Stderr might contain errors like "network not found".
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Example: "Error: No such network: my-nonexistent-net"
		if strings.Contains(string(stderr), "No such network") {
			// If idempotent behavior is desired (i.e., not an error if already gone),
			// this specific error could be ignored or handled differently.
			// For strict "remove", it's an error if not found.
		}
		return errors.Wrapf(err, "failed to remove docker network %s. Stderr: %s", networkIDOrName, string(stderr))
	}
	return nil
}

// ListDockerNetworks lists Docker networks.
// Corresponds to `docker network ls`.
func (r *defaultRunner) ListDockerNetworks(ctx context.Context, c connector.Connector, filters map[string]string) ([]NetworkResource, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmdArgs := []string{"docker", "network", "ls"}
	// Use --format "{{json .}}" for structured output
	cmdArgs = append(cmdArgs, "--format", "{{json .}}")

	if filters != nil {
		for key, value := range filters {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				return nil, errors.New("filter key and value cannot be empty")
			}
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout, // Listing networks should be quick
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list docker networks. Stderr: %s", string(stderr))
	}

	var networks []NetworkResource
	output := string(stdout)
	if strings.TrimSpace(output) == "" {
		return networks, nil // No networks found or empty output
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// The JSON output from `docker network ls --format "{{json .}}"` is a custom format.
		// We need an intermediate struct that matches the fields provided by this CLI format.
		var cliNetwork struct {
			ID         string
			Name       string
			Driver     string
			Scope      string
			IPv6       string // "true" or "false"
			Internal   string // "true" or "false"
			Attachable string // "true" or "false"
			Ingress    string // "true" or "false"
			ConfigOnly string // "true" or "false"
			// CreatedAt  string // `docker network ls` json format usually doesn't include CreatedAt directly.
			Labels     string // Comma-separated key=value
		}

		if err := json.Unmarshal([]byte(line), &cliNetwork); err != nil {
			return nil, errors.Wrapf(err, "failed to parse network JSON line: %s", line)
		}

		// Map `cliNetwork` to `NetworkResource`.
		netRes := NetworkResource{
			ID:         cliNetwork.ID,
			Name:       cliNetwork.Name,
			Driver:     cliNetwork.Driver,
			Scope:      cliNetwork.Scope,
			EnableIPv6: strings.ToLower(cliNetwork.IPv6) == "true",
			Internal:   strings.ToLower(cliNetwork.Internal) == "true",
			Attachable: strings.ToLower(cliNetwork.Attachable) == "true",
			Ingress:    strings.ToLower(cliNetwork.Ingress) == "true",
			ConfigOnly: strings.ToLower(cliNetwork.ConfigOnly) == "true",
			Options:    make(map[string]string), // `ls` format doesn't provide these, inspect does.
			Labels:     make(map[string]string),
			// Created: `docker network ls --format json` doesn't typically provide a parsable Created timestamp.
			//          `docker network inspect <id>` would. Set to zero time.
			Created: time.Time{},
		}

		// Parse Labels "label1=value1,label2=value2"
		if cliNetwork.Labels != "" {
			pairs := strings.Split(cliNetwork.Labels, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 && strings.TrimSpace(kv[0]) != "" {
					netRes.Labels[strings.TrimSpace(kv[0])] = kv[1]
				}
			}
		}
		networks = append(networks, netRes)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker network ls output")
	}

	return networks, nil
}


// ConnectContainerToNetwork connects a container to a Docker network.
// Corresponds to `docker network connect`.
func (r *defaultRunner) ConnectContainerToNetwork(ctx context.Context, c connector.Connector, networkIDOrName, containerNameOrID, ipAddress string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(networkIDOrName) == "" {
		return errors.New("networkIDOrName cannot be empty")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "network", "connect"}
	if strings.TrimSpace(ipAddress) != "" {
		cmdArgs = append(cmdArgs, "--ip", shellEscape(ipAddress))
	}
	// TODO: Add other options like --alias if needed.

	cmdArgs = append(cmdArgs, shellEscape(networkIDOrName))
	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 1 * time.Minute, // Connecting network should be relatively quick
	}

	// `docker network connect` doesn't typically output on success, but might on failure.
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Example errors:
		// "Error: No such container: <container>"
		// "Error: No such network: <network>"
		// "Error response from daemon: container <container_id> is already connected to network <network_name>"
		if strings.Contains(string(stderr), "already connected") {
			// Consider if this should be an error or handled as success (idempotency).
		}
		return errors.Wrapf(err, "failed to connect container %s to network %s. Stderr: %s", containerNameOrID, networkIDOrName, string(stderr))
	}
	return nil
}

// DisconnectContainerFromNetwork disconnects a container from a Docker network.
// Corresponds to `docker network disconnect`.
func (r *defaultRunner) DisconnectContainerFromNetwork(ctx context.Context, c connector.Connector, networkIDOrName, containerNameOrID string, force bool) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(networkIDOrName) == "" {
		return errors.New("networkIDOrName cannot be empty")
	}
	if strings.TrimSpace(containerNameOrID) == "" {
		return errors.New("containerNameOrID cannot be empty")
	}

	cmdArgs := []string{"docker", "network", "disconnect"}
	if force {
		cmdArgs = append(cmdArgs, "-f") // or --force
	}

	cmdArgs = append(cmdArgs, shellEscape(networkIDOrName))
	cmdArgs = append(cmdArgs, shellEscape(containerNameOrID))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 1 * time.Minute, // Disconnecting network should be quick
	}

	// `docker network disconnect` doesn't typically output on success.
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Example errors:
		// "Error: No such container: <container>"
		// "Error: No such network: <network>"
		// "Error response from daemon: container <container_id> is not connected to network <network_name>"
		if strings.Contains(string(stderr), "is not connected") {
			// Consider if this should be an error or handled as success (idempotency).
		}
		return errors.Wrapf(err, "failed to disconnect container %s from network %s. Stderr: %s", containerNameOrID, networkIDOrName, string(stderr))
	}
	return nil
}


// --- Docker Volume Methods ---

// CreateDockerVolume creates a new Docker volume.
// Corresponds to `docker volume create`.
func (r *defaultRunner) CreateDockerVolume(ctx context.Context, c connector.Connector, name, driver string, driverOpts, labels map[string]string) error {
	if c == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(name) == "" {
		// Docker allows unnamed volumes, but our API might require a name for management.
		// If unnamed volumes are to be supported, this check needs adjustment,
		// and the command would be just `docker volume create [OPTIONS]`.
		// For now, assume a name is required by this function's contract.
		return errors.New("volume name cannot be empty")
	}

	cmdArgs := []string{"docker", "volume", "create"}

	if strings.TrimSpace(driver) != "" {
		cmdArgs = append(cmdArgs, "--driver", shellEscape(driver))
	}

	if driverOpts != nil {
		for key, value := range driverOpts {
			if strings.TrimSpace(key) == "" {
				return errors.New("driver option key cannot be empty")
			}
			// Value can be empty for certain driver options.
			cmdArgs = append(cmdArgs, "--opt", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	if labels != nil {
		for key, value := range labels {
			if strings.TrimSpace(key) == "" {
				return errors.New("label key cannot be empty")
			}
			cmdArgs = append(cmdArgs, "--label", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	cmdArgs = append(cmdArgs, shellEscape(name)) // Volume name is the last argument
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 1 * time.Minute, // Volume creation should be quick
	}

	// `docker volume create` outputs the volume name on success.
	// Stderr might contain errors like "volume already exists".
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Example: "Error response from daemon: a volume with the name my-vol already exists"
		if strings.Contains(string(stderr), "already exists") {
			// Consider if "already exists" should be treated as success (idempotency).
		}
		return errors.Wrapf(err, "failed to create docker volume %s. Stderr: %s", name, string(stderr))
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

	cmdArgs := []string{"docker", "volume", "rm"}
	if force {
		cmdArgs = append(cmdArgs, "-f") // or --force
	}
	cmdArgs = append(cmdArgs, shellEscape(volumeName))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 30 * time.Second, // Volume removal should be quick
	}

	// `docker volume rm` outputs the volume name on success.
	// Stderr might contain errors like "volume in use" or "no such volume".
	_, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Example: "Error: No such volume: my-nonexistent-vol"
		// Example: "Error response from daemon: remove my-inuse-vol: volume is in use - [container_id_using_it]"
		if strings.Contains(string(stderr), "No such volume") {
			// Idempotency: if it's already gone, that might be okay.
		} else if strings.Contains(string(stderr), "volume is in use") && !force {
			// Specific error for "in use" if not forcing.
		}
		return errors.Wrapf(err, "failed to remove docker volume %s. Stderr: %s", volumeName, string(stderr))
	}
	return nil
}

// ListDockerVolumes lists Docker volumes.
// Corresponds to `docker volume ls`.
func (r *defaultRunner) ListDockerVolumes(ctx context.Context, c connector.Connector, filters map[string]string) ([]*Volume, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmdArgs := []string{"docker", "volume", "ls"}
	cmdArgs = append(cmdArgs, "--format", "{{json .}}") // Crucial for structured output

	if filters != nil {
		for key, value := range filters {
			if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
				return nil, errors.New("filter key and value cannot be empty")
			}
			// Common filters: "dangling=true", "driver=local", "label=key=value", "name=partial_name"
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout, // Listing volumes should be quick
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list docker volumes. Stderr: %s", string(stderr))
	}

	var volumes []*Volume
	output := string(stdout)
	if strings.TrimSpace(output) == "" {
		return volumes, nil // No volumes found or empty output
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		// The JSON output from `docker volume ls --format "{{json .}}"` is simpler than `docker volume inspect`.
		// We need an intermediate struct for the CLI JSON.
		var cliVolume struct {
			Name       string // Volume name
			Driver     string
			Labels     string // Comma-separated key=value
			Mountpoint string
			Scope      string // Not always present in basic `ls` JSON, more common in inspect
			Size       string // Only with --format "{{json .Size}}" explicitly, not default
			// CreatedAt, Options are generally not in `ls` output.
		}

		if err := json.Unmarshal([]byte(line), &cliVolume); err != nil {
			return nil, errors.Wrapf(err, "failed to parse volume JSON line: %s", line)
		}

		// Map `cliVolume` to our `Volume` type.
		vol := &Volume{
			Name:       cliVolume.Name,
			Driver:     cliVolume.Driver,
			Mountpoint: cliVolume.Mountpoint,
			Scope:      cliVolume.Scope, // Will be empty if not provided by `ls`
			Labels:     make(map[string]string),
			Options:    make(map[string]string), // `ls` doesn't give options
			// CreatedAt: Not available from `ls --format json`. Set to empty or zero time.
		}

		if cliVolume.Labels != "" {
			pairs := strings.Split(cliVolume.Labels, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 && strings.TrimSpace(kv[0]) != "" {
					vol.Labels[strings.TrimSpace(kv[0])] = kv[1]
				}
			}
		}
		// Note: `cliVolume.Size` if available from format, would need parsing with `parseDockerSize`.
		// However, the default `{{json .}}` for `volume ls` usually doesn't include Size.
		// If `Volume` struct had a size field, it would be populated here if `cliVolume.Size` was present.

		volumes = append(volumes, vol)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "error reading docker volume ls output")
	}

	return volumes, nil
}

// InspectDockerVolume returns information about a Docker volume.
// Corresponds to `docker volume inspect`.
func (r *defaultRunner) InspectDockerVolume(ctx context.Context, c connector.Connector, volumeName string) (*Volume, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(volumeName) == "" {
		return nil, errors.New("volumeName cannot be empty")
	}

	// `docker volume inspect` outputs a JSON array if multiple volumes are inspected,
	// or a single JSON object if one is inspected by name.
	cmd := fmt.Sprintf("docker volume inspect %s", shellEscape(volumeName))
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: DefaultDockerInspectTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Example: "Error: No such volume: my-nonexistent-vol"
		if strings.Contains(string(stderr), "No such volume") {
			return nil, errors.Wrapf(connector.ErrNotFound, "volume %s not found. Stderr: %s", volumeName, string(stderr))
		}
		return nil, errors.Wrapf(err, "failed to inspect docker volume %s. Stderr: %s", volumeName, string(stderr))
	}

	output := string(stdout)
	if strings.TrimSpace(output) == "" {
		return nil, errors.New("docker volume inspect returned empty output")
	}

	// `docker volume inspect <name>` returns a JSON array with a single element (or more if ambiguous name matches multiple).
	// We'll parse as an array and take the first. If only one volume is ever expected for a given name,
	// and Docker guarantees a single object JSON for unique name inspect, this could be simplified.
	// However, the API docs for `VolumeInspect` (which CLI mirrors) return `types.Volume`.
	// Let's assume the CLI for a single named volume might return a single object or an array of one.
	// The `Volume` struct we defined is based on `moby/api/types/volume.Volume`.

	var inspectResponses []*Volume // Expecting an array from inspect
	if err := json.Unmarshal([]byte(output), &inspectResponses); err != nil {
		// Fallback: try to unmarshal as a single object if array unmarshal fails.
		var singleResponse Volume
		if errSingle := json.Unmarshal([]byte(output), &singleResponse); errSingle != nil {
			return nil, errors.Wrapf(err, "failed to parse docker volume inspect output as JSON array or object for %s. Output: %s. Single object error: %v", volumeName, output, errSingle)
		}
		// Basic validation for single object unmarshal
		if singleResponse.Name == "" && singleResponse.Driver == "" { // If it's an empty struct essentially
			return nil, errors.Wrapf(err, "failed to parse docker volume inspect output for %s (parsed as single object but seems invalid). Output: %s", volumeName, output)
		}
		return &singleResponse, nil
	}

	if len(inspectResponses) == 0 {
		// This case implies the command succeeded but returned an empty JSON array, which is unusual for a named inspect.
		// More likely, "No such volume" would be an error from the command itself.
		return nil, errors.Errorf("docker volume inspect for %s returned an empty JSON array, though command succeeded", volumeName)
	}
	// If multiple volumes match a partial name, `docker volume inspect` might list them all.
	// For this function, we assume `volumeName` is specific enough to refer to one, or we take the first.
	if len(inspectResponses) > 1 {
		// log.Printf("Warning: docker volume inspect for %s returned multiple results (%d), using the first one.", volumeName, len(inspectResponses))
	}

	return inspectResponses[0], nil
}


// --- Docker System Methods ---

// DockerInfo displays system-wide information about Docker.
// Corresponds to `docker info`.
func (r *defaultRunner) DockerInfo(ctx context.Context, c connector.Connector) (*SystemInfo, error) {
	if c == nil {
		return nil, errors.New("connector cannot be nil")
	}

	// `docker info --format "{{json .}}"` provides structured output.
	cmd := "docker info --format {{json .}}"
	execOptions := &connector.ExecOptions{
		Sudo:    true, // `docker info` might require sudo depending on setup
		Timeout: DefaultDockerInspectTimeout,
	}

	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get docker info. Stderr: %s", string(stderr))
	}

	output := string(stdout)
	if strings.TrimSpace(output) == "" {
		return nil, errors.New("docker info returned empty output")
	}

	// The SystemInfo struct is a simplified version of `moby/api/types/info.Info`.
	// The JSON from `docker info --format {{json .}}` should be directly unmarshallable
	// into a struct that mirrors the fields provided by that JSON output.
	// This might be slightly different from `moby/api/types/info.Info` if our SystemInfo is simpler.
	// We will unmarshal into our local SystemInfo struct.
	var info SystemInfo
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		// It's possible the direct unmarshal to SystemInfo fails if the JSON structure from CLI
		// doesn't perfectly match our simplified SystemInfo (e.g. field type mismatches like int vs string for counts).
		// A more robust approach might unmarshal into map[string]interface{} or a more flexible intermediate struct first,
		// then selectively populate SystemInfo.
		// For now, assume direct unmarshal works or needs adjustment in SystemInfo struct.
		// Example: If 'Images' field in JSON is string "5 images" but SystemInfo.Images is int.
		// This needs careful alignment of SystemInfo struct with actual JSON output.

		// Let's try unmarshalling into a more generic map to inspect, then populate.
		var rawInfo map[string]interface{}
		if errRaw := json.Unmarshal([]byte(output), &rawInfo); errRaw != nil {
			return nil, errors.Wrapf(err, "failed to parse docker info JSON output (tried direct and raw map): %s. Raw map error: %v", output, errRaw)
		}

		// Helper to safely convert map interface{} values to int or int64
		toInt := func(val interface{}) int {
			switch v := val.(type) {
			case float64: // JSON numbers are often float64
				return int(v)
			case int:
				return v
			case int32:
				return int(v)
			case int64:
				return int(v)
			case string: // Attempt to parse if it's a string number
				i, _ := strconv.Atoi(v)
				return i
			default:
				return 0
			}
		}
		toInt64 := func(val interface{}) int64 {
			switch v := val.(type) {
			case float64:
				return int64(v)
			case int:
				return int64(v)
			case int32:
				return int64(v)
			case int64:
				return v
			case string:
				i, _ := strconv.ParseInt(v, 10, 64)
				return i
			default:
				return 0
			}
		}
		toString := func(val interface{}) string {
			if s, ok := val.(string); ok {
				return s
			}
			return ""
		}

		// Populate SystemInfo from rawInfo map
		info.ID = toString(rawInfo["ID"])
		info.Containers = toInt(rawInfo["Containers"])
		info.ContainersRunning = toInt(rawInfo["ContainersRunning"])
		info.ContainersPaused = toInt(rawInfo["ContainersPaused"])
		info.ContainersStopped = toInt(rawInfo["ContainersStopped"])
		info.Images = toInt(rawInfo["Images"])
		info.ServerVersion = toString(rawInfo["ServerVersion"])
		info.StorageDriver = toString(rawInfo["Driver"]) // Docker info JSON uses "Driver" for StorageDriver
		info.LoggingDriver = toString(rawInfo["LoggingDriver"])
		info.CgroupDriver = toString(rawInfo["CgroupDriver"])
		info.CgroupVersion = toString(rawInfo["CgroupVersion"])
		info.KernelVersion = toString(rawInfo["KernelVersion"])
		info.OperatingSystem = toString(rawInfo["OperatingSystem"])
		info.OSVersion = toString(rawInfo["OSVersion"])
		info.OSType = toString(rawInfo["OSType"])
		info.Architecture = toString(rawInfo["Architecture"])
		info.MemTotal = toInt64(rawInfo["MemTotal"])
		// Add other fields as necessary, ensuring SystemInfo struct has them.
	}


	return &info, nil
}

// DockerPrune removes unused Docker data (containers, networks, images, build cache).
// Corresponds to `docker system prune`.
func (r *defaultRunner) DockerPrune(ctx context.Context, c connector.Connector, pruneType string, filters map[string]string, all bool) (string, error) {
	if c == nil {
		return "", errors.New("connector cannot be nil")
	}

	var cmdArgs []string
	// Base command can be `docker system prune` or more specific like `docker image prune`
	switch strings.ToLower(pruneType) {
	case "system", "": // Default to system prune
		cmdArgs = append(cmdArgs, "docker", "system", "prune")
	case "image", "images":
		cmdArgs = append(cmdArgs, "docker", "image", "prune")
	case "container", "containers":
		cmdArgs = append(cmdArgs, "docker", "container", "prune")
	case "network", "networks":
		cmdArgs = append(cmdArgs, "docker", "network", "prune")
	case "volume", "volumes":
		// Note: `docker volume prune` is destructive and might require specific confirmation
		// if not using -f. Our runner always uses -f for prune operations.
		cmdArgs = append(cmdArgs, "docker", "volume", "prune")
	case "builder", "build":
		cmdArgs = append(cmdArgs, "docker", "builder", "prune")
	default:
		return "", errors.Errorf("unsupported pruneType: %s. Supported types: system, image, container, network, volume, builder", pruneType)
	}

	if all {
		// `-a` or `--all` is not applicable to `docker volume prune` or `docker builder prune` in the same way.
		// `docker system prune -a` removes all unused images, not just dangling ones.
		// `docker image prune -a` removes all unused images, not just dangling ones.
		// `docker builder prune --all` removes all build cache, not just dangling.
		// `docker volume prune` prunes all unused local volumes by default, `-a` is not a flag for it.
		// For simplicity, we'll add `-a` if `all` is true and the specific prune command supports it.
		// This might need refinement based on exact command behavior.
		if pruneType == "system" || pruneType == "" || pruneType == "image" || pruneType == "images" || pruneType == "builder" || pruneType == "build" {
			cmdArgs = append(cmdArgs, "-a")
		}
	}

	// Add --force to bypass interactive confirmation. This is generally expected for automated runners.
	cmdArgs = append(cmdArgs, "-f")

	if filters != nil {
		// Filters are primarily for `image prune`, `container prune`, `network prune`.
		// `system prune` also supports `--filter "until=..."`
		// `volume prune` supports `--filter "label..."`
		// `builder prune` supports `--filter "until=..."`
		// This needs to be context-aware of `pruneType`.
		// Example: `docker image prune -a -f --filter "until=24h"`
		// Example: `docker volume prune -f --filter "label!=keep"`
		for key, value := range filters {
			if strings.TrimSpace(key) == "" { // Value can be empty for some filters e.g. "label!=key"
				return "", errors.New("filter key cannot be empty")
			}
			cmdArgs = append(cmdArgs, "--filter", shellEscape(fmt.Sprintf("%s=%s", key, value)))
		}
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{
		Sudo:    true,
		Timeout: 10 * time.Minute, // Pruning can take time, especially with many objects
	}

	// `docker system prune` and variants output a summary of reclaimed space.
	stdout, stderr, err := c.Exec(ctx, cmd, execOptions)
	if err != nil {
		return string(stdout), errors.Wrapf(err, "failed to prune docker %s. Stderr: %s. Stdout: %s", pruneType, string(stderr), string(stdout))
	}

	// Return the stdout which usually contains information about reclaimed space.
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
