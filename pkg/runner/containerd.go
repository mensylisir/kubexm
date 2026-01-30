package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/templates"
	"github.com/mensylisir/kubexm/pkg/tool"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/mensylisir/kubexm/pkg/connector"
)

const (
	DefaultCtrTimeout    = 1 * time.Minute
	DefaultCrictlTimeout = 1 * time.Minute
	containerdConfigPath = common.ContainerdDefaultConfigFile
	crictlConfigPath     = common.CrictlDefaultConfigFile
)

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

func (r *defaultRunner) CtrListImages(ctx context.Context, conn connector.Connector, namespace string) ([]CtrImageInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("namespace cannot be empty for CtrListImages")
	}
	cmd := fmt.Sprintf("ctr -n %s images ls", namespace)
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list images in namespace %s. Stderr: %s", namespace, string(stderr))
	}

	var images []CtrImageInfo
	lines := strings.Split(string(stdout), "\n")
	if len(lines) <= 1 {
		return images, nil
	}

	reSpaces := regexp.MustCompile(`\s{2,}`)
	for _, line := range lines[1:] {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		parts := reSpaces.Split(trimmedLine, -1)
		if len(parts) < 3 {
			continue
		}

		imageInfo := CtrImageInfo{Name: parts[0], Digest: parts[2]}
		if len(parts) > 3 {
			imageInfo.Size = parts[3]
		}
		if len(parts) > 4 {
			imageInfo.OSArch = parts[4]
		}
		images = append(images, imageInfo)
	}
	return images, nil
}

