package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath" // Required for filepath.Dir
	"regexp"
	"strings"
	"time"


	"github.com/pkg/errors"
	"github.com/mensylisir/kubexm/pkg/connector"
	// Note: For a production-grade TOML manipulation, a library like "github.com/BurntSushi/toml" would be used.
	// As it's not available in this environment, TOML handling will be simplified or string-based.
)

const (
	DefaultCtrTimeout    = 1 * time.Minute
	DefaultCrictlTimeout = 1 * time.Minute
	containerdConfigPath = "/etc/containerd/config.toml"
	crictlConfigPath     = "/etc/crictl.yaml"
)

// --- Containerd/ctr Methods ---

// CtrListNamespaces lists all containerd namespaces.
// Corresponds to `ctr namespaces ls` or `ctr ns ls`.
func (r *defaultRunner) CtrListNamespaces(ctx context.Context, conn connector.Connector) ([]string, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
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
func (r *defaultRunner) CtrListImages(ctx context.Context, conn connector.Connector, namespace string) ([]CtrImageInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("namespace cannot be empty for CtrListImages")
	}
	cmd := fmt.Sprintf("ctr -n %s images ls", shellEscape(namespace))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list images in namespace %s. Stderr: %s", namespace, string(stderr))
	}

	var images []CtrImageInfo
	lines := strings.Split(string(stdout), "\n")
	if len(lines) <= 1 { return images, nil }

	reSpaces := regexp.MustCompile(`\s{2,}`)
	for _, line := range lines[1:] {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" { continue }
		parts := reSpaces.Split(trimmedLine, -1)
		if len(parts) < 3 { continue }

		imageInfo := CtrImageInfo{Name: parts[0], Digest: parts[2]}
		if len(parts) > 3 { imageInfo.Size = parts[3] }
		if len(parts) > 4 { imageInfo.OSArch = parts[4] }
		images = append(images, imageInfo)
	}
	return images, nil
}

// CtrPullImage pulls an image into a given containerd namespace.
func (r *defaultRunner) CtrPullImage(ctx context.Context, conn connector.Connector, namespace, imageName string, allPlatforms bool, user string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if namespace == "" { return errors.New("namespace cannot be empty for CtrPullImage") }
	if imageName == "" { return errors.New("imageName cannot be empty for CtrPullImage") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "images", "pull")
	if allPlatforms { cmdArgs = append(cmdArgs, "--all-platforms") }
	if user != "" { cmdArgs = append(cmdArgs, "--user", shellEscape(user)) }
	cmdArgs = append(cmdArgs, shellEscape(imageName))

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to pull image %s into namespace %s. Stderr: %s", imageName, namespace, string(stderr))
	}
	return nil
}

// CtrRemoveImage removes an image from a given containerd namespace.
func (r *defaultRunner) CtrRemoveImage(ctx context.Context, conn connector.Connector, namespace, imageName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if namespace == "" { return errors.New("namespace cannot be empty for CtrRemoveImage") }
	if imageName == "" { return errors.New("imageName cannot be empty for CtrRemoveImage") }

	cmd := fmt.Sprintf("ctr -n %s images rm %s", shellEscape(namespace), shellEscape(imageName))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") { return nil }
		return errors.Wrapf(err, "failed to remove image %s from namespace %s. Stderr: %s", imageName, namespace, string(stderr))
	}
	return nil
}

// CtrTagImage tags an image in a given containerd namespace.
func (r *defaultRunner) CtrTagImage(ctx context.Context, conn connector.Connector, namespace, sourceImage, targetImage string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if namespace == "" || sourceImage == "" || targetImage == "" {
		return errors.New("namespace, sourceImage, and targetImage are required for CtrTagImage")
	}
	cmd := fmt.Sprintf("ctr -n %s images tag %s %s", shellEscape(namespace), shellEscape(sourceImage), shellEscape(targetImage))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		return errors.Wrapf(err, "failed to tag image %s to %s in namespace %s. Stderr: %s", sourceImage, targetImage, namespace, string(stderr))
	}
	return nil
}

