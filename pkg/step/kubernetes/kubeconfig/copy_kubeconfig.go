package kubeconfig

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CopyKubeconfigStep struct {
	step.Base
}
type CopyKubeconfigStepBuilder struct {
	step.Builder[CopyKubeconfigStepBuilder, *CopyKubeconfigStep]
}

func NewCopyKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *CopyKubeconfigStepBuilder {
	s := &CopyKubeconfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy admin.conf to user's .kube directory for kubectl access", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(CopyKubeconfigStepBuilder).Init(s)
	return b
}
func (s *CopyKubeconfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	srcPath := filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)
	user := ctx.GetHost().GetUser()
	if user == "" {
		logger.Warn("No user specified for host, skipping copy of kubeconfig to home directory.")
		return true, nil
	}
	userHomeDir := fmt.Sprintf("/home/%s", user)
	if user == "root" {
		userHomeDir = "/root"
	}
	destPath := filepath.Join(userHomeDir, ".kube", "config")

	srcExists, err := runner.Exists(ctx.GoContext(), conn, srcPath)
	if err != nil || !srcExists {
		logger.Info("Source admin.conf does not exist, pre-check fails.")
		return false, err
	}

	destExists, err := runner.Exists(ctx.GoContext(), conn, destPath)
	if err != nil || !destExists {
		logger.Info("Destination ~/.kube/config does not exist. Step needs to run.")
		return false, nil
	}
	logger.Info("Destination ~/.kube/config already exists. Step is considered done.")
	return true, nil
}

func (s *CopyKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	user := ctx.GetHost().GetUser()
	if user == "" {
		logger.Warn("No user specified for host, skipping copy of kubeconfig.")
		return nil
	}

	srcPath := filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)
	userHomeDir := fmt.Sprintf("/home/%s", user)
	if user == "root" {
		userHomeDir = "/root"
	}
	kubeDir := filepath.Join(userHomeDir, ".kube")
	destPath := filepath.Join(kubeDir, "config")

	mkdirCmd := fmt.Sprintf("mkdir -p %s && chown %s:%s %s", kubeDir, user, user, kubeDir)
	logger.Infof("Creating .kube directory with command: %s", mkdirCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, mkdirCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	cpCmd := fmt.Sprintf("cp -f %s %s", srcPath, destPath)
	logger.Infof("Copying admin.conf with command: %s", cpCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cpCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to copy admin.conf: %w", err)
	}

	chownCmd := fmt.Sprintf("chown %s:%s %s", user, user, destPath)
	logger.Infof("Changing ownership of config file with command: %s", chownCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, chownCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to change ownership of kubeconfig: %w", err)
	}

	logger.Info("Successfully copied admin.conf to user's .kube directory.")
	return nil
}

func (s *CopyKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	user := ctx.GetHost().GetUser()
	if user == "" {
		logger.Warn("No user specified for host, skipping rollback of kubeconfig.")
		return nil
	}

	userHomeDir := fmt.Sprintf("/home/%s", user)
	if user == "root" {
		userHomeDir = "/root"
	}
	destPath := filepath.Join(userHomeDir, ".kube", "config")

	logger.Warnf("Rolling back by removing: %s", destPath)
	if err := runner.Remove(ctx.GoContext(), conn, destPath, true, true); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", destPath, err)
	}

	return nil
}

var _ step.Step = (*CopyKubeconfigStep)(nil)
