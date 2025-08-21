package addon

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // 严格使用 v1alpha1
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type ApplyAddonYamlStep struct {
	step.Base
	AddonName           string
	SourceIndex         int
	Namespace           string
	RemoteYamlPaths     []string
	AdminKubeconfigPath string
}

type ApplyAddonYamlStepBuilder struct {
	step.Builder[ApplyAddonYamlStepBuilder, *ApplyAddonYamlStep]
}

func NewApplyAddonYamlStepBuilder(ctx runtime.Context, addonName string, sourceIndex int) *ApplyAddonYamlStepBuilder {
	var targetAddon *v1alpha1.Addon
	for i := range ctx.GetClusterConfig().Spec.Addons {
		if ctx.GetClusterConfig().Spec.Addons[i].Name == addonName {
			targetAddon = &ctx.GetClusterConfig().Spec.Addons[i]
			break
		}
	}
	if targetAddon == nil ||
		(targetAddon.Enabled != nil && !*targetAddon.Enabled) ||
		sourceIndex >= len(targetAddon.Sources) ||
		targetAddon.Sources[sourceIndex].Yaml == nil ||
		len(targetAddon.Sources[sourceIndex].Yaml.Path) == 0 {
		return nil
	}

	yamlSource := targetAddon.Sources[sourceIndex].Yaml
	sourceNamespace := targetAddon.Sources[sourceIndex].Namespace

	s := &ApplyAddonYamlStep{
		AddonName:   addonName,
		SourceIndex: sourceIndex,
		Namespace:   sourceNamespace,
	}

	if s.Namespace == "" {
		s.Namespace = "default"
	}

	s.Base.Meta.Name = fmt.Sprintf("ApplyAddonYAML-%s-%d", addonName, sourceIndex)
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply YAML manifests for addon '%s' (source %d)", s.Base.Meta.Name, s.AddonName, s.SourceIndex)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	if targetAddon.TimeoutSeconds != nil {
		s.Base.Timeout = time.Duration(*targetAddon.TimeoutSeconds) * time.Second
	}

	for j := range yamlSource.Path {
		remoteYamlFileName := fmt.Sprintf("%s-yaml-%d-%d.yaml", s.AddonName, s.SourceIndex, j)
		remotePath := filepath.Join(ctx.GetUploadDir(), s.AddonName, yamlSource.Version, remoteYamlFileName)
		s.RemoteYamlPaths = append(s.RemoteYamlPaths, remotePath)
	}

	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	b := new(ApplyAddonYamlStepBuilder).Init(s)
	return b
}

func (s *ApplyAddonYamlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ApplyAddonYamlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	for _, path := range s.RemoteYamlPaths {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			return false, errors.Wrapf(err, "failed to check for remote yaml %s", path)
		}
		if !exists {
			logger.Warn("Required remote yaml manifest not found.", "path", path)
			return false, fmt.Errorf("required remote yaml manifest not found: %s", path)
		}
	}
	logger.Info("All required remote yaml manifests exist. Step will always run to ensure 'apply'.")
	return false, nil
}

func (s *ApplyAddonYamlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	for _, remotePath := range s.RemoteYamlPaths {
		var cmd string
		if s.Namespace != "" {
			cmd = fmt.Sprintf(
				"kubectl apply -f %s --namespace %s --kubeconfig %s",
				remotePath,
				s.Namespace,
				s.AdminKubeconfigPath,
			)
		} else {
			cmd = fmt.Sprintf(
				"kubectl apply -f %s --kubeconfig %s",
				remotePath,
				s.AdminKubeconfigPath,
			)
		}

		logger.Info("Applying YAML manifest.", "command", cmd)
		output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
		if err != nil {
			return errors.Wrapf(err, "failed to apply addon yaml manifest %s\nOutput:\n%s", remotePath, output)
		}
		logger.Debug("Command output.", "output", output)
	}

	logger.Info("Successfully applied all YAML manifests for this addon source.")
	return nil
}

func (s *ApplyAddonYamlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	for i := len(s.RemoteYamlPaths) - 1; i >= 0; i-- {
		remotePath := s.RemoteYamlPaths[i]
		var cmd string
		if s.Namespace != "" {
			cmd = fmt.Sprintf(
				"kubectl delete -f %s --namespace %s --kubeconfig %s --ignore-not-found=true",
				remotePath,
				s.Namespace,
				s.AdminKubeconfigPath,
			)
		} else {
			cmd = fmt.Sprintf(
				"kubectl delete -f %s --kubeconfig %s --ignore-not-found=true",
				remotePath,
				s.AdminKubeconfigPath,
			)
		}

		logger.Warn("Rolling back by deleting manifest.", "command", cmd)
		if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
			logger.Error(err, "Failed to delete addon yaml manifest during rollback.", "path", remotePath)
		}
	}

	logger.Info("Attempted to delete all YAML manifests for this addon source.")
	return nil
}

var _ step.Step = (*ApplyAddonYamlStep)(nil)
