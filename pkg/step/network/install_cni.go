package network

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type InstallCniStep struct {
	step.Base
	CNIType    common.CNIType
	PodCidr    string
	Kubeconfig string
}

type InstallCniStepBuilder struct {
	step.Builder[InstallCniStepBuilder, *InstallCniStep]
}

func NewInstallCniStepBuilder(ctx runtime.Context, instanceName string) *InstallCniStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	s := &InstallCniStep{
		CNIType:    cfg.Network.Plugin,
		PodCidr:    "10.244.0.0/16",
		Kubeconfig: common.DefaultKubeConfig,
	}

	if cfg.Network.PodCidr != "" {
		s.PodCidr = cfg.Network.PodCidr
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI provider", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCniStepBuilder).Init(s)
	return b
}

func (s *InstallCniStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCniStep) getTemplatePath() string {
	return fmt.Sprintf("cni/%s.yaml.tmpl", s.CNIType)
}

func (s *InstallCniStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(s.getTemplatePath())
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("cni.yaml").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse cni template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render cni template: %w", err)
	}
	return buf.String(), nil
}

func (s *InstallCniStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *InstallCniStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	content, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Installing CNI provider %s", s.CNIType)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("kubectl --kubeconfig=%s apply -f -", s.Kubeconfig), s.Sudo, bytes.NewReader([]byte(content)))
	if err != nil {
		return fmt.Errorf("failed to install CNI provider: %w", err)
	}

	return nil
}

func (s *InstallCniStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	content, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	logger.Warnf("Rolling back by deleting CNI provider %s", s.CNIType)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("kubectl --kubeconfig=%s delete -f -", s.Kubeconfig), s.Sudo, bytes.NewReader([]byte(content)))
	if err != nil {
		logger.Errorf("Failed to delete CNI provider during rollback: %v", err)
	}

	return nil
}
