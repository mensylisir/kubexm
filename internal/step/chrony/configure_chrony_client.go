package chrony

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

type ChronyClientTemplateData struct {
	NTPServers []string
}

type ConfigureChronyAsClientStep struct {
	step.Base
	ChronyConfPath string
}

type ConfigureChronyAsClientStepBuilder struct {
	step.Builder[ConfigureChronyAsClientStepBuilder, *ConfigureChronyAsClientStep]
}

func NewConfigureChronyAsClientStepBuilder(ctx runtime.ExecutionContext, instanceName string) *ConfigureChronyAsClientStepBuilder {
	s := &ConfigureChronyAsClientStep{
		ChronyConfPath: "/etc/chrony.conf",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Configure chronyd as an NTP client"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ConfigureChronyAsClientStepBuilder).Init(s)
	return b
}

func (s *ConfigureChronyAsClientStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureChronyAsClientStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for chronyd client configuration...")
	logger.Info("Precheck passed: Configuration will be applied.")
	return false, nil
}

func (s *ConfigureChronyAsClientStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Configuring chronyd as a client...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "Failed to get current host connector")
		return result, err
	}

	clusterSpec := ctx.GetClusterConfig().Spec

	if clusterSpec.System == nil || len(clusterSpec.System.NTPServers) == 0 {
		logger.Warn("No NTPServers defined in ClusterSpec, client configuration will be empty. Skipping.")
		result.MarkCompleted("No NTPServers defined, skipping configuration")
		return result, nil
	}

	templateData := ChronyClientTemplateData{
		NTPServers: clusterSpec.System.NTPServers,
	}

	tmpl, err := templates.Get("chrony/chrony.client.conf.tmpl")
	if err != nil {
		result.MarkFailed(fmt.Errorf("failed to get chrony client template: %w", err), "Failed to get chrony client template")
		return result, err
	}

	content, err := templates.Render(tmpl, templateData)
	if err != nil {
		result.MarkFailed(fmt.Errorf("failed to render chrony client config: %w", err), "Failed to render chrony client config")
		return result, err
	}

	logger.Infof("Writing chrony client configuration to %s", s.ChronyConfPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.ChronyConfPath, "0644", s.Sudo); err != nil {
		result.MarkFailed(fmt.Errorf("failed to write chrony client config file: %w", err), "Failed to write chrony client config file")
		return result, err
	}

	logger.Info("Chronyd client configuration applied successfully.")
	result.MarkCompleted("Chronyd client configuration applied successfully")
	return result, nil
}

func (s *ConfigureChronyAsClientStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for chrony configuration is not implemented. Manual restore may be needed.")
	return nil
}

var _ step.Step = (*ConfigureChronyAsClientStep)(nil)