// CtrImportImage imports an image from a tar archive.
func (r *defaultRunner) CtrImportImage(ctx context.Context, conn connector.Connector, namespace, filePath string, allPlatforms bool) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if namespace == "" || filePath == "" { return errors.New("namespace and filePath are required") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "images", "import")
	if allPlatforms { cmdArgs = append(cmdArgs, "--all-platforms") }
	cmdArgs = append(cmdArgs, shellEscape(filePath))

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to import image from %s. Stderr: %s", filePath, string(stderr))
	}
	return nil
}

// CtrExportImage exports an image to a tar archive.
func (r *defaultRunner) CtrExportImage(ctx context.Context, conn connector.Connector, namespace, imageName, outputFilePath string, allPlatforms bool) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if namespace == "" || imageName == "" || outputFilePath == "" { return errors.New("namespace, imageName, and outputFilePath are required") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "images", "export")
	if allPlatforms { cmdArgs = append(cmdArgs, "--all-platforms") }
	cmdArgs = append(cmdArgs, shellEscape(outputFilePath), shellEscape(imageName))

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to export image %s to %s. Stderr: %s", imageName, outputFilePath, string(stderr))
	}
	return nil
}

// CtrListContainers lists containers in a given containerd namespace.
func (r *defaultRunner) CtrListContainers(ctx context.Context, conn connector.Connector, namespace string) ([]CtrContainerInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	if namespace == "" { return nil, errors.New("namespace cannot be empty") }

	cmd := fmt.Sprintf("ctr -n %s containers ls", shellEscape(namespace))
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list containers in %s. Stderr: %s", namespace, string(stderr))
	}

	var containers []CtrContainerInfo
	lines := strings.Split(string(stdout), "\n")
	if len(lines) <= 1 { return containers, nil }

	reSpaces := regexp.MustCompile(`\s{2,}`)
	for _, line := range lines[1:] {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" { continue }
		parts := reSpaces.Split(trimmedLine, -1)
		if len(parts) < 1 { continue }
		cInfo := CtrContainerInfo{ID: parts[0]}
		if len(parts) > 1 { cInfo.Image = parts[1] }
		if len(parts) > 2 { cInfo.Runtime = parts[2] }
		containers = append(containers, cInfo)
	}
	return containers, nil
}

// CtrRunContainer creates and starts a new container.
func (r *defaultRunner) CtrRunContainer(ctx context.Context, conn connector.Connector, namespace string, opts ContainerdContainerCreateOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if namespace == "" || opts.ImageName == "" || opts.ContainerID == "" {
		return "", errors.New("namespace, imageName, and containerID are required")
	}

	if opts.RemoveExisting {
		_ = r.CtrStopContainer(ctx, conn, namespace, opts.ContainerID, 0)
		_ = r.CtrRemoveContainer(ctx, conn, namespace, opts.ContainerID)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "run")
	if opts.Snapshotter != "" { cmdArgs = append(cmdArgs, "--snapshotter", shellEscape(opts.Snapshotter)) }
	if opts.ConfigPath != "" { cmdArgs = append(cmdArgs, "--config", shellEscape(opts.ConfigPath)) }
	if opts.Runtime != "" { cmdArgs = append(cmdArgs, "--runtime", shellEscape(opts.Runtime)) }
	if opts.NetHost { cmdArgs = append(cmdArgs, "--net-host") }
	if opts.TTY { cmdArgs = append(cmdArgs, "--tty") }
	if opts.Privileged { cmdArgs = append(cmdArgs, "--privileged") }
	if opts.ReadOnlyRootFS { cmdArgs = append(cmdArgs, "--rootfs-readonly") }
	if opts.User != "" { cmdArgs = append(cmdArgs, "--user", shellEscape(opts.User)) }
	if opts.Cwd != "" { cmdArgs = append(cmdArgs, "--cwd", shellEscape(opts.Cwd)) }
	for _, envVar := range opts.Env { cmdArgs = append(cmdArgs, "--env", shellEscape(envVar)) }
	for _, mount := range opts.Mounts { cmdArgs = append(cmdArgs, "--mount", shellEscape(mount)) }
	if len(opts.Platforms) > 0 { cmdArgs = append(cmdArgs, "--platform", strings.Join(opts.Platforms, ",")) }
	cmdArgs = append(cmdArgs, "--rm", shellEscape(opts.ImageName), shellEscape(opts.ContainerID))
	if len(opts.Command) > 0 {
		for _, arg := range opts.Command { cmdArgs = append(cmdArgs, shellEscape(arg)) }
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute})
	if err != nil {
		return "", errors.Wrapf(err, "failed to run container %s. Stderr: %s", opts.ContainerID, string(stderr))
	}
	return opts.ContainerID, nil
}

