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

type InstallNodeLocalDNSStep struct {
	step.Base
	RemoteManifestPath  string
	AdminKubeconfigPath string
}

type InstallNodeLocalDNSStepBuilder struct {
	step.Builder[InstallNodeLocalDNSStepBuilder, *InstallNodeLocalDNSStep]
}

func NewInstallNodeLocalDNSStepBuilder(ctx runtime.Context, instanceName string) *InstallNodeLocalDNSStepBuilder {
	s := &InstallNodeLocalDNSStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade NodeLocal DNSCache by applying manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	s.RemoteManifestPath = filepath.Join(common.DefaultUploadTmpDir, "dns", "nodelocaldns.yaml")
	s.AdminKubeconfigPath = filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs", common.AdminKubeconfigFileName)

	b := new(InstallNodeLocalDNSStepBuilder).Init(s)
	return b
}

func (s *InstallNodeLocalDNSStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallNodeLocalDNSStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.DNS == nil || clusterCfg.Spec.DNS.NodeLocalDNS == nil || clusterCfg.Spec.DNS.NodeLocalDNS.Enabled == nil || !*clusterCfg.Spec.DNS.NodeLocalDNS.Enabled {
		logger.Info("NodeLocalDNS is disabled, skipping installation.")
		return true, nil
	}

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
		return false, fmt.Errorf("required NodeLocalDNS manifest not found at %s, cannot proceed", s.RemoteManifestPath)
	}

	logger.Info("NodeLocalDNS is enabled and manifest found on remote host. Ready to install.")
	return false, nil
}

func (s *InstallNodeLocalDNSStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Executing remote command to apply NodeLocalDNS manifest: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to apply NodeLocalDNS manifest: %w\nOutput: %s", err, output)
	}

	logger.Info("NodeLocalDNS manifest applied successfully.")
	logger.Debugf("kubectl apply output:\n%s", output)
	return nil
}

func (s *InstallNodeLocalDNSStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.DNS == nil || clusterCfg.Spec.DNS.NodeLocalDNS == nil || clusterCfg.Spec.DNS.NodeLocalDNS.Enabled == nil || !*clusterCfg.Spec.DNS.NodeLocalDNS.Enabled {
		logger.Info("NodeLocalDNS is disabled, skipping rollback.")
		return nil
	}

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

	logger.Warnf("Rolling back by deleting resources from NodeLocalDNS manifest...")
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Errorf("Failed to delete NodeLocalDNS resources (this may be expected if installation failed): %v", err)
	} else {
		logger.Info("Successfully executed kubectl delete for NodeLocalDNS resources.")
	}

	return nil
}

var _ step.Step = (*InstallNodeLocalDNSStep)(nil)
