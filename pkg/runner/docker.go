package runner

import (
	"context"
	"encoding/json" // Added for json.Unmarshal
	"errors"        // Added for errors.As
	"fmt"
	"strconv" // Added for strconv.ParseFloat
	"strings"   // Added for strings.TrimSpace
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// --- Docker Methods ---

// PullImage pulls a Docker image from a registry.
// It assumes Docker is installed and the daemon is running on the host.
// Sudo is used by default for Docker commands.
func (r *defaultRunner) PullImage(ctx context.Context, conn connector.Connector, imageName string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for PullImage")
	}
	if strings.TrimSpace(imageName) == "" {
		return fmt.Errorf("imageName cannot be empty for PullImage")
	}

	// Docker pull can take a significant amount of time depending on image size and network.
	// A default timeout is set here, but could be overridden by options if the method signature supported it.
	// For now, using a fixed default timeout or relying on context cancellation.
	// Let's use a default of 15 minutes for image pulls.
	cmdTimeout := 15 * time.Minute // Default timeout for docker pull

	// Check if the context has a deadline that is shorter than our default.
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < cmdTimeout {
			cmdTimeout = time.Until(deadline)
			if cmdTimeout < 0 { // If deadline already passed
				cmdTimeout = 0 // Will cause immediate timeout if not handled by RunWithOptions correctly
			}
		}
	}

	// The command itself
	cmd := fmt.Sprintf("docker pull %s", shellEscape(imageName))

	// RunWithOptions handles sudo and timeout.
	// Stdout from `docker pull` is usually empty on success. Progress is on stderr.
	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{
		Sudo:    true,
		Timeout: cmdTimeout,
	})

	if err != nil {
		// Include stderr in the error message as it often contains useful diagnostic info from Docker.
		return fmt.Errorf("failed to pull image '%s': %w. Stderr: %s", imageName, err, string(stderr))
	}

	// `docker pull` is generally idempotent; pulling an already existing image
	// will result in messages like "Image is up to date" on stderr and a 0 exit code.
	// No specific success message parsing is typically needed beyond checking the error.
	return nil
}

// ImageExists checks if a Docker image with the given name (e.g., "ubuntu:latest") exists locally.
// Sudo is used by default for Docker commands.
func (r *defaultRunner) ImageExists(ctx context.Context, conn connector.Connector, imageName string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil for ImageExists")
	}
	if strings.TrimSpace(imageName) == "" {
		return false, fmt.Errorf("imageName cannot be empty for ImageExists")
	}

	// `docker image inspect` is a reliable way to check if an image exists.
	// It exits with 0 if the image is found, and 1 if not found.
	// We suppress stdout as we only care about the exit code and stderr for actual errors.
	cmd := fmt.Sprintf("docker image inspect %s > /dev/null 2>&1", shellEscape(imageName))

	// Use a reasonable timeout for a metadata check command.
	cmdTimeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < cmdTimeout {
			cmdTimeout = time.Until(deadline)
			if cmdTimeout < 0 { cmdTimeout = 0 }
		}
	}

	_, stderrBytes, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{
		Sudo:    true,
		Timeout: cmdTimeout,
	})

	if err == nil {
		return true, nil // Command succeeded, image exists
	}

	// Check if the error is due to the image not being found.
	// Docker inspect for a non-existent image typically returns exit code 1.
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 {
			// Further check stderr if needed, though exit code 1 is often sufficient
			// e.g., strings.Contains(strings.ToLower(string(stderrBytes)), "no such image")
			return false, nil // Image does not exist
		}
	}

	// For any other error, return it
	return false, fmt.Errorf("failed to check if image '%s' exists: %w. Stderr: %s", imageName, err, string(stderrBytes))
}

// parseDockerSize parses a Docker size string (e.g., "1.23GB", "7.34MB", "500B") into int64 bytes.
func parseDockerSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))
	var multiplier float64 = 1.0

	if strings.HasSuffix(sizeStr, "KB") { // Docker uses KB for Kilobytes, not KiB
		multiplier = 1000
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1000 * 1000
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1000 * 1000 * 1000
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "TB") {
		multiplier = 1000 * 1000 * 1000 * 1000
		sizeStr = strings.TrimSuffix(sizeStr, "TB")
	} else if strings.HasSuffix(sizeStr, "B") {
		multiplier = 1
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	}
	// KiB, MiB, GiB for binary prefixes if Docker ever outputs these (uncommon for `docker images`)
	// For consistency, `docker inspect` often shows bytes directly. `docker images` is human-readable.
	// If docker changes to KiB/MiB, this will need adjustment. For now, assuming decimal KB/MB/GB.

	val, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size value from '%s': %w", sizeStr, err)
	}

	return int64(val * multiplier), nil
}


