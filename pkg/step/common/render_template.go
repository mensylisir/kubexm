package common

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util"
)

type RenderTemplateStep struct {
	step.Base
	TemplateContent string
	Data            interface{}
	RemoteDestPath  string
	Permissions     string
}

type RenderTemplateStepBuilder struct {
	step.Builder[RenderTemplateStepBuilder, *RenderTemplateStep]
}

func NewRenderTemplateStepBuilder(ctx runtime.ExecutionContext, instanceName, remotePath string) *RenderTemplateStepBuilder {
	cs := &RenderTemplateStep{
		RemoteDestPath: remotePath,
		Permissions:    "0644",
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Render template to [%s]", instanceName, remotePath)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(RenderTemplateStepBuilder).Init(cs)
}

func (b *RenderTemplateStepBuilder) WithTemplate(templateContent string) *RenderTemplateStepBuilder {
	b.Step.TemplateContent = templateContent
	return b
}

func (b *RenderTemplateStepBuilder) WithData(data interface{}) *RenderTemplateStepBuilder {
	b.Step.Data = data
	return b
}

func (b *RenderTemplateStepBuilder) WithPermissions(permissions string) *RenderTemplateStepBuilder {
	b.Step.Permissions = permissions
	return b
}

func (s *RenderTemplateStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderTemplateStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	expectedContent, errRender := util.RenderTemplate(s.TemplateContent, s.Data)
	if errRender != nil {
		return false, fmt.Errorf("failed to render template for precheck on step %s: %w", s.Base.Meta.Name, errRender)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemoteDestPath)
	if err != nil {
		logger.Warnf("Failed to check existence of remote file, assuming it needs to be rendered. Path: %s, Error: %v", s.RemoteDestPath, err)
		return false, nil
	}

	if !exists {
		logger.Infof("Remote destination file does not exist. Template needs to be rendered. Path: %s", s.RemoteDestPath)
		return false, nil
	}

	logger.Infof("Remote destination file exists, reading content for comparison. Path: %s", s.RemoteDestPath)
	remoteContentBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, s.RemoteDestPath)
	if errRead != nil {
		logger.Warnf("Failed to read remote file for comparison, assuming it needs to be re-rendered. Path: %s, Error: %v", s.RemoteDestPath, errRead)
		return false, nil
	}
	remoteContent := string(remoteContentBytes)

	normalizedExpectedContent := strings.ReplaceAll(strings.TrimSpace(expectedContent), "\r\n", "\n")
	normalizedRemoteContent := strings.ReplaceAll(strings.TrimSpace(remoteContent), "\r\n", "\n")

	if normalizedExpectedContent == normalizedRemoteContent {
		logger.Infof("Remote file content matches rendered template. Step considered done. Path: %s", s.RemoteDestPath)
		return true, nil
	}

	logger.Info("Remote file content does not match rendered template. Step needs to run.", "path", s.RemoteDestPath)
	return false, nil
}

func (s *RenderTemplateStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Infof("Rendering template for destination: %s", s.RemoteDestPath)
	renderedContent, err := util.RenderTemplate(s.TemplateContent, s.Data)
	if err != nil {
		return fmt.Errorf("failed to render template for %s: %w", s.RemoteDestPath, err)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Infof("Writing rendered template to remote host. Path: %s, Permissions: %s, Sudo: %t", s.RemoteDestPath, s.Permissions, s.Sudo)
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(renderedContent), s.RemoteDestPath, s.Permissions, s.Sudo)
	if err != nil {
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) {
			logger.Errorf("Failed to write rendered template to remote host. Stderr: %s, Stdout: %s", cmdErr.Stderr, cmdErr.Stdout)
		}
		return fmt.Errorf("failed to write rendered template to %s on host %s: %w", s.RemoteDestPath, ctx.GetHost().GetName(), err)
	}

	logger.Info("Template rendered and written successfully.")
	return nil
}

func (s *RenderTemplateStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Infof("Attempting rollback: removing remote file: %s", s.RemoteDestPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("rollback: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemoteDestPath, s.Sudo, false); err != nil {
		if os.IsNotExist(err) {
			logger.Warnf("Remote file was not present for rollback. Path: %s", s.RemoteDestPath)
			return nil
		}
		logger.Warnf("Failed to remove remote file during rollback (best effort). Path: %s, Error: %v", s.RemoteDestPath, err)
	} else {
		logger.Info("Remote file removed successfully during rollback.")
	}
	return nil
}

var _ step.Step = (*RenderTemplateStep)(nil)
