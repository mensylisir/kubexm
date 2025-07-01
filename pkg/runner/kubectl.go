package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/utils" // For ParseCPU and ParseMemory
)

const (
	DefaultKubectlTimeout = 2 * time.Minute
)

// KubectlApply applies a configuration to a resource by filename or stdin.
func (r *defaultRunner) KubectlApply(ctx context.Context, conn connector.Connector, opts KubectlApplyOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if len(opts.Filenames) == 0 && opts.FileContent == "" { return errors.New("Filenames or FileContent must be provided") }
	if utils.ContainsString(opts.Filenames, "-") && opts.FileContent == "" { return errors.New("FileContent must be provided with filename '-'")}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "apply")
	for _, filename := range opts.Filenames { cmdArgs = append(cmdArgs, "-f", shellEscape(filename)) }
	if opts.Recursive { cmdArgs = append(cmdArgs, "--recursive") }
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Force { cmdArgs = append(cmdArgs, "--force") }
	if opts.Prune {
		cmdArgs = append(cmdArgs, "--prune")
		if opts.Selector != "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	}
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false") }

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout}
	if utils.ContainsString(opts.Filenames, "-") { execOptions.Stdin = []byte(opts.FileContent) }

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil { return errors.Wrapf(err, "kubectl apply failed. Stdout: %s, Stderr: %s", string(stdout), string(stderr)) }
	return nil
}

// KubectlGet retrieves information about one or more resources.
func (r *defaultRunner) KubectlGet(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlGetOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if resourceType == "" { return "", errors.New("resourceType is required") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "get", shellEscape(resourceType))
	if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.AllNamespaces { cmdArgs = append(cmdArgs, "--all-namespaces") }
	if opts.OutputFormat != "" { cmdArgs = append(cmdArgs, "-o", shellEscape(opts.OutputFormat)) }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.FieldSelector != "" { cmdArgs = append(cmdArgs, "--field-selector", shellEscape(opts.FieldSelector)) }
	if opts.Watch { cmdArgs = append(cmdArgs, "--watch") }
	if opts.IgnoreNotFound { cmdArgs = append(cmdArgs, "--ignore-not-found") }
	if opts.ChunkSize > 0 { cmdArgs = append(cmdArgs, "--chunk-size", fmt.Sprintf("%d", opts.ChunkSize)) }
	if opts.ShowLabels { cmdArgs = append(cmdArgs, "--show-labels") }
	for _, lc := range opts.LabelColumns { cmdArgs = append(cmdArgs, "--label-columns", shellEscape(lc)) }

	cmd := strings.Join(cmdArgs, " ")
	execTimeout := DefaultKubectlTimeout; if opts.Watch { execTimeout = 1 * time.Hour }
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	if err != nil {
		if opts.IgnoreNotFound && (strings.Contains(string(stderr), "NotFound") || strings.Contains(string(stderr), "not found")) { return "", nil }
		return string(stdout), errors.Wrapf(err, "kubectl get %s %s failed. Stdout: %s, Stderr: %s", resourceType, resourceName, string(stdout), string(stderr))
	}
	return string(stdout), nil
}

// KubectlDelete deletes resources.
func (r *defaultRunner) KubectlDelete(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlDeleteOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	hasTarget := (resourceType != "" && (resourceName != "" || opts.Selector != "")) || len(opts.Filenames) > 0 || opts.FileContent != ""
	if !hasTarget { return errors.New("resources to delete must be specified") }
	if utils.ContainsString(opts.Filenames, "-") && opts.FileContent == "" { return errors.New("FileContent must be provided with filename '-'")}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "delete")
	if resourceType != "" {
		cmdArgs = append(cmdArgs, shellEscape(resourceType))
		if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	}
	for _, filename := range opts.Filenames { cmdArgs = append(cmdArgs, "-f", shellEscape(filename)) }
	if opts.Recursive { cmdArgs = append(cmdArgs, "--recursive") }
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" && resourceName == "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.Force { cmdArgs = append(cmdArgs, "--force") }
	if opts.GracePeriod != nil { cmdArgs = append(cmdArgs, fmt.Sprintf("--grace-period=%d", *opts.GracePeriod)) }
	if opts.Timeout > 0 { cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String()) }
	if opts.Wait { cmdArgs = append(cmdArgs, "--wait") }
	if opts.IgnoreNotFound { cmdArgs = append(cmdArgs, "--ignore-not-found") }
	if opts.Cascade != "" { cmdArgs = append(cmdArgs, "--cascade="+shellEscape(opts.Cascade)) }

	cmd := strings.Join(cmdArgs, " ")
	execOpt := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout}
	if opts.Timeout > 0 && opts.Wait { execOpt.Timeout = opts.Timeout + (1 * time.Minute) }
	if utils.ContainsString(opts.Filenames, "-") { execOpt.Stdin = []byte(opts.FileContent) }

	stdout, stderr, err := conn.Exec(ctx, cmd, execOpt)
	if err != nil {
		if opts.IgnoreNotFound && (strings.Contains(string(stderr), "NotFound") || strings.Contains(string(stderr), "not found")) { return nil }
		return errors.Wrapf(err, "kubectl delete failed. Stdout: %s, Stderr: %s", string(stdout), string(stderr))
	}
	return nil
}

