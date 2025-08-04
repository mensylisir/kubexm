package etcd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"io/fs"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type EtcdDropInData struct {
	ExecStart string
}

type ConfigureEtcdDropInStep struct {
	step.Base
	EtcdBinaryPath  string
	ServicePath     string
	renderedContent []byte
}

type ConfigureEtcdDropInStepBuilder struct {
	step.Builder[ConfigureEtcdDropInStepBuilder, *ConfigureEtcdDropInStep]
}

func NewConfigureEtcdDropInStepBuilder(ctx runtime.Context, instanceName string) *ConfigureEtcdDropInStepBuilder {
	s := &ConfigureEtcdDropInStep{
		EtcdBinaryPath: filepath.Join(common.DefaultBinDir, "etcd"),
		ServicePath:    common.EtcdDropInFile,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure etcd systemd drop-in on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ConfigureEtcdDropInStepBuilder).Init(s)
	return b
}

func (b *ConfigureEtcdDropInStepBuilder) WithEtcdBinaryPath(path string) *ConfigureEtcdDropInStepBuilder {
	b.Step.EtcdBinaryPath = path
	return b
}

func (b *ConfigureEtcdDropInStepBuilder) WithServicePath(path string) *ConfigureEtcdDropInStepBuilder {
	b.Step.ServicePath = path
	return b
}

func (s *ConfigureEtcdDropInStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureEtcdDropInStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	content, err := s.renderDropInFile()
	if err != nil {
		return false, fmt.Errorf("failed to render expected drop-in file for precheck: %w", err)
	}
	s.renderedContent = content

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remotePath := s.ServicePath
	exists, err := runner.Exists(ctx.GoContext(), conn, remotePath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of %s: %w", remotePath, err)
	}
	if !exists {
		logger.Info("etcd drop-in file does not exist. Step needs to run.", "path", remotePath)
		return false, nil
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remotePath)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			logger.Warn("Failed to read remote etcd drop-in file, will re-run step.", "path", remotePath)
			return false, nil
		}
		return false, fmt.Errorf("failed to read remote file %s for content check: %w", remotePath, err)
	}

	expectedSum := sha256.Sum256(s.renderedContent)
	remoteSum := sha256.Sum256(remoteContent)

	if hex.EncodeToString(expectedSum[:]) == hex.EncodeToString(remoteSum[:]) {
		logger.Info("Remote etcd drop-in file is up-to-date. Step is done.", "path", remotePath)
		return true, nil
	}

	logger.Info("Remote etcd drop-in file content has changed. Step needs to run.", "path", remotePath)
	return false, nil
}

func (s *ConfigureEtcdDropInStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	if s.renderedContent == nil {
		content, err := s.renderDropInFile()
		if err != nil {
			return err
		}
		s.renderedContent = content
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	dropInDir := filepath.Dir(s.ServicePath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, dropInDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create systemd drop-in directory %s: %w", dropInDir, err)
	}

	remotePath := s.ServicePath
	logger.Info("Writing systemd drop-in file", "path", remotePath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(s.renderedContent), remotePath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write etcd drop-in file to %s: %w", remotePath, err)
	}

	logger.Info("Reloading systemd daemon to apply drop-in configuration...")
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts for daemon-reload: %w", err)
	}
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	logger.Info("Etcd systemd drop-in configured and daemon reloaded successfully.")
	return nil
}

func (s *ConfigureEtcdDropInStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	runner := ctx.GetRunner()

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}

	remotePath := s.ServicePath
	logger.Warn("Rolling back by removing etcd drop-in file", "path", remotePath)

	if err := runner.Remove(ctx.GoContext(), conn, remotePath, s.Sudo, false); err != nil {
		logger.Error(err, "Failed to remove etcd drop-in file during rollback")
	}

	logger.Warn("Reloading systemd daemon after removing drop-in file...")
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		logger.Error(err, "Failed to get host facts for daemon-reload during rollback")
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			logger.Error(err, "Failed to reload systemd daemon during rollback")
		}
	}

	return nil
}

func (s *ConfigureEtcdDropInStep) renderDropInFile() ([]byte, error) {
	templateContent, err := templates.Get("etcd/etcd-drop-in.conf.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get embedded etcd-drop-in.conf template: %w", err)
	}

	remoteConfPath := filepath.Join(common.EtcdDefaultConfDirTarget, "etcd.conf.yaml")
	data := EtcdDropInData{
		ExecStart: fmt.Sprintf("%s --config-file=%s", s.EtcdBinaryPath, remoteConfPath),
	}

	tmpl, err := template.New("etcd-drop-in.conf").Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse etcd drop-in template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return nil, fmt.Errorf("failed to render etcd drop-in template: %w", err)
	}

	return buffer.Bytes(), nil
}

var _ step.Step = (*ConfigureEtcdDropInStep)(nil)
