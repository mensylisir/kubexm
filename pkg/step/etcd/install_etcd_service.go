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
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type EtcdServiceData struct {
	User           string
	Group          string
	EtcdBinaryPath string
}

type InstallEtcdServiceStep struct {
	step.Base
	EtcdBinaryPath  string
	User            string
	Group           string
	ServicePath     string
	renderedContent []byte
}

type InstallEtcdServiceStepBuilder struct {
	step.Builder[InstallEtcdServiceStepBuilder, *InstallEtcdServiceStep]
}

func NewInstallEtcdServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallEtcdServiceStepBuilder {
	s := &InstallEtcdServiceStep{
		EtcdBinaryPath: filepath.Join(common.DefaultBinDir, "etcd"),
		User:           "etcd",
		Group:          "etcd",
		ServicePath:    common.EtcdSystemdFile,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install base etcd systemd service on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallEtcdServiceStepBuilder).Init(s)
	return b
}

func (b *InstallEtcdServiceStepBuilder) WithEtcdBinaryPath(path string) *InstallEtcdServiceStepBuilder {
	b.Step.EtcdBinaryPath = path
	return b
}

func (b *InstallEtcdServiceStepBuilder) WithUser(user string) *InstallEtcdServiceStepBuilder {
	b.Step.User = user
	return b
}

func (b *InstallEtcdServiceStepBuilder) WithGroup(group string) *InstallEtcdServiceStepBuilder {
	b.Step.Group = group
	return b
}

func (b *InstallEtcdServiceStepBuilder) WithServicePath(path string) *InstallEtcdServiceStepBuilder {
	b.Step.ServicePath = path
	return b
}
func (s *InstallEtcdServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallEtcdServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	content, err := s.renderServiceFile()
	if err != nil {
		return false, fmt.Errorf("failed to render expected service file for precheck: %w", err)
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
		logger.Info("etcd.service does not exist. Step needs to run.", "path", remotePath)
		return false, nil
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remotePath)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			logger.Warn("Failed to read remote etcd.service due to permission error, will re-run step to fix it.", "path", remotePath)
			return false, nil
		}
		return false, fmt.Errorf("failed to read remote file %s for content check: %w", remotePath, err)
	}

	expectedSum := sha256.Sum256(s.renderedContent)
	remoteSum := sha256.Sum256(remoteContent)

	if hex.EncodeToString(expectedSum[:]) == hex.EncodeToString(remoteSum[:]) {
		logger.Info("Remote etcd.service is up-to-date. Step is done.", "path", remotePath)
		return true, nil
	}

	logger.Info("Remote etcd.service content has changed. Step needs to run to update it.", "path", remotePath)
	return false, nil
}

func (s *InstallEtcdServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	if s.renderedContent == nil {
		content, err := s.renderServiceFile()
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

	remotePath := s.ServicePath
	logger.Info("Writing systemd service file", "node", ctx.GetHost().GetName(), "path", remotePath)

	if err := helpers.WriteContentToRemote(ctx, conn, string(s.renderedContent), remotePath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write etcd.service file to %s: %w", remotePath, err)
	}

	logger.Info("Reloading systemd daemon")
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts for daemon-reload: %w", err)
	}
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	logger.Info("Base etcd systemd service file installed successfully.")
	return nil
}

func (s *InstallEtcdServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	runner := ctx.GetRunner()

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}

	remotePath := s.ServicePath
	logger.Warn("Rolling back by removing etcd.service file", "path", remotePath)

	logger.Warnf("Rolling back by removing: %s", remotePath)
	if err := runner.Remove(ctx.GoContext(), conn, remotePath, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s' during rollback: %v", remotePath, err)
		}
	}
	return nil
}

func (s *InstallEtcdServiceStep) renderServiceFile() ([]byte, error) {
	templateContent, err := templates.Get("etcd/etcd.service.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get embedded etcd.service template: %w", err)
	}

	data := EtcdServiceData{
		User:           s.User,
		Group:          s.Group,
		EtcdBinaryPath: s.EtcdBinaryPath,
	}

	tmpl, err := template.New("etcd.service").Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse etcd.service template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return nil, fmt.Errorf("failed to render etcd.service template: %w", err)
	}

	return buffer.Bytes(), nil
}

var _ step.Step = (*InstallEtcdServiceStep)(nil)