// CtrStopContainer stops/kills a container's task.
func (r *defaultRunner) CtrStopContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, timeout time.Duration) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if namespace == "" || containerID == "" { return errors.New("namespace and containerID are required") }

	killCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", shellEscape(namespace), shellEscape(containerID))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderrTerm, errTerm := conn.Exec(ctx, killCmdTerm, execOptions)

	if errTerm != nil {
		if strings.Contains(string(stderrTerm), "no such process") || strings.Contains(string(stderrTerm), "not found") {
			return nil
		}
	}
	if timeout > 0 { time.Sleep(timeout) }

	killCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", shellEscape(namespace), shellEscape(containerID))
	_, stderrKill, errKill := conn.Exec(ctx, killCmdKill, execOptions)
	if errKill != nil {
		if strings.Contains(string(stderrKill), "no such process") || strings.Contains(string(stderrKill), "not found") { return nil }
		if errTerm == nil { return nil } // SIGTERM was ok, SIGKILL finding it gone is ok.
		return errors.Wrapf(errKill, "failed to SIGKILL task for %s. Stderr: %s", containerID, string(stderrKill))
	}
	return nil
}

// CtrRemoveContainer removes container metadata.
func (r *defaultRunner) CtrRemoveContainer(ctx context.Context, conn connector.Connector, namespace, containerID string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if namespace == "" || containerID == "" { return errors.New("namespace and containerID are required") }

	cmd := fmt.Sprintf("ctr -n %s containers rm %s", shellEscape(namespace), shellEscape(containerID))
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		if strings.Contains(string(stderr), "No such container") || strings.Contains(string(stderr), "not found") { return nil }
		if strings.Contains(string(stderr), "has active task") {
			if stopErr := r.CtrStopContainer(ctx, conn, namespace, containerID, 0); stopErr == nil {
				_, stderrRetry, errRetry := conn.Exec(ctx, cmd, execOptions)
				if errRetry != nil {
					if strings.Contains(string(stderrRetry), "No such container") || strings.Contains(string(stderrRetry), "not found") { return nil }
					return errors.Wrapf(errRetry, "failed to rm container %s after task kill. Stderr: %s", containerID, string(stderrRetry))
				}
				return nil
			}
		}
		return errors.Wrapf(err, "failed to rm container %s. Stderr: %s", containerID, string(stderr))
	}
	return nil
}

