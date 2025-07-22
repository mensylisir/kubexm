package docker

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.comcom/mensylisir/kubexm/pkg/templates"
)

const (
	CriDockerdDropInTemplatePath = "docker/cri-dockerd.conf.tmpl"
)

type ConfigureCriDockerdDropInStep struct {
	step.Base
	TargetPath string
	HTTPProxy  string
	HTTPSProxy string
	NOProxy    string
}

type ConfigureCriDockerdDropInStepBuilder struct {
	step.Builder[ConfigureCriDockerdDropInStepBuilder, *ConfigureCriDockerdDropInStep]
}

func NewConfigureCriDockerdDropInStepBuilder(ctx runtime.Context, instanceName string) *ConfigureCriDockerdDropInStepBuilder {
	s := &ConfigureCriDockerdDropInStep{
		TargetPath: common.CriDockerdDefaultDropInFile,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure cri-dockerd systemd drop-in file", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureCriDockerdDropInStepBuilder).Init(s)
	return b
}

func (b *ConfigureCriDockerdDropInStepBuilder) WithTargetPath(path string) *ConfigureCriDockerdDropInStepBuilder {
	b.Step.TargetPath = path
	return b
}

func (b *ConfigureCriDockerdDropInStepBuilder) WithHTTPProxy(httpProxy string) *ConfigureCriDockerdDropInStepBuilder {
	b.Step.HTTPProxy = httpProxy
	return b
}

func (b *ConfigureCriDockerdDropInStepBuilder) WithHTTPSProxy(httpsProxy string) *ConfigureCriDockerdDropInStepBuilder {
	b.Step.HTTPSProxy = httpsProxy
	return b
}

func (b *ConfigureCriDockerdDropInStepBuilder) WithNOProxy(noProxy string) *ConfigureCriDockerdDropInStepBuilder {
	b.Step.NOProxy = noProxy
	return b
}
func (s *ConfigureCriDockerdDropInStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureCriDockerdDropInStep) renderContent() (string, error) {
	tmplStr, err := templates.Get(CriDockerdDropInTemplatePath)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("cri-dockerd.conf").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse cri-dockerd drop-in template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render cri-dockerd drop-in template: %w", err)
	}

	if buf.Len() == 0 || len(strings.TrimSpace(buf.String())) == 0 {
		return "", nil
	}

	return buf.String(), nil
}

func (s *ConfigureCriDockerdDropInStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderContent()
	if err != nil {
		return false, err
	}

	if expectedContent == "" {
		exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			return false, err
		}
		if exists {
			logger.Infof("No drop-in configuration needed, but file '%s' exists. Step needs to run to remove it.", s.TargetPath)
			return false, nil
		}
		logger.Info("No drop-in configuration needed and file does not exist. Step is done.")
		return true, nil
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, err
	}
	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func (s *ConfigureCriDockerdDropInStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	if content == "" {
		logger.Info("No proxy configuration provided, ensuring drop-in file is removed.")
		if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				return fmt.Errorf("failed to remove drop-in file: %w", err)
			}
		}
	} else {
		targetDir := filepath.Dir(s.TargetPath)
		if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create drop-in directory '%s': %w", targetDir, err)
		}
		logger.Infof("Writing systemd drop-in file to %s", s.TargetPath)
		if err := runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.TargetPath, "0644", s.Sudo); err != nil {
			return fmt.Errorf("failed to write drop-in file: %w", err)
		}
	}

	logger.Info("Reloading systemd daemon")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return err
	}
	return runner.DaemonReload(ctx.GoContext(), conn, facts)
}

func (s *ConfigureCriDockerdDropInStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}
	runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false)
	facts, _ := runner.GatherFacts(ctx.GoContext(), conn)
	runner.DaemonReload(ctx.GoContext(), conn, facts)
	return nil
}
