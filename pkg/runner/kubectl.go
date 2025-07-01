package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/mensylisir/kubexm/pkg/connector"
)

const (
	DefaultKubectlTimeout = 2 * time.Minute
)

// KubectlApply applies a configuration to a resource by filename or stdin.
// Corresponds to `kubectl apply -f FILENAME [options]`.
func (r *defaultRunner) KubectlApply(ctx context.Context, conn connector.Connector, opts KubectlApplyOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if len(opts.Filenames) == 0 && opts.FileContent == "" {
		return errors.New("either Filenames or FileContent must be provided for KubectlApply")
	}
	if len(opts.Filenames) > 0 && opts.Filenames[0] == "-" && opts.FileContent == "" {
		return errors.New("FileContent must be provided when filename is '-' (stdin)")
	}


	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "apply")

	for _, filename := range opts.Filenames {
		if filename == "-" {
			cmdArgs = append(cmdArgs, "-f", "-")
		} else {
			cmdArgs = append(cmdArgs, "-f", shellEscape(filename))
		}
	}

	if opts.Recursive {
		cmdArgs = append(cmdArgs, "--recursive")
	}
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace))
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath))
	}
	if opts.Force {
		cmdArgs = append(cmdArgs, "--force")
	}
	if opts.Prune {
		cmdArgs = append(cmdArgs, "--prune")
		if opts.Selector != "" { // Prune selector is typically -l label=value
			cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector))
		}
	}
	if opts.DryRun != "" && opts.DryRun != "none" { // "client", "server"
		cmdArgs = append(cmdArgs, "--dry-run="+shellEscape(opts.DryRun))
	}
	if !opts.Validate { // Default is true, so add flag if false
		cmdArgs = append(cmdArgs, "--validate=false")
	}

	cmd := strings.Join(cmdArgs, " ")

	execOptions := &connector.ExecOptions{
		Sudo:    opts.Sudo,
		Timeout: DefaultKubectlTimeout,
	}

	if len(opts.Filenames) > 0 && opts.Filenames[0] == "-" {
		execOptions.Stdin = []byte(opts.FileContent)
	}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "kubectl apply failed. Stdout: %s, Stderr: %s", string(stdout), string(stderr))
	}
	// Kubectl apply often prints to stdout on success (e.g., "deployment.apps/nginx-deployment configured")
	// We don't parse this success message for now, just ensure no error.
	return nil
}

// KubectlGet retrieves information about one or more resources.
// Corresponds to `kubectl get TYPE [NAME] [options]`.
// This is a generic get, specific typed versions (e.g., GetPods) would parse the output.
func (r *defaultRunner) KubectlGet(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlGetOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if resourceType == "" {
		return "", errors.New("resourceType is required for KubectlGet")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "get", shellEscape(resourceType))
	if resourceName != "" {
		cmdArgs = append(cmdArgs, shellEscape(resourceName))
	}

	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace))
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath))
	}
	if opts.AllNamespaces {
		cmdArgs = append(cmdArgs, "--all-namespaces")
	}
	if opts.OutputFormat != "" {
		cmdArgs = append(cmdArgs, "-o", shellEscape(opts.OutputFormat))
	}
	if opts.Selector != "" {
		cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector))
	}
	if opts.FieldSelector != "" {
		cmdArgs = append(cmdArgs, "--field-selector", shellEscape(opts.FieldSelector))
	}
	if opts.Watch {
		cmdArgs = append(cmdArgs, "--watch")
	}
	if opts.IgnoreNotFound {
		cmdArgs = append(cmdArgs, "--ignore-not-found")
	}
	if opts.ChunkSize > 0 {
		cmdArgs = append(cmdArgs, "--chunk-size", fmt.Sprintf("%d", opts.ChunkSize))
	}
	if opts.ShowLabels {
		cmdArgs = append(cmdArgs, "--show-labels")
	}
	for _, lc := range opts.LabelColumns {
		cmdArgs = append(cmdArgs, "--label-columns", shellEscape(lc))
	}

	cmd := strings.Join(cmdArgs, " ")
	execTimeout := DefaultKubectlTimeout
	if opts.Watch { // Watch can be long-running
		execTimeout = 1 * time.Hour // Arbitrary long timeout for watch, should be context-controlled ideally
	}

	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout}
	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// If IgnoreNotFound is true and error is "not found", return empty string, no error.
		if opts.IgnoreNotFound && (strings.Contains(string(stderr), "NotFound") || strings.Contains(string(stderr), "not found")) {
			return "", nil
		}
		return string(stdout), errors.Wrapf(err, "kubectl get %s %s failed. Stdout: %s, Stderr: %s", resourceType, resourceName, string(stdout), string(stderr))
	}
	return string(stdout), nil
}

