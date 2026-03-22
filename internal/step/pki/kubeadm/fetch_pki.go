package kubeadm

import (
	"fmt"
	"os"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

const DefaultKubeadmPKIDir = common.DefaultPKIPath

type KubeadmFetchFullPKIStep struct {
	step.Base
	remotePKIDir    string
	localNodePKIDir string
}

type KubeadmFetchFullPKIStepBuilder struct {
	step.Builder[KubeadmFetchFullPKIStepBuilder, *KubeadmFetchFullPKIStep]
}

func NewKubeadmFetchFullPKIStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmFetchFullPKIStepBuilder {
	s := &KubeadmFetchFullPKIStep{
		remotePKIDir: DefaultKubeadmPKIDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Fetch the entire PKI directory from the master node to a local, node-specific workspace"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmFetchFullPKIStepBuilder).Init(s)
	return b
}

func (s *KubeadmFetchFullPKIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmFetchFullPKIStep) getLocalNodePKIDir(ctx runtime.ExecutionContext) string {
	baseLocalCertsDir := ctx.GetKubernetesCertsDir()
	return baseLocalCertsDir
}

func (s *KubeadmFetchFullPKIStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if remote PKI directory needs to be fetched...")

	s.localNodePKIDir = s.getLocalNodePKIDir(ctx)

	if _, err := os.Stat(s.localNodePKIDir); err == nil {
		entries, err := os.ReadDir(s.localNodePKIDir)
		if err == nil && len(entries) > 0 {
			logger.Infof("Local PKI directory for this node ('%s') already exists and is not empty. Step is done.", s.localNodePKIDir)
			return true, nil
		}
	}

	logger.Info("Local PKI directory for this node is missing or empty. Fetch is required.")
	return false, nil
}

func (s *KubeadmFetchFullPKIStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get host connector")
		return result, err
	}

	s.localNodePKIDir = s.getLocalNodePKIDir(ctx)

	if err := os.MkdirAll(s.localNodePKIDir, 0755); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create local node-specific directory '%s'", s.localNodePKIDir))
		return result, err
	}

	logger.Infof("Fetching entire remote PKI directory from '%s:%s' to local '%s'...",
		ctx.GetHost().GetName(), s.remotePKIDir, s.localNodePKIDir)

	if err := runner.Fetch(ctx.GoContext(), conn, s.remotePKIDir, s.localNodePKIDir, s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to fetch remote directory '%s'", s.remotePKIDir))
		return result, err
	}

	logger.Info("Successfully fetched the entire PKI directory for this node.")
	result.MarkCompleted("PKI fetched successfully")
	return result, nil
}

func (s *KubeadmFetchFullPKIStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	s.localNodePKIDir = s.getLocalNodePKIDir(ctx)

	logger.Warnf("Rolling back by deleting fetched PKI directory from local workspace: '%s'", s.localNodePKIDir)

	if err := os.RemoveAll(s.localNodePKIDir); err != nil {
		logger.Errorf("Failed to remove fetched directory '%s' during rollback: %v", s.localNodePKIDir, err)
	}

	return nil
}

var _ step.Step = (*KubeadmFetchFullPKIStep)(nil)