func (r *defaultRunner) CtrPullImage(ctx context.Context, conn connector.Connector, namespace, imageName string, allPlatforms bool, user string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" {
		return errors.New("namespace cannot be empty for CtrPullImage")
	}
	if imageName == "" {
		return errors.New("imageName cannot be empty for CtrPullImage")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", namespace, "images", "pull")
	if allPlatforms {
		cmdArgs = append(cmdArgs, "--all-platforms")
	}
	if user != "" {
		cmdArgs = append(cmdArgs, "--user", user)
	}
	cmdArgs = append(cmdArgs, imageName)

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to pull image %s into namespace %s. Stderr: %s", imageName, namespace, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CtrRemoveImage(ctx context.Context, conn connector.Connector, namespace, imageName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" {
		return errors.New("namespace cannot be empty for CtrRemoveImage")
	}
	if imageName == "" {
		return errors.New("imageName cannot be empty for CtrRemoveImage")
	}

	cmd := fmt.Sprintf("ctr -n %s images rm %s", namespace, imageName)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil
		}
		return errors.Wrapf(err, "failed to remove image %s from namespace %s. Stderr: %s", imageName, namespace, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CtrTagImage(ctx context.Context, conn connector.Connector, namespace, sourceImage, targetImage string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || sourceImage == "" || targetImage == "" {
		return errors.New("namespace, sourceImage, and targetImage are required for CtrTagImage")
	}
	cmd := fmt.Sprintf("ctr -n %s images tag %s %s", namespace, sourceImage, targetImage)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		return errors.Wrapf(err, "failed to tag image %s to %s in namespace %s. Stderr: %s", sourceImage, targetImage, namespace, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CtrImportImage(ctx context.Context, conn connector.Connector, namespace, filePath string, allPlatforms bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || filePath == "" {
		return errors.New("namespace and filePath are required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", namespace, "images", "import")
	if allPlatforms {
		cmdArgs = append(cmdArgs, "--all-platforms")
	}
	cmdArgs = append(cmdArgs, filePath)

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to import image from %s. Stderr: %s", filePath, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CtrExportImage(ctx context.Context, conn connector.Connector, namespace, imageName, outputFilePath string, allPlatforms bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || imageName == "" || outputFilePath == "" {
		return errors.New("namespace, imageName, and outputFilePath are required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", namespace, "images", "export")
	if allPlatforms {
		cmdArgs = append(cmdArgs, "--all-platforms")
	}
	cmdArgs = append(cmdArgs, outputFilePath, imageName)

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to export image %s to %s. Stderr: %s", imageName, outputFilePath, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CtrListContainers(ctx context.Context, conn connector.Connector, namespace string) ([]CtrContainerInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if namespace == "" {
		return nil, errors.New("namespace cannot be empty")
	}

	cmd := fmt.Sprintf("ctr -n %s containers ls", namespace)
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list containers in %s. Stderr: %s", namespace, string(stderr))
	}

	var containers []CtrContainerInfo
	lines := strings.Split(string(stdout), "\n")
	if len(lines) <= 1 {
		return containers, nil
	}

	reSpaces := regexp.MustCompile(`\s{2,}`)
	for _, line := range lines[1:] {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		parts := reSpaces.Split(trimmedLine, -1)
		if len(parts) < 1 {
			continue
		}
		cInfo := CtrContainerInfo{ID: parts[0]}
		if len(parts) > 1 {
			cInfo.Image = parts[1]
		}
		if len(parts) > 2 {
			cInfo.Runtime = parts[2]
		}
		containers = append(containers, cInfo)
	}
	return containers, nil
}

func (r *defaultRunner) CtrRunContainer(ctx context.Context, conn connector.Connector, namespace string, opts ContainerdContainerCreateOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if namespace == "" || opts.ImageName == "" || opts.ContainerID == "" {
		return "", errors.New("namespace, imageName, and containerID are required")
	}

	if opts.RemoveExisting {
		_ = r.CtrStopContainer(ctx, conn, namespace, opts.ContainerID, 0)
		_ = r.CtrRemoveContainer(ctx, conn, namespace, opts.ContainerID)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", namespace, "run")
	if opts.Snapshotter != "" {
		cmdArgs = append(cmdArgs, "--snapshotter", opts.Snapshotter)
	}
	if opts.ConfigPath != "" {
		cmdArgs = append(cmdArgs, "--config", opts.ConfigPath)
	}
	if opts.Runtime != "" {
		cmdArgs = append(cmdArgs, "--runtime", opts.Runtime)
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
		cmdArgs = append(cmdArgs, "--user", opts.User)
	}
	if opts.Cwd != "" {
		cmdArgs = append(cmdArgs, "--cwd", opts.Cwd)
	}
	for _, envVar := range opts.Env {
		cmdArgs = append(cmdArgs, "--env", envVar)
	}
	for _, mount := range opts.Mounts {
		cmdArgs = append(cmdArgs, "--mount", mount)
	}
	if len(opts.Platforms) > 0 {
		cmdArgs = append(cmdArgs, "--platform", strings.Join(opts.Platforms, ","))
	}
	cmdArgs = append(cmdArgs, "--rm", opts.ImageName, opts.ContainerID)
	if len(opts.Command) > 0 {
		for _, arg := range opts.Command {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute})
	if err != nil {
		return "", errors.Wrapf(err, "failed to run container %s. Stderr: %s", opts.ContainerID, string(stderr))
	}
	return opts.ContainerID, nil
}

func (r *defaultRunner) CtrStopContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, timeout time.Duration) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || containerID == "" {
		return errors.New("namespace and containerID are required")
	}

	killCmdTerm := fmt.Sprintf("ctr -n %s task kill -s SIGTERM %s", namespace, containerID)
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderrTerm, errTerm := conn.Exec(ctx, killCmdTerm, execOptions)

	if errTerm != nil {
		if strings.Contains(string(stderrTerm), "no such process") || strings.Contains(string(stderrTerm), "not found") {
			return nil
		}
	}
	if timeout > 0 {
		time.Sleep(timeout)
	}

	killCmdKill := fmt.Sprintf("ctr -n %s task kill -s SIGKILL %s", namespace, containerID)
	_, stderrKill, errKill := conn.Exec(ctx, killCmdKill, execOptions)
	if errKill != nil {
		if strings.Contains(string(stderrKill), "no such process") || strings.Contains(string(stderrKill), "not found") {
			return nil
		}
		if errTerm == nil {
			return nil
		}
		return errors.Wrapf(errKill, "failed to SIGKILL task for %s. Stderr: %s", containerID, string(stderrKill))
	}
	return nil
}

func (r *defaultRunner) CtrRemoveContainer(ctx context.Context, conn connector.Connector, namespace, containerID string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if namespace == "" || containerID == "" {
		return errors.New("namespace and containerID are required")
	}

	cmd := fmt.Sprintf("ctr -n %s containers rm %s", namespace, containerID)
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout}
	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		if strings.Contains(string(stderr), "No such container") || strings.Contains(string(stderr), "not found") {
			return nil
		}
		if strings.Contains(string(stderr), "has active task") {
			if stopErr := r.CtrStopContainer(ctx, conn, namespace, containerID, 0); stopErr == nil {
				_, stderrRetry, errRetry := conn.Exec(ctx, cmd, execOptions)
				if errRetry != nil {
					if strings.Contains(string(stderrRetry), "No such container") || strings.Contains(string(stderrRetry), "not found") {
						return nil
					}
					return errors.Wrapf(errRetry, "failed to rm container %s after task kill. Stderr: %s", containerID, string(stderrRetry))
				}
				return nil
			}
		}
		return errors.Wrapf(err, "failed to rm container %s. Stderr: %s", containerID, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CtrExecInContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, opts CtrExecOptions, cmdToExec []string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if namespace == "" || containerID == "" || len(cmdToExec) == 0 {
		return "", errors.New("namespace, containerID, and command are required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "ctr", "-n", namespace, "task", "exec")
	if opts.TTY {
		cmdArgs = append(cmdArgs, "--tty")
	}
	if opts.User != "" {
		cmdArgs = append(cmdArgs, "--user", opts.User)
	}
	if opts.Cwd != "" {
		cmdArgs = append(cmdArgs, "--cwd", opts.Cwd)
	}

	execID := fmt.Sprintf("kubexm-exec-%d", time.Now().UnixNano())
	cmdArgs = append(cmdArgs, "--exec-id", execID)
	cmdArgs = append(cmdArgs, containerID)
	for _, arg := range cmdToExec {
		cmdArgs = append(cmdArgs, arg)
	}

	execTimeout := 5 * time.Minute
	execOptions := &connector.ExecOptions{Sudo: true, Timeout: execTimeout}
	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), execOptions)
	combinedOutput := string(stdout) + string(stderr)
	if err != nil {
		return combinedOutput, errors.Wrapf(err, "failed to exec in container %s. Output: %s", containerID, combinedOutput)
	}
	return combinedOutput, nil
}

func (r *defaultRunner) CtrContainerInfo(ctx context.Context, conn connector.Connector, namespace, containerID string) (*CtrContainerInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil for CtrContainerInfo")
	}
	if namespace == "" {
		return nil, errors.New("namespace is required for CtrContainerInfo")
	}
	if containerID == "" {
		return nil, errors.New("containerID is required for CtrContainerInfo")
	}

	cmd := fmt.Sprintf("ctr -n %s container info %s", namespace, containerID)
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCtrTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "No such container") || strings.Contains(string(stderr), "not found") {
			return nil, nil
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
			parts := strings.Fields(info.Runtime)
			if len(parts) > 0 {
				info.Runtime = parts[0]
			}
		} else if strings.HasPrefix(line, "Labels:") {
			inLabelsSection = true
			continue
		} else if strings.HasPrefix(line, "Spec:") || strings.HasPrefix(line, "Snapshotter:") || line == "" {
			inLabelsSection = false
		}

		if inLabelsSection {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, "\"")
				labels[key] = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrapf(err, "failed to parse ctr container info output for %s", containerID)
	}
	info.Labels = labels
	taskPsCmd := fmt.Sprintf("ctr -n %s task ps %s", namespace, containerID)
	_, _, taskErr := conn.Exec(ctx, taskPsCmd, &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Second})
	if taskErr == nil {
		info.Status = "RUNNING"
	} else {
		if strings.Contains(taskErr.Error(), "no such process") || strings.Contains(taskErr.Error(), "not found") {
			info.Status = "STOPPED" // Or "CREATED" - hard to distinguish without more info
		} else {
			info.Status = "UNKNOWN"
		}
	}

	return &info, nil
}