// KubectlVersion gets client and server Kubernetes versions.
func (r *defaultRunner) KubectlVersion(ctx context.Context, conn connector.Connector) (*KubectlVersionInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	stdout, stderr, err := conn.Exec(ctx, "kubectl version -o json", &connector.ExecOptions{Sudo: false, Timeout: 30 * time.Second})
	if err != nil {
		var versionInfo KubectlVersionInfo
		if len(stdout) > 0 && json.Unmarshal(stdout, &versionInfo) == nil && versionInfo.ClientVersion.GitVersion != "" {
			return &versionInfo, errors.Wrapf(err, "kubectl version (server error?). Stderr: %s", string(stderr))
		}
		return nil, errors.Wrapf(err, "kubectl version failed. Stdout: %s, Stderr: %s", string(stdout), string(stderr))
	}
	var versionInfo KubectlVersionInfo
	if err := json.Unmarshal(stdout, &versionInfo); err != nil {
		return nil, errors.Wrapf(err, "failed to parse kubectl version JSON. Output: %s", string(stdout))
	}
	return &versionInfo, nil
}

// KubectlDescribe displays detailed information about resources.
func (r *defaultRunner) KubectlDescribe(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlDescribeOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if resourceType == "" { return "", errors.New("resourceType is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "describe", shellEscape(resourceType))
	if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" && resourceName == "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	output := string(stdout) + string(stderr)
	if err != nil { return output, errors.Wrapf(err, "kubectl describe %s %s failed. Output: %s", resourceType, resourceName, output) }
	return string(stdout), nil
}

// KubectlLogs prints logs for a container in a pod.
func (r *defaultRunner) KubectlLogs(ctx context.Context, conn connector.Connector, podName string, opts KubectlLogOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if podName == "" { return "", errors.New("podName is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "logs", shellEscape(podName))
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Container != "" { cmdArgs = append(cmdArgs, "-c", shellEscape(opts.Container)) }
	if opts.Follow { cmdArgs = append(cmdArgs, "-f") }
	if opts.Previous { cmdArgs = append(cmdArgs, "-p") }
	if opts.SinceTime != "" { cmdArgs = append(cmdArgs, "--since-time="+shellEscape(opts.SinceTime)) }
	if opts.SinceSeconds != nil { cmdArgs = append(cmdArgs, fmt.Sprintf("--since=%ds", *opts.SinceSeconds)) }
	if opts.TailLines != nil { cmdArgs = append(cmdArgs, fmt.Sprintf("--tail=%d", *opts.TailLines)) }
	if opts.LimitBytes != nil { cmdArgs = append(cmdArgs, fmt.Sprintf("--limit-bytes=%d", *opts.LimitBytes)) }
	if opts.Timestamps { cmdArgs = append(cmdArgs, "--timestamps") }

	execTimeout := DefaultKubectlTimeout; if opts.Follow { execTimeout = 1 * time.Hour }
	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	if err != nil { return string(stdout), errors.Wrapf(err, "kubectl logs for %s failed. Stdout: %s, Stderr: %s", podName, string(stdout), string(stderr)) }
	return string(stdout), nil
}

// KubectlExec executes a command in a container.
func (r *defaultRunner) KubectlExec(ctx context.Context, conn connector.Connector, podName, containerName string, command []string, opts KubectlExecOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if podName == "" || len(command) == 0 { return "", errors.New("podName and command are required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "exec")
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if containerName != "" { cmdArgs = append(cmdArgs, "-c", shellEscape(containerName)) }
	if opts.Stdin { cmdArgs = append(cmdArgs, "-i") }
	if opts.TTY { cmdArgs = append(cmdArgs, "-t") }
	cmdArgs = append(cmdArgs, shellEscape(podName), "--")
	for _, arg := range command { cmdArgs = append(cmdArgs, shellEscape(arg)) }

	execTimeout := DefaultKubectlTimeout
	if opts.CommandTimeout > 0 { execTimeout = opts.CommandTimeout
	} else if opts.Stdin || opts.TTY { execTimeout = 1 * time.Hour }

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	output := string(stdout) + string(stderr)
	if err != nil { return output, errors.Wrapf(err, "kubectl exec in %s (cmd: %v) failed. Output: %s", podName, command, output) }
	return output, nil
}

// KubectlClusterInfo displays cluster information.
func (r *defaultRunner) KubectlClusterInfo(ctx context.Context, conn connector.Connector) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	stdout, stderr, err := conn.Exec(ctx, "kubectl cluster-info", &connector.ExecOptions{Sudo: false, Timeout: DefaultKubectlTimeout})
	output := string(stdout)+string(stderr)
	if err != nil { return output, errors.Wrapf(err, "kubectl cluster-info failed. Output: %s", output) }
	return string(stdout), nil
}

// KubectlGetNodes retrieves a list of nodes.
func (r *defaultRunner) KubectlGetNodes(ctx context.Context, conn connector.Connector, opts KubectlGetOptions) ([]KubectlNodeInfo, error) {
	opts.OutputFormat = "json"; rawJSON, err := r.KubectlGet(ctx, conn, "nodes", "", opts)
	if err != nil { return nil, errors.Wrap(err, "failed to get nodes raw JSON") }
	if rawJSON == "" { return []KubectlNodeInfo{}, nil }
	var list struct{ Items []KubectlNodeInfo `json:"items"`};
	if err := json.Unmarshal([]byte(rawJSON), &list); err != nil { return nil, errors.Wrapf(err, "failed to unmarshal nodes. Raw: %s", rawJSON) }
	return list.Items, nil
}

// KubectlGetPods retrieves a list of pods.
func (r *defaultRunner) KubectlGetPods(ctx context.Context, conn connector.Connector, namespace string, opts KubectlGetOptions) ([]KubectlPodInfo, error) {
	opts.OutputFormat = "json"; opts.Namespace = namespace
	rawJSON, err := r.KubectlGet(ctx, conn, "pods", "", opts)
	if err != nil { return nil, errors.Wrap(err, "failed to get pods raw JSON") }
	if rawJSON == "" { return []KubectlPodInfo{}, nil }
	var list struct{ Items []KubectlPodInfo `json:"items"`}
	if err := json.Unmarshal([]byte(rawJSON), &list); err != nil { return nil, errors.Wrapf(err, "failed to unmarshal pods. Raw: %s", rawJSON) }
	return list.Items, nil
}

// KubectlGetServices retrieves a list of services.
func (r *defaultRunner) KubectlGetServices(ctx context.Context, conn connector.Connector, namespace string, opts KubectlGetOptions) ([]KubectlServiceInfo, error) {
	opts.OutputFormat = "json"; opts.Namespace = namespace
	rawJSON, err := r.KubectlGet(ctx, conn, "services", "", opts)
	if err != nil { return nil, errors.Wrap(err, "failed to get services raw JSON") }
	if rawJSON == "" { return []KubectlServiceInfo{}, nil }
	var list struct { Items []KubectlServiceInfo `json:"items"` }
	if err := json.Unmarshal([]byte(rawJSON), &list); err != nil { return nil, errors.Wrapf(err, "failed to unmarshal services. Raw: %s", rawJSON) }
	return list.Items, nil
}

// KubectlGetDeployments retrieves a list of deployments.
func (r *defaultRunner) KubectlGetDeployments(ctx context.Context, conn connector.Connector, namespace string, opts KubectlGetOptions) ([]KubectlDeploymentInfo, error) {
	opts.OutputFormat = "json"; opts.Namespace = namespace
	rawJSON, err := r.KubectlGet(ctx, conn, "deployments", "", opts)
	if err != nil { return nil, errors.Wrap(err, "failed to get deployments raw JSON") }
	if rawJSON == "" { return []KubectlDeploymentInfo{}, nil }
	var list struct { Items []KubectlDeploymentInfo `json:"items"`	}
	if err := json.Unmarshal([]byte(rawJSON), &list); err != nil { return nil, errors.Wrapf(err, "failed to unmarshal deployments. Raw: %s", rawJSON) }
	return list.Items, nil
}

// KubectlRolloutStatus checks the status of a rollout.
func (r *defaultRunner) KubectlRolloutStatus(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlRolloutOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if resourceType == "" || resourceName == "" { return "", errors.New("resourceType and resourceName are required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "rollout", "status", fmt.Sprintf("%s/%s", shellEscape(resourceType), shellEscape(resourceName)))
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Watch { cmdArgs = append(cmdArgs, "--watch") }
	if opts.Timeout > 0 { cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String()) }

	execTimeout := DefaultKubectlTimeout
	if opts.Watch && opts.Timeout > 0 { execTimeout = opts.Timeout + 1*time.Minute } else if opts.Watch { execTimeout = 15 * time.Minute }
	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	output := string(stdout) + string(stderr)
	if err != nil { return output, errors.Wrapf(err, "kubectl rollout status for %s/%s failed. Output: %s", resourceType, resourceName, output) }
	return output, nil
}

// KubectlScale scales a resource.
func (r *defaultRunner) KubectlScale(ctx context.Context, conn connector.Connector, resourceType, resourceName string, replicas int, opts KubectlScaleOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if resourceType == "" || resourceName == "" { return "", errors.New("resourceType and resourceName are required")}
	if replicas < 0 { return "", errors.New("replicas must be non-negative")}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "scale", shellEscape(resourceType), shellEscape(resourceName), fmt.Sprintf("--replicas=%d", replicas))
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.CurrentReplicas != nil { cmdArgs = append(cmdArgs, fmt.Sprintf("--current-replicas=%d", *opts.CurrentReplicas)) }
	if opts.ResourceVersion != nil { cmdArgs = append(cmdArgs, "--resource-version=%s", shellEscape(*opts.ResourceVersion)) }
	if opts.Timeout > 0 { cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String()) }

	execTimeout := DefaultKubectlTimeout; if opts.Timeout > 0 { execTimeout = opts.Timeout + 1*time.Minute }
	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	output := string(stdout) + string(stderr)
	if err != nil { return output, errors.Wrapf(err, "kubectl scale for %s/%s failed. Output: %s", resourceType, resourceName, output) }
	return output, nil
}

// KubectlConfigView displays merged kubeconfig settings.
func (r *defaultRunner) KubectlConfigView(ctx context.Context, conn connector.Connector, opts KubectlConfigViewOptions) (*KubectlConfigInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "config", "view", "-o", "json")
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Minify { cmdArgs = append(cmdArgs, "--minify") }
	if opts.Raw { cmdArgs = append(cmdArgs, "--raw") }

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return nil, errors.Wrapf(err, "kubectl config view failed. Stderr: %s", string(stderr)) }
	var cfgInfo KubectlConfigInfo
	if err := json.Unmarshal(stdout, &cfgInfo); err != nil { return nil, errors.Wrapf(err, "failed to parse config view JSON. Output: %s", string(stdout)) }
	return &cfgInfo, nil
}

// KubectlConfigGetContexts displays one or many contexts.
func (r *defaultRunner) KubectlConfigGetContexts(ctx context.Context, conn connector.Connector) ([]KubectlContextInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	fullCfg, err := r.KubectlConfigView(ctx, conn, KubectlConfigViewOptions{OutputFormat: "json"})
	if err != nil { return nil, errors.Wrap(err, "failed to get full config for GetContexts") }
	if fullCfg == nil { return []KubectlContextInfo{}, nil }
	var contexts []KubectlContextInfo
	for _, entry := range fullCfg.Contexts {
		contexts = append(contexts, KubectlContextInfo{
			Name: entry.Name, Cluster: entry.Context.Cluster, AuthInfo: entry.Context.User,
			Namespace: entry.Context.Namespace, Current: entry.Name == fullCfg.CurrentContext,
		})
	}
	return contexts, nil
}

// KubectlConfigUseContext sets the current-context in a kubeconfig file.
func (r *defaultRunner) KubectlConfigUseContext(ctx context.Context, conn connector.Connector, contextName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if contextName == "" { return errors.New("contextName is required") }
	_, stderr, err := conn.Exec(ctx, fmt.Sprintf("kubectl config use-context %s", shellEscape(contextName)), &connector.ExecOptions{Sudo: false, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl config use-context %s failed. Stderr: %s", contextName, string(stderr)) }
	return nil
}

// KubectlTopNodes displays resource (CPU/Memory) usage for nodes.
func (r *defaultRunner) KubectlTopNodes(ctx context.Context, conn connector.Connector, opts KubectlTopOptions) ([]KubectlMetricsInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "top", "nodes")
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "--selector", shellEscape(opts.Selector)) }
	if opts.SortBy != "" { cmdArgs = append(cmdArgs, "--sort-by", shellEscape(opts.SortBy)) }
	if opts.UseHeapster { cmdArgs = append(cmdArgs, "--heapster-namespace", "kube-system") } // Example, actual flags may vary
	cmdArgs = append(cmdArgs, "-o", "json")

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return nil, errors.Wrapf(err, "kubectl top nodes failed. Stderr: %s", string(stderr)) }

	var result struct{ Items []KubectlMetricsInfo `json:"items"` }
	if err := json.Unmarshal(stdout, &result); err != nil { return nil, errors.Wrapf(err, "failed to parse top nodes JSON. Output: %s", string(stdout)) }
	for i := range result.Items {
		if result.Items[i].CPU.UsageNanoCores != "" {
			parsed, _ := utils.ParseCPU(result.Items[i].CPU.UsageNanoCores)
			result.Items[i].CPU.UsageCoreNanos = &parsed
		}
		if result.Items[i].Memory.UsageBytes != "" {
			parsed, _ := utils.ParseMemory(result.Items[i].Memory.UsageBytes)
			result.Items[i].Memory.UsageBytesParsed = &parsed
		}
	}
	return result.Items, nil
}

