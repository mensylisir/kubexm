package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"regexp"

	"github.com/pkg/errors"
	"github.com/mensylisir/kubexm/pkg/connector"
)

const (
	DefaultCtrTimeout    = 1 * time.Minute
	DefaultCrictlTimeout = 1 * time.Minute
)

// --- Containerd/ctr Methods ---

// CtrListNamespaces lists all containerd namespaces.
// Corresponds to `ctr namespaces ls` or `ctr ns ls`.
func (r *defaultRunner) CtrListNamespaces(ctx context.Context, conn connector.Connector) ([]string, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	// `ctr ns ls -q` gives a quiet output, only names.
	cmd := "ctr ns ls -q"
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list containerd namespaces. Stderr: %s", string(stderr))
	}

	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	var namespaces []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			namespaces = append(namespaces, trimmedLine)
		}
	}
	return namespaces, nil
}

// CtrListImages lists images in a given containerd namespace.
// Corresponds to `ctr -n <namespace> images ls`.
func (r *defaultRunner) CtrListImages(ctx context.Context, conn connector.Connector, namespace string) ([]CtrImageInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("namespace cannot be empty for CtrListImages")
	}

	// `ctr -n <namespace> i ls -q` for quiet output (names only) is an option,
	// but to get more details like digest and size, we need to parse the default table output.
	// The default output is tabular. Parsing this can be fragile.
	// Example output:
	// REF                                                       TYPE                                                 DIGEST                                                                  SIZE    PLATFORMS   LABELS
	// docker.io/library/alpine:latest                           application/vnd.docker.distribution.manifest.v2+json sha256:21a3deaa0d32a8057914f36584b5288d2e5ecc984380bc0118285c70fa8c9300 2.83 MiB  linux/amd64 -
	// k8s.gcr.io/pause:3.5                                      application/vnd.docker.distribution.manifest.v2+json sha256:221177c60ce5107572697c109b00c6e9415809cfe0510b5a9800334731ffa9f7 303 KiB   linux/amd64 -
	//
	// Using `--quiet` (`-q`) just lists image names.
	// For more structured output, if `ctr` supported JSON, that would be ideal. It does not seem to.
	// We will parse the tabular output. This is brittle.
	cmd := fmt.Sprintf("ctr -n %s images ls", shellEscape(namespace))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list images in namespace %s. Stderr: %s", namespace, string(stderr))
	}

	var images []CtrImageInfo
	lines := strings.Split(string(stdout), "\n")

	if len(lines) <= 1 { // Only header or empty
		return images, nil
	}

	// Simple tabular parsing: assumes fixed columns and splits by multiple spaces.
	// This is highly dependent on `ctr images ls` output format.
	header := lines[0]
	// Heuristic to find column start indexes - this is very fragile
	// A better way would be to use a library that can parse fixed-width or space-aligned tables if ctr output is consistent.
	// For now, let's try a regex based split, assuming 2+ spaces as delimiter.
	reSpaces := regexp.MustCompile(`\s{2,}`)

	for _, line := range lines[1:] { // Skip header line
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		parts := reSpaces.Split(trimmedLine, -1) // Split by 2 or more spaces

		if len(parts) < 3 { // Expect at least REF, TYPE, DIGEST. Size, Platforms, Labels might be truncated or complex.
			// log.Printf("Skipping unparsable line in CtrListImages output: %s (got %d parts)", trimmedLine, len(parts))
			continue
		}

		imageInfo := CtrImageInfo{
			Name:   parts[0],
			// Type: parts[1], // TYPE is not in CtrImageInfo struct currently
			Digest: parts[2],
		}
		if len(parts) > 3 {
			imageInfo.Size = parts[3]
		}
		if len(parts) > 4 {
			imageInfo.OSArch = parts[4]
		}
		// Labels are usually '-' or key=value pairs, not easily parsed from this simple split.
		// If labels are needed, a more robust parsing or different ctr command would be required.

		images = append(images, imageInfo)
	}
	return images, nil
}

