package harbor

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
)

type RemoveHarborCACertStep struct {
	step.Base
	HarborDomain string
}

type RemoveHarborCACertStepBuilder struct {
	step.Builder[RemoveHarborCACertStepBuilder, *RemoveHarborCACertStep]
}

func NewRemoveHarborCACertStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveHarborCACertStepBuilder {
	provider := binary.NewBinaryProvider(ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	if cfg.Registry == nil || cfg.Registry.MirroringAndRewriting == nil || cfg.Registry.MirroringAndRewriting.PrivateRegistry == "" {
		return nil
	}

	domain := cfg.Registry.MirroringAndRewriting.PrivateRegistry
	if u, err := url.Parse("scheme://" + domain); err == nil {
		domain = u.Host
	}

	s := &RemoveHarborCACertStep{
		HarborDomain: domain,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Harbor CA certificate from all nodes", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(RemoveHarborCACertStepBuilder).Init(s)
	return b
}

func (s *RemoveHarborCACertStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveHarborCACertStep) getRemoteCertDirs(ctx runtime.ExecutionContext) []string {
	return []string{
		filepath.Join("/etc/containerd/certs.d", s.HarborDomain),
		filepath.Join("/etc/docker/certs.d", s.HarborDomain),
	}
}

func (s *RemoveHarborCACertStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	oneDirExists := false
	for _, certDir := range s.getRemoteCertDirs(ctx) {
		exists, err := runner.Exists(ctx.GoContext(), conn, certDir)
		if err != nil {
			return false, fmt.Errorf("failed to check for directory '%s': %w", certDir, err)
		}
		if exists {
			oneDirExists = true
			break
		}
	}

	if !oneDirExists {
		logger.Info("Harbor CA certificate directories already removed. Step is done.")
		return true, nil
	}

	logger.Info("Harbor CA certificate directory found. Removal is required.")
	return false, nil
}

// Run 执行删除目录操作。
func (s *RemoveHarborCACertStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	for _, certDir := range s.getRemoteCertDirs(ctx) {
		logger.Infof("Attempting to remove Harbor CA certificate directory: %s", certDir)

		if err := runner.Remove(ctx.GoContext(), conn, certDir, s.Sudo, true); err != nil {
			if os.IsNotExist(err) {
				logger.Debugf("Directory '%s' was not found, assuming it was already removed.", certDir)
				continue
			}
			err := fmt.Errorf("failed to remove directory '%s': %w", certDir, err)
			result.MarkFailed(err, err.Error())
			return result, err
		}
		logger.Infof("Successfully removed directory: %s", certDir)
	}

	logger.Info("Successfully removed Harbor CA certificate configurations from the node.")
	result.MarkCompleted("Harbor CA certificate configurations removed successfully")
	return result, nil
}

func (s *RemoveHarborCACertStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a CA certificate removal step is a no-op.")
	return nil
}

var _ step.Step = (*RemoveHarborCACertStep)(nil)