// ListImages lists Docker images available locally on the host.
// Sudo is used by default for Docker commands.
func (r *defaultRunner) ListImages(ctx context.Context, conn connector.Connector, all bool) ([]ImageInfo, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil for ListImages")
	}

	cmdArgs := []string{"docker", "images"}
	if all {
		cmdArgs = append(cmdArgs, "--all")
	}
	// Using JSON format for easier and more robust parsing.
	// Each image will be printed as a JSON object on a new line.
	cmdArgs = append(cmdArgs, "--format", "{{json .}}")
	cmd := strings.Join(cmdArgs, " ")

	cmdTimeout := 1 * time.Minute // Listing images should be relatively quick.
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < cmdTimeout {
			cmdTimeout = time.Until(deadline)
			if cmdTimeout < 0 { cmdTimeout = 0}
		}
	}

	stdoutBytes, stderrBytes, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{
		Sudo:    true,
		Timeout: cmdTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w. Stderr: %s", err, string(stderrBytes))
	}

	var images []ImageInfo
	outputLines := strings.Split(strings.TrimSpace(string(stdoutBytes)), "\n")

	// Temporary struct to unmarshal the JSON output from `docker images --format "{{json .}}"`.
	// Fields are based on typical output of this command.
	type dockerImageJSON struct {
		ID           string `json:"ID"`
		Repository   string `json:"Repository"`
		Tag          string `json:"Tag"`
		Digest       string `json:"Digest"` // Not directly in ImageInfo but useful for context
		CreatedSince string `json:"CreatedSince"` // e.g., "About an hour ago"
		CreatedAt    string `json:"CreatedAt"`    // e.g., "2023-10-27 18:42:02 +0000 UTC"
		Size         string `json:"Size"`         // Human-readable, e.g., "1.23GB"
		// Docker's json output for `images` doesn't have VirtualSize consistently,
		// it's often the same as Size or N/A. We'll use Size for both if VirtualSize isn't distinct.
	}

	for _, line := range outputLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var dj dockerImageJSON
		if err := json.Unmarshal([]byte(line), &dj); err != nil {
			// Log or collect parsing errors for individual lines?
			// For now, let's return an error if any line fails to parse.
			return nil, fmt.Errorf("failed to parse image JSON line '%s': %w", line, err)
		}

		sizeBytes, err := parseDockerSize(dj.Size)
		if err != nil {
			return nil, fmt.Errorf("failed to parse size '%s' for image ID %s: %w", dj.Size, dj.ID, err)
		}

		// Construct RepoTags. If Repository is "<none>", Tag might also be "<none>" or a digest.
		// If an image has multiple tags, `docker images` lists it multiple times.
		// Each JSON line represents one such listing.
		var repoTags []string
		if dj.Repository != "<none>" && dj.Tag != "<none>" {
			repoTags = append(repoTags, fmt.Sprintf("%s:%s", dj.Repository, dj.Tag))
		} else if dj.Repository != "<none>" && dj.Tag == "<none>" {
			// This case can happen (e.g. intermediate layers shown with --all, or untagged images)
			// but typically users are interested in tagged images.
			// For now, include repository if it's not <none>.
			repoTags = append(repoTags, dj.Repository)
		}
		// If both are <none>, RepoTags will be empty, which is fine.

		images = append(images, ImageInfo{
			ID:          dj.ID,
			RepoTags:    repoTags,
			Created:     dj.CreatedSince, // Using CreatedSince as per ImageInfo struct's intent for human-readable
			Size:        sizeBytes,
			VirtualSize: sizeBytes, // Assuming VirtualSize is same as Size for `docker images` output simplicity
			                           // A more accurate VirtualSize might require `docker inspect`.
		})
	}

	return images, nil
}

// RemoveImage removes a Docker image from the host.
// Sudo is used by default for Docker commands.
func (r *defaultRunner) RemoveImage(ctx context.Context, conn connector.Connector, imageName string, force bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for RemoveImage")
	}
	if strings.TrimSpace(imageName) == "" {
		return fmt.Errorf("imageName cannot be empty for RemoveImage")
	}

	cmdArgs := []string{"docker", "rmi"}
	if force {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmdArgs = append(cmdArgs, shellEscape(imageName))
	cmd := strings.Join(cmdArgs, " ")

	cmdTimeout := 5 * time.Minute // Removing an image can take time if layers are shared/complex.
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < cmdTimeout {
			cmdTimeout = time.Until(deadline)
			if cmdTimeout < 0 { cmdTimeout = 0 }
		}
	}

	_, stderrBytes, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{
		Sudo:    true,
		Timeout: cmdTimeout,
	})

	if err != nil {
		// docker rmi usually returns exit code 1 for "no such image" or "image is in use".
		// The specific error message is on stderr.
		return fmt.Errorf("failed to remove image '%s' (force: %v): %w. Stderr: %s", imageName, force, err, string(stderrBytes))
	}

	// Successful removal usually prints the IDs of untagged/deleted layers to stdout.
	// We don't need to parse stdout for this method, just ensure no error.
	return nil
}