// CtrPullImage pulls an image into a given containerd namespace.
// Corresponds to `ctr -n <namespace> images pull <imageName>`.
func (r *defaultRunner) CtrPullImage(ctx context.Context, conn connector.Connector, namespace, imageName string, allPlatforms bool, user string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return errors.New("namespace cannot be empty for CtrPullImage")
	}
	if strings.TrimSpace(imageName) == "" {
		return errors.New("imageName cannot be empty for CtrPullImage")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "images", "pull")
	if allPlatforms {
		cmdArgs = append(cmdArgs, "--all-platforms")
	}
	if strings.TrimSpace(user) != "" {
		// Format: "user:password"
		cmdArgs = append(cmdArgs, "--user", shellEscape(user))
	}
	cmdArgs = append(cmdArgs, shellEscape(imageName))
	cmd := strings.Join(cmdArgs, " ")

	// Pulling can take a long time.
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute}
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to pull image %s into namespace %s. Stderr: %s", imageName, namespace, string(stderr))
	}
	// ctr pull output often includes progress bars to stderr and final image ref to stdout.
	// We mainly care about the error status.
	return nil
}

// CtrRemoveImage removes an image from a given containerd namespace.
// Corresponds to `ctr -n <namespace> images rm <imageName>`.
func (r *defaultRunner) CtrRemoveImage(ctx context.Context, conn connector.Connector, namespace, imageName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return errors.New("namespace cannot be empty for CtrRemoveImage")
	}
	if strings.TrimSpace(imageName) == "" {
		return errors.New("imageName cannot be empty for CtrRemoveImage")
	}

	cmd := fmt.Sprintf("ctr -n %s images rm %s", shellEscape(namespace), shellEscape(imageName))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Idempotency: if image not found, ctr usually exits with an error.
		// Example: "ctr: image "docker.io/library/nosuchimage:latest": not found"
		if strings.Contains(string(stderr), "not found") {
			return nil // Consider "not found" as success for removal idempotency
		}
		return errors.Wrapf(err, "failed to remove image %s from namespace %s. Stderr: %s", imageName, namespace, string(stderr))
	}
	return nil
}

// CtrTagImage tags an image in a given containerd namespace.
// Corresponds to `ctr -n <namespace> images tag <sourceImage> <targetImage>`.
func (r *defaultRunner) CtrTagImage(ctx context.Context, conn connector.Connector, namespace, sourceImage, targetImage string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || sourceImage == "" || targetImage == "" {
		return errors.New("namespace, sourceImage, and targetImage are required for CtrTagImage")
	}
	cmd := fmt.Sprintf("ctr -n %s images tag %s %s", shellEscape(namespace), shellEscape(sourceImage), shellEscape(targetImage))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to tag image %s to %s in namespace %s. Stderr: %s", sourceImage, targetImage, namespace, string(stderr))
	}
	return nil
}

// CtrImportImage imports an image from a tar archive into a containerd namespace.
// Corresponds to `ctr -n <namespace> images import <filePath>`.
func (r *defaultRunner) CtrImportImage(ctx context.Context, conn connector.Connector, namespace, filePath string, allPlatforms bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || filePath == "" {
		return errors.New("namespace and filePath are required for CtrImportImage")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "images", "import")
	if allPlatforms {
		cmdArgs = append(cmdArgs, "--all-platforms") // May or may not be applicable depending on archive content
	}
	// Add other options like --base-name, --digests, etc. if needed by extending ContainerdImageImportOptions
	cmdArgs = append(cmdArgs, shellEscape(filePath))
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute} // Importing can be slow
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to import image from %s into namespace %s. Stderr: %s", filePath, namespace, string(stderr))
	}
	return nil
}

// CtrExportImage exports an image from a containerd namespace to a tar archive.
// Corresponds to `ctr -n <namespace> images export <outputFilePath> <imageName>`.
func (r *defaultRunner) CtrExportImage(ctx context.Context, conn connector.Connector, namespace, imageName, outputFilePath string, allPlatforms bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || imageName == "" || outputFilePath == "" {
		return errors.New("namespace, imageName, and outputFilePath are required for CtrExportImage")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "images", "export")
	if allPlatforms {
		cmdArgs = append(cmdArgs, "--all-platforms")
	}
	cmdArgs = append(cmdArgs, shellEscape(outputFilePath), shellEscape(imageName)) // Order: output file, then image name(s)
	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute} // Exporting can be slow
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to export image %s from namespace %s to %s. Stderr: %s", imageName, namespace, outputFilePath, string(stderr))
	}
	return nil
}


