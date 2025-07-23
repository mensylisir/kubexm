package helm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

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
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install helm chart", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(InstallChartStepBuilder).Init(s)
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
	return false, nil
}

func (s *InstallChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmd := "helm"
	args := []string{"install", s.ReleaseName, s.ChartName}
	if s.RepoName != "" {
		args = append(args, "--repo", s.RepoName)
	}
	if s.Namespace != "" {
		args = append(args, "--namespace", s.Namespace, "--create-namespace")
	}
	if s.ValuesFile != "" {
		args = append(args, "-f", s.ValuesFile)
	}
	if s.Version != "" {
		args = append(args, "--version", s.Version)
	}
	if len(s.ExtraArgs) > 0 {
		args = append(args, s.ExtraArgs...)
	}

	logger.Infof("Installing helm chart %s with release name %s", s.ChartName, s.ReleaseName)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install helm chart: %w", err)
	}

	return nil
}

func (s *InstallChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	cmd := "helm"
	args := []string{"uninstall", s.ReleaseName}
	if s.Namespace != "" {
		args = append(args, "--namespace", s.Namespace)
	}

	logger.Warnf("Rolling back by uninstalling helm chart %s", s.ReleaseName)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), s.Sudo)
	if err != nil {
		logger.Errorf("Failed to uninstall helm chart during rollback: %v", err)
	}

	return nil
}
