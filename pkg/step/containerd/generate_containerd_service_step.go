package containerd

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
)

const (
	// ContainerdServiceFileRemotePath is the default path for containerd.service.
	ContainerdServiceFileRemotePath = "/etc/systemd/system/containerd.service"
)

// ContainerdServiceData holds data for templating the containerd.service file.
type ContainerdServiceData struct {
	Description      string
	Documentation    string
	After            string // e.g., "network.target local-fs.target"
	Wants            string // e.g., "network-online.target"
	ExecStartPre     []string // Commands for ExecStartPre, e.g., "-/sbin/modprobe overlay"
	ExecStart        string   // Full command for ExecStart, e.g., "/usr/local/bin/containerd"
	ExecReload       string   // Optional: e.g., "/bin/kill -s HUP $MAINPID"
	Delegate         string   // e.g., "yes"
	KillMode         string   // e.g., "process"
	Restart          string   // e.g., "always"
	RestartSec       string   // e.g., "5"
	LimitNOFILE      string   // e.g., "1048576"
	LimitNPROC       string   // e.g., "infinity"
	LimitCORE        string   // e.g., "infinity"
	TasksMax         string   // e.g., "infinity"
	Environment      []string // Environment variables
	WantedBy         string   // e.g., "multi-user.target"
}

// GenerateContainerdServiceStep renders the containerd.service systemd unit file.
type GenerateContainerdServiceStep struct {
	meta             spec.StepMeta
	ServiceData      ContainerdServiceData
	RemoteUnitPath   string
	Sudo             bool
	TemplateContent  string
}

// NewGenerateContainerdServiceStep creates a new GenerateContainerdServiceStep.
func NewGenerateContainerdServiceStep(instanceName string, serviceData ContainerdServiceData, remotePath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "GenerateContainerdSystemdServiceFile"
	}
	rPath := remotePath
	if rPath == "" {
		rPath = ContainerdServiceFileRemotePath
	}

	// Apply defaults to serviceData
	if serviceData.Description == "" {
		serviceData.Description = "containerd container runtime"
	}
	if serviceData.Documentation == "" {
		serviceData.Documentation = "https://containerd.io"
	}
	if serviceData.After == "" {
		serviceData.After = "network.target local-fs.target"
	}
	if serviceData.Wants == "" {
		serviceData.Wants = "network-online.target" // Common for services needing network
	}
	if serviceData.ExecStart == "" {
		serviceData.ExecStart = "/usr/local/bin/containerd"
	}
	if serviceData.Delegate == "" {
		serviceData.Delegate = "yes"
	}
	if serviceData.KillMode == "" {
		serviceData.KillMode = "process"
	}
	if serviceData.Restart == "" {
		serviceData.Restart = "always"
	}
	if serviceData.RestartSec == "" {
		serviceData.RestartSec = "5s"
	}
	if serviceData.LimitNOFILE == "" {
		serviceData.LimitNOFILE = "1048576" // Common high limit
	}
	if serviceData.LimitNPROC == "" {
		serviceData.LimitNPROC = "infinity"
	}
	if serviceData.LimitCORE == "" {
		serviceData.LimitCORE = "infinity"
	}
	if serviceData.TasksMax == "" { // Added TasksMax
		serviceData.TasksMax = "infinity"
	}
	if serviceData.WantedBy == "" {
		serviceData.WantedBy = "multi-user.target"
	}
	// ExecStartPre is often empty or specific like "-/sbin/modprobe overlay"

	return &GenerateContainerdServiceStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Generates containerd.service systemd unit file at %s.", rPath),
		},
		ServiceData:      serviceData,
		RemoteUnitPath:   rPath,
		Sudo:             true, // Writing to /etc/systemd/system usually needs sudo
		TemplateContent:  "",   // Use default template
	}
}

func (s *GenerateContainerdServiceStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *GenerateContainerdServiceStep) renderServiceFile(data *ContainerdServiceData) (string, error) {
	tmplContent := s.TemplateContent
	if tmplContent == "" {
		tmplContent = defaultContainerdServiceTemplate
	}

	tmpl, err := template.New("containerdService").Funcs(sprig.TxtFuncMap()).Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse containerd service template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute containerd service template: %w", err)
	}
	return buf.String(), nil
}

func (s *GenerateContainerdServiceStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemoteUnitPath)
	if err != nil {
		logger.Warn("Failed to check for existing service file, will attempt generation.", "path", s.RemoteUnitPath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Service file does not exist.", "path", s.RemoteUnitPath)
		return false, nil
	}

	logger.Info("Service file exists. Verifying content...")
	currentContentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.RemoteUnitPath)
	if err != nil {
		logger.Warn("Failed to read existing service file for comparison, will regenerate.", "error", err)
		return false, nil
	}

	expectedContent, err := s.renderServiceFile(&s.ServiceData)
	if err != nil {
		logger.Error("Failed to render expected service file for comparison.", "error", err)
		return false, fmt.Errorf("failed to render expected service file: %w", err)
	}

	currentContent := strings.TrimSpace(strings.ReplaceAll(string(currentContentBytes), "\r\n", "\n"))
	expectedContent = strings.TrimSpace(strings.ReplaceAll(expectedContent, "\r\n", "\n"))

	if currentContent == expectedContent {
		logger.Info("Existing service file content matches expected content.")
		return true, nil
	}

	logger.Info("Existing service file content does not match. Will regenerate.")
	return false, nil
}

func (s *GenerateContainerdServiceStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	serviceFileContent, err := s.renderServiceFile(&s.ServiceData)
	if err != nil {
		return err
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	remoteDir := filepath.Dir(s.RemoteUnitPath)
	logger.Info("Ensuring remote directory for service file exists.", "path", remoteDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
	}

	logger.Info("Writing systemd service file.", "path", s.RemoteUnitPath)
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(serviceFileContent), s.RemoteUnitPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write systemd service file to %s: %w", s.RemoteUnitPath, err)
	}

	logger.Info("Systemd service file generated successfully. Run 'systemctl daemon-reload' to apply changes.")
	return nil
}

func (s *GenerateContainerdServiceStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove systemd service file for rollback.", "path", s.RemoteUnitPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for rollback.", "error", err)
		return nil
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemoteUnitPath, s.Sudo); err != nil {
		logger.Warn("Failed to remove systemd service file during rollback (best effort).", "error", err)
	} else {
		logger.Info("Successfully removed systemd service file (if it existed).")
	}
	return nil
}

var _ step.Step = (*GenerateContainerdServiceStep)(nil)

const defaultContainerdServiceTemplate = `[Unit]
Description={{ .Description }}
Documentation={{ .Documentation }}
After={{ .After }}
{{- if .Wants }}
Wants={{ .Wants }}
{{- end }}

[Service]
{{- range .ExecStartPre }}
ExecStartPre={{ . }}
{{- end }}
ExecStart={{ .ExecStart }}
{{- if .ExecReload }}
ExecReload={{ .ExecReload }}
{{- end }}
{{- if .Environment }}
{{- range .Environment }}
Environment="{{ . }}"
{{- end }}
{{- end }}
Delegate={{ .Delegate }}
KillMode={{ .KillMode }}
Restart={{ .Restart }}
RestartSec={{ .RestartSec }}
LimitNOFILE={{ .LimitNOFILE }}
LimitNPROC={{ .LimitNPROC }}
LimitCORE={{ .LimitCORE }}
TasksMax={{ .TasksMax }}

[Install]
WantedBy={{ .WantedBy }}
`