func (r *defaultRunner) GetContainerdConfig(ctx context.Context, conn connector.Connector) (*ContainerdConfigOptions, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil for GetContainerdConfig")
	}

	configContentBytes, err := r.ReadFile(ctx, conn, containerdConfigPath)
	if err != nil {
		exists, _ := r.Exists(ctx, conn, containerdConfigPath)
		if !exists {
			return &ContainerdConfigOptions{}, nil
		}
		return nil, errors.Wrapf(err, "failed to read containerd config file %s", containerdConfigPath)
	}

	if len(configContentBytes) == 0 {
		return &ContainerdConfigOptions{}, nil
	}

	var opts ContainerdConfigOptions
	if err := toml.Unmarshal(configContentBytes, &opts); err != nil {
		return nil, errors.Wrapf(err, "failed to parse containerd config TOML from %s", containerdConfigPath)
	}

	return &opts, nil
}

func (r *defaultRunner) EnsureDefaultContainerdConfig(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return errors.New("connector cannot be nil for EnsureDefaultContainerdConfig")
	}
	content, err := r.ReadFile(ctx, conn, containerdConfigPath)
	if err == nil && len(strings.TrimSpace(string(content))) > 0 {
		return nil
	}

	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to read existing config at %s", containerdConfigPath)
	}

	templateContent, err := templates.Get("containerd/config.toml.tmpl")
	if err != nil {
		return errors.Wrap(err, "critical: failed to load embedded containerd config template")
	}

	currentFacts := facts
	if currentFacts == nil {
		var errFacts error
		currentFacts, errFacts = r.GatherFacts(ctx, conn)
		if errFacts != nil {
			return errors.Wrap(errFacts, "failed to gather facts for rendering containerd template")
		}
	}

	tmpl, err := template.New("containerd-config").Parse(templateContent)
	if err != nil {
		return errors.Wrap(err, "critical: failed to parse embedded containerd config template")
	}
	templateData := struct {
		SystemdCgroup bool
		SandboxImage  string
	}{
		SystemdCgroup: currentFacts.InitSystem != nil && currentFacts.InitSystem.Type == InitSystemSystemd,
		SandboxImage:  common.DefaultContainerdPauseImage,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, &templateData); err != nil {
		return errors.Wrap(err, "failed to render containerd config template")
	}
	defaultConfigContent := buf.Bytes()
	if err := r.Mkdirp(ctx, conn, filepath.Dir(containerdConfigPath), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create directory for %s", containerdConfigPath)
	}
	if err := r.WriteFile(ctx, conn, defaultConfigContent, containerdConfigPath, "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write default containerd config to %s", containerdConfigPath)
	}

	log := logger.Get()
	log.Info("Default containerd config written, restarting service...", "host", conn.GetConnectionConfig().Host)

	if err := r.DaemonReload(ctx, conn, currentFacts); err != nil {
		r.logger.Errorf("%v Warning: failed to daemon-reload: %v\n", os.Stderr, err)
	}

	if err := r.RestartService(ctx, conn, currentFacts, "containerd"); err != nil {
		if errAlt := r.RestartService(ctx, conn, currentFacts, "containerd.service"); errAlt != nil {
			return errors.Wrapf(errAlt, "failed to restart containerd after writing default config. Original error: %v", err)
		}
	}

	return nil
}

