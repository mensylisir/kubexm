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

type CleanCoreDNSStep struct {
	step.Base
	RemoteManifestPath  string
	AdminKubeconfigPath string
}

type CleanCoreDNSStepBuilder struct {
	step.Builder[CleanCoreDNSStepBuilder, *CleanCoreDNSStep]
}

func NewCleanCoreDNSStepBuilder(ctx runtime.Context, instanceName string) *CleanCoreDNSStepBuilder {
	s := &CleanCoreDNSStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean up CoreDNS resources by deleting from manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	s.RemoteManifestPath = filepath.Join(ctx.GetUploadDir(), ctx.GetHost().GetName(), "coredns.yaml")
	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	b := new(CleanCoreDNSStepBuilder).Init(s)
	return b
}

func (s *CleanCoreDNSStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanCoreDNSStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CleanCoreDNSStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteManifestPath)
	if err != nil {
		logger.Warnf("Failed to check for CoreDNS manifest file, skipping cleanup: %v", err)
		return nil
	}
	if !exists {
		logger.Infof("CoreDNS manifest %s not found on remote host, assuming resources are already cleaned up.", s.RemoteManifestPath)
		return nil
	}

	cmd := fmt.Sprintf(
		"kubectl delete -f %s --kubeconfig %s --ignore-not-found=true",
		s.RemoteManifestPath,
		s.AdminKubeconfigPath,
	)

	logger.Warnf("Cleaning up CoreDNS resources with command: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Warnf("Failed to delete CoreDNS resources: %v", err)
	} else {
		logger.Info("Successfully executed kubectl delete for CoreDNS resources.")
	}

	logger.Infof("Removing remote CoreDNS manifest file: %s", s.RemoteManifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteManifestPath, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove remote CoreDNS manifest file: %v", err)
	}

	return nil
}

func (s *CleanCoreDNSStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a cleanup step. No action taken.")
	return nil
}

var _ step.Step = (*CleanCoreDNSStep)(nil)
