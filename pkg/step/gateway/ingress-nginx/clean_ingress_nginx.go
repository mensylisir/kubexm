package ingressnginx

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanIngressNginxStep struct {
	step.Base
	ReleaseName string
	Namespace   string
}

type CleanIngressNginxStepBuilder struct {
	step.Builder[CleanIngressNginxStepBuilder, *CleanIngressNginxStep]
}

func NewCleanIngressNginxStepBuilder(ctx runtime.Context, instanceName string) *CleanIngressNginxStepBuilder {
	s := &CleanIngressNginxStep{
		ReleaseName: "ingress-nginx",
		Namespace:   "ingress-nginx",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Ingress-Nginx Helm release and cleanup resources", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(CleanIngressNginxStepBuilder).Init(s)
	return b
}

func (s *CleanIngressNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanIngressNginxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			logger.Info("Ingress-Nginx Helm release not found. Step is done.")
			return true, nil
		}
		logger.Warnf("Failed to check Helm status, assuming cleanup is required. Error: %v", err)
		return false, nil
	}

	logger.Info("Ingress-Nginx Helm release found. Cleanup is required.")
	return false, nil
}

func (s *CleanIngressNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	uninstallCmd := fmt.Sprintf("helm uninstall %s -n %s --wait --timeout 5m", s.ReleaseName, s.Namespace)
	logger.Infof("Uninstalling Ingress-Nginx Helm release with command: %s", uninstallCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, uninstallCmd, s.Sudo); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "release: not found") {
			logger.Warnf("Helm uninstall command failed (this may be ok): %v", err)
		}
	}

	deleteNsCmd := fmt.Sprintf("kubectl delete namespace %s --ignore-not-found=true --force --grace-period=0", s.Namespace)
	logger.Infof("Ensuring Ingress-Nginx namespace '%s' is deleted", s.Namespace)
	if _, err := runner.Run(ctx.GoContext(), conn, deleteNsCmd, s.Sudo); err != nil {
		logger.Warnf("Failed to delete Ingress-Nginx namespace: %v", err)
	}

	logger.Info("Ingress-Nginx cleanup process finished.")
	return nil
}

func (s *CleanIngressNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}

var _ step.Step = (*CleanIngressNginxStep)(nil)
