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

// CtrContainerInfo retrieves information about a specific container using ctr.
// Corresponds to `ctr -n <namespace> container info <containerID>`.
func (r *defaultRunner) CtrContainerInfo(ctx context.Context, conn connector.Connector, namespace, containerID string) (*CtrContainerInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil for CtrContainerInfo") }
	if namespace == "" { return nil, errors.New("namespace is required for CtrContainerInfo") }
	if containerID == "" { return nil, errors.New("containerID is required for CtrContainerInfo") }

	cmd := fmt.Sprintf("ctr -n %s container info %s", shellEscape(namespace), shellEscape(containerID))
	// ctr container info output is not JSON by default, it's a custom format.
	// We need to parse it. Example:
	// Image: docker.io/library/alpine:latest
	// Runtime: io.containerd.runc.v2
	// Snapshotter: overlayfs
	// Spec: ... (OCI Spec JSON)
	// Labels:
	//   io.cri-containerd.container.name: test-container
	//   io.kubernetes.pod.namespace: default

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "No such container") || strings.Contains(string(stderr), "not found") {
			return nil, nil // Not found, return nil, nil
		}
		return nil, errors.Wrapf(err, "ctr container info for %s in namespace %s failed. Stderr: %s", containerID, namespace, string(stderr))
	}

	info := CtrContainerInfo{ID: containerID}
	labels := make(map[string]string)
	inLabelsSection := false

	scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Image:") {
			info.Image = strings.TrimSpace(strings.TrimPrefix(line, "Image:"))
		} else if strings.HasPrefix(line, "Runtime:") {
			info.Runtime = strings.TrimSpace(strings.TrimPrefix(line, "Runtime:"))
			// Example: "io.containerd.runc.v2" "args=[-c]"
			parts := strings.Fields(info.Runtime)
			if len(parts) > 0 {
				info.Runtime = parts[0] // Keep only the runtime name
			}
		} else if strings.HasPrefix(line, "Labels:") {
			inLabelsSection = true
			continue
		} else if strings.HasPrefix(line, "Spec:") || strings.HasPrefix(line, "Snapshotter:") || line == "" {
			// If we encounter another top-level key or empty line, labels section ends.
			inLabelsSection = false
		}

		if inLabelsSection {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes if present, as ctr output might have them for some label values
				value = strings.Trim(value, "\"")
				labels[key] = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "failed to parse ctr container info output for %s", containerID)
	}
	info.Labels = labels

	// Status is not directly available from `ctr container info`.
	// It needs to be fetched from `ctr task info` or `ctr task ls`.
	// For now, status will remain empty or be set by a separate call if needed.
	// We can attempt a `ctr task ps <containerID>` to see if a task is running.
	taskPsCmd := fmt.Sprintf("ctr -n %s task ps %s", shellEscape(namespace), shellEscape(containerID))
	_, _, taskErr := conn.Exec(ctx, taskPsCmd, &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Second}) // Short timeout for status check
	if taskErr == nil {
		info.Status = "RUNNING" // If `task ps` succeeds, assume running.
	} else {
		// If `task ps` fails, it could be STOPPED, CREATED, or other states.
		// `no such process` usually means no active task.
		if strings.Contains(taskErr.Error(), "no such process") || strings.Contains(taskErr.Error(), "not found") {
			info.Status = "STOPPED" // Or "CREATED" - hard to distinguish without more info
		} else {
			// Some other error trying to get task status
			// fmt.Fprintf(os.Stderr, "Warning: could not determine task status for %s: %v\n", containerID, taskErr)
			info.Status = "UNKNOWN"
		}
	}


	return &info, nil
}


// --- Containerd Configuration ---