// KubectlTopPods displays resource (CPU/Memory) usage for pods.
func (r *defaultRunner) KubectlTopPods(ctx context.Context, conn connector.Connector, opts KubectlTopOptions) ([]KubectlMetricsInfo, error) {
	if conn == nil { return nil, errors.New("connector cannot be nil") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "top", "pods")
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.AllNamespaces { cmdArgs = append(cmdArgs, "--all-namespaces") }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "--selector", shellEscape(opts.Selector)) }
	if opts.SortBy != "" { cmdArgs = append(cmdArgs, "--sort-by", shellEscape(opts.SortBy)) }
	if opts.Containers { cmdArgs = append(cmdArgs, "--containers") }
	cmdArgs = append(cmdArgs, "-o", "json")

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return nil, errors.Wrapf(err, "kubectl top pods failed. Stderr: %s", string(stderr)) }

	var result struct{ Items []KubectlMetricsInfo `json:"items"` }
	if err := json.Unmarshal(stdout, &result); err != nil { return nil, errors.Wrapf(err, "failed to parse top pods JSON. Output: %s", string(stdout)) }
	for i := range result.Items {
		// Pod level metrics (if not --containers)
		if result.Items[i].CPU.UsageNanoCores != "" {
			parsed, _ := utils.ParseCPU(result.Items[i].CPU.UsageNanoCores)
			result.Items[i].CPU.UsageCoreNanos = &parsed
		}
		if result.Items[i].Memory.UsageBytes != "" {
			parsed, _ := utils.ParseMemory(result.Items[i].Memory.UsageBytes)
			result.Items[i].Memory.UsageBytesParsed = &parsed
		}
		// Container level metrics
		for j := range result.Items[i].Containers {
			if result.Items[i].Containers[j].CPU.UsageNanoCores != "" {
				parsed, _ := utils.ParseCPU(result.Items[i].Containers[j].CPU.UsageNanoCores)
				result.Items[i].Containers[j].CPU.UsageCoreNanos = &parsed
			}
			if result.Items[i].Containers[j].Memory.UsageBytes != "" {
				parsed, _ := utils.ParseMemory(result.Items[i].Containers[j].Memory.UsageBytes)
				result.Items[i].Containers[j].Memory.UsageBytesParsed = &parsed
			}
		}
	}
	return result.Items, nil
}

