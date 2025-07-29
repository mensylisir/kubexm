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

type StopAndRemoveHarborStep struct {
	step.Base
	RemoteInstallDir string
}

type StopAndRemoveHarborStepBuilder struct {
	step.Builder[StopAndRemoveHarborStepBuilder, *StopAndRemoveHarborStep]
}

func NewStopAndRemoveHarborStepBuilder(ctx runtime.Context, instanceName string) *StopAndRemoveHarborStepBuilder {
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

	s := &StopAndRemoveHarborStep{
		RemoteInstallDir: filepath.Join(installRoot, "harbor"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop and remove Harbor services on the registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(StopAndRemoveHarborStepBuilder).Init(s)
	return b
}

func (s *StopAndRemoveHarborStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopAndRemoveHarborStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	checkCmd := "docker ps -a --filter name=harbor-core --format '{{.Names}}'"

	output, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err != nil {
		logger.Warnf("Failed to check for harbor-core container, will attempt removal. Error: %v", err)
		return false, nil
	}

	if !strings.Contains(output, "harbor-core") {
		logger.Info("Harbor core container does not exist. Step is done.")
		return true, nil
	}

	logger.Info("Harbor containers still exist. Removal is required.")
	return false, nil
}

func (s *StopAndRemoveHarborStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	composeFilePath := filepath.Join(s.RemoteInstallDir, "docker-compose.yml")
	if exists, _ := runner.Exists(ctx.GoContext(), conn, composeFilePath); !exists {
		logger.Warnf("Harbor docker-compose.yml not found at '%s', assuming services are already removed.", composeFilePath)
		return nil
	}

	uninstallCmd := fmt.Sprintf("cd %s && docker-compose down -v", s.RemoteInstallDir)
	logger.Infof("Executing Harbor removal command on remote host: %s", uninstallCmd)

	output, err := runner.Run(ctx.GoContext(), conn, uninstallCmd, s.Sudo)
	if err != nil {
		logger.Errorf("Harbor removal failed. Full output:\n%s", output)
		return fmt.Errorf("failed to execute docker-compose down: %w", err)
	}

	logger.Info("Harbor services and data volumes removed successfully.")
	logger.Debugf("Harbor removal output:\n%s", output)
	return nil
}

func (s *StopAndRemoveHarborStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a stop-and-remove step is a no-op.")
	return nil
}

var _ step.Step = (*StopAndRemoveHarborStep)(nil)