// GetContainerdConfig attempts to read and heuristically parse parts of containerd's config.toml.
// Due to the lack of a TOML parser, this is a best-effort extraction of common simple values.
func (r *defaultRunner) GetContainerdConfig(ctx context.Context, conn connector.Connector) (*ContainerdConfigOptions, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil for GetContainerdConfig") }

	configContentBytes, err := r.ReadFile(ctx, conn, containerdConfigPath)
	if err != nil {
		exists, _ := r.Exists(ctx, conn, containerdConfigPath)
		if !exists {
			return &ContainerdConfigOptions{}, nil // Not an error if not found, return empty config
		}
		return nil, errors.Wrapf(err, "failed to read containerd config file %s", containerdConfigPath)
	}

	if len(configContentBytes) == 0 {
		return &ContainerdConfigOptions{}, nil // Empty file, return empty config
	}

	configContent := string(configContentBytes)
	opts := &ContainerdConfigOptions{
		RegistryMirrors: make(map[string][]string), // Initialize map
	}

	// Helper to extract simple string values like key = "value" or key = value
	extractString := func(content, key string) *string {
		// Regex to find key = "value" or key = 'value' or key = value_without_quotes
		// It tries to handle comments by ensuring the line doesn't start with #
		// This is still very basic.
		re := regexp.MustCompile(fmt.Sprintf(`^\s*%s\s*=\s*(?:\"(.*?)\"|'(.*?)'|([^#\s]+))`, regexp.QuoteMeta(key)))
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			if matches[1] != "" { // double quotes
				val := matches[1]
				return &val
			}
			if matches[2] != "" { // single quotes
				val := matches[2]
				return &val
			}
			if matches[3] != "" { // no quotes
				val := matches[3]
				return &val
			}
		}
		return nil
	}
	extractInt := func(content, key string) *int {
		strValPtr := extractString(content, key)
		if strValPtr != nil {
			if intVal, errAtoi := strconv.Atoi(*strValPtr); errAtoi == nil {
				return &intVal
			}
		}
		return nil
	}
	extractBool := func(content, key string) *bool {
		strValPtr := extractString(content, key)
		if strValPtr != nil {
			if boolVal, errParseBool := strconv.ParseBool(*strValPtr); errParseBool == nil {
				return &boolVal
			}
		}
		return nil
	}

	// Top-level options
	opts.Version = extractInt(configContent, "version")
	opts.Root = extractString(configContent, "root")
	opts.State = extractString(configContent, "state")
	// oom_score is usually not quoted
	reOOM := regexp.MustCompile(`^\s*oom_score\s*=\s*(-?\d+)`)
	oomMatches := reOOM.FindStringSubmatch(configContent)
	if len(oomMatches) > 1 {
		if oomVal, errAtoi := strconv.Atoi(oomMatches[1]); errAtoi == nil {
			opts.OOMScore = &oomVal
		}
	}


	// [grpc] section - find the section first
	grpcSectionRegex := regexp.MustCompile(`(?s)\[grpc\](.*?)(\n\s*\[|\z)`)
	grpcSectionMatch := grpcSectionRegex.FindStringSubmatch(configContent)
	if len(grpcSectionMatch) > 1 {
		grpcContent := grpcSectionMatch[1]
		opts.GRPC = &ContainerdGRPCConfig{}
		opts.GRPC.Address = extractString(grpcContent, "address")
		opts.GRPC.UID = extractInt(grpcContent, "uid")
		opts.GRPC.GID = extractInt(grpcContent, "gid")
	}

	// [plugins."io.containerd.grpc.v1.cri"] section
	criPluginRegex := regexp.MustCompile(`(?s)\[plugins\.("io\.containerd\.grpc\.v1\.cri"|'io\.containerd\.grpc\.v1\.cri')\](.*?)(\n\s*\[|\z)`)
	criPluginMatch := criPluginRegex.FindStringSubmatch(configContent)
	if len(criPluginMatch) > 2 {
		criPluginContent := criPluginMatch[2]
		sandboxImage := extractString(criPluginContent, "sandbox_image")

		// [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
		runcOptionsRegex := regexp.MustCompile(`(?s)\[plugins\.("io\.containerd\.grpc\.v1\.cri"|'io\.containerd\.grpc\.v1\.cri')\.containerd\.runtimes\.runc\.options\](.*?)(\n\s*\[|\z)`)
		runcOptionsMatch := runcOptionsRegex.FindStringSubmatch(configContent) // Search in full config for this specific nested table
		var systemdCgroup *bool
		if len(runcOptionsMatch) > 2 {
			systemdCgroup = extractBool(runcOptionsMatch[2], "SystemdCgroup")
		}

		// Simplified representation for PluginConfigs for these specific values
		if sandboxImage != nil || systemdCgroup != nil {
			if opts.PluginConfigs == nil {
				opts.PluginConfigs = make(map[string]interface{})
			}
			criPlugins, ok := opts.PluginConfigs["io.containerd.grpc.v1.cri"].(map[string]interface{})
			if !ok {
				criPlugins = make(map[string]interface{})
				opts.PluginConfigs["io.containerd.grpc.v1.cri"] = criPlugins
			}
			if sandboxImage != nil {
				criPlugins["sandbox_image"] = *sandboxImage
			}

			if systemdCgroup != nil {
				containerdPlugins, okC := criPlugins["containerd"].(map[string]interface{})
				if !okC {
					containerdPlugins = make(map[string]interface{})
					criPlugins["containerd"] = containerdPlugins
				}
				runtimesPlugins, okR := containerdPlugins["runtimes"].(map[string]interface{})
				if !okR {
					runtimesPlugins = make(map[string]interface{})
					containerdPlugins["runtimes"] = runtimesPlugins
				}
				runcPlugin, okRN := runtimesPlugins["runc"].(map[string]interface{})
				if !okRN {
					runcPlugin = make(map[string]interface{})
					runtimesPlugins["runc"] = runcPlugin
				}
				runcOptions, okRO := runcPlugin["options"].(map[string]interface{})
				if !okRO {
					runcOptions = make(map[string]interface{})
					runcPlugin["options"] = runcOptions
				}
				runcOptions["SystemdCgroup"] = *systemdCgroup
			}
		}
	}
	// Note: Parsing registry mirrors is complex with regex, not attempted here.
	// ConfigureContainerd handles adding them by appending.

	return opts, nil
}

