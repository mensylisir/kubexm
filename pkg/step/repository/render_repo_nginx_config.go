package repository

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
	"github.com/pkg/errors"
)

type RenderRepoNginxConfigStep struct {
	step.Base
	RepoDir    string
	ListenPort int
	ConfigPath string
}

type RenderRepoNginxConfigStepBuilder struct {
	step.Builder[RenderRepoNginxConfigStepBuilder, *RenderRepoNginxConfigStep]
}

func NewRenderRepoNginxConfigStepBuilder(ctx runtime.Context, instanceName string) *RenderRepoNginxConfigStepBuilder {
	s := &RenderRepoNginxConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Render NGINX config for repository server", instanceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(RenderRepoNginxConfigStepBuilder).Init(s)
}

func (b *RenderRepoNginxConfigStepBuilder) WithRepoDir(repoDir string) *RenderRepoNginxConfigStepBuilder {
	b.Step.RepoDir = repoDir
	return b
}

func (b *RenderRepoNginxConfigStepBuilder) WithListenPort(port int) *RenderRepoNginxConfigStepBuilder {
	b.Step.ListenPort = port
	return b
}

func (b *RenderRepoNginxConfigStepBuilder) WithConfigPath(path string) *RenderRepoNginxConfigStepBuilder {
	b.Step.ConfigPath = path
	return b
}

func (s *RenderRepoNginxConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderRepoNginxConfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RenderRepoNginxConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	templateContent, err := templates.Get("repository/nginx.conf.tmpl")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get repository nginx config template")
	}

	data := map[string]interface{}{
		"ListenPort": s.ListenPort,
		"RepoDir":    s.RepoDir,
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render repository nginx config template")
	}
	return []byte(renderedConfig), nil
}

func (s *RenderRepoNginxConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Uploading repository NGINX config to %s:%s", ctx.GetHost().GetName(), s.ConfigPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), s.ConfigPath, "0644", s.Sudo); err != nil {
		return errors.Wrap(err, "failed to upload repository nginx config")
	}

	return nil
}

func (s *RenderRepoNginxConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	// Rollback would be to remove the file, handled by the task if needed.
	return nil
}

var _ step.Step = (*RenderRepoNginxConfigStep)(nil)