// CtrExecInContainer executes a command in a running container.
func (r *defaultRunner) CtrExecInContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, opts CtrExecOptions, cmdToExec []string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if namespace == "" || containerID == "" || len(cmdToExec) == 0 {
		return "", errors.New("namespace, containerID, and command are required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", shellEscape(namespace), "task", "exec")
	if opts.TTY { cmdArgs = append(cmdArgs, "--tty") }
	if opts.User != "" { cmdArgs = append(cmdArgs, "--user", shellEscape(opts.User)) }
	if opts.Cwd != "" { cmdArgs = append(cmdArgs, "--cwd", shellEscape(opts.Cwd)) }

	execID := fmt.Sprintf("kubexm-exec-%d", time.Now().UnixNano())
	cmdArgs = append(cmdArgs, "--exec-id", shellEscape(execID))
	cmdArgs = append(cmdArgs, shellEscape(containerID))
	for _, arg := range cmdToExec { cmdArgs = append(cmdArgs, shellEscape(arg)) }

	execTimeout := 5 * time.Minute
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: execTimeout}
	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), execOptions)
	combinedOutput := string(stdout) + string(stderr)
	if err != nil {
		return combinedOutput, errors.Wrapf(err, "failed to exec in container %s. Output: %s", containerID, combinedOutput)
	}
	return combinedOutput, nil
}

// --- Containerd Configuration ---

// GetContainerdConfig attempts to read and return parts of containerd's config.toml.
// Simplified: returns mostly empty struct due to TOML parsing complexity in this environment.
func (r *defaultRunner) GetContainerdConfig(ctx context.Context, conn connector.Connector) (*ContainerdConfigOptions, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }

	// A real implementation would use a TOML library.
	// For this simulation, we acknowledge the limitation.
	// We could read the file and try to regex out specific simple things if needed, but not parse fully.
	_, err := r.ReadFile(ctx, conn, containerdConfigPath)
	if err != nil {
		exists, _ := r.Exists(ctx, conn, containerdConfigPath)
		if !exists { return &ContainerdConfigOptions{}, nil } // Not an error if not found, return empty
		return nil, errors.Wrapf(err, "failed to read containerd config %s", containerdConfigPath)
	}
	// If file exists but cannot be parsed into ContainerdConfigOptions without a TOML library:
	return &ContainerdConfigOptions{}, errors.New("GetContainerdConfig: TOML parsing not implemented, cannot populate options struct from file content")
}

// ConfigureContainerd modifies containerd's config.toml.
// Simplified: focuses on appending registry mirror configurations. Robust TOML editing is complex.
func (r *defaultRunner) ConfigureContainerd(ctx context.Context, conn connector.Connector, opts ContainerdConfigOptions, restartService bool) error {
	if conn == nil { return errors.New("connector cannot be nil") }

	if err := r.Mkdirp(ctx, conn, filepath.Dir(containerdConfigPath), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create dir for %s", containerdConfigPath)
	}

	var tomlSnippets []string
	if opts.RegistryMirrors != nil {
		for registry, endpoints := range opts.RegistryMirrors {
			if len(endpoints) > 0 {
				escapedEndpoints := make([]string, len(endpoints))
				for i, ep := range endpoints { escapedEndpoints[i] = fmt.Sprintf("%q", ep) }
				// This TOML structure for mirrors is common:
				// [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
				//   endpoint = ["https://mirror.example.com"]
				snippet := fmt.Sprintf("\n[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"%s\"]\n  endpoint = [%s]",
					registry, strings.Join(escapedEndpoints, ", "))
				tomlSnippets = append(tomlSnippets, snippet)
			}
		}
	}
	// Add more options here if they can be simply appended or set with basic TOML syntax.
	// e.g. top-level string/int/bool values. Nested tables are harder.

	if len(tomlSnippets) > 0 {
		// This is a very basic append. A proper implementation would parse existing TOML, merge, and rewrite.
		// For now, we'll append. This might lead to issues if sections are duplicated or malformed.
		// It's better to ensure the config file is managed by a template or a single writer if using this method.

		// Create a command to append the snippets.
		// This avoids reading/writing the file directly from here if complex merge is too hard.
		// `echo "snippet" | sudo tee -a /etc/containerd/config.toml`
		// However, `WriteFile` with append mode is not directly available.
		// So, read, append string, write back.

		currentContent, err := r.ReadFile(ctx, conn, containerdConfigPath)
		if err != nil {
			// If file doesn't exist, that's fine, we'll create it.
			// But other read errors are problematic.
			exists, _ := r.Exists(ctx, conn, containerdConfigPath)
			if exists { // File exists but couldn't read it
				return errors.Wrapf(err, "failed to read existing containerd config at %s", containerdConfigPath)
			}
			currentContent = []byte{} // Treat as empty
		}

		newContentStr := string(currentContent)
		if len(newContentStr) > 0 && !strings.HasSuffix(newContentStr, "\n") {
			newContentStr += "\n"
		}
		newContentStr += strings.Join(tomlSnippets, "\n") + "\n"

		if err := r.WriteFile(ctx, conn, []byte(newContentStr), containerdConfigPath, "0644", true); err != nil {
			return errors.Wrapf(err, "failed to write updated containerd config to %s", containerdConfigPath)
		}
	}

	if restartService {
		facts, errFacts := r.GatherFacts(ctx, conn)
		if errFacts != nil { return errors.Wrap(errFacts, "failed to gather facts for containerd restart") }

		errRestart := r.RestartService(ctx, conn, facts, "containerd")
		if errRestart != nil {
			// Try common alternative name like containerd.service
			errRestartAlt := r.RestartService(ctx, conn, facts, "containerd.service")
			if errRestartAlt != nil {
				return errors.Wrapf(errRestart, "failed to restart containerd (tried 'containerd', 'containerd.service'). Original error: %v", errRestartAlt)
			}
		}
	}
	return nil
}

