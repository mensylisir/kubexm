package harbor

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type RestartHarborStep struct {
	step.Base
	RemoteInstallDir string
}

type RestartHarborStepBuilder struct {
	step.Builder[RestartHarborStepBuilder, *RestartHarborStep]
}

func NewRestartHarborStepBuilder(ctx runtime.Context, instanceName string) *RestartHarborStepBuilder {
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

	s := &RestartHarborStep{
		RemoteInstallDir: filepath.Join(installRoot, "harbor"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart Harbor services on the registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	b := new(RestartHarborStepBuilder).Init(s)
	return b
}

func (s *RestartHarborStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartHarborStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// A restart operation should generally always run.
	return false, nil
}

func (s *RestartHarborStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	composeFilePath := filepath.Join(s.RemoteInstallDir, "docker-compose.yml")
	if exists, _ := runner.Exists(ctx.GoContext(), conn, composeFilePath); !exists {
		logger.Warnf("Harbor docker-compose.yml not found at '%s', cannot restart.", composeFilePath)
		return nil
	}

	restartCmd := fmt.Sprintf("cd %s && docker-compose restart", s.RemoteInstallDir)
	logger.Infof("Executing Harbor restart command on remote host: %s", restartCmd)

	output, err := runner.Run(ctx.GoContext(), conn, restartCmd, s.Sudo)
	if err != nil {
		logger.Errorf("Harbor restart failed. Full output:\n%s", output)
		return fmt.Errorf("failed to execute docker-compose restart: %w", err)
	}

	logger.Info("Harbor services restarted successfully.")
	logger.Debugf("Harbor restart output:\n%s", output)
	return nil
}

func (s *RestartHarborStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a restart step is a no-op.")
	return nil
}

var _ step.Step = (*RestartHarborStep)(nil)
