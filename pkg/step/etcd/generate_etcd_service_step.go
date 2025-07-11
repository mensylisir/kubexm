package etcd

import (
	"fmt"
	"path/filepath" // Ensure filepath is imported

	// "strings" // No longer needed directly here
	// "text/template" // No longer needed directly here

	// "github.com/Masterminds/sprig/v3" // No longer needed directly here
	// "github.com/mensylisir/kubexm/pkg/connector" // No longer needed directly here
	// "github.com/mensylisir/kubexm/pkg/runtime" // No longer needed directly here
	// "github.com/mensylisir/kubexm/pkg/spec" // No longer needed directly here
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/templates"
	stepcommon "github.com/mensylisir/kubexm/pkg/step/common"
)

const EtcdServiceTemplateName = "etcd/etcd.service.tmpl"

// EtcdServiceData holds data for templating the etcd.service file.
type EtcdServiceData struct {
	Description      string
	ExecStart        string // Full command for ExecStart, e.g., "/usr/local/bin/etcd --config-file=/etc/etcd/etcd.yaml"
	User             string // User to run etcd as, e.g., "etcd"
	Group            string // Group to run etcd as, e.g., "etcd"
	Restart          string // e.g., "on-failure"
	RestartSec       string // e.g., "5s"
	LimitNOFILE      string // e.g., "65536"
	Environment      []string // Environment variables, e.g., ["ETCD_UNSUPPORTED_ARCH=arm64"]
	WorkingDirectory string // Optional: WorkingDirectory for etcd process
}

// NewGenerateEtcdServiceStep creates a step that renders the etcd.service systemd unit file on an etcd node.
// It now utilizes the common.RenderTemplateStep for its execution.
func NewGenerateEtcdServiceStep(instanceName string, serviceData EtcdServiceData, remotePath string, sudo bool) (step.Step, error) {
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
		// Ensure common.EtcdDefaultBinDir and common.EtcdDefaultConfFile are defined and accessible
		execStartCmd := filepath.Join(common.EtcdDefaultBinDir, "etcd") + " --config-file=" + common.EtcdDefaultConfFile
		serviceData.ExecStart = execStartCmd
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
	// Note: Environment can be set directly in serviceData before calling this constructor.

	templateContent, err := templates.Get(EtcdServiceTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd service template '%s': %w", EtcdServiceTemplateName, err)
	}

	// common.RenderTemplateStep handles its own meta name and description formatting if instanceName is ""
	// For consistency, we can use a more specific name if instanceName is empty.
	renderStepName := name
	if instanceName == "" {
		renderStepName = fmt.Sprintf("RenderEtcdServiceTo-%s", rPath)
	}


	// Systemd unit files are typically 0644.
	return stepcommon.NewRenderTemplateStep(renderStepName, templateContent, serviceData, rPath, "0644", sudo), nil
}

// Note: The original GenerateEtcdServiceStep struct and its methods (Meta, Precheck, Run, Rollback)
// are no longer needed as NewGenerateEtcdServiceStep now returns a common.RenderTemplateStep.
// The defaultEtcdServiceTemplate const is also removed as it's now in pkg/templates/etcd/etcd.service.tmpl.
