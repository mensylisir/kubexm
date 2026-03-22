package nginx

import (
	"bytes"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

type GenerateNginxConfigStep struct {
	step.Base
}
type GenerateNginxConfigStepBuilder struct {
	step.Builder[GenerateNginxConfigStepBuilder, *GenerateNginxConfigStep]
}

func NewGenerateNginxConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateNginxConfigStepBuilder {
	s := &GenerateNginxConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate NGINX configuration for API Server LB", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateNginxConfigStepBuilder).Init(s)
	return b
}
func (s *GenerateNginxConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type NginxTemplateData struct {
	ListenAddress   string
	ListenPort      int
	UpstreamServers []UpstreamServer
}

type UpstreamServer struct {
	Address string
	Port    int
}

func (s *GenerateNginxConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()

	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterNodes) == 0 {
		return nil, fmt.Errorf("no master nodes found to generate NGINX upstream servers")
	}

	data := NginxTemplateData{
		ListenAddress:   cluster.Spec.ControlPlaneEndpoint.Address,
		ListenPort:      cluster.Spec.ControlPlaneEndpoint.Port,
		UpstreamServers: make([]UpstreamServer, len(masterNodes)),
	}

	for i, node := range masterNodes {
		data.UpstreamServers[i] = UpstreamServer{
			Address: node.GetInternalAddress(),
			Port:    common.DefaultAPIServerPort,
		}
	}

	templateContent, err := templates.Get("loadbalancer/nginx/nginx.conf.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get nginx config template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render nginx config template: %w", err)
	}
	return []byte(renderedConfig), nil
}

func (s *GenerateNginxConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := common.DefaultNginxConfigFilePath
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	expectedContent, err := s.renderContent(ctx)
	if err != nil {
		return false, err
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, err
	}
	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		logger.Info("NGINX configuration is up to date.")
		return true, nil
	}

	logger.Info("NGINX configuration has changed. Step needs to run.")
	return false, nil
}

func (s *GenerateNginxConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render content")
		return result, err
	}

	remoteConfigDir := common.DefaultNginxConfigDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		result.MarkFailed(err, "failed to create remote directory")
		return result, err
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, "nginx.conf")
	logger.Infof("Uploading NGINX configuration to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteConfigPath, "0644", true); err != nil {
		result.MarkFailed(err, "failed to upload NGINX config file")
		return result, err
	}

	logger.Info("NGINX configuration generated and uploaded successfully.")
	result.MarkCompleted("NGINX configuration generated and uploaded successfully")
	return result, nil
}

func (s *GenerateNginxConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteConfigPath := common.DefaultNginxConfigFilePath
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateNginxConfigStep)(nil)