// EnsureDefaultContainerdConfig ensures that a default containerd configuration exists and is applied.
// It creates a default config.toml if one doesn't exist or is empty.
func (r *defaultRunner) EnsureDefaultContainerdConfig(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return errors.New("connector cannot be nil for EnsureDefaultContainerdConfig")
	}

	exists, err := r.Exists(ctx, conn, containerdConfigPath)
	if err != nil {
		return errors.Wrapf(err, "failed to check existence of %s", containerdConfigPath)
	}

	createDefaults := false
	if !exists {
		createDefaults = true
	} else {
		content, errRead := r.ReadFile(ctx, conn, containerdConfigPath)
		if errRead != nil {
			return errors.Wrapf(errRead, "failed to read existing %s to check if empty", containerdConfigPath)
		}
		trimmedContent := strings.TrimSpace(string(content))
		// Consider config empty if it's empty, just "version = 2", or just comments.
		// A more sophisticated check might involve trying to parse it lightly.
		if len(trimmedContent) == 0 || trimmedContent == "{}" || trimmedContent == "version = 2" {
			createDefaults = true
		} else {
			// If file exists and has some content, check if CRI plugin is configured.
			// This is a heuristic. A proper TOML parser would be better.
			if !strings.Contains(trimmedContent, "[plugins.\"io.containerd.grpc.v1.cri\"]") {
				// If CRI is not explicitly configured, we might still want to ensure our defaults.
				// However, overwriting a user's partial config is risky.
				// For now, if there's content but no CRI, we'll assume user knows what they're doing,
				// or they will use ConfigureContainerd for specific changes.
				// Alternative: append default CRI if missing.
				// For this function, "EnsureDefault" implies creating if absent or truly minimal.
				// So, if it has substantial content, we don't overwrite with this function.
			}
		}
	}

	if createDefaults {
		// Default configuration content.
		// Enables CRI plugin, sets up basic sandbox image, and common plugin settings.
		// SystemdCgroup is typically preferred on modern Linux systems.
		defaultConfigContent := `version = 2
root = "/var/lib/containerd"
state = "/run/containerd"
oom_score = -999

[grpc]
  address = "/run/containerd/containerd.sock"
  uid = 0
  gid = 0
  max_recv_message_size = 16777216
  max_send_message_size = 16777216

[debug]
  address = "" # No debug socket by default
  level = "info"

[metrics]
  address = "" # No metrics endpoint by default
  grpc_histogram = false

[cgroup]
  path = "" # Let containerd detect

# Plugins section
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "registry.k8s.io/pause:3.9" # Or appropriate version
    [plugins."io.containerd.grpc.v1.cri".containerd]
      snapshotter = "overlayfs" # Common default, ensure kernel support
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
        runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
          SystemdCgroup = true # Set to true if systemd is the cgroup driver
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "/opt/cni/bin"
      conf_dir = "/etc/cni/net.d"
      # max_conf_num = 1 # Max number of CNI config files to load
      # conf_template = "" # Path to CNI config template
  # Minimal registry configuration. Mirrors can be added via ConfigureContainerd.
  [plugins."io.containerd.grpc.v1.cri".registry]
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
        endpoint = ["https://registry-1.docker.io"]
`
		// Adjust SystemdCgroup based on facts if possible
		// This is a simple string replacement. A real TOML library would be better.
		if facts != nil && facts.InitSystem != nil && facts.InitSystem.Type == InitSystemSystemd {
			// Already set to true in the template above.
			// If it were false by default:
			// defaultConfigContent = strings.Replace(defaultConfigContent, "SystemdCgroup = false", "SystemdCgroup = true", 1)
		} else {
			// If not systemd, it should be false.
			defaultConfigContent = strings.Replace(defaultConfigContent, "SystemdCgroup = true", "SystemdCgroup = false", 1)
		}


		if errMkdir := r.Mkdirp(ctx, conn, filepath.Dir(containerdConfigPath), "0755", true); errMkdir != nil {
			return errors.Wrapf(errMkdir, "failed to create directory for %s", containerdConfigPath)
		}
		if errWrite := r.WriteFile(ctx, conn, []byte(defaultConfigContent), containerdConfigPath, "0644", true); errWrite != nil {
			return errors.Wrapf(errWrite, "failed to write default containerd config to %s", containerdConfigPath)
		}

		// Restart containerd service
		if facts == nil { // Gather facts if not provided, for service restart
			var errFacts error
			facts, errFacts = r.GatherFacts(ctx, conn)
			if errFacts != nil {
				return errors.Wrap(errFacts, "failed to gather facts for restarting containerd service after ensuring default config")
			}
		}
		if errRestart := r.RestartService(ctx, conn, facts, "containerd"); errRestart != nil {
			// Try common alternative name like containerd.service
			errRestartAlt := r.RestartService(ctx, conn, facts, "containerd.service")
			if errRestartAlt != nil {
				return errors.Wrapf(errRestart, "failed to restart containerd (tried 'containerd', 'containerd.service') after writing default config. Original error: %v", errRestartAlt)
			}
		}
	}
	return nil
}