// CtrListContainers lists containers in a given containerd namespace.
// Corresponds to `ctr -n <namespace> containers ls` or `ctr -n <namespace> c ls`.
func (r *defaultRunner) CtrListContainers(ctx context.Context, conn connector.Connector, namespace string) ([]CtrContainerInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("namespace cannot be empty for CtrListContainers")
	}

	// `ctr -n <namespace> c ls -q` only gives IDs. We need more info.
	// Default output:
	// CONTAINER    IMAGE                             RUNTIME
	// container1   docker.io/library/alpine:latest   io.containerd.runc.v2
	// container2   k8s.gcr.io/pause:3.5              io.containerd.runc.v2
	// This doesn't include status directly. `ctr -n <ns> task ls` shows running tasks (status).
	// For simplicity, we'll parse `c ls` and leave Status to be potentially enriched by `task ls` or inspect.
	cmd := fmt.Sprintf("ctr -n %s containers ls", shellEscape(namespace))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list containers in namespace %s. Stderr: %s", namespace, string(stderr))
	}

	var containers []CtrContainerInfo
	lines := strings.Split(string(stdout), "\n")
	if len(lines) <= 1 {
		return containers, nil
	}

	reSpaces := regexp.MustCompile(`\s{2,}`)
	for _, line := range lines[1:] { // Skip header
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		parts := reSpaces.Split(trimmedLine, -1)
		if len(parts) < 1 { // At least CONTAINER ID
			continue
		}
		cInfo := CtrContainerInfo{ID: parts[0]}
		if len(parts) > 1 {
			cInfo.Image = parts[1]
		}
		if len(parts) > 2 {
			cInfo.Runtime = parts[2]
		}
		// Status is not in `ctr c ls`. It would require `ctr -n <ns> task ls` or inspect.
		// For now, Status will be empty from this list command.
		containers = append(containers, cInfo)
	}
	return containers, nil
}

// CtrRunContainer creates and starts a new container.
// Corresponds to `ctr -n <namespace> run [options] <image> <id> [command]`.
// This is a complex command. The options struct `ContainerdContainerCreateOptions` helps manage this.
// Returns the container ID if successful (which is usually provided as input anyway).
func (r *defaultRunner) CtrRunContainer(ctx context.Context, conn connector.Connector, namespace string, opts ContainerdContainerCreateOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if namespace == "" || opts.ImageName == "" || opts.ContainerID == "" {
		return "", errors.New("namespace, imageName, and containerID are required for CtrRunContainer")
	}

	if opts.RemoveExisting {
		// Attempt to remove container if it exists, ignore error if it doesn't.
		_ = r.CtrRemoveContainer(ctx, conn, namespace, opts.ContainerID)
		// Also stop/kill task if any
		_ = r.CtrStopContainer(ctx, conn, namespace, opts.ContainerID, 0) // 0 timeout for kill
	}


	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "run")

	if opts.Snapshotter != "" {
		cmdArgs = append(cmdArgs, "--snapshotter", shellEscape(opts.Snapshotter))
	}
	if opts.ConfigPath != "" {
		cmdArgs = append(cmdArgs, "--config", shellEscape(opts.ConfigPath))
	}
	if opts.Runtime != "" {
		cmdArgs = append(cmdArgs, "--runtime", shellEscape(opts.Runtime))
	}
	if opts.NetHost {
		cmdArgs = append(cmdArgs, "--net-host")
	}
	if opts.TTY {
		cmdArgs = append(cmdArgs, "--tty")
	}
	if opts.Privileged {
		cmdArgs = append(cmdArgs, "--privileged")
	}
	if opts.ReadOnlyRootFS {
		cmdArgs = append(cmdArgs, "--rootfs-readonly")
	}
	if opts.User != "" {
		cmdArgs = append(cmdArgs, "--user", shellEscape(opts.User))
	}
	if opts.Cwd != "" {
		cmdArgs = append(cmdArgs, "--cwd", shellEscape(opts.Cwd))
	}

	for _, envVar := range opts.Env {
		cmdArgs = append(cmdArgs, "--env", shellEscape(envVar))
	}
	for _, mount := range opts.Mounts {
		cmdArgs = append(cmdArgs, "--mount", shellEscape(mount))
	}
	if len(opts.Platforms) > 0 {
		cmdArgs = append(cmdArgs, "--platform", strings.Join(opts.Platforms, ","))
	}

	// TODO: Handle Labels (--label)
	// for k, v := range opts.Labels {
	//    cmdArgs = append(cmdArgs, "--label", shellEscape(fmt.Sprintf("%s=%s", k,v)))
	// }


	cmdArgs = append(cmdArgs, "--rm") // Add --rm to cleanup task and container on exit for simple `run`
	cmdArgs = append(cmdArgs, shellEscape(opts.ImageName), shellEscape(opts.ContainerID))

	if len(opts.Command) > 0 {
		for _, arg := range opts.Command {
			cmdArgs = append(cmdArgs, shellEscape(arg))
		}
	} else {
		// If no command, ctr run might need "sh" or similar for some images to start a shell.
		// However, many images have default CMD/ENTRYPOINT.
		// For typical daemon containers, this is fine. For interactive, command is needed.
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute} // Running can take time, especially with pulls.

	// `ctr run` can be interactive or detached. This runner assumes detached.
	// Output of `ctr run` is typically empty on success if detached.
	// Stderr might have info or errors.
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run container %s with image %s in namespace %s. Stderr: %s", opts.ContainerID, opts.ImageName, namespace, string(stderr))
	}
	return opts.ContainerID, nil
}


