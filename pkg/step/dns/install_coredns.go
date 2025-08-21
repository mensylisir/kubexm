package dns

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallCoreDNSStep struct {
	step.Base
	RemoteManifestPath  string
	AdminKubeconfigPath string
}

type InstallCoreDNSStepBuilder struct {
	step.Builder[InstallCoreDNSStepBuilder, *InstallCoreDNSStep]
}

func NewInstallCoreDNSStepBuilder(ctx runtime.Context, instanceName string) *InstallCoreDNSStepBuilder {
	s := &InstallCoreDNSStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade CoreDNS by applying manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	s.RemoteManifestPath = filepath.Join(ctx.GetUploadDir(), ctx.GetHost().GetName(), "coredns.yaml")
	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	b := new(InstallCoreDNSStepBuilder).Init(s)
	return b
}

func (s *InstallCoreDNSStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCoreDNSStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	manifestExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteManifestPath)
	if err != nil {
		return false, err
	}
	if !manifestExists {
		return false, fmt.Errorf("required CoreDNS manifest not found at %s, cannot proceed", s.RemoteManifestPath)
	}

	logger.Info("CoreDNS manifest found on remote host. Ready to install.")
	return false, nil
}

func (s *InstallCoreDNSStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"kubectl apply -f %s --kubeconfig %s",
		s.RemoteManifestPath,
		s.AdminKubeconfigPath,
	)

	logger.Info("Executing remote command to apply CoreDNS manifest.", "command", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to apply CoreDNS manifest: %w\nOutput: %s", err, output)
	}

	logger.Info("CoreDNS manifest applied successfully.")
	logger.Debug("kubectl apply output.", "output", output)
	return nil
}

func (s *InstallCoreDNSStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	cmd := fmt.Sprintf(
		"kubectl delete -f %s --kubeconfig %s --ignore-not-found=true",
		s.RemoteManifestPath,
		s.AdminKubeconfigPath,
	)

	logger.Warn("Rolling back by deleting resources from CoreDNS manifest.")
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Error(err, "Failed to delete CoreDNS resources (this may be expected if installation failed).")
	} else {
		logger.Info("Successfully executed kubectl delete for CoreDNS resources.")
	}

	return nil
}

var _ step.Step = (*InstallCoreDNSStep)(nil)
