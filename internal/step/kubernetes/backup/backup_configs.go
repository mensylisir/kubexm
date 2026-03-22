package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

const (
	DefaultRemoteK8sConfigDir = "/etc/kubernetes"
)

type BinaryFetchConfigsStep struct {
	step.Base
	remoteConfigDir string
	localNodeDir    string
}

type BinaryFetchConfigsStepBuilder struct {
	step.Builder[BinaryFetchConfigsStepBuilder, *BinaryFetchConfigsStep]
}

func NewBinaryFetchConfigsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *BinaryFetchConfigsStepBuilder {
	s := &BinaryFetchConfigsStep{
		remoteConfigDir: DefaultRemoteK8sConfigDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Fetch the entire /etc/kubernetes directory from the node to a local workspace"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(BinaryFetchConfigsStepBuilder).Init(s)
	return b
}

func (s *BinaryFetchConfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *BinaryFetchConfigsStep) getLocalNodeDir(ctx runtime.ExecutionContext) string {
	baseWorkDir := ctx.GetClusterWorkDir()
	return filepath.Join(baseWorkDir, "remote-configs", ctx.GetHost().GetName())
}

func (s *BinaryFetchConfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if remote Kubernetes configs need to be fetched...")

	s.localNodeDir = s.getLocalNodeDir(ctx)

	if _, err := os.Stat(s.localNodeDir); err == nil {
		entries, err := os.ReadDir(s.localNodeDir)
		if err == nil && len(entries) > 0 {
			logger.Infof("Local config directory for this node ('%s') already exists. Step is done.", s.localNodeDir)
			return true, nil
		}
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	checkCmd := fmt.Sprintf("[ -d %s ]", s.remoteConfigDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: source directory '%s' not found on host '%s'", s.remoteConfigDir, ctx.GetHost().GetName())
	}

	logger.Info("Local config directory is missing or empty. Fetch is required.")
	return false, nil
}

func (s *BinaryFetchConfigsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	s.localNodeDir = s.getLocalNodeDir(ctx)

	if err := os.MkdirAll(s.localNodeDir, 0755); err != nil {
		err = fmt.Errorf("failed to create local node-specific directory '%s': %w", s.localNodeDir, err)
		result.MarkFailed(err, "failed to create local directory")
		return result, err
	}

	logger.Infof("Fetching entire remote config directory from '%s:%s' to local '%s'...",
		ctx.GetHost().GetName(), s.remoteConfigDir, s.localNodeDir)

	if err := runner.Fetch(ctx.GoContext(), conn, s.remoteConfigDir, s.localNodeDir, s.Sudo); err != nil {
		err = fmt.Errorf("failed to fetch remote directory '%s': %w", s.remoteConfigDir, err)
		result.MarkFailed(err, "failed to fetch remote directory")
		return result, err
	}

	logger.Info("Successfully fetched the entire Kubernetes config directory for this node.")
	result.MarkCompleted("configs fetched successfully")
	return result, nil
}

func (s *BinaryFetchConfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	s.localNodeDir = s.getLocalNodeDir(ctx)

	logger.Warnf("Rolling back by deleting fetched config directory from local workspace: '%s'", s.localNodeDir)

	if err := os.RemoveAll(s.localNodeDir); err != nil {
		logger.Errorf("Failed to remove fetched directory '%s' during rollback: %v", s.localNodeDir, err)
	}

	return nil
}

var _ step.Step = (*BinaryFetchConfigsStep)(nil)
