package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type DistributeCABundleStep struct {
	step.Base
	localBundlePath string
	remoteCAPath    string
}

type DistributeCABundleStepBuilder struct {
	step.Builder[DistributeCABundleStepBuilder, *DistributeCABundleStep]
}

func NewDistributeCAStepBuilder(ctx runtime.Context, instanceName string) *DistributeCABundleStepBuilder {
	localCertsDir := ctx.GetEtcdCertsDir()
	s := &DistributeCABundleStep{
		localBundlePath: filepath.Join(localCertsDir, "ca.pem"),
		remoteCAPath:    filepath.Join(DefaultRemoteEtcdCertsDir, common.EtcdCaPemFileName),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute the transitional CA bundle to the etcd node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(DistributeCABundleStepBuilder).Init(s)
	return b
}

func (s *DistributeCABundleStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeCABundleStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if cleanup is necessary...")
	if !helpers.IsFileExist(s.localBundlePath) {
		logger.Warn("Transitional CA bundle not found. Assuming that cleanup is not necessary.")
		return false, fmt.Errorf("local CA bundle file '%s' not found. Ensure the preparation step ran successfully", s.localBundlePath)
	}
	return false, nil
}

func (s *DistributeCABundleStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	logger.Infof("Distributing CA bundle to %s", s.remoteCAPath)

	bundleContent, err := os.ReadFile(s.localBundlePath)
	if err != nil {
		return fmt.Errorf("failed to read local CA bundle file '%s': %w", s.localBundlePath, err)
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := helpers.WriteContentToRemote(ctx, conn, string(bundleContent), s.remoteCAPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write CA bundle to remote path '%s': %w", s.remoteCAPath, err)
	}

	logger.Info("CA bundle distributed successfully.")
	return nil
}

func (s *DistributeCABundleStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*DistributeCABundleStep)(nil)
