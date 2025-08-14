package nginx

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanNginxStaticPodStep struct {
	step.Base
}

type CleanNginxStaticPodStepBuilder struct {
	step.Builder[CleanNginxStaticPodStepBuilder, *CleanNginxStaticPodStep]
}

func NewCleanNginxStaticPodStepBuilder(ctx runtime.Context, instanceName string) *CleanNginxStaticPodStepBuilder {
	s := &CleanNginxStaticPodStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean NGINX static pod manifest and related configurations", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(CleanNginxStaticPodStepBuilder).Init(s)
	return b
}

func (s *CleanNginxStaticPodStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanNginxStaticPodStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	manifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-nginx-lb.yaml")
	configPath := common.DefaultNginxConfigFilePath

	manifestExists, err := runner.Exists(ctx.GoContext(), conn, manifestPath)
	if err != nil {
		return false, err
	}
	configExists, err := runner.Exists(ctx.GoContext(), conn, configPath)
	if err != nil {
		return false, err
	}

	if !manifestExists && !configExists {
		ctx.GetLogger().Infof("NGINX static pod manifest and config file do not exist. Cleanup is done.")
		return true, nil
	}

	return false, nil
}

func (s *CleanNginxStaticPodStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-nginx-lb.yaml")
	logger.Infof("Removing NGINX static pod manifest: %s", manifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, manifestPath, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove static pod manifest %s (may not exist): %v", manifestPath, err)
	}

	configPath := common.DefaultNginxConfigFilePath
	logger.Infof("Removing NGINX config file: %s", configPath)
	if err := runner.Remove(ctx.GoContext(), conn, configPath, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove config file %s (may not exist): %v", configPath, err)
	}

	logger.Infof("NGINX static pod resources cleaned up successfully on %s.", ctx.GetHost().GetName())
	return nil
}

func (s *CleanNginxStaticPodStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step. No action taken.")
	return nil
}

var _ step.Step = (*CleanNginxStaticPodStep)(nil)