// CtrStopContainer stops/kills a container's task.
// Corresponds to `ctr -n <namespace> task kill [-s <signal>] <containerID>`
// and then `ctr -n <namespace> task rm <containerID>` if needed.
// For simplicity, this will just send SIGTERM then SIGKILL via `task kill`.
// `timeout` is how long to wait for graceful shutdown after SIGTERM before SIGKILL.
// If timeout is 0, it might directly SIGKILL or use a default short timeout.
func (r *defaultRunner) CtrStopContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, timeout time.Duration) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || containerID == "" {
		return errors.New("namespace and containerID are required for CtrStopContainer")
	}

	// Attempt graceful shutdown first with SIGTERM
	killCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", shellEscape(namespace), shellEscape(containerID))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderrTerm, errTerm := conn.Exec(ctx, killCmdTerm, execOptions)

	if errTerm != nil {
		// If "no such process" or "not found", container might already be stopped.
		if strings.Contains(string(stderrTerm), "no such process") || strings.Contains(string(stderrTerm), "not found") {
			// Task is already gone. We might still need to delete container metadata if `ctr run --rm` wasn't used.
			return nil
		}
		// Other error, but if timeout is 0 or very short, we might proceed to SIGKILL anyway.
	}

	if timeout > 0 {
		// Poll for task to exit or timeout
		// `ctr -n <ns> task ls -q` could list tasks. If gone, it's stopped.
		// This is complex to implement here. A simpler way is to sleep for timeout.
		time.Sleep(timeout) // Simplified wait

		// Check if task still exists (e.g. `ctr -n <ns> task ps <containerID>`)
		// If it does, then SIGKILL
	}

	// If still running (or if timeout was 0, or if SIGTERM failed and we want to ensure kill)
	// Send SIGKILL
	// Note: `ctr task kill <id>` without -s defaults to SIGTERM.
	// To ensure it's killed, we might need to check state or just issue SIGKILL.
	// For simplicity, if errTerm was nil (SIGTERM sent) and timeout passed, or if errTerm indicated it was already gone, we might be done.
	// If errTerm occurred and it wasn't "not found", or if we want to be sure:
	killCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", shellEscape(namespace), shellEscape(containerID))
	_, stderrKill, errKill := conn.Exec(ctx, killCmdKill, execOptions)
	if errKill != nil {
		if strings.Contains(string(stderrKill), "no such process") || strings.Contains(string(stderrKill), "not found") {
			return nil // Already stopped/gone
		}
		// If SIGTERM succeeded, this SIGKILL might also say "no such process".
		// This logic is a bit simplified. Robust stop needs state checking.
		if errTerm == nil { // SIGTERM was sent successfully. If SIGKILL now fails with "not found", it's fine.
			return nil
		}
		return errors.Wrapf(errKill, "failed to SIGKILL task for container %s in namespace %s. Stderr: %s", containerID, namespace, string(stderrKill))
	}

	// After task is killed, `ctr run --rm` should remove the container metadata.
	// If not using --rm, a separate `ctr c rm` would be needed. This func assumes task kill is enough for "stop".
	return nil
}

