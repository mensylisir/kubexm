package dns

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanCoreDNSStep struct {
	step.Base
	RemoteManifestPath  string
	AdminKubeconfigPath string
}

type CleanCoreDNSStepBuilder struct {
	step.Builder[CleanCoreDNSStepBuilder, *CleanCoreDNSStep]
}

func NewCleanCoreDNSStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanCoreDNSStepBuilder {
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
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Cleanup step will always run if scheduled to ensure resources are removed.")
	return false, nil
}

func (s *CleanCoreDNSStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteManifestPath)
	if err != nil {
		logger.Warn(err, "Failed to check for CoreDNS manifest file, skipping cleanup.", "path", s.RemoteManifestPath)
		result.MarkCompleted("cleanup skipped")
		return result, nil
	}
	if !exists {
		logger.Info("CoreDNS manifest not found on remote host, assuming resources are already cleaned up.", "path", s.RemoteManifestPath)
		result.MarkCompleted("resources already cleaned")
		return result, nil
	}

	cmd := fmt.Sprintf(
		"kubectl delete -f %s --kubeconfig %s --ignore-not-found=true",
		s.RemoteManifestPath,
		s.AdminKubeconfigPath,
	)

	logger.Warn("Cleaning up CoreDNS resources.", "command", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Warn(err, "Failed to delete CoreDNS resources.")
	} else {
		logger.Info("Successfully executed kubectl delete for CoreDNS resources.")
	}

	logger.Info("Removing remote CoreDNS manifest file.", "path", s.RemoteManifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteManifestPath, s.Sudo, true); err != nil {
		logger.Warn(err, "Failed to remove remote CoreDNS manifest file.")
	}

	result.MarkCompleted("cleanup completed")
	return result, nil
}

func (s *CleanCoreDNSStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step. No action taken.")
	return nil
}

var _ step.Step = (*CleanCoreDNSStep)(nil)