// ConfigureContainerd modifies containerd's config.toml.
// This is a best-effort modification due to lack of a full TOML parser.
// It focuses on setting simple top-level values and appending registry mirrors.
// More complex structural changes to the TOML are not reliably supported.
func (r *defaultRunner) ConfigureContainerd(ctx context.Context, conn connector.Connector, opts ContainerdConfigOptions, restartService bool) error {
	if conn == nil { return errors.New("connector cannot be nil for ConfigureContainerd") }

	if err := r.Mkdirp(ctx, conn, filepath.Dir(containerdConfigPath), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create dir for %s", containerdConfigPath)
	}

	currentContentBytes, err := r.ReadFile(ctx, conn, containerdConfigPath)
	if err != nil {
		exists, _ := r.Exists(ctx, conn, containerdConfigPath)
		if exists { // File exists but couldn't read it
			return errors.Wrapf(err, "failed to read existing containerd config at %s", containerdConfigPath)
		}
		currentContentBytes = []byte{} // Treat as empty if not found
	}
	currentContent := string(currentContentBytes)
	lines := strings.Split(currentContent, "\n")
	newLines := make([]string, 0, len(lines)+20) // Preallocate some space

	modified := false

	// Helper to replace or append a simple key-value line: key = value
	// sectionHeader is like "[grpc]" or `[plugins."io.containerd.grpc.v1.cri"]` (can be empty for top-level)
	// key is the actual key name, e.g., "address" or "sandbox_image"
	// value can be string (will be quoted), int, or bool
	replaceOrAppendValue := func(currentLines []string, sectionHeader, key string, value interface{}) ([]string, bool) {
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = fmt.Sprintf("%q", v)
		case *string:
			if v == nil { return currentLines, false }
			valueStr = fmt.Sprintf("%q", *v)
		case int:
			valueStr = strconv.Itoa(v)
		case *int:
			if v == nil { return currentLines, false }
			valueStr = strconv.Itoa(*v)
		case bool:
			valueStr = strconv.FormatBool(v)
		case *bool:
			if v == nil { return currentLines, false }
			valueStr = strconv.FormatBool(*v)
		default:
			return currentLines, false // Unsupported type
		}

		linePattern := regexp.MustCompile(fmt.Sprintf(`^\s*%s\s*=\s*.*`, regexp.QuoteMeta(key)))
		newLine := fmt.Sprintf("%s = %s", key, valueStr)

		var inSection bool
		var addedOrReplaced bool
		resultLines := make([]string, 0, len(currentLines)+1)

		if sectionHeader == "" { // Top-level
			inSection = true
		}

		for _, line := range currentLines {
			trimmedLine := strings.TrimSpace(line)
			if sectionHeader != "" && strings.HasPrefix(trimmedLine, sectionHeader) {
				inSection = true
				resultLines = append(resultLines, line)
				continue
			}
			// If we encounter another section header, the current section ends
			if sectionHeader != "" && inSection && strings.HasPrefix(trimmedLine, "[") && !strings.HasPrefix(trimmedLine, sectionHeader) {
				if !addedOrReplaced { // Append before the new section if not found in the target section
					resultLines = append(resultLines, newLine)
					addedOrReplaced = true
				}
				inSection = false
			}

			if inSection && linePattern.MatchString(line) {
				originalLineContent := strings.TrimSpace(line)
				newLineWithIndent := line[:len(line)-len(originalLineContent)] + newLine // Preserve original indent
				resultLines = append(resultLines, newLineWithIndent)
				addedOrReplaced = true
			} else {
				resultLines = append(resultLines, line)
			}
		}
		if !addedOrReplaced { // If not found anywhere (or section not found), append at the end (or create section and append)
			if sectionHeader != "" && !strings.Contains(strings.Join(resultLines,"\n"), sectionHeader) {
				resultLines = append(resultLines, "", sectionHeader) // Add section header
			}
			resultLines = append(resultLines, newLine)
			addedOrReplaced = true
		}
		return resultLines, addedOrReplaced
	}

	// Apply top-level config changes
	if opts.Root != nil { lines, modified = replaceOrAppendValue(lines, "", "root", opts.Root); modified = true }
	if opts.State != nil { lines, modified = replaceOrAppendValue(lines, "", "state", opts.State); modified = true }
	if opts.OOMScore != nil { lines, modified = replaceOrAppendValue(lines, "", "oom_score", opts.OOMScore); modified = true }
	if opts.Version != nil { lines, modified = replaceOrAppendValue(lines, "", "version", opts.Version); modified = true}


	// Apply [grpc] config changes
	if opts.GRPC != nil {
		if opts.GRPC.Address != nil { lines, modified = replaceOrAppendValue(lines, "[grpc]", "address", opts.GRPC.Address); modified = true }
		if opts.GRPC.UID != nil { lines, modified = replaceOrAppendValue(lines, "[grpc]", "uid", opts.GRPC.UID); modified = true }
		if opts.GRPC.GID != nil { lines, modified = replaceOrAppendValue(lines, "[grpc]", "gid", opts.GRPC.GID); modified = true }
	}

	// Apply specific plugin values if present in opts.PluginConfigs (very simplified)
	if criPluginMap, ok := opts.PluginConfigs["io.containerd.grpc.v1.cri"].(map[string]interface{}); ok {
		if sandboxImage, ok := criPluginMap["sandbox_image"].(string); ok {
			lines, modified = replaceOrAppendValue(lines, `[plugins."io.containerd.grpc.v1.cri"]`, "sandbox_image", sandboxImage); modified = true
		}
		if containerdMap, ok := criPluginMap["containerd"].(map[string]interface{}); ok {
			if runtimesMap, ok := containerdMap["runtimes"].(map[string]interface{}); ok {
				if runcMap, ok := runtimesMap["runc"].(map[string]interface{}); ok {
					if optionsMap, ok := runcMap["options"].(map[string]interface{}); ok {
						if systemdCgroup, ok := optionsMap["SystemdCgroup"].(bool); ok {
							lines, modified = replaceOrAppendValue(lines, `[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]`, "SystemdCgroup", systemdCgroup); modified = true
						}
					}
				}
			}
		}
	}


	// Handle Registry Mirrors (primarily by appending, as robust merging is hard)
	// We will ensure the main registry section exists, then append mirror blocks.
	// This part remains similar to before, focusing on append for safety.
	var mirrorSnippets []string
	if opts.RegistryMirrors != nil && len(opts.RegistryMirrors) > 0 {
		registrySectionHeader := `[plugins."io.containerd.grpc.v1.cri".registry]`
		registryMirrorsHeader := `[plugins."io.containerd.grpc.v1.cri".registry.mirrors]`

		// Ensure registry and mirrors parent sections exist or are added
		currentFullContent := strings.Join(lines, "\n")
		if !strings.Contains(currentFullContent, registrySectionHeader) {
			lines = append(lines, "", registrySectionHeader)
			modified = true
		}
		if !strings.Contains(currentFullContent, registryMirrorsHeader) {
			// Find where to insert registryMirrorsHeader: after registrySectionHeader
			tempLines := make([]string, 0, len(lines)+1)
			inserted := false
			for _, line := range lines {
				tempLines = append(tempLines, line)
				if strings.TrimSpace(line) == registrySectionHeader && !inserted {
					tempLines = append(tempLines, registryMirrorsHeader)
					inserted = true
					modified = true
				}
			}
			if !inserted { // if registrySectionHeader was just added at the end
				tempLines = append(tempLines, registryMirrorsHeader)
				modified = true
			}
			lines = tempLines
		}

		currentFullContent = strings.Join(lines, "\n") // Update currentFullContent after potential additions

		for registry, endpoints := range opts.RegistryMirrors {
			if len(endpoints) > 0 {
				escapedEndpoints := make([]string, len(endpoints))
				for i, ep := range endpoints { escapedEndpoints[i] = fmt.Sprintf("%q", ep) }

				mirrorBlockHeader := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s"]`, registry)
				mirrorBlockContent := fmt.Sprintf("  endpoint = [%s]", strings.Join(escapedEndpoints, ", "))

				// Check if this specific mirror block already exists
				if strings.Contains(currentFullContent, mirrorBlockHeader) {
					// Simple strategy: remove old block and append new one.
					// This is safer than trying to replace lines within a multi-line block with regex.
					// This part is complex to do robustly with line-by-line processing.
					// For now, we'll stick to appending if not exactly present, or let user manage duplicates.
					// A more robust solution would be to parse TOML.
					// Let's assume for now if the header exists, we don't re-add to avoid many duplicates.
					// TODO: Improve this to replace existing mirror blocks if opting for full management.
					// For now, if the mirror for `registry` already exists, we skip adding it again to avoid simple duplicates.
					// This means it won't *update* existing mirror endpoints for a given registry, only add new ones.
					if !strings.Contains(currentFullContent, fmt.Sprintf("%s\n%s", mirrorBlockHeader, mirrorBlockContent)) &&
					   !strings.Contains(currentFullContent, fmt.Sprintf("%s\r\n%s", mirrorBlockHeader, mirrorBlockContent)) { // also check windows newlines
						// If the exact content isn't there, append. This is still not perfect.
						mirrorSnippets = append(mirrorSnippets, "", mirrorBlockHeader, mirrorBlockContent)
						modified = true
					}
				} else {
					mirrorSnippets = append(mirrorSnippets, "", mirrorBlockHeader, mirrorBlockContent)
					modified = true
				}
			}
		}
	}

	if len(mirrorSnippets) > 0 {
		// Find where to append these snippets: inside the [plugins."io.containerd.grpc.v1.cri".registry.mirrors] block
		// This is tricky. Simplest is to append at the end of the file if the section exists.
		finalLines := make([]string, 0, len(lines)+len(mirrorSnippets))
		registryMirrorsBlockEnd := false
		appendedSnippets := false

		// Try to append snippets at the end of the registry.mirrors section
		// This assumes registry.mirrors is the last sub-table in registry, which is common
		// but not guaranteed by TOML spec.
		// A safer bet is just appending to the end of the file if the headers were ensured.
		lines = append(lines, mirrorSnippets...)
		// Note: this simple append might place them outside the intended section if other sections follow.
		// The previous logic tries to ensure the parent headers.
		modified = true // If we have snippets, something was modified or added.
	}

	if modified {
		finalConfigContent := strings.Join(lines, "\n")
		// Add a final newline if missing
		if !strings.HasSuffix(finalConfigContent, "\n") && finalConfigContent != "" {
			finalConfigContent += "\n"
		}

		if err := r.WriteFile(ctx, conn, []byte(finalConfigContent), containerdConfigPath, "0644", true); err != nil {
			return errors.Wrapf(err, "failed to write updated containerd config to %s", containerdConfigPath)
		}
	}

	if modified && restartService {
		var currentFacts *Facts = facts // Use provided facts if available
		if currentFacts == nil {
			var errFacts error
			currentFacts, errFacts = r.GatherFacts(ctx, conn)
			if errFacts != nil {
				return errors.Wrap(errFacts, "failed to gather facts for containerd restart")
			}
		}

		errRestart := r.RestartService(ctx, conn, currentFacts, "containerd")
		if errRestart != nil {
			errRestartAlt := r.RestartService(ctx, conn, currentFacts, "containerd.service")
			if errRestartAlt != nil {
				return errors.Wrapf(errRestart, "failed to restart containerd (tried 'containerd', 'containerd.service') after configuration. Original error: %v", errRestartAlt)
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

// CrictlRunPodSandbox runs a pod sandbox.
// Corresponds to `crictl runp [--runtime <runtime>] <pod-config.json>`.
func (r *defaultRunner) CrictlRunPodSandbox(ctx context.Context, conn connector.Connector, podSandboxConfigFile string, runtimeHandler string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil for CrictlRunPodSandbox") }
	if podSandboxConfigFile == "" { return "", errors.New("podSandboxConfigFile is required for CrictlRunPodSandbox") }

	exists, err := r.Exists(ctx, conn, podSandboxConfigFile)
	if err != nil { return "", errors.Wrapf(err, "failed to check existence of pod sandbox config %s", podSandboxConfigFile) }
	if !exists { return "", errors.Errorf("pod sandbox config file %s does not exist", podSandboxConfigFile) }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "runp")
	if strings.TrimSpace(runtimeHandler) != "" {
		cmdArgs = append(cmdArgs, "--runtime", shellEscape(runtimeHandler))
	}
	cmdArgs = append(cmdArgs, shellEscape(podSandboxConfigFile))

	cmd := strings.Join(cmdArgs, " ")
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "crictl runp failed. Stderr: %s", string(stderr))
	}
	return strings.TrimSpace(string(stdout)), nil
}