// ConfigureCrictl writes the crictl configuration file (/etc/crictl.yaml).
func (r *defaultRunner) ConfigureCrictl(ctx context.Context, conn connector.Connector, configFileContent string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if strings.TrimSpace(configFileContent) == "" { return errors.New("configFileContent cannot be empty") }

	if err := r.Mkdirp(ctx, conn, filepath.Dir(crictlConfigPath), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create dir for %s", crictlConfigPath)
	}
	if err := r.WriteFile(ctx, conn, []byte(configFileContent), crictlConfigPath, "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write crictl config to %s", crictlConfigPath)
	}
	return nil
}


// --- crictl Methods (Continued) ---

// CrictlListImages lists images visible to the CRI runtime.
func (r *defaultRunner) CrictlListImages(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlImageInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "images")
	if filters != nil {
		for key, value := range filters {
			if key == "image" || key == "digest" {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key), shellEscape(value))
			} else {
				cmdArgs = append(cmdArgs, "--label", shellEscape(fmt.Sprintf("%s=%s", key, value)))
			}
		}
	}
	cmdArgs = append(cmdArgs, "-o", "json")

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl images failed. Stderr: %s", string(stderr))
	}
	var result struct { Images []CrictlImageInfo `json:"images"` }
	if err := json.Unmarshal(stdout, &result); err != nil {
		if strings.TrimSpace(string(stdout)) == "[]" || strings.TrimSpace(string(stdout)) == "" { return []CrictlImageInfo{}, nil }
		return nil, errors.Wrapf(err, "failed to parse crictl images JSON. Output: %s", string(stdout))
	}
	return result.Images, nil
}

// CrictlPullImage pulls an image using crictl.
func (r *defaultRunner) CrictlPullImage(ctx context.Context, conn connector.Connector, imageName string, authCreds string, sandboxConfigPath string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if imageName == "" { return errors.New("imageName cannot be empty") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "pull")
	if authCreds != "" { cmdArgs = append(cmdArgs, "--auth", shellEscape(authCreds)) }
	// sandboxConfigPath is not directly used by `crictl pull` flags. crictl uses /etc/crictl.yaml.
	cmdArgs = append(cmdArgs, shellEscape(imageName))

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "crictl pull %s failed. Stderr: %s", imageName, string(stderr))
	}
	return nil
}

