package docker

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const (
	dockerDropInFilePath     = common.DockerDefaultDropInFile
	dockerDropInTemplatePath = "docker/kubexm-docker-dropin.conf.tmpl"
)

type DropInConfig struct {
	ExecStart  string
	ExtraArgs  string
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
}

type SetupDockerDropInStep struct {
	step.Base
	Config         DropInConfig
	DropInFilePath string
}

type SetupDockerDropInStepBuilder struct {
	step.Builder[SetupDockerDropInStepBuilder, *SetupDockerDropInStep]
}

func NewSetupDockerDropInStepBuilder(ctx runtime.Context, instanceName string) *SetupDockerDropInStepBuilder {
	execStartPath := filepath.Join(common.DefaultBinDir, "dockerd")

	s := &SetupDockerDropInStep{
		DropInFilePath: dockerDropInFilePath,
		Config: DropInConfig{
			ExecStart: execStartPath,
			ExtraArgs: "",
		},
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Setup Docker systemd drop-in file (with proxy support)", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(SetupDockerDropInStepBuilder).Init(s)
	return b
}

func (b *SetupDockerDropInStepBuilder) WithExtraArgs(args []string) *SetupDockerDropInStepBuilder {
	if len(args) > 0 {
		b.Step.Config.ExtraArgs = strings.Join(args, " ")
	}
	return b
}

func (b *SetupDockerDropInStepBuilder) WithHTTPProxy(proxyURL string) *SetupDockerDropInStepBuilder {
	if proxyURL != "" {
		b.Step.Config.HTTPProxy = proxyURL
	}
	return b
}

func (b *SetupDockerDropInStepBuilder) WithHTTPSProxy(proxyURL string) *SetupDockerDropInStepBuilder {
	if proxyURL != "" {
		b.Step.Config.HTTPSProxy = proxyURL
	}
	return b
}

func (b *SetupDockerDropInStepBuilder) WithNoProxy(noProxy string) *SetupDockerDropInStepBuilder {
	if noProxy != "" {
		b.Step.Config.NoProxy = noProxy
	}
	return b
}

func (s *SetupDockerDropInStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetupDockerDropInStep) renderDropInFile() (string, error) {
	templateContent, err := templates.Get(dockerDropInTemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("docker-dropin.conf").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse docker drop-in template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.Config); err != nil {
		return "", fmt.Errorf("failed to execute docker drop-in template: %w", err)
	}
	return buf.String(), nil
}

func (s *SetupDockerDropInStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.DropInFilePath)
	if err != nil || !exists {
		return false, nil
	}

	currentContent, err := runner.ReadFile(ctx.GoContext(), conn, s.DropInFilePath)
	if err != nil {
		logger.Warn("Failed to read existing Docker drop-in file, will regenerate.", "error", err)
		return false, nil
	}

	expectedContent, err := s.renderDropInFile()
	if err != nil {
		return false, fmt.Errorf("failed to render expected drop-in file for precheck: %w", err)
	}

	if strings.TrimSpace(string(currentContent)) == strings.TrimSpace(expectedContent) {
		logger.Info("Existing Docker drop-in file content matches expected content.")
		return true, nil
	}

	logger.Info("Existing Docker drop-in file content does not match. Regeneration is required.")
	return false, nil
}

func (s *SetupDockerDropInStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	dropInContent, err := s.renderDropInFile()
	if err != nil {
		return err
	}

	dropInDir := filepath.Dir(s.DropInFilePath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, dropInDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create systemd drop-in directory %s: %w", dropInDir, err)
	}

	logger.Info("Writing Docker systemd drop-in file.", "path", s.DropInFilePath)
	err = helpers.WriteContentToRemote(ctx, conn, dropInContent, s.DropInFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write Docker drop-in file to %s: %w", s.DropInFilePath, err)
	}

	logger.Info("Successfully wrote Docker drop-in file. Running 'systemctl daemon-reload' is required next.")
	return nil
}

func (s *SetupDockerDropInStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.DropInFilePath)
	if err := runner.Remove(ctx.GoContext(), conn, s.DropInFilePath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove Docker drop-in file during rollback.")
	}

	logger.Info("Running 'systemctl daemon-reload' after rollback.")
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
		logger.Warn("Failed to run 'systemctl daemon-reload' during rollback.", "error", err)
	}
	return nil
}

var _ step.Step = (*SetupDockerDropInStep)(nil)
