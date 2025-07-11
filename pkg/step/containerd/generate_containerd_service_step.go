package containerd

import (
	"fmt"
	// "path/filepath" // No longer needed directly here
	// "strings" // No longer needed directly here
	// "text/template" // No longer needed directly here
	// "bytes" // No longer needed directly here

	// "github.com/Masterminds/sprig/v3" // No longer needed directly here
	// "github.com/mensylisir/kubexm/pkg/connector" // No longer needed directly here
	// "github.com/mensylisir/kubexm/pkg/runtime" // No longer needed directly here
	// "github.com/mensylisir/kubexm/pkg/spec" // No longer needed directly here
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
	stepcommon "github.com/mensylisir/kubexm/pkg/step/common"
)

const (
	// ContainerdServiceFileRemotePath is the default path for containerd.service.
	ContainerdServiceFileRemotePath = "/etc/systemd/system/containerd.service"
	ContainerdServiceTemplateName   = "containerd/containerd.service.tmpl"
)

// ContainerdServiceData holds data for templating the containerd.service file.
type ContainerdServiceData struct {
	Description   string
	Documentation string
	After         string   // e.g., "network.target local-fs.target"
	Wants         string   // e.g., "network-online.target"
	ExecStartPre  []string // Commands for ExecStartPre, e.g., "-/sbin/modprobe overlay"
	ExecStart     string   // Full command for ExecStart, e.g., "/usr/local/bin/containerd"
	ExecReload    string   // Optional: e.g., "/bin/kill -s HUP $MAINPID"
	Delegate      string   // e.g., "yes"
	KillMode      string   // e.g., "process"
	Restart       string   // e.g., "always"
	RestartSec    string   // e.g., "5"
	LimitNOFILE   string   // e.g., "1048576"
	LimitNPROC    string   // e.g., "infinity"
	LimitCORE     string   // e.g., "infinity"
	TasksMax      string   // e.g., "infinity"
	Environment   []string // Environment variables
	WantedBy      string   // e.g., "multi-user.target"
}

// NewGenerateContainerdServiceStep creates a step that renders the containerd.service systemd unit file.
// It now utilizes the common.RenderTemplateStep for its execution.
func NewGenerateContainerdServiceStep(instanceName string, serviceData ContainerdServiceData, remotePath string, sudo bool) (step.Step, error) {
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
		serviceData.ExecStart = "/usr/local/bin/containerd" // This path might need to come from a common constant or runtime config
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
	if serviceData.TasksMax == "" {
		serviceData.TasksMax = "infinity"
	}
	if serviceData.WantedBy == "" {
		serviceData.WantedBy = "multi-user.target"
	}

	templateContent, err := templates.Get(ContainerdServiceTemplateName)
	if err != nil {
		return nil, fmt.Errorf("failed to get containerd service template '%s': %w", ContainerdServiceTemplateName, err)
	}

	renderStepName := name
	if instanceName == "" {
		renderStepName = fmt.Sprintf("RenderContainerdServiceTo-%s", rPath)
	}

	// Writing to /etc/systemd/system usually needs sudo, so sudo is true by default.
	// Systemd unit files are typically 0644.
	return stepcommon.NewRenderTemplateStep(renderStepName, templateContent, serviceData, rPath, "0644", sudo), nil
}

// Note: The original GenerateContainerdServiceStep struct and its methods (Meta, Precheck, Run, Rollback, renderServiceFile)
// are no longer needed. The defaultContainerdServiceTemplate const is also removed as it's now in
// pkg/templates/containerd/containerd.service.tmpl.