// CrictlStopPodSandbox stops a running pod sandbox.
// Corresponds to `crictl stopp <pod-id>`.
func (r *defaultRunner) CrictlStopPodSandbox(ctx context.Context, conn connector.Connector, podID string) error {
	if conn == nil { return errors.New("connector cannot be nil for CrictlStopPodSandbox") }
	if podID == "" { return errors.New("podID is required for CrictlStopPodSandbox") }

	cmd := fmt.Sprintf("crictl stopp %s", shellEscape(podID))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		// Idempotency: if already stopped or not found, crictl might return error but it's effectively stopped.
		// crictl stopp specific errors:
		// - "pod sandbox ... not found": This means it's already gone or never existed.
		// - If it was already stopped, it might return success or a specific message.
		// For simplicity, we consider "not found" as success for idempotency.
		if strings.Contains(string(stderr), "not found") {
			return nil
		}
		return errors.Wrapf(err, "crictl stopp %s failed. Stderr: %s", podID, string(stderr))
	}
	return nil
}

// CrictlRemovePodSandbox removes a pod sandbox.
// Corresponds to `crictl rmp <pod-id>`.
func (r *defaultRunner) CrictlRemovePodSandbox(ctx context.Context, conn connector.Connector, podID string) error {
	if conn == nil { return errors.New("connector cannot be nil for CrictlRemovePodSandbox") }
	if podID == "" { return errors.New("podID is required for CrictlRemovePodSandbox") }

	cmd := fmt.Sprintf("crictl rmp %s", shellEscape(podID))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") { // Idempotency
			return nil
		}
		return errors.Wrapf(err, "crictl rmp %s failed. Stderr: %s", podID, string(stderr))
	}
	return nil
}

// CrictlInspectPod retrieves information about a specific pod sandbox.
// Corresponds to `crictl inspectp <pod-id> -o json`.
func (r *defaultRunner) CrictlInspectPod(ctx context.Context, conn connector.Connector, podID string) (*CrictlPodDetails, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil for CrictlInspectPod") }
	if podID == "" { return nil, errors.New("podID is required for CrictlInspectPod") }

	cmd := fmt.Sprintf("crictl inspectp %s -o json", shellEscape(podID))
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil, nil // Return nil, nil if pod not found, consistent with Docker inspect behavior
		}
		return nil, errors.Wrapf(err, "crictl inspectp %s failed. Stderr: %s", podID, string(stderr))
	}

	var details CrictlPodDetails
	if err := json.Unmarshal(stdout, &details); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl inspectp JSON for pod %s. Output: %s", podID, string(stdout))
	}
	return &details, nil
}

// CrictlPodSandboxStatus retrieves the status of a pod sandbox.
// Note: `crictl inspectp` provides status. This can be a wrapper or alias.
// For more detailed status, `crictl inspectp` is generally used.
// `crictl ps -id <pod-id> -o json` could also provide basic status.
func (r *defaultRunner) CrictlPodSandboxStatus(ctx context.Context, conn connector.Connector, podID string, verbose bool) (*CrictlPodDetails, error) {
	// verbose flag can be used to decide if more info than just basic state is needed.
	// CrictlInspectPod already gets all details.
	if verbose {
		return r.CrictlInspectPod(ctx, conn, podID)
	}

	// If not verbose, we might want a lighter query, but crictl inspectp is standard for details.
	// Let's use inspectp and the caller can decide what to do with the CrictlPodDetails.
	// Alternative for non-verbose: `crictl ps --id <ID> -o json` and parse its limited output.
	// For now, stick to inspectp for consistency.
	return r.CrictlInspectPod(ctx, conn, podID)
}


