package haproxy

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"path/filepath"
	"time"
)

type CleanHAProxyStaticPodStep struct {
	step.Base
}

type CleanHAProxyStaticPodStepBuilder struct {
	step.Builder[CleanHAProxyStaticPodStepBuilder, *CleanHAProxyStaticPodStep]
}

func NewCleanHAProxyStaticPodStepBuilder(ctx runtime.Context, instanceName string) *CleanHAProxyStaticPodStepBuilder {
	s := &CleanHAProxyStaticPodStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean HAProxy static pod manifest and related configurations", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(CleanHAProxyStaticPodStepBuilder).Init(s)
	return b
}

func (s *CleanHAProxyStaticPodStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanHAProxyStaticPodStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	manifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-haproxy.yaml")
	configDir := common.HAProxyDefaultConfDirTarget

	manifestExists, err := runner.Exists(ctx.GoContext(), conn, manifestPath)
	if err != nil {
		return false, err
	}
	configExists, err := runner.Exists(ctx.GoContext(), conn, configDir)
	if err != nil {
		return false, err
	}

	if !manifestExists && !configExists {
		ctx.GetLogger().Infof("HAProxy static pod manifest and config directory do not exist. Cleanup is done.")
		return true, nil
	}

	return false, nil
}

func (s *CleanHAProxyStaticPodStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-haproxy.yaml")
	logger.Infof("Removing HAProxy static pod manifest: %s", manifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, manifestPath, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove static pod manifest %s (may not exist): %v", manifestPath, err)
	}

	configDir := common.HAProxyDefaultConfDirTarget
	logger.Infof("Removing HAProxy config directory: %s", configDir)
	if err := runner.Remove(ctx.GoContext(), conn, configDir, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove config directory %s (may not exist): %v", configDir, err)
	}

	logger.Infof("HAProxy static pod resources cleaned up successfully on %s.", ctx.GetHost().GetName())
	return nil
}

func (s *CleanHAProxyStaticPodStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step. No action taken.")
	return nil
}

var _ step.Step = (*CleanHAProxyStaticPodStep)(nil)