// KubectlDelete deletes resources by filenames, stdin, resources and names, or by resources and label selector.
// Corresponds to `kubectl delete (TYPE NAME | TYPE -lSELECTOR | -f FILENAME) [options]`.
func (r *defaultRunner) KubectlDelete(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlDeleteOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	// Validation: Must specify resources by type/name, selector, or filename.
	hasTarget := false
	if resourceType != "" && (resourceName != "" || opts.Selector != "") {
		hasTarget = true
	}
	if len(opts.Filenames) > 0 || opts.FileContent != "" {
		hasTarget = true
	}
	if !hasTarget {
		return errors.New("resources to delete must be specified by type/name, selector, or filename(s) for KubectlDelete")
	}
	if len(opts.Filenames) > 0 && opts.Filenames[0] == "-" && opts.FileContent == "" {
		return errors.New("FileContent must be provided when filename is '-' (stdin) for KubectlDelete")
	}


	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "delete")

	if resourceType != "" {
		cmdArgs = append(cmdArgs, shellEscape(resourceType))
		if resourceName != "" {
			cmdArgs = append(cmdArgs, shellEscape(resourceName))
		}
	}

	for _, filename := range opts.Filenames {
		if filename == "-" {
			cmdArgs = append(cmdArgs, "-f", "-")
		} else {
			cmdArgs = append(cmdArgs, "-f", shellEscape(filename))
		}
	}
	if opts.Recursive { // Used with -f
		cmdArgs = append(cmdArgs, "--recursive")
	}

	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace))
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath))
	}
	if opts.Selector != "" && resourceName == "" { // Selector is usually used when not specifying a name
		cmdArgs = append(cmdArgs, "-l", shellEscape(opts.Selector))
	}
	if opts.Force {
		cmdArgs = append(cmdArgs, "--force") // This implies --grace-period=0
	}
	if opts.GracePeriod != nil {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--grace-period=%d", *opts.GracePeriod))
	}
	if opts.Timeout > 0 {
		cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String())
	}
	if opts.Wait {
		cmdArgs = append(cmdArgs, "--wait")
	}
	if opts.IgnoreNotFound {
		cmdArgs = append(cmdArgs, "--ignore-not-found")
	}
	if opts.Cascade != "" { // "true", "false", "orphan" (kubectl default is true or foreground)
		// Helm 3 uses --cascade= (true|false|orphan|background|foreground)
		// Kubectl uses --cascade=(true|false) or --cascade=orphan (older versions)
		// Newer kubectl uses --cascade=(background|foreground|orphan). Default: foreground.
		// For simplicity, let's map true/false to the boolean flag and orphan separately.
		// This might need adjustment based on kubectl version targeted.
		// Assuming newer kubectl:
		if opts.Cascade == "true" || opts.Cascade == "foreground" {
			cmdArgs = append(cmdArgs, "--cascade=foreground")
		} else if opts.Cascade == "false" || opts.Cascade == "orphan" {
			cmdArgs = append(cmdArgs, "--cascade=orphan")
		} else if opts.Cascade == "background" {
			cmdArgs = append(cmdArgs, "--cascade=background")
		}
		// Older kubectl:
		// if opts.Cascade == "true" { cmdArgs = append(cmdArgs, "--cascade=true") }
		// if opts.Cascade == "false" { cmdArgs = append(cmdArgs, "--cascade=false") }
		// if opts.Cascade == "orphan" { /* handle separately or map to false if appropriate */ }

	}


	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{
		Sudo:    opts.Sudo,
		Timeout: DefaultKubectlTimeout, // Can be overridden by opts.Timeout for the wait part
	}
	if opts.Timeout > 0 && opts.Wait { // If waiting, the exec timeout should be longer
		execOptions.Timeout = opts.Timeout + (1 * time.Minute)
	}


	if len(opts.Filenames) > 0 && opts.Filenames[0] == "-" {
		execOptions.Stdin = []byte(opts.FileContent)
	}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		if opts.IgnoreNotFound && (strings.Contains(string(stderr), "NotFound") || strings.Contains(string(stderr), "not found")) {
			return nil
		}
		return errors.Wrapf(err, "kubectl delete failed. Stdout: %s, Stderr: %s", string(stdout), string(stderr))
	}
	return nil
}