func (r *defaultRunner) ConfigureContainerd(ctx context.Context, conn connector.Connector, facts *Facts, opts ContainerdConfigOptions, restartService bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil for ConfigureContainerd")
	}

	if err := r.EnsureDefaultContainerdConfig(ctx, conn, facts); err != nil {
		return errors.Wrap(err, "failed to ensure base containerd config exists before configuring")
	}

	currentContentBytes, err := r.ReadFile(ctx, conn, containerdConfigPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read containerd config at %s for merging", containerdConfigPath)
	}

	var modifiedContentBytes = currentContentBytes
	var modified bool = false

	updateValue := func(path string, value interface{}) error {
		if value == nil {
			return nil
		}
		v := reflect.ValueOf(value)
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return nil
			}
			value = v.Elem().Interface()
		}

		newBytes, err := tool.SetTomlValue(modifiedContentBytes, path, value)
		if err != nil {
			return errors.Wrapf(err, "failed to set containerd config value for path '%s'", path)
		}

		if !bytes.Equal(modifiedContentBytes, newBytes) {
			modifiedContentBytes = newBytes
			modified = true
		}
		return nil
	}

	if err := updateValue("version", opts.Version); err != nil {
		return err
	}
	if err := updateValue("root", opts.Root); err != nil {
		return err
	}
	if err := updateValue("state", opts.State); err != nil {
		return err
	}
	if err := updateValue("oom_score", opts.OOMScore); err != nil {
		return err
	}
	if err := updateValue("disabled_plugins", opts.DisabledPlugins); err != nil {
		return err
	}

	if opts.GRPC != nil {
		if err := updateValue("grpc.address", opts.GRPC.Address); err != nil {
			return err
		}
		if err := updateValue("grpc.uid", opts.GRPC.UID); err != nil {
			return err
		}
		if err := updateValue("grpc.gid", opts.GRPC.GID); err != nil {
			return err
		}
		if err := updateValue("grpc.max_recv_message_size", opts.GRPC.MaxRecvMsgSize); err != nil {
			return err
		}
		if err := updateValue("grpc.max_send_message_size", opts.GRPC.MaxSendMsgSize); err != nil {
			return err
		}
	}

	if opts.Metrics != nil {
		if err := updateValue("metrics.address", opts.Metrics.Address); err != nil {
			return err
		}
		if err := updateValue("metrics.grpc_histogram", opts.Metrics.GRPCHistogram); err != nil {
			return err
		}
	}

	if opts.PluginConfigs != nil {
		for pluginName, pluginConfig := range *opts.PluginConfigs {
			configMap, ok := pluginConfig.(map[string]interface{})
			if !ok {
				continue
			}

			for key, val := range configMap {
				path := fmt.Sprintf(`plugins."%s".%s`, pluginName, key)
				if err := updateValue(path, val); err != nil {
					return err
				}
			}
		}
	}

	if opts.RegistryMirrors != nil {
		for registry, endpoints := range opts.RegistryMirrors {
			path := fmt.Sprintf(`plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s".endpoint`, registry)
			if err := updateValue(path, endpoints); err != nil {
				return err
			}
		}
	}

	if modified {
		if err := r.WriteFile(ctx, conn, modifiedContentBytes, containerdConfigPath, "0644", true); err != nil {
			return errors.Wrapf(err, "failed to write merged containerd config to %s", containerdConfigPath)
		}

		if restartService {
			currentFacts := facts
			if currentFacts == nil {
				var errFacts error
				currentFacts, errFacts = r.GatherFacts(ctx, conn)
				if errFacts != nil {
					return errors.Wrap(errFacts, "failed to gather facts for containerd restart")
				}
			}

			if err := r.DaemonReload(ctx, conn, currentFacts); err != nil {
				r.logger.Errorf("%v Warning: failed to daemon-reload: %v\n", os.Stderr, err)
			}

			if err := r.RestartService(ctx, conn, currentFacts, "containerd"); err != nil {
				if errAlt := r.RestartService(ctx, conn, currentFacts, "containerd.service"); errAlt != nil {
					return errors.Wrapf(errAlt, "failed to restart containerd. Original error: %v", err)
				}
			}
		}
	}

	return nil
}