func (r *defaultRunner) BuildImage(ctx context.Context, conn connector.Connector, dockerfilePath string, imageNameAndTag string, contextPath string, buildArgs map[string]string) error {
	return fmt.Errorf("not implemented: BuildImage")
}
func (r *defaultRunner) CreateContainer(ctx context.Context, conn connector.Connector, options ContainerCreateOptions) (string, error) {
	return "", fmt.Errorf("not implemented: CreateContainer")
}
func (r *defaultRunner) ContainerExists(ctx context.Context, conn connector.Connector, containerNameOrID string) (bool, error) {
	return false, fmt.Errorf("not implemented: ContainerExists")
}
func (r *defaultRunner) StartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error {
	return fmt.Errorf("not implemented: StartContainer")
}
func (r *defaultRunner) StopContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error {
	return fmt.Errorf("not implemented: StopContainer")
}
func (r *defaultRunner) RestartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error {
	return fmt.Errorf("not implemented: RestartContainer")
}
func (r *defaultRunner) RemoveContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, force bool, removeVolumes bool) error {
	return fmt.Errorf("not implemented: RemoveContainer")
}
func (r *defaultRunner) ListContainers(ctx context.Context, conn connector.Connector, all bool, filters map[string]string) ([]ContainerInfo, error) {
	return nil, fmt.Errorf("not implemented: ListContainers")
}
func (r *defaultRunner) GetContainerLogs(ctx context.Context, conn connector.Connector, containerNameOrID string, options ContainerLogOptions) (string, error) {
	return "", fmt.Errorf("not implemented: GetContainerLogs")
}
func (r *defaultRunner) GetContainerStats(ctx context.Context, conn connector.Connector, containerNameOrID string, stream bool) (<-chan ContainerStats, error) {
	// For a stub, returning a closed channel or nil with error is appropriate.
	// A closed channel signals no data immediately.
	// ch := make(chan ContainerStats)
	// close(ch)
	// return ch, fmt.Errorf("not implemented: GetContainerStats")
	return nil, fmt.Errorf("not implemented: GetContainerStats")
}
func (r *defaultRunner) InspectContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) (*ContainerDetails, error) {
	return nil, fmt.Errorf("not implemented: InspectContainer")
}
func (r *defaultRunner) PauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error {
	return fmt.Errorf("not implemented: PauseContainer")
}
func (r *defaultRunner) UnpauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error {
	return fmt.Errorf("not implemented: UnpauseContainer")
}
func (r *defaultRunner) ExecInContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, cmd []string, user string, workDir string, tty bool) (string, error) {
	return "", fmt.Errorf("not implemented: ExecInContainer")
}
func (r *defaultRunner) CreateDockerNetwork(ctx context.Context, conn connector.Connector, name string, driver string, subnet string, gateway string, options map[string]string) error {
	return fmt.Errorf("not implemented: CreateDockerNetwork")
}
func (r *defaultRunner) RemoveDockerNetwork(ctx context.Context, conn connector.Connector, networkNameOrID string) error {
	return fmt.Errorf("not implemented: RemoveDockerNetwork")
}
func (r *defaultRunner) ListDockerNetworks(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerNetworkInfo, error) {
	return nil, fmt.Errorf("not implemented: ListDockerNetworks")
}
func (r *defaultRunner) ConnectContainerToNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, ipAddress string) error {
	return fmt.Errorf("not implemented: ConnectContainerToNetwork")
}
func (r *defaultRunner) DisconnectContainerFromNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, force bool) error {
	return fmt.Errorf("not implemented: DisconnectContainerFromNetwork")
}
func (r *defaultRunner) CreateDockerVolume(ctx context.Context, conn connector.Connector, name string, driver string, driverOpts map[string]string, labels map[string]string) error {
	return fmt.Errorf("not implemented: CreateDockerVolume")
}
func (r *defaultRunner) RemoveDockerVolume(ctx context.Context, conn connector.Connector, volumeName string, force bool) error {
	return fmt.Errorf("not implemented: RemoveDockerVolume")
}
func (r *defaultRunner) ListDockerVolumes(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerVolumeInfo, error) {
	return nil, fmt.Errorf("not implemented: ListDockerVolumes")
}
func (r *defaultRunner) InspectDockerVolume(ctx context.Context, conn connector.Connector, volumeName string) (*DockerVolumeDetails, error) {
	return nil, fmt.Errorf("not implemented: InspectDockerVolume")
}
func (r *defaultRunner) DockerInfo(ctx context.Context, conn connector.Connector) (*DockerSystemInfo, error) {
	return nil, fmt.Errorf("not implemented: DockerInfo")
}
func (r *defaultRunner) DockerPrune(ctx context.Context, conn connector.Connector, pruneType string, filters map[string]string, all bool) (string, error) {
	return "", fmt.Errorf("not implemented: DockerPrune")
}