// KubectlVersion gets the client and server Kubernetes version.
// Corresponds to `kubectl version -o json`.
func (r *defaultRunner) KubectlVersion(ctx context.Context, conn connector.Connector) (*KubectlVersionInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}

	cmd := "kubectl version -o json"
	execOptions := &connector.ExecOptions{Sudo: false, Timeout: 30 * time.Second} // Sudo usually not needed

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// `kubectl version` can partially succeed (e.g. client version ok, server error).
		// If stdout contains clientVersion, try to parse it even if there's an error.
		if len(stdout) > 0 {
			var versionInfo KubectlVersionInfo
			if parseErr := json.Unmarshal(stdout, &versionInfo); parseErr == nil && versionInfo.ClientVersion.GitVersion != "" {
				// Return partial info with original error if server part failed
				return &versionInfo, errors.Wrapf(err, "kubectl version command returned an error (server might be unreachable), but client version was parsed. Stderr: %s", string(stderr))
			}
		}
		return nil, errors.Wrapf(err, "kubectl version failed. Stdout: %s, Stderr: %s", string(stdout), string(stderr))
	}

	var versionInfo KubectlVersionInfo
	if err := json.Unmarshal(stdout, &versionInfo); err != nil {
		return nil, errors.Wrapf(err, "failed to parse kubectl version JSON output. Output: %s", string(stdout))
	}
	return &versionInfo, nil
}


// --- Placeholder implementations for other Kubectl methods ---

func (r *defaultRunner) KubectlDescribe(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlDescribeOptions) (string, error) {
	return "", errors.New("not implemented: KubectlDescribe")
}

func (r *defaultRunner) KubectlLogs(ctx context.Context, conn connector.Connector, podName string, opts KubectlLogOptions) (string, error) {
	return "", errors.New("not implemented: KubectlLogs")
}

func (r *defaultRunner) KubectlExec(ctx context.Context, conn connector.Connector, podName, containerName string, command []string, opts KubectlExecOptions) (string, error) {
	return "", errors.New("not implemented: KubectlExec")
}

func (r *defaultRunner) KubectlClusterInfo(ctx context.Context, conn connector.Connector) (string, error) {
	return "", errors.New("not implemented: KubectlClusterInfo")
}

func (r *defaultRunner) KubectlGetNodes(ctx context.Context, conn connector.Connector, opts KubectlGetOptions) ([]KubectlNodeInfo, error) {
	return nil, errors.New("not implemented: KubectlGetNodes")
}

