package etcd

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/common" // Added for common constants
)

// EtcdServiceData holds data for templating the etcd.service file.
type EtcdServiceData struct {
	Description    string
	ExecStart      string // Full command for ExecStart, e.g., "/usr/local/bin/etcd --config-file=/etc/etcd/etcd.yaml"
	User           string // User to run etcd as, e.g., "etcd"
	Group          string // Group to run etcd as, e.g., "etcd"
	Restart        string // e.g., "on-failure"
	RestartSec     string // e.g., "5s"
	LimitNOFILE    string // e.g., "65536"
	Environment    []string // Environment variables, e.g., ["ETCD_UNSUPPORTED_ARCH=arm64"]
	WorkingDirectory string // Optional: WorkingDirectory for etcd process
}

// GenerateEtcdServiceStep renders the etcd.service systemd unit file on an etcd node.
type GenerateEtcdServiceStep struct {
	meta             spec.StepMeta
	ServiceData      EtcdServiceData // Direct data for this node.
	RemoteUnitPath   string          // Path on the target node where etcd.service will be written.
	Sudo             bool
	TemplateContent  string          // Optional: if not using default, provide custom template content.
}

// NewGenerateEtcdServiceStep creates a new GenerateEtcdServiceStep.
func NewGenerateEtcdServiceStep(instanceName string, serviceData EtcdServiceData, remotePath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "GenerateEtcdSystemdServiceFile"
	}
	rPath := remotePath
	if rPath == "" {
		rPath = common.EtcdDefaultSystemdFile
	}
	// Set defaults for ServiceData if not provided
	if serviceData.Description == "" {
		serviceData.Description = "etcd Distributed Key-Value Store"
	}
	if serviceData.ExecStart == "" {
		serviceData.ExecStart = "/usr/local/bin/etcd --config-file=" + common.EtcdDefaultConfFile // Use common constant
	}
	if serviceData.User == "" {
		serviceData.User = "etcd"
	}
	if serviceData.Group == "" {
		serviceData.Group = "etcd"
	}
	if serviceData.Restart == "" {
		serviceData.Restart = "on-failure"
	}
	if serviceData.RestartSec == "" {
		serviceData.RestartSec = "5s"
	}
	if serviceData.LimitNOFILE == "" {
		serviceData.LimitNOFILE = "65536"
	}


	return &GenerateEtcdServiceStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Generates etcd.service systemd unit file at %s.", rPath),
		},
		ServiceData:      serviceData,
		RemoteUnitPath:   rPath,
		Sudo:             sudo,
	}
}

func (s *GenerateEtcdServiceStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *GenerateEtcdServiceStep) renderEtcdServiceFile(data *EtcdServiceData) (string, error) {
	tmplContent := s.TemplateContent
	if tmplContent == "" {
		tmplContent = defaultEtcdServiceTemplate
	}

	tmpl, err := template.New("etcdService").Funcs(sprig.TxtFuncMap()).Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse etcd service template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute etcd service template: %w", err)
	}
	return buf.String(), nil
}

func (s *GenerateEtcdServiceStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemoteUnitPath)
	if err != nil {
		logger.Warn("Failed to check for existing etcd service file, will attempt generation.", "path", s.RemoteUnitPath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Etcd service file does not exist.", "path", s.RemoteUnitPath)
		return false, nil
	}

	logger.Info("Etcd service file exists. Verifying content...")
	currentContentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.RemoteUnitPath)
	if err != nil {
		logger.Warn("Failed to read existing etcd service file for comparison, will regenerate.", "path", s.RemoteUnitPath, "error", err)
		return false, nil
	}

	expectedContent, err := s.renderEtcdServiceFile(&s.ServiceData)
	if err != nil {
		logger.Error("Failed to render expected etcd service file for comparison.", "error", err)
		return false, fmt.Errorf("failed to render expected etcd service file: %w", err)
	}

	currentContent := strings.ReplaceAll(string(currentContentBytes), "\r\n", "\n")
	if strings.TrimSpace(currentContent) == strings.TrimSpace(expectedContent) {
		logger.Info("Existing etcd service file content matches expected content.")
		return true, nil
	}

	logger.Info("Existing etcd service file content does not match expected content. Regeneration needed.")
	return false, nil
}

func (s *GenerateEtcdServiceStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	serviceFileContent, err := s.renderEtcdServiceFile(&s.ServiceData)
	if err != nil {
		return err
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	remoteDir := filepath.Dir(s.RemoteUnitPath)
	logger.Info("Ensuring remote directory for etcd service file exists.", "path", remoteDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s for etcd service file: %w", remoteDir, err)
	}

	logger.Info("Writing etcd systemd service file.", "path", s.RemoteUnitPath)
	// Systemd unit files are typically 0644.
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(serviceFileContent), s.RemoteUnitPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write etcd systemd service file to %s: %w", s.RemoteUnitPath, err)
	}

	logger.Info("Etcd systemd service file generated successfully.")
	// Note: This step does NOT run `systemctl daemon-reload`. That should be a separate step.
	return nil
}

func (s *GenerateEtcdServiceStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove etcd systemd service file for rollback.", "path", s.RemoteUnitPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil // Best effort
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemoteUnitPath, s.Sudo); err != nil {
		logger.Warn("Failed to remove etcd systemd service file during rollback (best effort).", "path", s.RemoteUnitPath, "error", err)
	} else {
		logger.Info("Successfully removed etcd systemd service file (if it existed).", "path", s.RemoteUnitPath)
	}
	// Note: This step does NOT run `systemctl daemon-reload` after removal.
	return nil
}

var _ step.Step = (*GenerateEtcdServiceStep)(nil)

const defaultEtcdServiceTemplate = `[Unit]
Description={{ .Description | default "etcd Distributed Key-Value Store" }}
Documentation=https://github.com/etcd-io/etcd
After=network.target
Wants=network-online.target

[Service]
User={{ .User | default "etcd" }}
Group={{ .Group | default "etcd" }}
Type=notify
ExecStart={{ .ExecStart }}
{{- if .WorkingDirectory }}
WorkingDirectory={{ .WorkingDirectory }}
{{- end }}
Restart={{ .Restart | default "on-failure" }}
RestartSec={{ .RestartSec | default "5s" }}
LimitNOFILE={{ .LimitNOFILE | default "65536" }}
{{- range .Environment }}
Environment="{{ . }}"
{{- end }}

[Install]
WantedBy=multi-user.target
`