func (r *defaultRunner) ConfigureCrictl(ctx context.Context, conn connector.Connector, opts CrictlConfigOptions, configFilePath string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}

	if err := r.EnsureDefaultCrictlConfig(ctx, conn); err != nil {
		return errors.Wrap(err, "failed to ensure base crictl config exists before configuring")
	}

	filePath := configFilePath
	if filePath == "" {
		filePath = crictlConfigPath
	}

	currentContentBytes, err := r.ReadFile(ctx, conn, filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read existing crictl config at %s", filePath)
	}

	newOptsBytes, err := yaml.Marshal(opts)
	if err != nil {
		return errors.Wrap(err, "failed to marshal new crictl options")
	}
	newOptsMap, err := tool.YamlToMap(newOptsBytes)
	if err != nil {
		return errors.Wrap(err, "failed to convert new crictl options to map")
	}

	if len(newOptsMap) == 0 {
		return nil
	}
	var modifiedContentBytes = currentContentBytes
	for k, v := range newOptsMap {
		newBytes, err := tool.SetYamlValue(modifiedContentBytes, k, v)
		if err != nil {
			return errors.Wrapf(err, "failed to set crictl config value for key '%s'", k)
		}
		modifiedContentBytes = newBytes
	}

	if !bytes.Equal(currentContentBytes, modifiedContentBytes) {
		if err := r.WriteFile(ctx, conn, modifiedContentBytes, filePath, "0644", true); err != nil {
			return errors.Wrapf(err, "failed to write updated crictl config to %s", filePath)
		}
	}

	return nil
}

func (r *defaultRunner) CrictlListImages(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlImageInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "images")
	if filters != nil {
		for key, value := range filters {
			if key == "image" || key == "digest" {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key), value)
			} else {
				cmdArgs = append(cmdArgs, "--label", fmt.Sprintf("%s=%s", key, value))
			}
		}
	}
	cmdArgs = append(cmdArgs, "-o", "json")

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl images failed. Stderr: %s", string(stderr))
	}
	var result struct {
		Images []CrictlImageInfo `json:"images"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		if strings.TrimSpace(string(stdout)) == "[]" || strings.TrimSpace(string(stdout)) == "" {
			return []CrictlImageInfo{}, nil
		}
		return nil, errors.Wrapf(err, "failed to parse crictl images JSON. Output: %s", string(stdout))
	}
	return result.Images, nil
}

func (r *defaultRunner) CrictlPullImage(ctx context.Context, conn connector.Connector, imageName string, authCreds string, sandboxConfigPath string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if imageName == "" {
		return errors.New("imageName cannot be empty")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "pull")
	if authCreds != "" {
		cmdArgs = append(cmdArgs, "--auth", authCreds)
	}
	cmdArgs = append(cmdArgs, imageName)

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 15 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "crictl pull %s failed. Stderr: %s", imageName, string(stderr))
	}
	return nil
}
func (r *defaultRunner) CrictlRemoveImage(ctx context.Context, conn connector.Connector, imageName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if imageName == "" {
		return errors.New("imageName cannot be empty")
	}

	cmd := fmt.Sprintf("crictl rmi %s", imageName)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil
		}
		return errors.Wrapf(err, "crictl rmi %s failed. Stderr: %s", imageName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CrictlInspectImage(ctx context.Context, conn connector.Connector, imageName string) (*CrictlImageDetails, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if imageName == "" {
		return nil, errors.New("imageName cannot be empty")
	}

	cmd := fmt.Sprintf("crictl inspecti %s -o json", imageName)
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "crictl inspecti %s failed. Stderr: %s", imageName, string(stderr))
	}
	var details CrictlImageDetails
	if err := json.Unmarshal(stdout, &details); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl inspecti JSON. Output: %s", string(stdout))
	}
	return &details, nil
}

func (r *defaultRunner) CrictlImageFSInfo(ctx context.Context, conn connector.Connector) ([]CrictlFSInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	cmd := "crictl imagefsinfo -o json"
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl imagefsinfo failed. Stderr: %s", string(stderr))
	}
	var result struct {
		FileSystems []CrictlFSInfo `json:"filesystems"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl imagefsinfo JSON. Output: %s", string(stdout))
	}
	return result.FileSystems, nil
}