// KubectlPortForward forwards one or more local ports to a pod.
// This is a placeholder as true port-forwarding is complex for a simple runner.
func (r *defaultRunner) KubectlPortForward(ctx context.Context, conn connector.Connector, resourceTypeOrPodName string, resourceNameIfType string, ports []string, opts KubectlPortForwardOptions) error {
	return errors.New("KubectlPortForward is not fully implemented in this runner due to its long-running nature")
}

// KubectlExplain gets documentation for a resource.
func (r *defaultRunner) KubectlExplain(ctx context.Context, conn connector.Connector, resourceType string, opts KubectlExplainOptions) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if resourceType == "" { return "", errors.New("resourceType is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "explain", shellEscape(resourceType))
	if opts.APIVersion != "" { cmdArgs = append(cmdArgs, "--api-version", shellEscape(opts.APIVersion)) }
	if opts.Recursive { cmdArgs = append(cmdArgs, "--recursive") }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return string(stdout)+string(stderr), errors.Wrapf(err, "kubectl explain %s failed. Output: %s", resourceType, string(stdout)+string(stderr)) }
	return string(stdout), nil
}

// KubectlDrainNode drains a node in preparation for maintenance.
func (r *defaultRunner) KubectlDrainNode(ctx context.Context, conn connector.Connector, nodeName string, opts KubectlDrainOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if nodeName == "" { return errors.New("nodeName is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "drain", shellEscape(nodeName))
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Force { cmdArgs = append(cmdArgs, "--force") }
	if opts.GracePeriod >= 0 { cmdArgs = append(cmdArgs, fmt.Sprintf("--grace-period=%d", opts.GracePeriod)) } // -1 is default
	if opts.IgnoreDaemonSets { cmdArgs = append(cmdArgs, "--ignore-daemonsets") }
	if opts.DeleteLocalData { cmdArgs = append(cmdArgs, "--delete-emptydir-data") } // Newer kubectl, was --delete-local-data
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "--pod-selector", shellEscape(opts.Selector))} // Drains only pods matching selector
	if opts.Timeout > 0 { cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String()) }
	if opts.DisableEviction { cmdArgs = append(cmdArgs, "--disable-eviction") }
	if opts.SkipWaitForDeleteTimeout > 0 { cmdArgs = append(cmdArgs, fmt.Sprintf("--skip-wait-for-delete-timeout=%d", opts.SkipWaitForDeleteTimeout))}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: opts.Timeout + (5 * time.Minute)}) // Drain can take long
	if err != nil { return errors.Wrapf(err, "kubectl drain %s failed. Stderr: %s", nodeName, string(stderr)) }
	return nil
}