// CrictlCreateContainerInPod creates a container within an existing pod sandbox.
// Corresponds to `crictl create <pod-id> <container-config.json> <pod-config.json>`.
func (r *defaultRunner) CrictlCreateContainerInPod(ctx context.Context, conn connector.Connector, podID string, containerConfigFile string, podSandboxConfigFile string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil for CrictlCreateContainerInPod") }
	if podID == "" { return "", errors.New("podID is required for CrictlCreateContainerInPod") }
	if containerConfigFile == "" { return "", errors.New("containerConfigFile is required for CrictlCreateContainerInPod") }
	if podSandboxConfigFile == "" { return "", errors.New("podSandboxConfigFile is required for CrictlCreateContainerInPod") }

	for _, p := range []string{containerConfigFile, podSandboxConfigFile} {
		exists, err := r.Exists(ctx, conn, p)
		if err != nil { return "", errors.Wrapf(err, "failed to check existence of config file %s", p) }
		if !exists { return "", errors.Errorf("config file %s does not exist", p) }
	}

	cmd := fmt.Sprintf("crictl create %s %s %s",
		shellEscape(podID),
		shellEscape(containerConfigFile),
		shellEscape(podSandboxConfigFile))

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "crictl create container in pod %s failed. Stderr: %s", podID, string(stderr))
	}
	// crictl create outputs the Container ID on success
	return strings.TrimSpace(string(stdout)), nil
}

// CrictlStartContainerInPod starts a created container within a pod.
// Corresponds to `crictl start <container-id>`.
func (r *defaultRunner) CrictlStartContainerInPod(ctx context.Context, conn connector.Connector, containerID string) error {
	if conn == nil { return errors.New("connector cannot be nil for CrictlStartContainerInPod") }
	if containerID == "" { return errors.New("containerID is required for CrictlStartContainerInPod") }

	cmd := fmt.Sprintf("crictl start %s", shellEscape(containerID))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		// crictl start can fail if already started.
		// Example error: "FATA[0000] starting container "ID": ProcessUtility.StartProcess: function not implemented" (if runtime issue)
		// Or "container ... is already running" - this should not be an error for idempotency.
		// However, crictl start usually exits 0 if already running.
		return errors.Wrapf(err, "crictl start %s failed. Stderr: %s", containerID, string(stderr))
	}
	return nil
}

// CrictlStopContainerInPod stops a running container in a pod.
// Corresponds to `crictl stop [--timeout <sec>] <container-id>`.
func (r *defaultRunner) CrictlStopContainerInPod(ctx context.Context, conn connector.Connector, containerID string, timeout int64) error {
	if conn == nil { return errors.New("connector cannot be nil for CrictlStopContainerInPod") }
	if containerID == "" { return errors.New("containerID is required for CrictlStopContainerInPod") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "stop")
	if timeout >= 0 { // crictl default is 0 (no timeout, meaning it might wait indefinitely for graceful stop by runtime)
		// However, a common k8s behavior is a specific grace period.
		// If timeout is 0, it means stop immediately (kill).
		// If timeout > 0, it's a grace period.
		// crictl's --timeout flag: "Seconds to wait for stop before killing the container"
		cmdArgs = append(cmdArgs, "--timeout", fmt.Sprintf("%d", timeout))
	}
	cmdArgs = append(cmdArgs, shellEscape(containerID))
	cmd := strings.Join(cmdArgs, " ")

	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout + time.Duration(timeout)*time.Second})
	if err != nil {
		if strings.Contains(string(stderr), "not found") || strings.Contains(string(stderr), "already stopped") || strings.Contains(string(stderr), "isn't running") {
			return nil // Idempotency
		}
		return errors.Wrapf(err, "crictl stop %s failed. Stderr: %s", containerID, string(stderr))
	}
	return nil
}

// CrictlRemoveContainerInPod removes a container from a pod.
// Corresponds to `crictl rm [-f] <container-id>`.
func (r *defaultRunner) CrictlRemoveContainerInPod(ctx context.Context, conn connector.Connector, containerID string, force bool) error {
	if conn == nil { return errors.New("connector cannot be nil for CrictlRemoveContainerInPod") }
	if containerID == "" { return errors.New("containerID is required for CrictlRemoveContainerInPod") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "rm")
	if force {
		cmdArgs = append(cmdArgs, "-f") // or --force
	}
	cmdArgs = append(cmdArgs, shellEscape(containerID))
	cmd := strings.Join(cmdArgs, " ")

	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil // Idempotency
		}
		return errors.Wrapf(err, "crictl rm %s failed. Stderr: %s", containerID, string(stderr))
	}
	return nil
}

// CrictlInspectContainerInPod retrieves information about a specific container in a pod.
// Corresponds to `crictl inspect <container-id> -o json`.
func (r *defaultRunner) CrictlInspectContainerInPod(ctx context.Context, conn connector.Connector, containerID string) (*CrictlContainerDetails, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil for CrictlInspectContainerInPod") }
	if containerID == "" { return nil, errors.New("containerID is required for CrictlInspectContainerInPod") }

	cmd := fmt.Sprintf("crictl inspect %s -o json", shellEscape(containerID))
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil, nil // Return nil, nil if container not found
		}
		return nil, errors.Wrapf(err, "crictl inspect %s failed. Stderr: %s", containerID, string(stderr))
	}

	var details CrictlContainerDetails
	if err := json.Unmarshal(stdout, &details); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl inspect JSON for container %s. Output: %s", containerID, string(stdout))
	}
	return &details, nil
}

// CrictlContainerStatus retrieves the status of a container.
// Similar to InspectContainerInPod, as `crictl inspect` provides status info.
func (r *defaultRunner) CrictlContainerStatus(ctx context.Context, conn connector.Connector, containerID string, verbose bool) (*CrictlContainerDetails, error) {
	// `crictl inspect` is the primary way to get detailed status.
	// `crictl ps --id <ID> -o json` can provide a summary.
	// For now, using inspect for both verbose and non-verbose, caller can extract needed info.
	return r.CrictlInspectContainerInPod(ctx, conn, containerID)
}