func (r *defaultRunner) KubectlGetPods(ctx context.Context, conn connector.Connector, namespace string, opts KubectlGetOptions) ([]KubectlPodInfo, error) {
	return nil, errors.New("not implemented: KubectlGetPods")
}

// ... and so on for KubectlGetServices, KubectlGetDeployments, etc. ...
// KubectlGetServices, KubectlGetDeployments, KubectlRolloutStatus, KubectlScale, KubectlPortForward
// KubectlConfigView, KubectlConfigGetContexts, KubectlConfigUseContext
// KubectlTopNode, KubectlTopPod

// Add specific Get functions as needed, e.g.
// func (r *defaultRunner) KubectlGetServices(ctx context.Context, conn connector.Connector, namespace string, opts KubectlGetOptions) ([]KubectlServiceInfo, error) {
//  rawJson, err := r.KubectlGet(ctx, conn, "service", "", KubectlGetOptions{Namespace: namespace, OutputFormat: "json", AllNamespaces: opts.AllNamespaces /* pass other opts */})
//  if err != nil { return nil, err }
//  var list struct{ Items []KubectlServiceInfo `json:"items"` }
//  if err := json.Unmarshal([]byte(rawJson), &list); err != nil { return nil, err }
//  return list.Items, nil
// }

// ... other placeholder Kubectl functions ...
func (r *defaultRunner) KubectlGetServices(ctx context.Context, conn connector.Connector, namespace string, opts KubectlGetOptions) (string, error) { // Placeholder, should return []KubectlServiceInfo
    return "", errors.New("not implemented: KubectlGetServices, use KubectlGet with type 'service' and parse JSON")
}
func (r *defaultRunner) KubectlGetDeployments(ctx context.Context, conn connector.Connector, namespace string, opts KubectlGetOptions) (string, error) { // Placeholder
    return "", errors.New("not implemented: KubectlGetDeployments, use KubectlGet with type 'deployment' and parse JSON")
}
func (r *defaultRunner) KubectlRolloutStatus(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlGetOptions /*KubectlRolloutOptions*/) (string, error) {
    return "", errors.New("not implemented: KubectlRolloutStatus")
}
func (r *defaultRunner) KubectlScale(ctx context.Context, conn connector.Connector, resourceType, resourceName string, replicas int, opts KubectlGetOptions /*KubectlScaleOptions*/) (string, error) {
    return "", errors.New("not implemented: KubectlScale")
}
func (r *defaultRunner) KubectlPortForward(ctx context.Context, conn connector.Connector, resourceTypeOrPodName string, resourceNameIfType string, ports []string, opts KubectlGetOptions /* KubectlPortForwardOptions */) (string, error) {
    return "", errors.New("not implemented: KubectlPortForward")
}
func (r *defaultRunner) KubectlConfigView(ctx context.Context, conn connector.Connector, opts KubectlGetOptions /*KubectlConfigViewOptions*/) (string, error) { // Placeholder, should return *KubectlConfigInfo
    return "", errors.New("not implemented: KubectlConfigView")
}
func (r *defaultRunner) KubectlConfigGetContexts(ctx context.Context, conn connector.Connector) (string, error) { // Placeholder, should return []KubectlContextInfo
    return "", errors.New("not implemented: KubectlConfigGetContexts")
}
func (r *defaultRunner) KubectlConfigUseContext(ctx context.Context, conn connector.Connector, contextName string) error {
    return errors.New("not implemented: KubectlConfigUseContext")
}
func (r *defaultRunner) KubectlTopNode(ctx context.Context, conn connector.Connector, nodeName string) (string, error) { // Placeholder *KubectlMetricsInfo
    return "", errors.New("not implemented: KubectlTopNode")
}
func (r *defaultRunner) KubectlTopPod(ctx context.Context, conn connector.Connector, podName string, namespace string, opts KubectlGetOptions /*KubectlTopPodOptions*/) (string, error) { // Placeholder
    return "", errors.New("not implemented: KubectlTopPod")
}
