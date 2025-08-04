package kubectl

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type ConfigureKubectlStep struct {
	step.Base
	AdminKubeconfigFile  string
	RemoteKubeconfigDest string
	EnableCompletion     bool
}

type ConfigureKubectlStepBuilder struct {
	step.Builder[ConfigureKubectlStepBuilder, *ConfigureKubectlStep]
}

func NewConfigureKubectlStepBuilder(ctx runtime.Context, instanceName string) *ConfigureKubectlStepBuilder {
	s := &ConfigureKubectlStep{
		AdminKubeconfigFile:  filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName),
		RemoteKubeconfigDest: common.DefaultKubeconfigPath,
		EnableCompletion:     true,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kubectl for root user", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ConfigureKubectlStepBuilder).Init(s)
	return b
}

func (s *ConfigureKubectlStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *ConfigureKubectlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteKubeconfigDest)
	if err != nil {
		return false, err
	}
	if !exists {
		logger.Infof("kubectl config file %s does not exist. Configuration is required.", s.RemoteKubeconfigDest)
		return false, nil
	}

	sourceContent, err := runner.ReadFile(ctx.GoContext(), conn, s.AdminKubeconfigFile)
	if err != nil {
		logger.Warnf("Source admin.conf %s could not be read, cannot verify content. Assuming step needs to run. Error: %v", s.AdminKubeconfigFile, err)
		return false, nil
	}

	destContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteKubeconfigDest)
	if err != nil {
		logger.Warnf("Destination kubectl config %s could not be read, cannot verify content. Assuming step needs to run. Error: %v", s.RemoteKubeconfigDest, err)
		return false, nil
	}

	if string(sourceContent) == string(destContent) {
		logger.Info("kubectl config file is already configured and up to date. Step is done.")
		return true, nil
	}

	logger.Warn("kubectl config file content mismatches with admin.conf. Re-configuration is required.")
	return false, nil
}

func (s *ConfigureKubectlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	kubeDir := filepath.Dir(s.RemoteKubeconfigDest)
	logger.Infof("Ensuring kubectl config directory %s exists", kubeDir)
	if err := runner.Mkdirp(ctx.GoContext(), conn, kubeDir, "0700", s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", kubeDir, err)
	}

	logger.Infof("Copying %s to %s", s.AdminKubeconfigFile, s.RemoteKubeconfigDest)
	copyCmd := fmt.Sprintf("cp %s %s", s.AdminKubeconfigFile, s.RemoteKubeconfigDest)
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, copyCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to copy kubeconfig file: %w", err)
	}

	logger.Infof("Setting ownership of %s", s.RemoteKubeconfigDest)
	if err := runner.Chown(ctx.GoContext(), conn, s.RemoteKubeconfigDest, "root", "root", false); err != nil {
		return fmt.Errorf("failed to set ownership on %s: %w", s.RemoteKubeconfigDest, err)
	}

	if s.EnableCompletion {
		logger.Info("Configuring kubectl bash completion...")
		completionScriptContent, err := templates.Get("kubernetes/kubectl-completion.sh.tmpl")
		if err != nil {
			return fmt.Errorf("failed to get kubectl completion script template: %w", err)
		}

		remoteScriptPath := filepath.Join(common.DefaultUploadTmpDir, "kubectl-completion.sh")
		if err := helpers.WriteContentToRemote(ctx, conn, completionScriptContent, remoteScriptPath, "0755", false); err != nil {
			return fmt.Errorf("failed to write completion script: %w", err)
		}

		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, remoteScriptPath, s.Sudo); err != nil {
			logger.Warnf("Failed to execute kubectl completion script. This is not a fatal error. Error: %v", err)
		} else {
			logger.Info("kubectl bash completion configured successfully.")
		}
		_ = runner.Remove(ctx.GoContext(), conn, remoteScriptPath, false, false)
	}

	logger.Info("kubectl has been configured successfully for the root user.")
	return nil
}

func (s *ConfigureKubectlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.RemoteKubeconfigDest)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteKubeconfigDest, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove kubectl config file during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*ConfigureKubectlStep)(nil)