// CrictlLogsForContainer retrieves logs for a specific container.
// Corresponds to `crictl logs <container-id> [options]`.
func (r *defaultRunner) CrictlLogsForContainer(ctx context.Context, conn connector.Connector, containerID string, opts CrictlLogOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil for CrictlLogsForContainer") }
	if containerID == "" { return "", errors.New("containerID is required for CrictlLogsForContainer") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "logs")
	if opts.Follow { cmdArgs = append(cmdArgs, "-f") }
	if opts.Timestamps { cmdArgs = append(cmdArgs, "--timestamps") }
	if opts.Since != "" { cmdArgs = append(cmdArgs, "--since", shellEscape(opts.Since)) }
	if opts.TailLines != nil && *opts.TailLines > 0 {
		cmdArgs = append(cmdArgs, "--tail", fmt.Sprintf("%d", *opts.TailLines))
	} else if opts.NumLines != nil && *opts.NumLines > 0 { // Support for older --lines
		cmdArgs = append(cmdArgs, "--lines", fmt.Sprintf("%d", *opts.NumLines))
	}
	// No --latest in modern crictl, handled by tail or since.

	cmdArgs = append(cmdArgs, shellEscape(containerID))
	cmd := strings.Join(cmdArgs, " ")

	// Timeout for logs can be longer, especially if not following.
	// If following, the command might run for a long time, relying on context for cancellation.
	logTimeout := DefaultCrictlTimeout
	if opts.Follow { logTimeout = 10 * time.Minute } // Longer timeout for follow, but context should cancel.

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: logTimeout})
	output := string(stdout) + string(stderr)
	if err != nil {
		// If context was cancelled and we were following, it's not a true error of the command.
		if opts.Follow && ctx.Err() != nil {
			return output, nil
		}
		return output, errors.Wrapf(err, "crictl logs for %s failed. Output: %s", containerID, output)
	}
	return string(stdout), nil // crictl logs sends logs to stdout
}

// CrictlExecInContainerSync executes a command synchronously inside a container.
// Corresponds to `crictl exec -s <container-id> <cmd> [args...]`.
func (r *defaultRunner) CrictlExecInContainerSync(ctx context.Context, conn connector.Connector, containerID string, timeout time.Duration, cmdToExec []string) (string, string, error) {
	if conn == nil { return "", "", errors.New("connector cannot be nil for CrictlExecInContainerSync") }
	if containerID == "" { return "", "", errors.New("containerID is required for CrictlExecInContainerSync") }
	if len(cmdToExec) == 0 { return "", "", errors.New("command to exec is required") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "exec", "-s") // -s for sync
	if timeout > 0 {
		cmdArgs = append(cmdArgs, "--timeout", fmt.Sprintf("%ds", int(timeout.Seconds())))
	}
	cmdArgs = append(cmdArgs, shellEscape(containerID))
	for _, arg := range cmdToExec {
		cmdArgs = append(cmdArgs, shellEscape(arg))
	}
	cmd := strings.Join(cmdArgs, " ")

	execCmdTimeout := DefaultCrictlTimeout // Default timeout for the crictl command itself
	if timeout > 0 {
		execCmdTimeout += timeout // Add the user-specified timeout to the command's own execution timeout
	}


	stdoutBytes, stderrBytes, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: execCmdTimeout})
	stdout := string(stdoutBytes)
	stderr := string(stderrBytes)

	if err != nil {
		// crictl exec returns non-zero exit code if the command in container fails.
		// This is an error from the perspective of the Exec call.
		return stdout, stderr, errors.Wrapf(err, "crictl exec sync in %s failed. Stdout: %s, Stderr: %s", containerID, stdout, stderr)
	}
	return stdout, stderr, nil
}

// CrictlExecInContainerAsync executes a command asynchronously. Placeholder.
func (r *defaultRunner) CrictlExecInContainerAsync(ctx context.Context, conn connector.Connector, containerID string, cmdToExec []string) (string, error) {
	return "", errors.New("not implemented: CrictlExecInContainerAsync (crictl does not have a direct async exec that returns a request ID; typically uses attach for async-like behavior)")
}


// CrictlPortForward forwards ports from the host to a pod. Placeholder.
func (r *defaultRunner) CrictlPortForward(ctx context.Context, conn connector.Connector, podID string, ports []string) (string, error) {
	// crictl port-forward <pod-id> [port:port ...]
	// This command is typically long-running.
	if conn == nil { return "", errors.New("connector cannot be nil for CrictlPortForward") }
	if podID == "" { return "", errors.New("podID is required for CrictlPortForward") }
	if len(ports) == 0 { return "", errors.New("at least one port mapping is required for CrictlPortForward") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "port-forward", shellEscape(podID))
	for _, p := range ports {
		cmdArgs = append(cmdArgs, shellEscape(p))
	}
	cmd := strings.Join(cmdArgs, " ")

	// This is tricky. port-forward blocks. How do we handle this?
	// Option 1: Run in background and return. Caller needs to manage.
	// Option 2: This runner function blocks. Not ideal for automation.
	// Option 3: Return an error saying it's not suitable for this type of runner.
	// For now, let's assume it's a command we can run and get output from, even if it's just help text or an error.
	// A true port-forwarding solution would need more sophisticated handling (background process, cancellation).
	// Let's try to run it with a timeout. If it doesn't error out immediately, it means it's trying to forward.
	// We'll return the stdout (which might be empty if successful and blocking).
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second}) // Short timeout
	if err != nil {
		// If timeout, it means it was likely trying to run.
		if ctx.Err() == context.DeadlineExceeded {
			return string(stdout), errors.New("CrictlPortForward started but command is long-running; actual forwarding not guaranteed by this call")
		}
		return string(stdout) + string(stderr), errors.Wrapf(err, "crictl port-forward for pod %s failed. Stderr: %s", podID, string(stderr))
	}
	return string(stdout), nil // If it returns quickly without error, it might be help text or an issue.
}