// KubectlCordonNode marks a node as unschedulable.
func (r *defaultRunner) KubectlCordonNode(ctx context.Context, conn connector.Connector, nodeName string, opts KubectlCordonUncordonOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if nodeName == "" { return errors.New("nodeName is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "cordon", shellEscape(nodeName))
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "--selector", shellEscape(opts.Selector))}
	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl cordon %s failed. Stderr: %s", nodeName, string(stderr)) }
	return nil
}

// KubectlUncordonNode marks a node as schedulable.
func (r *defaultRunner) KubectlUncordonNode(ctx context.Context, conn connector.Connector, nodeName string, opts KubectlCordonUncordonOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if nodeName == "" { return errors.New("nodeName is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "uncordon", shellEscape(nodeName))
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "--selector", shellEscape(opts.Selector))}
	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl uncordon %s failed. Stderr: %s", nodeName, string(stderr)) }
	return nil
}

// KubectlTaintNode updates taints on one or more nodes.
func (r *defaultRunner) KubectlTaintNode(ctx context.Context, conn connector.Connector, nodeName string, taints []string, opts KubectlTaintOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if nodeName == "" && !opts.All { return errors.New("nodeName or opts.All is required") }
	if len(taints) == 0 { return errors.New("at least one taint must be specified") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "taint", "nodes")
	if opts.All { cmdArgs = append(cmdArgs, "--all")
	} else { cmdArgs = append(cmdArgs, shellEscape(nodeName)) }
	for _, taint := range taints { cmdArgs = append(cmdArgs, shellEscape(taint)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "--selector", shellEscape(opts.Selector)) }
	if opts.Overwrite { cmdArgs = append(cmdArgs, "--overwrite") }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl taint failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlCreateSecretGeneric creates a generic secret.
func (r *defaultRunner) KubectlCreateSecretGeneric(ctx context.Context, conn connector.Connector, namespace, name string, fromLiterals map[string]string, fromFiles map[string]string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" { return errors.New("secret name is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "secret", "generic", shellEscape(name))
	if namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}
	for k, v := range fromLiterals { cmdArgs = append(cmdArgs, fmt.Sprintf("--from-literal=%s=%s", shellEscape(k), shellEscape(v))) }
	for k, v := range fromFiles { cmdArgs = append(cmdArgs, fmt.Sprintf("--from-file=%s=%s", shellEscape(k), shellEscape(v))) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create secret generic %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateSecretDockerRegistry creates a docker-registry secret.
func (r *defaultRunner) KubectlCreateSecretDockerRegistry(ctx context.Context, conn connector.Connector, namespace, name, dockerServer, dockerUsername, dockerPassword, dockerEmail string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || dockerServer == "" || dockerUsername == "" || dockerPassword == "" { return errors.New("name, server, username, password required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "secret", "docker-registry", shellEscape(name))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--docker-server=%s", shellEscape(dockerServer)))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--docker-username=%s", shellEscape(dockerUsername)))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--docker-password=%s", shellEscape(dockerPassword)))
	if dockerEmail != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--docker-email=%s", shellEscape(dockerEmail))) }
	if namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create secret docker-registry %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateSecretTLS creates a TLS secret.