// CrictlRemoveImage removes an image using crictl.
func (r *defaultRunner) CrictlRemoveImage(ctx context.Context, conn connector.Connector, imageName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if imageName == "" { return errors.New("imageName cannot be empty") }

	cmd := fmt.Sprintf("crictl rmi %s", shellEscape(imageName))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") { return nil }
		return errors.Wrapf(err, "crictl rmi %s failed. Stderr: %s", imageName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CrictlInspectImage(ctx context.Context, conn connector.Connector, imageName string) (*CrictlImageDetails, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	if imageName == "" { return nil, errors.New("imageName cannot be empty") }

	cmd := fmt.Sprintf("crictl inspecti %s -o json", shellEscape(imageName))
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") { return nil, nil } // Not found is not an application error here
		return nil, errors.Wrapf(err, "crictl inspecti %s failed. Stderr: %s", imageName, string(stderr))
	}
	var details CrictlImageDetails
	if err := json.Unmarshal(stdout, &details); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl inspecti JSON. Output: %s", string(stdout))
	}
	return &details, nil
}

func (r *defaultRunner) CrictlImageFSInfo(ctx context.Context, conn connector.Connector) ([]CrictlFSInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	cmd := "crictl imagefsinfo -o json"
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl imagefsinfo failed. Stderr: %s", string(stderr))
	}
	// Output is `{"filesystems": [...]}`
	var result struct {
		FileSystems []CrictlFSInfo `json:"filesystems"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl imagefsinfo JSON. Output: %s", string(stdout))
	}
	return result.FileSystems, nil
}

func (r *defaultRunner) CrictlListPods(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlPodInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "pods")
	if filters != nil {
		for key, value := range filters {
			// Common filters: --label, --name, --namespace, --state
			if key == "label" || key == "name" || key == "namespace" || key == "state" || key == "id" {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s",key), shellEscape(value))
			}
		}
	}
	cmdArgs = append(cmdArgs, "-o", "json")
	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl pods failed. Stderr: %s", string(stderr))
	}
	var result struct { Pods []CrictlPodInfo `json:"items"` } // crictl pods -o json uses "items"
	if err := json.Unmarshal(stdout, &result); err != nil {
		if strings.TrimSpace(string(stdout)) == "[]" || strings.TrimSpace(string(stdout)) == "" || strings.TrimSpace(string(stdout)) == "null" { return []CrictlPodInfo{}, nil }
		return nil, errors.Wrapf(err, "failed to parse crictl pods JSON. Output: %s", string(stdout))
	}
	return result.Pods, nil
}


func (r *defaultRunner) CrictlRunPod(ctx context.Context, conn connector.Connector, podSandboxConfigFile string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if podSandboxConfigFile == "" { return "", errors.New("podSandboxConfigFile is required") }

	// Ensure the config file exists
	exists, err := r.Exists(ctx, conn, podSandboxConfigFile)
	if err != nil { return "", errors.Wrapf(err, "failed to check existence of %s", podSandboxConfigFile) }
	if !exists { return "", errors.Errorf("pod sandbox config file %s does not exist", podSandboxConfigFile) }

	cmd := fmt.Sprintf("crictl runp %s", shellEscape(podSandboxConfigFile))
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "crictl runp failed. Stderr: %s", string(stderr))
	}
	// crictl runp outputs the Pod ID on success
	return strings.TrimSpace(string(stdout)), nil
}

func (r *defaultRunner) CrictlStopPod(ctx context.Context, conn connector.Connector, podID string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if podID == "" { return errors.New("podID is required") }
	cmd := fmt.Sprintf("crictl stopp %s", shellEscape(podID))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") { return nil } // Idempotency
		return errors.Wrapf(err, "crictl stopp %s failed. Stderr: %s", podID, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CrictlRemovePod(ctx context.Context, conn connector.Connector, podID string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if podID == "" { return errors.New("podID is required") }
	cmd := fmt.Sprintf("crictl rmp %s", shellEscape(podID))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") { return nil } // Idempotency
		return errors.Wrapf(err, "crictl rmp %s failed. Stderr: %s", podID, string(stderr))
	}
	return nil
}

// Remaining crictl functions are still placeholders
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

[end of pkg/runner/containerd.go]
