package network

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type InstallCalicoStep struct {
	step.Base
	PodSubnet           string
	AdminKubeconfigPath string
}

type InstallCalicoStepBuilder struct {
	step.Builder[InstallCalicoStepBuilder, *InstallCalicoStep]
}

func NewInstallCalicoStepBuilder(ctx runtime.Context, instanceName string) *InstallCalicoStepBuilder {
	s := &InstallCalicoStep{
		PodSubnet:           ctx.GetClusterConfig().Spec.Network.KubePodsCIDR,
		AdminKubeconfigPath: filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs", common.AdminKubeconfigFileName),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Calico CNI plugin", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(InstallCalicoStepBuilder).Init(s)
	return b
}

func (s *InstallCalicoStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCalicoStep) render() (string, error) {
	tmplContent, err := templates.Get("cni/calico.yaml.tmpl")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("calico").Parse(tmplContent)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (s *InstallCalicoStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	cmd := fmt.Sprintf("kubectl --kubeconfig=%s get daemonset calico-node -n kube-system", s.AdminKubeconfigPath)
	exists, err := ctx.GetRunner().Check(ctx.GoContext(), nil, cmd, s.Sudo)
	if err != nil {
		logger.Warnf("Failed to check for calico-node daemonset, assuming it does not exist: %v", err)
		return false, nil
	}

	if exists {
		logger.Info("Calico CNI seems to be already installed. Step is done.")
		return true, nil
	}

	return false, nil
}

func (s *InstallCalicoStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()

	manifestContent, err := s.render()
	if err != nil {
		return err
	}
	applyCmd := fmt.Sprintf("kubectl --kubeconfig=%s apply -f -", s.AdminKubeconfigPath)

	logger.Info("Applying Calico CNI manifest to the cluster...")

	execOpts := &connector.ExecOptions{
		Sudo:  s.Sudo,
		Stdin: strings.NewReader(manifestContent),
	}

	stdout, stderr, err := runner.RunWithOptions(ctx.GoContext(), nil, applyCmd, execOpts)
	if err != nil {
		return fmt.Errorf("failed to apply calico manifest: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	logger.Info("Calico CNI manifest applied successfully.")
	logger.Info(string(stdout))

	return nil
}

// Rollback 执行 kubectl delete
func (s *InstallCalicoStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	runner := ctx.GetRunner()

	manifestContent, err := s.render()
	if err != nil {
		logger.Errorf("Failed to render manifest for rollback, skipping deletion: %v", err)
		return nil
	}

	deleteCmd := fmt.Sprintf("kubectl --kubeconfig=%s delete -f -", s.AdminKubeconfigPath)

	logger.Warn("Rolling back by deleting Calico CNI resources...")
	execOpts := &runtime.ExecOptions{
		Sudo:  s.Sudo,
		Stdin: strings.NewReader(manifestContent),
	}

	_, _, err = runner.RunWithOptions(ctx.GoContext(), nil, deleteCmd, execOpts)
	if err != nil {
		logger.Errorf("Failed to delete calico resources during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*InstallCNIStep)(nil) // 假设有一个通用的 CNI 接口