// CrictlVersion retrieves crictl and runtime version information.
// Corresponds to `crictl version -o json`.
func (r *defaultRunner) CrictlVersion(ctx context.Context, conn connector.Connector) (*CrictlVersionInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil for CrictlVersion") }

	cmd := "crictl version -o json"
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl version failed. Stderr: %s", string(stderr))
	}

	var versionInfo CrictlVersionInfo
	if err := json.Unmarshal(stdout, &versionInfo); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl version JSON. Output: %s", string(stdout))
	}
	return &versionInfo, nil
}

// CrictlInfo retrieves information about the CRI runtime.
// Corresponds to `crictl info -o json`.
func (r *defaultRunner) CrictlInfo(ctx context.Context, conn connector.Connector) (*CrictlRuntimeInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil for CrictlInfo") }

	cmd := "crictl info -o json"
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl info failed. Stderr: %s", string(stderr))
	}
	var runtimeInfo CrictlRuntimeInfo
	if err := json.Unmarshal(stdout, &runtimeInfo); err != nil {
		// crictl info -o json can sometimes produce invalid JSON if some fields are missing or malformed by the runtime.
		// Try to be a bit resilient if the main parts are there.
		// However, a strict parse is usually better.
		return nil, errors.Wrapf(err, "failed to parse crictl info JSON. Output: %s", string(stdout))
	}
	return &runtimeInfo, nil
}


// CrictlRuntimeConfig retrieves the runtime configuration.
// Corresponds to `crictl runtime-config -o json` (though crictl runtime-config is more for testing specific handlers).
// `crictl info` is generally what provides the runtime config details.
// This might be a bit redundant with CrictlInfo.
func (r *defaultRunner) CrictlRuntimeConfig(ctx context.Context, conn connector.Connector) (string, error) {
	// `crictl info -o json` is likely what's intended here for general config.
	// `crictl runtime-config` is for a specific runtime handler.
	// Let's assume the user wants the output of `crictl info` as a string.
	info, err := r.CrictlInfo(ctx, conn)
	if err != nil {
		return "", errors.Wrap(err, "failed to get runtime info for CrictlRuntimeConfig")
	}
	jsonBytes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal runtime info to JSON for CrictlRuntimeConfig")
	}
	return string(jsonBytes), nil
}

// CrictlStats retrieves resource usage statistics for pods or containers.
// `crictl stats [-o json] [<pod-id>|<container-id>]`
func (r *defaultRunner) CrictlStats(ctx context.Context, conn connector.Connector, resourceID string, outputFormat string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil for CrictlStats") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "stats")
	if strings.TrimSpace(outputFormat) == "json" {
		cmdArgs = append(cmdArgs, "-o", "json")
	}
	if strings.TrimSpace(resourceID) != "" {
		cmdArgs = append(cmdArgs, shellEscape(resourceID))
	}
	// Note: crictl stats -o json output is a stream of JSON objects if resourceID is empty or for multiple items.
	// If resourceID is given, it's a single JSON object (or stream if it's a pod and it has multiple containers).
	// This function returning a single string is best for a single resource JSON output.
	// For streaming or multiple objects, the parsing would be more complex.

	cmd := strings.Join(cmdArgs, " ")
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return string(stdout) + string(stderr), errors.Wrapf(err, "crictl stats for '%s' failed. Stderr: %s", resourceID, string(stderr))
	}
	return string(stdout), nil
}


// CrictlPodStats retrieves resource usage statistics for all containers in one or more pods.
// `crictl ps -o json [--pod <pod-id>]`
func (r *defaultRunner) CrictlPodStats(ctx context.Context, conn connector.Connector, outputFormat string, podID string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil for CrictlPodStats") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "stats") // `crictl stats` is the command for this
	if strings.TrimSpace(outputFormat) == "json" {
		cmdArgs = append(cmdArgs, "-o", "json")
	}
	if strings.TrimSpace(podID) != "" {
		cmdArgs = append(cmdArgs, shellEscape(podID)) // If podID is given, it filters stats for that pod.
	}
	// If podID is empty, it lists stats for all pods/containers.
	// The JSON output structure can be complex (streaming JSON objects).
	// This function expects to return a single string, so it's best if podID is specified and output is json.
	cmd := strings.Join(cmdArgs, " ")

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return string(stdout) + string(stderr), errors.Wrapf(err, "crictl pod stats for '%s' failed. Stderr: %s", podID, string(stderr))
	}
	return string(stdout), nil
}

// EnsureDefaultCrictlConfig ensures that a default crictl configuration file exists.
// It creates a default /etc/crictl.yaml if one doesn't exist or is empty.
func (r *defaultRunner) EnsureDefaultCrictlConfig(ctx context.Context, conn connector.Connector) error {
	if conn == nil {
		return errors.New("connector cannot be nil for EnsureDefaultCrictlConfig")
	}

	exists, err := r.Exists(ctx, conn, crictlConfigPath)
	if err != nil {
		return errors.Wrapf(err, "failed to check existence of %s", crictlConfigPath)
	}

	createDefaults := false
	if !exists {
		createDefaults = true
	} else {
		content, errRead := r.ReadFile(ctx, conn, crictlConfigPath)
		if errRead != nil {
			return errors.Wrapf(errRead, "failed to read existing %s to check if empty", crictlConfigPath)
		}
		trimmedContent := strings.TrimSpace(string(content))
		if len(trimmedContent) == 0 || trimmedContent == "{}" { // Simple check for empty or empty JSON
			createDefaults = true
		}
		// If there's some content, we assume it's user-managed unless it's clearly a placeholder.
		// Unlike containerd's complex TOML, crictl.yaml is simpler.
		// We could check if runtime-endpoint is missing, but for "EnsureDefault",
		// creating if absent or truly minimal is the main goal.
	}

	if createDefaults {
		// Default crictl.yaml content
		// Points to the standard containerd socket.
		defaultCrictlConfigContent := `runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 10 # seconds
debug: false
pull-image-on-create: false
`
		// The ConfigureCrictl method handles Mkdirp and WriteFile.
		if err := r.ConfigureCrictl(ctx, conn, defaultCrictlConfigContent); err != nil {
			return errors.Wrap(err, "failed to apply default crictl configuration")
		}
	}
	return nil
}