func (r *defaultRunner) KubectlCreateSecretTLS(ctx context.Context, conn connector.Connector, namespace, name, certPath, keyPath string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || certPath == "" || keyPath == "" { return errors.New("name, certPath, keyPath required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "secret", "tls", shellEscape(name))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--cert=%s", shellEscape(certPath)))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--key=%s", shellEscape(keyPath)))
	if namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create secret tls %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateConfigMap creates a configmap.
func (r *defaultRunner) KubectlCreateConfigMap(ctx context.Context, conn connector.Connector, namespace, name string, fromLiterals map[string]string, fromFiles map[string]string, fromEnvFile string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" { return errors.New("configmap name is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "configmap", shellEscape(name))
	if namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}
	for k, v := range fromLiterals { cmdArgs = append(cmdArgs, fmt.Sprintf("--from-literal=%s=%s", shellEscape(k), shellEscape(v))) }
	for k, v := range fromFiles { cmdArgs = append(cmdArgs, fmt.Sprintf("--from-file=%s=%s", shellEscape(k), shellEscape(v))) }
	if fromEnvFile != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--from-env-file=%s", shellEscape(fromEnvFile))) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create configmap %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateServiceAccount creates a service account.
func (r *defaultRunner) KubectlCreateServiceAccount(ctx context.Context, conn connector.Connector, namespace, name string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" { return errors.New("serviceaccount name is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "serviceaccount", shellEscape(name))
	if namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create serviceaccount %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateRole creates a role.