func (r *defaultRunner) CrictlListPods(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlPodInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "pods")
	if filters != nil {
		for key, value := range filters {
			if key == "label" || key == "name" || key == "namespace" || key == "state" || key == "id" {
				cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key), value)
			}
		}
	}
	cmdArgs = append(cmdArgs, "-o", "json")
	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl pods failed. Stderr: %s", string(stderr))
	}
	var result struct {
		Pods []CrictlPodInfo `json:"items"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		if strings.TrimSpace(string(stdout)) == "[]" || strings.TrimSpace(string(stdout)) == "" || strings.TrimSpace(string(stdout)) == "null" {
			return []CrictlPodInfo{}, nil
		}
		return nil, errors.Wrapf(err, "failed to parse crictl pods JSON. Output: %s", string(stdout))
	}
	return result.Pods, nil
}

func (r *defaultRunner) CrictlRunPodSandbox(ctx context.Context, conn connector.Connector, podSandboxConfigFile string, runtimeHandler string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil for CrictlRunPodSandbox")
	}
	if podSandboxConfigFile == "" {
		return "", errors.New("podSandboxConfigFile is required for CrictlRunPodSandbox")
	}

	exists, err := r.Exists(ctx, conn, podSandboxConfigFile)
	if err != nil {
		return "", errors.Wrapf(err, "failed to check existence of pod sandbox config %s", podSandboxConfigFile)
	}
	if !exists {
		return "", errors.Errorf("pod sandbox config file %s does not exist", podSandboxConfigFile)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "runp")
	if strings.TrimSpace(runtimeHandler) != "" {
		cmdArgs = append(cmdArgs, "--runtime", runtimeHandler)
	}
	cmdArgs = append(cmdArgs, podSandboxConfigFile)

	cmd := strings.Join(cmdArgs, " ")
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "crictl runp failed. Stderr: %s", string(stderr))
	}
	return strings.TrimSpace(string(stdout)), nil
}

func (r *defaultRunner) CrictlStopPodSandbox(ctx context.Context, conn connector.Connector, podID string) error {
	if conn == nil {
		return errors.New("connector cannot be nil for CrictlStopPodSandbox")
	}
	if podID == "" {
		return errors.New("podID is required for CrictlStopPodSandbox")
	}

	cmd := fmt.Sprintf("crictl stopp %s", podID)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") {
			return nil
		}
		return errors.Wrapf(err, "crictl stopp %s failed. Stderr: %s", podID, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CrictlRemovePodSandbox(ctx context.Context, conn connector.Connector, podID string) error {
	if conn == nil {
		return errors.New("connector cannot be nil for CrictlRemovePodSandbox")
	}
	if podID == "" {
		return errors.New("podID is required for CrictlRemovePodSandbox")
	}

	cmd := fmt.Sprintf("crictl rmp %s", podID)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "not found") { // Idempotency
			return nil
		}
		return errors.Wrapf(err, "crictl rmp %s failed. Stderr: %s", podID, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CrictlInspectPod(ctx context.Context, conn connector.Connector, podID string) (*CrictlPodDetails, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil for CrictlInspectPod")
	}
	if podID == "" {
		return nil, errors.New("podID is required for CrictlInspectPod")
	}

	cmd := fmt.Sprintf("crictl inspectp %s -o json", podID)
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

func (r *defaultRunner) CrictlPodSandboxStatus(ctx context.Context, conn connector.Connector, podID string, verbose bool) (*CrictlPodDetails, error) {
	if verbose {
		return r.CrictlInspectPod(ctx, conn, podID)
	}
	return r.CrictlInspectPod(ctx, conn, podID)
}

func (r *defaultRunner) CrictlCreateContainerInPod(ctx context.Context, conn connector.Connector, podID string, containerConfigFile string, podSandboxConfigFile string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil for CrictlCreateContainerInPod")
	}
	if podID == "" {
		return "", errors.New("podID is required for CrictlCreateContainerInPod")
	}
	if containerConfigFile == "" {
		return "", errors.New("containerConfigFile is required for CrictlCreateContainerInPod")
	}
	if podSandboxConfigFile == "" {
		return "", errors.New("podSandboxConfigFile is required for CrictlCreateContainerInPod")
	}

	for _, p := range []string{containerConfigFile, podSandboxConfigFile} {
		exists, err := r.Exists(ctx, conn, p)
		if err != nil {
			return "", errors.Wrapf(err, "failed to check existence of config file %s", p)
		}
		if !exists {
			return "", errors.Errorf("config file %s does not exist", p)
		}
	}

	cmd := fmt.Sprintf("crictl create %s %s %s",
		podID,
		containerConfigFile,
		podSandboxConfigFile)

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "crictl create container in pod %s failed. Stderr: %s", podID, string(stderr))
	}
	return strings.TrimSpace(string(stdout)), nil
}

func (r *defaultRunner) CrictlStartContainerInPod(ctx context.Context, conn connector.Connector, containerID string) error {
	if conn == nil {
		return errors.New("connector cannot be nil for CrictlStartContainerInPod")
	}
	if containerID == "" {
		return errors.New("containerID is required for CrictlStartContainerInPod")
	}

	cmd := fmt.Sprintf("crictl start %s", containerID)
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

func (r *defaultRunner) CrictlStopContainerInPod(ctx context.Context, conn connector.Connector, containerID string, timeout int64) error {
	if conn == nil {
		return errors.New("connector cannot be nil for CrictlStopContainerInPod")
	}
	if containerID == "" {
		return errors.New("containerID is required for CrictlStopContainerInPod")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "stop")
	if timeout >= 0 {
		cmdArgs = append(cmdArgs, "--timeout", fmt.Sprintf("%d", timeout))
	}
	cmdArgs = append(cmdArgs, containerID)
	cmd := strings.Join(cmdArgs, " ")

	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout + time.Duration(timeout)*time.Second})
	if err != nil {
		if strings.Contains(string(stderr), "not found") || strings.Contains(string(stderr), "already stopped") || strings.Contains(string(stderr), "isn't running") {
			return nil
		}
		return errors.Wrapf(err, "crictl stop %s failed. Stderr: %s", containerID, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CrictlRemoveContainerInPod(ctx context.Context, conn connector.Connector, containerID string, force bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil for CrictlRemoveContainerInPod")
	}
	if containerID == "" {
		return errors.New("containerID is required for CrictlRemoveContainerInPod")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "rm")
	if force {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmdArgs = append(cmdArgs, containerID)
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

func (r *defaultRunner) CrictlInspectContainerInPod(ctx context.Context, conn connector.Connector, containerID string) (*CrictlContainerDetails, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil for CrictlInspectContainerInPod")
	}
	if containerID == "" {
		return nil, errors.New("containerID is required for CrictlInspectContainerInPod")
	}

	cmd := fmt.Sprintf("crictl inspect %s -o json", containerID)
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

func (r *defaultRunner) CrictlContainerStatus(ctx context.Context, conn connector.Connector, containerID string, verbose bool) (*CrictlContainerDetails, error) {
	return r.CrictlInspectContainerInPod(ctx, conn, containerID)
}

func (r *defaultRunner) CrictlLogsForContainer(ctx context.Context, conn connector.Connector, containerID string, opts CrictlLogOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil for CrictlLogsForContainer")
	}
	if containerID == "" {
		return "", errors.New("containerID is required for CrictlLogsForContainer")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "logs")
	if opts.Follow {
		cmdArgs = append(cmdArgs, "-f")
	}
	if opts.Timestamps {
		cmdArgs = append(cmdArgs, "--timestamps")
	}
	if opts.Since != "" {
		cmdArgs = append(cmdArgs, "--since", opts.Since)
	}
	if opts.TailLines != nil && *opts.TailLines > 0 {
		cmdArgs = append(cmdArgs, "--tail", fmt.Sprintf("%d", *opts.TailLines))
	} else if opts.NumLines != nil && *opts.NumLines > 0 { // Support for older --lines
		cmdArgs = append(cmdArgs, "--lines", fmt.Sprintf("%d", *opts.NumLines))
	}

	cmdArgs = append(cmdArgs, containerID)
	cmd := strings.Join(cmdArgs, " ")

	logTimeout := DefaultCrictlTimeout
	if opts.Follow {
		logTimeout = 10 * time.Minute
	}

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: logTimeout})
	output := string(stdout) + string(stderr)
	if err != nil {
		if opts.Follow && ctx.Err() != nil {
			return output, nil
		}
		return output, errors.Wrapf(err, "crictl logs for %s failed. Output: %s", containerID, output)
	}
	return string(stdout), nil
}

func (r *defaultRunner) CrictlExecInContainerSync(ctx context.Context, conn connector.Connector, containerID string, timeout time.Duration, cmdToExec []string) (string, string, error) {
	if conn == nil {
		return "", "", errors.New("connector cannot be nil for CrictlExecInContainerSync")
	}
	if containerID == "" {
		return "", "", errors.New("containerID is required for CrictlExecInContainerSync")
	}
	if len(cmdToExec) == 0 {
		return "", "", errors.New("command to exec is required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "exec", "-s")
	if timeout > 0 {
		cmdArgs = append(cmdArgs, "--timeout", fmt.Sprintf("%ds", int(timeout.Seconds())))
	}
	cmdArgs = append(cmdArgs, containerID)
	for _, arg := range cmdToExec {
		cmdArgs = append(cmdArgs, arg)
	}
	cmd := strings.Join(cmdArgs, " ")

	execCmdTimeout := DefaultCrictlTimeout
	if timeout > 0 {
		execCmdTimeout += timeout
	}

	stdoutBytes, stderrBytes, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: execCmdTimeout})
	stdout := string(stdoutBytes)
	stderr := string(stderrBytes)

	if err != nil {
		return stdout, stderr, errors.Wrapf(err, "crictl exec sync in %s failed. Stdout: %s, Stderr: %s", containerID, stdout, stderr)
	}
	return stdout, stderr, nil
}

func (r *defaultRunner) CrictlExecInContainerAsync(ctx context.Context, conn connector.Connector, containerID string, cmdToExec []string) (string, error) {
	return "", errors.New("not implemented: CrictlExecInContainerAsync (crictl does not have a direct async exec that returns a request ID; typically uses attach for async-like behavior)")
}

func (r *defaultRunner) CrictlPortForward(ctx context.Context, conn connector.Connector, podID string, ports []string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil for CrictlPortForward")
	}
	if podID == "" {
		return "", errors.New("podID is required for CrictlPortForward")
	}
	if len(ports) == 0 {
		return "", errors.New("at least one port mapping is required for CrictlPortForward")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "port-forward", podID)
	for _, p := range ports {
		cmdArgs = append(cmdArgs, p)
	}
	cmd := strings.Join(cmdArgs, " ")

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return string(stdout), errors.New("CrictlPortForward started but command is long-running; actual forwarding not guaranteed by this call")
		}
		return string(stdout) + string(stderr), errors.Wrapf(err, "crictl port-forward for pod %s failed. Stderr: %s", podID, string(stderr))
	}
	return string(stdout), nil
}

func (r *defaultRunner) CrictlVersion(ctx context.Context, conn connector.Connector) (*CrictlVersionInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil for CrictlVersion")
	}

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

func (r *defaultRunner) CrictlInfo(ctx context.Context, conn connector.Connector) (*CrictlRuntimeInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil for CrictlInfo")
	}

	cmd := "crictl info -o json"
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "crictl info failed. Stderr: %s", string(stderr))
	}
	var runtimeInfo CrictlRuntimeInfo
	if err := json.Unmarshal(stdout, &runtimeInfo); err != nil {
		return nil, errors.Wrapf(err, "failed to parse crictl info JSON. Output: %s", string(stdout))
	}
	return &runtimeInfo, nil
}

func (r *defaultRunner) CrictlRuntimeConfig(ctx context.Context, conn connector.Connector) (string, error) {
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

func (r *defaultRunner) CrictlStats(ctx context.Context, conn connector.Connector, resourceID string, outputFormat string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil for CrictlStats")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "stats")
	if strings.TrimSpace(outputFormat) == "json" {
		cmdArgs = append(cmdArgs, "-o", "json")
	}
	if strings.TrimSpace(resourceID) != "" {
		cmdArgs = append(cmdArgs, resourceID)
	}

	cmd := strings.Join(cmdArgs, " ")
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return string(stdout) + string(stderr), errors.Wrapf(err, "crictl stats for '%s' failed. Stderr: %s", resourceID, string(stderr))
	}
	return string(stdout), nil
}

func (r *defaultRunner) CrictlPodStats(ctx context.Context, conn connector.Connector, outputFormat string, podID string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil for CrictlPodStats")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "crictl", "stats")
	if strings.TrimSpace(outputFormat) == "json" {
		cmdArgs = append(cmdArgs, "-o", "json")
	}
	if strings.TrimSpace(podID) != "" {
		cmdArgs = append(cmdArgs, podID)
	}
	cmd := strings.Join(cmdArgs, " ")

	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: DefaultCrictlTimeout})
	if err != nil {
		return string(stdout) + string(stderr), errors.Wrapf(err, "crictl pod stats for '%s' failed. Stderr: %s", podID, string(stderr))
	}
	return string(stdout), nil
}

func (r *defaultRunner) EnsureDefaultCrictlConfig(ctx context.Context, conn connector.Connector) error {
	if conn == nil {
		return errors.New("connector cannot be nil for EnsureDefaultCrictlConfig")
	}

	exists, err := r.Exists(ctx, conn, crictlConfigPath)
	if err != nil {
		return errors.Wrapf(err, "failed to check existence of %s", crictlConfigPath)
	}

	if exists {
		content, errRead := r.ReadFile(ctx, conn, crictlConfigPath)
		if errRead == nil && len(strings.TrimSpace(string(content))) > 0 {
			return nil
		}
	}

	templateContent, err := templates.Get("os/crictl.yaml.tmpl")
	if err != nil {
		return errors.Wrap(err, "critical: failed to load embedded crictl config template")
	}

	templateData := struct {
		RuntimeEndpoint string
		ImageEndpoint   string
	}{
		RuntimeEndpoint: common.ContainerdDefaultEndpoint,
		ImageEndpoint:   common.ContainerdDefaultEndpoint,
	}

	tmpl, err := template.New("crictl-config").Parse(templateContent)
	if err != nil {
		return errors.Wrap(err, "critical: failed to parse embedded crictl config template")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, &templateData); err != nil {
		return errors.Wrap(err, "failed to render crictl config template")
	}

	if err := r.Mkdirp(ctx, conn, filepath.Dir(crictlConfigPath), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create directory for %s", crictlConfigPath)
	}
	if err := r.WriteFile(ctx, conn, buf.Bytes(), crictlConfigPath, "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write default crictl config to %s", crictlConfigPath)
	}

	return nil
}

func (r *defaultRunner) EnsureContainerdService(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if facts == nil || facts.InitSystem == nil || facts.InitSystem.Type != InitSystemSystemd {
		return nil
	}

	exists, err := r.Exists(ctx, conn, common.ContainerdDefaultSystemdFile)
	if err != nil {
		return errors.Wrapf(err, "failed to check for existing containerd service file at %s", common.ContainerdDefaultSystemdFile)
	}
	if !exists {
		templateContent, err := templates.Get("containerd/containerd.service.tmpl")
		if err != nil {
			return errors.Wrap(err, "critical: failed to load embedded containerd.service template")
		}

		if err := r.WriteFile(ctx, conn, []byte(templateContent), common.ContainerdDefaultSystemdFile, "0644", true); err != nil {
			return errors.Wrapf(err, "failed to write containerd systemd service file to %s", common.ContainerdDefaultSystemdFile)
		}
		if err := r.DaemonReload(ctx, conn, facts); err != nil {
			return errors.Wrap(err, "failed to run daemon-reload after creating containerd service file")
		}
	}
	return nil
}

func (r *defaultRunner) ConfigureContainerdDropIn(ctx context.Context, conn connector.Connector, facts *Facts, content string) error {
	if facts == nil || facts.InitSystem == nil || facts.InitSystem.Type != InitSystemSystemd {
		return nil
	}
	if err := r.Mkdirp(ctx, conn, filepath.Dir(common.ContainerdDefaultDropInFile), "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create directory for containerd drop-in file")
	}

	if err := r.WriteFile(ctx, conn, []byte(content), common.ContainerdDefaultDropInFile, "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write containerd drop-in file")
	}
	if err := r.DaemonReload(ctx, conn, facts); err != nil {
		return errors.Wrap(err, "failed to run daemon-reload after creating drop-in file")
	}

	return nil
}
