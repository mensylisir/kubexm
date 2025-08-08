package helm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type helmStatusOutput struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Info      struct {
		Status string `json:"status"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Version string `json:"version"`
		} `json:"metadata"`
	} `json:"chart"`
}

type InstallChartStep struct {
	step.Base
	RepoName    string
	ChartName   string
	ReleaseName string
	Namespace   string
	ValuesFile  string
	Version     string
	ExtraArgs   []string
}

type InstallChartStepBuilder struct {
	step.Builder[InstallChartStepBuilder, *InstallChartStep]
}

func NewInstallChartStepBuilder(ctx runtime.Context, instanceName string) *InstallChartStepBuilder {
	s := &InstallChartStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade a Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(InstallChartStepBuilder).Init(s)
	return b
}

func (b *InstallChartStepBuilder) WithSudo(sudo bool) *InstallChartStepBuilder {
	b.Step.Sudo = sudo
	return b
}
func (b *InstallChartStepBuilder) WithRepoName(repoName string) *InstallChartStepBuilder {
	b.Step.RepoName = repoName
	return b
}

func (b *InstallChartStepBuilder) WithChartName(chartName string) *InstallChartStepBuilder {
	b.Step.ChartName = chartName
	return b
}

func (b *InstallChartStepBuilder) WithReleaseName(releaseName string) *InstallChartStepBuilder {
	b.Step.ReleaseName = releaseName
	return b
}

func (b *InstallChartStepBuilder) WithNamespace(namespace string) *InstallChartStepBuilder {
	b.Step.Namespace = namespace
	return b
}

func (b *InstallChartStepBuilder) WithValuesFile(valuesFile string) *InstallChartStepBuilder {
	b.Step.ValuesFile = valuesFile
	return b
}

func (b *InstallChartStepBuilder) WithVersion(version string) *InstallChartStepBuilder {
	b.Step.Version = version
	return b
}

func (b *InstallChartStepBuilder) WithExtraArgs(extraArgs []string) *InstallChartStepBuilder {
	b.Step.ExtraArgs = extraArgs
	return b
}
func (s *InstallChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if s.ReleaseName == "" || s.ChartName == "" {
		return false, errors.New("ReleaseName and ChartName must be provided")
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		return false, errors.Wrap(err, "helm command not found on remote host")
	}

	statusCmd := fmt.Sprintf("helm status %s", s.ReleaseName)
	if s.Namespace != "" {
		statusCmd += fmt.Sprintf(" --namespace %s", s.Namespace)
	}
	statusCmd += " -o json"

	output, err := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
	if err != nil {
		logger.Infof("Helm release '%s' not found. Installation is required.", s.ReleaseName)
		return false, nil
	}

	var status helmStatusOutput
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		logger.Warnf("Failed to parse helm status for release '%s', assuming upgrade is needed: %v", s.ReleaseName, err)
		return false, nil
	}

	if s.Version == "" && status.Info.Status == "deployed" {
		logger.Infof("Helm release '%s' is already deployed (any version). Skipping.", s.ReleaseName)
		return true, nil
	}

	if status.Info.Status == "deployed" && status.Chart.Metadata.Version == s.Version {
		logger.Infof("Helm release '%s' version %s is already deployed and up-to-date. Skipping.", s.ReleaseName, s.Version)
		return true, nil
	}

	logger.Infof("Helm release '%s' requires upgrade (current version: %s, status: %s).", s.ReleaseName, status.Chart.Metadata.Version, status.Info.Status)
	return false, nil
}

func (s *InstallChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	var cmdBuilder strings.Builder
	cmdBuilder.WriteString(fmt.Sprintf("helm upgrade --install %s %s", s.ReleaseName, s.ChartName))

	if s.RepoName != "" {
		cmdBuilder.WriteString(fmt.Sprintf(" --repo %s", s.RepoName))
	}
	if s.Namespace != "" {
		cmdBuilder.WriteString(fmt.Sprintf(" --namespace %s --create-namespace", s.Namespace))
	}
	if s.ValuesFile != "" {
		cmdBuilder.WriteString(fmt.Sprintf(" -f %s", s.ValuesFile))
	}
	if s.Version != "" {
		cmdBuilder.WriteString(fmt.Sprintf(" --version %s", s.Version))
	}
	if len(s.ExtraArgs) > 0 {
		cmdBuilder.WriteString(" ")
		cmdBuilder.WriteString(strings.Join(s.ExtraArgs, " "))
	}
	cmdBuilder.WriteString(" --wait --atomic")

	cmd := cmdBuilder.String()

	logger.Infof("Installing/upgrading helm chart with command: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Helm chart installed/upgraded successfully.")
	logger.Debugf("Command output:\n%s", output)
	return nil
}

func (s *InstallChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	var cmdBuilder strings.Builder
	cmdBuilder.WriteString(fmt.Sprintf("helm uninstall %s", s.ReleaseName))
	if s.Namespace != "" {
		cmdBuilder.WriteString(fmt.Sprintf(" --namespace %s", s.Namespace))
	}

	cmd := cmdBuilder.String()

	logger.Warnf("Rolling back by uninstalling helm release with command: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Errorf("Failed to uninstall helm chart during rollback: %v", err)
	} else {
		logger.Info("Successfully executed helm uninstall.")
	}

	return nil
}
