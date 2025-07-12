package kube

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubectlDeleteStep executes 'kubectl delete' for a given resource or manifest.
type KubectlDeleteStep struct {
	meta             spec.StepMeta
	ManifestPath     string // Path to the manifest file on the target host (use if deleting by -f)
	ResourceType     string // e.g., "pod", "service", "deployment" (use if deleting by type and name)
	ResourceName     string // Name of the resource (use if deleting by type and name)
	Namespace        string // Namespace of the resource
	KubeconfigPath   string // Optional: Path to kubeconfig file on the target host
	Sudo             bool   // If kubectl command needs sudo
	Force            bool   // Add --force flag
	GracePeriod      int    // Add --grace-period flag (0 for immediate, -1 for default)
	Timeout          string // Add --timeout flag (e.g., "30s")
	IgnoreNotFound   bool   // If true, do not error if the resource is not found
}

// NewKubectlDeleteStep creates a new KubectlDeleteStep.
func NewKubectlDeleteStep(instanceName, manifestPath, resourceType, resourceName, namespace, kubeconfigPath string, sudo, force, ignoreNotFound bool, gracePeriod int, timeout string) step.Step {
	name := instanceName
	if name == "" {
		if manifestPath != "" {
			name = fmt.Sprintf("KubectlDeleteByManifest-%s", manifestPath)
		} else {
			name = fmt.Sprintf("KubectlDelete-%s-%s", resourceType, resourceName)
		}
	}
	return &KubectlDeleteStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Deletes Kubernetes resource(s) (manifest: %s, type: %s, name: %s, ns: %s)", manifestPath, resourceType, resourceName, namespace),
		},
		ManifestPath:     manifestPath,
		ResourceType:     resourceType,
		ResourceName:     resourceName,
		Namespace:        namespace,
		KubeconfigPath:   kubeconfigPath,
		Sudo:             sudo,
		Force:            force,
		GracePeriod:      gracePeriod,
		Timeout:          timeout,
		IgnoreNotFound:   ignoreNotFound,
	}
}

func (s *KubectlDeleteStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubectlDeleteStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	// Precheck for delete: if the resource already doesn't exist, the step is done.
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	var getCmdArgs []string
	getCmdArgs = append(getCmdArgs, "kubectl", "get")

	if s.ManifestPath != "" {
		// Cannot easily check existence from a manifest without applying it or parsing it,
		// which is too complex for a simple Precheck. Assume delete needs to run.
		logger.Info("Deleting by manifest, precheck will assume action is needed.", "manifest", s.ManifestPath)
		return false, nil
	} else if s.ResourceType != "" && s.ResourceName != "" {
		getCmdArgs = append(getCmdArgs, s.ResourceType, s.ResourceName)
		if s.Namespace != "" {
			getCmdArgs = append(getCmdArgs, "-n", s.Namespace)
		}
		if s.KubeconfigPath != "" {
			getCmdArgs = append(getCmdArgs, "--kubeconfig", s.KubeconfigPath)
		}
		// Ignore output fields, just care about exit code for existence
		getCmdArgs = append(getCmdArgs, "-o=name")

		getCmd := strings.Join(getCmdArgs, " ")
		logger.Debug("Checking resource existence for delete precheck", "command", getCmd)
		// We expect an error (non-zero exit code) if the resource is not found.
		_, _, errGet := runnerSvc.RunWithOptions(ctx.GoContext(), conn, getCmd, &connector.ExecOptions{Sudo: s.Sudo, Check: true})
		if errGet != nil {
			if cmdErr, ok := errGet.(*connector.CommandError); ok && cmdErr.ExitCode != 0 { // Non-zero means not found or other error
				// Check stderr for "NotFound"
				if strings.Contains(strings.ToLower(cmdErr.Stderr), "notfound") || strings.Contains(strings.ToLower(cmdErr.Stderr), "no resources found") {
					logger.Info("Resource already not found. Delete step considered done.", "type", s.ResourceType, "name", s.ResourceName)
					return true, nil
				}
			}
			// Some other error occurred trying to get the resource. Let Run proceed.
			logger.Warn("Failed to get resource for precheck, assuming delete is needed.", "error", errGet)
			return false, nil
		}
		// If errGet is nil, it means 'kubectl get' succeeded, so resource exists.
		logger.Info("Resource found. Delete step needs to run.", "type", s.ResourceType, "name", s.ResourceName)
		return false, nil

	} else {
		logger.Warn("Neither ManifestPath nor ResourceType/ResourceName specified for KubectlDeleteStep. Precheck cannot determine target.")
		return false, fmt.Errorf("delete target not specified for step %s", s.meta.Name)
	}
}

func (s *KubectlDeleteStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubectl", "delete")

	if s.ManifestPath != "" {
		cmdArgs = append(cmdArgs, "-f", s.ManifestPath)
	} else if s.ResourceType != "" && s.ResourceName != "" {
		cmdArgs = append(cmdArgs, s.ResourceType, s.ResourceName)
		if s.Namespace != "" {
			cmdArgs = append(cmdArgs, "-n", s.Namespace)
		}
	} else {
		return fmt.Errorf("delete target not specified: must provide ManifestPath or ResourceType and ResourceName for step %s", s.meta.Name)
	}

	if s.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", s.KubeconfigPath)
	}
	if s.Force {
		cmdArgs = append(cmdArgs, "--force")
	}
	if s.GracePeriod >= 0 { // 0 is valid (immediate), -1 means use server default
		cmdArgs = append(cmdArgs, fmt.Sprintf("--grace-period=%d", s.GracePeriod))
	}
	if s.Timeout != "" {
		cmdArgs = append(cmdArgs, "--timeout="+s.Timeout)
	}
	if s.IgnoreNotFound {
		cmdArgs = append(cmdArgs, "--ignore-not-found=true")
	}

	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Running kubectl delete command.", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		// If IgnoreNotFound is true, and the error indicates "not found", then it's not a failure for this step.
		if s.IgnoreNotFound {
			if cmdErr, ok := err.(*connector.CommandError); ok {
				if strings.Contains(strings.ToLower(cmdErr.Stderr), "notfound") || strings.Contains(strings.ToLower(cmdErr.Stdout), "not found") {
					logger.Info("Resource not found, but IgnoreNotFound is true. Delete considered successful.", "output", string(stdout)+string(stderr))
					return nil
				}
			}
		}
		logger.Error("kubectl delete command failed.", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("kubectl delete command '%s' failed: %w. Stdout: %s, Stderr: %s", cmd, err, string(stdout), string(stderr))
	}

	logger.Info("kubectl delete completed.", "output", string(stdout))
	return nil
}

func (s *KubectlDeleteStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for KubectlDeleteStep is not applicable (would mean re-applying or re-creating the resource).")
	return nil
}

var _ step.Step = (*KubectlDeleteStep)(nil)