func (r *defaultRunner) KubectlCreateRole(ctx context.Context, conn connector.Connector, namespace, name string, verbs, resources, resourceNames []string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || len(verbs) == 0 || len(resources) == 0 { return errors.New("name, verbs, and resources are required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "role", shellEscape(name))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--verb=%s", shellEscape(strings.Join(verbs, ","))))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--resource=%s", shellEscape(strings.Join(resources, ","))))
	if len(resourceNames) > 0 { cmdArgs = append(cmdArgs, fmt.Sprintf("--resource-name=%s", shellEscape(strings.Join(resourceNames, ","))))}
	if namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create role %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateClusterRole creates a clusterrole.
func (r *defaultRunner) KubectlCreateClusterRole(ctx context.Context, conn connector.Connector, name string, verbs, resources, resourceNames []string, aggregationRule string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || len(verbs) == 0 || len(resources) == 0 { return errors.New("name, verbs, and resources are required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "clusterrole", shellEscape(name))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--verb=%s", shellEscape(strings.Join(verbs, ","))))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--resource=%s", shellEscape(strings.Join(resources, ","))))
	if len(resourceNames) > 0 { cmdArgs = append(cmdArgs, fmt.Sprintf("--resource-name=%s", shellEscape(strings.Join(resourceNames, ","))))}
	if aggregationRule != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--aggregation-rule=%s", shellEscape(aggregationRule))) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create clusterrole %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateRoleBinding creates a rolebinding.
func (r *defaultRunner) KubectlCreateRoleBinding(ctx context.Context, conn connector.Connector, namespace, name, role, serviceAccount string, users, groups []string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || role == "" { return errors.New("name and role are required") }
	if serviceAccount == "" && len(users) == 0 && len(groups) == 0 { return errors.New("serviceaccount, user, or group is required")}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "rolebinding", shellEscape(name))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--role=%s", shellEscape(role)))
	if serviceAccount != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--serviceaccount=%s:%s", shellEscape(namespace), shellEscape(serviceAccount)))} // Assume SA is in same namespace
	for _, u := range users { cmdArgs = append(cmdArgs, fmt.Sprintf("--user=%s", shellEscape(u))) }
	for _, g := range groups { cmdArgs = append(cmdArgs, fmt.Sprintf("--group=%s", shellEscape(g))) }
	if namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create rolebinding %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlCreateClusterRoleBinding creates a clusterrolebinding.
func (r *defaultRunner) KubectlCreateClusterRoleBinding(ctx context.Context, conn connector.Connector, name, clusterRole, serviceAccount string, users, groups []string, opts KubectlCreateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || clusterRole == "" { return errors.New("name and clusterRole are required") }
	if serviceAccount == "" && len(users) == 0 && len(groups) == 0 { return errors.New("serviceaccount, user, or group is required")}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "create", "clusterrolebinding", shellEscape(name))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--clusterrole=%s", shellEscape(clusterRole)))
	if serviceAccount != "" {
		saParts := strings.Split(serviceAccount, ":") // Expects "namespace:name"
		if len(saParts) != 2 {return errors.New("serviceAccount for ClusterRoleBinding must be in format 'namespace:name'")}
		cmdArgs = append(cmdArgs, fmt.Sprintf("--serviceaccount=%s:%s", shellEscape(saParts[0]), shellEscape(saParts[1])))
	}
	for _, u := range users { cmdArgs = append(cmdArgs, fmt.Sprintf("--user=%s", shellEscape(u))) }
	for _, g := range groups { cmdArgs = append(cmdArgs, fmt.Sprintf("--group=%s", shellEscape(g))) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if !opts.Validate { cmdArgs = append(cmdArgs, "--validate=false")}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl create clusterrolebinding %s failed. Stderr: %s", name, string(stderr)) }
	return nil
}

// KubectlSetImage updates the image of a pod template.
func (r *defaultRunner) KubectlSetImage(ctx context.Context, conn connector.Connector, resourceType, resourceName, containerName, newImage string, opts KubectlSetOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" || newImage == "" { return errors.New("resourceType and newImage are required") }
	// resourceName can be empty if --all or -l is used
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "set", "image", shellEscape(resourceType))
	if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	containerSpec := newImage
	if containerName != "" { containerSpec = fmt.Sprintf("%s=%s", shellEscape(containerName), shellEscape(newImage))
	} else { containerSpec = fmt.Sprintf("*=%s", shellEscape(newImage)) } // Wildcard for all containers if name not specified
	cmdArgs = append(cmdArgs, containerSpec)

	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.All { cmdArgs = append(cmdArgs, "--all") }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.Local { cmdArgs = append(cmdArgs, "--local") }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl set image failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlSetEnv updates environment variables on a pod template.
func (r *defaultRunner) KubectlSetEnv(ctx context.Context, conn connector.Connector, resourceType, resourceName, containerName string, envVars map[string]string, removeEnvVars []string, fromSecret, fromConfigMap string, opts KubectlSetOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" { return errors.New("resourceType is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "set", "env", shellEscape(resourceType))
	if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	if containerName != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--containers=%s", shellEscape(containerName))) } // or -c
	for k,v := range envVars { cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", shellEscape(k), shellEscape(v))) }
	for _, k := range removeEnvVars { cmdArgs = append(cmdArgs, fmt.Sprintf("%s-", shellEscape(k))) } // Suffix with '-' to remove
	if fromSecret != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--from=secret/%s", shellEscape(fromSecret))) }
	if fromConfigMap != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--from=configmap/%s", shellEscape(fromConfigMap))) }

	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.All { cmdArgs = append(cmdArgs, "--all") }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.Local { cmdArgs = append(cmdArgs, "--local") }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl set env failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlSetResources updates resource requests/limits on a pod template.
func (r *defaultRunner) KubectlSetResources(ctx context.Context, conn connector.Connector, resourceType, resourceName, containerName string, limits, requests map[string]string, opts KubectlSetOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" { return errors.New("resourceType is required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "set", "resources", shellEscape(resourceType))
	if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	if containerName != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--containers=%s", shellEscape(containerName))) }

	var limitsArgs, requestsArgs []string
	for k,v := range limits { limitsArgs = append(limitsArgs, fmt.Sprintf("%s=%s", k, v)) }
	if len(limitsArgs) > 0 { cmdArgs = append(cmdArgs, fmt.Sprintf("--limits=%s", shellEscape(strings.Join(limitsArgs, ","))))}
	for k,v := range requests { requestsArgs = append(requestsArgs, fmt.Sprintf("%s=%s", k,v)) }
	if len(requestsArgs) > 0 { cmdArgs = append(cmdArgs, fmt.Sprintf("--requests=%s", shellEscape(strings.Join(requestsArgs, ","))))}

	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.All { cmdArgs = append(cmdArgs, "--all") }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.Local { cmdArgs = append(cmdArgs, "--local") }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl set resources failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlAutoscale creates an HPA that automatically PDB.
func (r *defaultRunner) KubectlAutoscale(ctx context.Context, conn connector.Connector, resourceType, resourceName string, minReplicas, maxReplicas int32, cpuPercent int32, opts KubectlAutoscaleOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" || resourceName == "" || maxReplicas == 0 { return errors.New("resourceType, resourceName, maxReplicas required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "autoscale", shellEscape(resourceType), shellEscape(resourceName))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--min=%d", minReplicas))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--max=%d", maxReplicas))
	if cpuPercent > 0 { cmdArgs = append(cmdArgs, fmt.Sprintf("--cpu-percent=%d", cpuPercent)) }
	if opts.Name != "" { cmdArgs = append(cmdArgs, fmt.Sprintf("--name=%s", shellEscape(opts.Name))) }
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl autoscale failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlCompletion outputs shell completion code.
func (r *defaultRunner) KubectlCompletion(ctx context.Context, conn connector.Connector, shell string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if shell == "" { return "", errors.New("shell is required") }
	cmd := fmt.Sprintf("kubectl completion %s", shellEscape(shell))
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: false, Timeout: DefaultKubectlTimeout})
	if err != nil { return string(stdout)+string(stderr), errors.Wrapf(err, "kubectl completion %s failed. Output: %s", shell, string(stdout)+string(stderr)) }
	return string(stdout), nil
}

