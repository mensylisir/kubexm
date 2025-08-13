package cilium

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

type UninstallCiliumHelmChartStep struct {
	step.Base
	ReleaseName string
	Namespace   string
}

type UninstallCiliumHelmChartStepBuilder struct {
	step.Builder[UninstallCiliumHelmChartStepBuilder, *UninstallCiliumHelmChartStep]
}

func NewUninstallCiliumHelmChartStepBuilder(ctx runtime.Context, instanceName string) *UninstallCiliumHelmChartStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	ciliumChart := helmProvider.GetChart("cilium")
	if ciliumChart == nil {
		b := new(UninstallCiliumHelmChartStepBuilder).Init(&UninstallCiliumHelmChartStep{})
		b.Error = fmt.Errorf("cilium chart information not found in BOM")
		return b
	}

	s := &UninstallCiliumHelmChartStep{
		ReleaseName: ciliumChart.ChartName(),
		Namespace:   "kube-system",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Cilium Helm release and CRDs", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(UninstallCiliumHelmChartStepBuilder).Init(s)
	return b
}

func (s *UninstallCiliumHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UninstallCiliumHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	checkCmd := fmt.Sprintf("helm status %s -n %s", s.ReleaseName, s.Namespace)
	output, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err != nil {
		if strings.Contains(strings.ToLower(output), "release: not found") || strings.Contains(strings.ToLower(err.Error()), "release: not found") {
			logger.Info("Cilium Helm release not found. Step is done.")
			return true, nil
		}
		logger.Warnf("Failed to check Helm status, assuming cleanup is required. Error: %v", err)
		return false, nil
	}

	logger.Info("Cilium Helm release found. Cleanup is required.")
	return false, nil
}

func (s *UninstallCiliumHelmChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	uninstallCmd := fmt.Sprintf("helm uninstall %s -n %s --wait --timeout 10m", s.ReleaseName, s.Namespace)
	logger.Infof("Uninstalling Cilium Helm release with command: %s", uninstallCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, uninstallCmd, s.Sudo); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "release: not found") {
			logger.Warnf("Helm uninstall command failed (this may be ok): %v", err)
		}
	}

	logger.Warn("Forcefully deleting Cilium CRDs...")
	crdDeleteCmd := "kubectl get crd -o name | grep 'cilium.io' | xargs -r kubectl delete"
	if _, err := runner.Run(ctx.GoContext(), conn, crdDeleteCmd, s.Sudo); err != nil {
		logger.Warnf("Failed to delete Cilium CRDs (this may be ok if they were already gone): %v", err)
	}

	logger.Info("Cilium cluster resources cleanup finished.")
	return nil
}

func (s *UninstallCiliumHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*UninstallCiliumHelmChartStep)(nil)