// CtrRemoveContainer removes container metadata. Task must be stopped/killed first.
// Corresponds to `ctr -n <namespace> containers rm <containerID>` or `ctr -n <namespace> c rm <containerID>`.
func (r *defaultRunner) CtrRemoveContainer(ctx context.Context, conn connector.Connector, namespace, containerID string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || containerID == "" {
		return errors.New("namespace and containerID are required for CtrRemoveContainer")
	}

	// Ensure task is killed first (best effort, could be done before calling this)
	// For idempotency, we can try to stop it here too.
	// _ = r.CtrStopContainer(ctx, conn, namespace, containerID, 0) // 0 timeout for quick kill attempt

	cmd := fmt.Sprintf("ctr -n %s containers rm %s", shellEscape(namespace), shellEscape(containerID))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// If "No such container" or "not found"
		if strings.Contains(string(stderr), "No such container") || strings.Contains(string(stderr), "not found") {
			return nil // Idempotent
		}
		// If "container is not stopped" - this means task is still there.
		if strings.Contains(string(stderr), "is not stopped") || strings.Contains(string(stderr), "has active task") {
			// Attempt to kill task then retry remove.
			// log.Printf("Container %s has active task, attempting to kill before removing.", containerID)
			stopErr := r.CtrStopContainer(ctx, conn, namespace, containerID, 0) // Force kill
			if stopErr != nil {
				// log.Printf("Failed to stop active task for container %s during remove: %v", containerID, stopErr)
				// Fall through to return original error, or wrap it.
			} else {
				// Retry remove
				_, stderrRetry, errRetry := conn.Exec(ctx, cmd, execOptions)
				if errRetry != nil {
					if strings.Contains(string(stderrRetry), "No such container") || strings.Contains(string(stderrRetry), "not found") {
						return nil
					}
					return errors.Wrapf(errRetry, "failed to remove container %s in namespace %s after task kill retry. Stderr: %s", containerID, namespace, string(stderrRetry))
				}
				return nil // Success after retry
			}
		}
		return errors.Wrapf(err, "failed to remove container %s in namespace %s. Stderr: %s", containerID, namespace, string(stderr))
	}
	return nil
}

// CtrExecInContainer executes a command in a running container.
// Corresponds to `ctr -n <namespace> task exec [options] <containerID> <cmd> [args...]`.
func (r *defaultRunner) CtrExecInContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, opts CtrExecOptions, cmdToExec []string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if namespace == "" || containerID == "" || len(cmdToExec) == 0 {
		return "", errors.New("namespace, containerID, and command are required for CtrExecInContainer")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "task", "exec")
	if opts.TTY {
		cmdArgs = append(cmdArgs, "--tty")
	}
	if opts.User != "" {
		cmdArgs = append(cmdArgs, "--user", shellEscape(opts.User))
	}
	if opts.Cwd != "" {
		cmdArgs = append(cmdArgs, "--cwd", shellEscape(opts.Cwd))
	}

	// ctr task exec requires an --exec-id. Generate a unique one.
	// Using a simple timestamp-based ID. For production, a UUID might be better if collisions are a concern.
	execID := fmt.Sprintf("kubexm-exec-%d", time.Now().UnixNano())
	cmdArgs = append(cmdArgs, "--exec-id", shellEscape(execID))

	cmdArgs = append(cmdArgs, shellEscape(containerID))
	for _, arg := range cmdToExec {
		cmdArgs = append(cmdArgs, shellEscape(arg))
	}
	cmd := strings.Join(cmdArgs, " ")

	// Timeout for exec should be reasonably long or configurable via CtrExecOptions if needed.
	execTimeout := 5 * time.Minute
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: execTimeout}

	// `ctr task exec` might write command's stdout to its stdout, and stderr to its stderr.
	// If the executed command fails (non-zero exit), `ctr task exec` itself might also exit non-zero.
	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)

	combinedOutput := string(stdout) + string(stderr)

	if err != nil {
		// Even if err is not nil (e.g. command in container exited non-zero),
		// stdout/stderr from the command might still be useful.
		return combinedOutput, errors.Wrapf(err, "failed to exec in container %s (namespace %s, cmd: %v). Combined Output: %s", containerID, namespace, cmdToExec, combinedOutput)
	}

	// If err is nil, it implies ctr command itself succeeded.
	// The executed command inside container also likely exited 0.
	return combinedOutput, nil
}