// KubectlWait waits for a specific condition on one or many resources.
func (r *defaultRunner) KubectlWait(ctx context.Context, conn connector.Connector, resourceType, resourceName string, condition string, opts KubectlWaitOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" || condition == "" { return errors.New("resourceType and condition are required") }
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "wait")
	if resourceName != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("%s/%s", shellEscape(resourceType), shellEscape(resourceName)))
	} else {
		cmdArgs = append(cmdArgs, shellEscape(resourceType)) // For selector-based wait
	}
	cmdArgs = append(cmdArgs, fmt.Sprintf("--for=%s", shellEscape(condition)))
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.AllNamespaces { cmdArgs = append(cmdArgs, "--all-namespaces") }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.FieldSelector != "" { cmdArgs = append(cmdArgs, "--field-selector", shellEscape(opts.FieldSelector)) }
	if opts.Timeout > 0 { cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String()) }

	execTimeout := DefaultKubectlTimeout; if opts.Timeout > 0 { execTimeout = opts.Timeout + 1*time.Minute } else { execTimeout = 30 * time.Minute } // Default long timeout for wait
	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl wait failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlLabel adds or updates labels for a resource.
func (r *defaultRunner) KubectlLabel(ctx context.Context, conn connector.Connector, resourceType, resourceName string, labels map[string]string, overwrite bool, opts KubectlLabelOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" || (resourceName == "" && opts.Selector == "" && !opts.AllNamespaces) { return errors.New("resourceType and (resourceName or selector or all) required")}
	if len(labels) == 0 { return errors.New("at least one label must be provided")}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "label")
	if overwrite { cmdArgs = append(cmdArgs, "--overwrite") }
	cmdArgs = append(cmdArgs, shellEscape(resourceType))
	if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	for k,v := range labels { cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", shellEscape(k), shellEscape(v))) }

	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.AllNamespaces && resourceName == "" && opts.Selector == "" { cmdArgs = append(cmdArgs, "--all") } // --all is usually for all resources of a type
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" && resourceName == "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.Local { cmdArgs = append(cmdArgs, "--local") }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if opts.Timeout > 0 { cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String()) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl label failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlAnnotate adds or updates annotations for a resource.
func (r *defaultRunner) KubectlAnnotate(ctx context.Context, conn connector.Connector, resourceType, resourceName string, annotations map[string]string, overwrite bool, opts KubectlAnnotateOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" || (resourceName == "" && opts.Selector == "" && !opts.AllNamespaces) { return errors.New("resourceType and (resourceName or selector or all) required")}
	if len(annotations) == 0 { return errors.New("at least one annotation must be provided")}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "annotate")
	if overwrite { cmdArgs = append(cmdArgs, "--overwrite") }
	cmdArgs = append(cmdArgs, shellEscape(resourceType))
	if resourceName != "" { cmdArgs = append(cmdArgs, shellEscape(resourceName)) }
	for k,v := range annotations { cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%s", shellEscape(k), shellEscape(v))) }

	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.AllNamespaces && resourceName == "" && opts.Selector == "" { cmdArgs = append(cmdArgs, "--all") }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Selector != "" && resourceName == "" { cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector)) }
	if opts.Local { cmdArgs = append(cmdArgs, "--local") }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }
	if opts.Timeout > 0 { cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String()) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl annotate failed. Stderr: %s", string(stderr)) }
	return nil
}

// KubectlPatch updates fields of a resource using a patch.
func (r *defaultRunner) KubectlPatch(ctx context.Context, conn connector.Connector, resourceType, resourceName string, patchType, patchContent string, opts KubectlPatchOptions) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if resourceType == "" || resourceName == "" || patchContent == "" { return errors.New("resourceType, resourceName, and patchContent required")}
	if patchType == "" { patchType = "strategic" } // Default patch type
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "patch", shellEscape(resourceType), shellEscape(resourceName))
	cmdArgs = append(cmdArgs, "--type", shellEscape(patchType))
	cmdArgs = append(cmdArgs, "-p", shellEscape(patchContent))
	if opts.Namespace != "" { cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace)) }
	if opts.KubeconfigPath != "" { cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath)) }
	if opts.Local { cmdArgs = append(cmdArgs, "--local") }
	if opts.DryRun != "" && opts.DryRun != "none" { cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun)) }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultKubectlTimeout})
	if err != nil { return errors.Wrapf(err, "kubectl patch failed. Stderr: %s", string(stderr)) }
	return nil
}
