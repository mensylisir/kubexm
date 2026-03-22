package registry

import (
	"bytes"
	"fmt"
		"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
)

const registryServicePath = "/etc/systemd/system/registry.service"

type SetupRegistryServiceStep struct {
	step.Base
}

type SetupRegistryServiceStepBuilder struct {
	step.Builder[SetupRegistryServiceStepBuilder, *SetupRegistryServiceStep]
}

func NewSetupRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *SetupRegistryServiceStepBuilder {
	// 依赖于 Distribute 步骤，所以不需要再检查 provider
	s := &SetupRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Setup registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(SetupRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *SetupRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetupRegistryServiceStep) renderServiceContent() (string, error) {
	tmplStr := `[Unit]
Description=Docker Registry
After=network.target

[Service]
Type=simple
ExecStart={{.ExecStart}}
Restart=always
RestartSec=10s

[Install]
WantedBy=multi-user.target
`
	execStart := fmt.Sprintf("%s serve %s", filepath.Join(common.DefaultBinDir, "registry"), "/etc/docker/registry/config.yml")
	tmpl, err := template.New("registryService").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"ExecStart": execStart}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *SetupRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	return runner.Exists(ctx.GoContext(), conn, registryServicePath)
}

func (s *SetupRegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	serviceContent, err := s.renderServiceContent()
	if err != nil {
		result.MarkFailed(err, "failed to render service content")
		return result, err
	}

	logger.Infof("Writing registry.service file to %s", registryServicePath)
	if err := helpers.WriteContentToRemote(ctx, conn, serviceContent, registryServicePath, "0644", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to write registry.service file")
		return result, err
	}
	result.MarkCompleted("registry service file written successfully")
	return result, nil
}

func (s *SetupRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_ = runner.Remove(ctx.GoContext(), conn, registryServicePath, s.Sudo, false)
	return nil
}

var _ step.Step = (*SetupRegistryServiceStep)(nil)