// --- Containerd/crictl Methods ---
// These will be placeholders for now, as their implementation is extensive.

func (r *defaultRunner) CrictlListImages(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlImageInfo, error) {
	return nil, errors.New("not implemented: CrictlListImages")
}
func (r *defaultRunner) CrictlPullImage(ctx context.Context, conn connector.Connector, imageName string, authCreds string, sandboxConfigPath string) error {
	return errors.New("not implemented: CrictlPullImage")
}
func (r *defaultRunner) CrictlRemoveImage(ctx context.Context, conn connector.Connector, imageName string) error {
	return errors.New("not implemented: CrictlRemoveImage")
}
func (r *defaultRunner) CrictlInspectImage(ctx context.Context, conn connector.Connector, imageName string) (*CrictlImageDetails, error) {
	return nil, errors.New("not implemented: CrictlInspectImage")
}
func (r *defaultRunner) CrictlImageFSInfo(ctx context.Context, conn connector.Connector) ([]CrictlFSInfo, error) {
	return nil, errors.New("not implemented: CrictlImageFSInfo")
}
func (r *defaultRunner) CrictlListPods(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlPodInfo, error) {
	return nil, errors.New("not implemented: CrictlListPods")
}
func (r *defaultRunner) CrictlRunPod(ctx context.Context, conn connector.Connector, podSandboxConfigFile string) (string, error) {
	return "", errors.New("not implemented: CrictlRunPod")
}
func (r *defaultRunner) CrictlStopPod(ctx context.Context, conn connector.Connector, podID string) error {
	return errors.New("not implemented: CrictlStopPod")
}
func (r *defaultRunner) CrictlRemovePod(ctx context.Context, conn connector.Connector, podID string) error {
	return errors.New("not implemented: CrictlRemovePod")
}
func (r *defaultRunner) CrictlInspectPod(ctx context.Context, conn connector.Connector, podID string) (*CrictlPodDetails, error) {
	return nil, errors.New("not implemented: CrictlInspectPod")
}
func (r *defaultRunner) CrictlCreateContainer(ctx context.Context, conn connector.Connector, podID string, containerConfigFile string, podSandboxConfigFile string) (string, error) {
	return "", errors.New("not implemented: CrictlCreateContainer")
}
func (r *defaultRunner) CrictlStartContainer(ctx context.Context, conn connector.Connector, containerID string) error {
	return errors.New("not implemented: CrictlStartContainer")
}
func (r *defaultRunner) CrictlStopContainer(ctx context.Context, conn connector.Connector, containerID string, timeout int64) error {
	return errors.New("not implemented: CrictlStopContainer")
}
func (r *defaultRunner) CrictlRemoveContainerForce(ctx context.Context, conn connector.Connector, containerID string) error {
	return errors.New("not implemented: CrictlRemoveContainerForce")
}
func (r *defaultRunner) CrictlInspectContainer(ctx context.Context, conn connector.Connector, containerID string) (*CrictlContainerDetails, error) {
	return nil, errors.New("not implemented: CrictlInspectContainer")
}
func (r *defaultRunner) CrictlLogs(ctx context.Context, conn connector.Connector, containerID string, opts CrictlLogOptions) (string, error) {
	return "", errors.New("not implemented: CrictlLogs")
}
func (r *defaultRunner) CrictlExec(ctx context.Context, conn connector.Connector, containerID string, timeout time.Duration, sync bool, cmd []string) (string, error) {
	return "", errors.New("not implemented: CrictlExec")
}
func (r *defaultRunner) CrictlPortForward(ctx context.Context, conn connector.Connector, podID string, ports []string) (string, error) {
	return "", errors.New("not implemented: CrictlPortForward")
}
func (r *defaultRunner) CrictlVersion(ctx context.Context, conn connector.Connector) (*CrictlVersionInfo, error) {
	return nil, errors.New("not implemented: CrictlVersion")
}
func (r *defaultRunner) CrictlRuntimeConfig(ctx context.Context, conn connector.Connector) (string, error) {
	return "", errors.New("not implemented: CrictlRuntimeConfig")
}
