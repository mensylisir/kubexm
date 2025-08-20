package harbor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type StopHarborServiceStep struct {
	step.Base
	RemoteInstallDir string
}

type StopHarborServiceStepBuilder struct {
	step.Builder[StopHarborServiceStepBuilder, *StopHarborServiceStep]
}

func NewStopHarborServiceStepBuilder(ctx runtime.Context, instanceName string) *StopHarborServiceStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	localCfg := cfg.Registry.LocalDeployment

	if localCfg == nil || localCfg.Type != "harbor" {
		return nil
	}

	installRoot := "/opt"
	if localCfg.DataRoot != "" {
		installRoot = localCfg.DataRoot
	}

	s := &StopHarborServiceStep{
		RemoteInstallDir: filepath.Join(installRoot, "harbor"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop Harbor services on the registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(StopHarborServiceStepBuilder).Init(s)
	return b
}

func (s *StopHarborServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopHarborServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	// Check if key harbor services are running. If not, the step is done.
	checkCmd := "docker ps --filter name=harbor-core --filter status=running --format '{{.Names}}'"
	output, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err != nil {
		logger.Warnf("Failed to check for running harbor-core container, will attempt to stop anyway. Error: %v", err)
		return false, nil
	}

	if !strings.Contains(output, "harbor-core") {
		logger.Info("Harbor core container is not running. Step is done.")
		return true, nil
	}

	logger.Info("Harbor core container is running. Stop is required.")
	return false, nil
}

func (s *StopHarborServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	composeFilePath := filepath.Join(s.RemoteInstallDir, "docker-compose.yml")
	if exists, _ := runner.Exists(ctx.GoContext(), conn, composeFilePath); !exists {
		logger.Warnf("Harbor docker-compose.yml not found at '%s', assuming services are already stopped.", composeFilePath)
		return nil
	}

	stopCmd := fmt.Sprintf("cd %s && docker-compose down", s.RemoteInstallDir)
	logger.Infof("Executing Harbor stop command on remote host: %s", stopCmd)

	output, err := runner.Run(ctx.GoContext(), conn, stopCmd, s.Sudo)
	if err != nil {
		logger.Errorf("Harbor stop failed. Full output:\n%s", output)
		return fmt.Errorf("failed to execute docker-compose down: %w", err)
	}

	logger.Info("Harbor services stopped successfully.")
	logger.Debugf("Harbor stop output:\n%s", output)
	return nil
}

func (s *StopHarborServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	installScriptPath := filepath.Join(s.RemoteInstallDir, "install.sh")
	if exists, _ := runner.Exists(ctx.GoContext(), conn, installScriptPath); !exists {
		logger.Warnf("Harbor install script not found at '%s', cannot run start for rollback.", installScriptPath)
		return nil
	}

	startCmd := fmt.Sprintf("cd %s && ./install.sh", s.RemoteInstallDir)
	logger.Warnf("Rolling back by running start command: %s", startCmd)

	if _, err := runner.Run(ctx.GoContext(), conn, startCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to run start command during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*StopHarborServiceStep)(nil)
