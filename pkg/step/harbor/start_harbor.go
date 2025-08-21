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

type InstallAndStartHarborStep struct {
	step.Base
	RemoteInstallDir string
}

type InstallAndStartHarborStepBuilder struct {
	step.Builder[InstallAndStartHarborStepBuilder, *InstallAndStartHarborStep]
}

func NewInstallAndStartHarborStepBuilder(ctx runtime.Context, instanceName string) *InstallAndStartHarborStepBuilder {
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

	s := &InstallAndStartHarborStep{
		RemoteInstallDir: filepath.Join(installRoot, "harbor"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install and start Harbor services on the registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 30 * time.Minute

	b := new(InstallAndStartHarborStepBuilder).Init(s)
	return b
}

func (s *InstallAndStartHarborStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallAndStartHarborStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runner.Run(ctx.GoContext(), conn, "command -v docker-compose", s.Sudo); err != nil {
		return false, fmt.Errorf("docker-compose not found in PATH on remote host, ensure it's installed first")
	}

	// Check for key harbor services. If they are all running, we can consider the step as done.
	checkCmd := "docker ps --filter name=harbor-core --filter status=running --format '{{.Names}}' | grep . > /dev/null && " +
		"docker ps --filter name=harbor-db --filter status=running --format '{{.Names}}' | grep . > /dev/null && " +
		"docker ps --filter name=harbor-jobservice --filter status=running --format '{{.Names}}' | grep . > /dev/null && " +
		"docker ps --filter name=harbor-portal --filter status=running --format '{{.Names}}' | grep . > /dev/null"

	_, err = runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err == nil {
		logger.Info("Key Harbor containers (core, db, jobservice, portal) are already running. Step is done.")
		return true, nil
	}

	logger.Info("One or more key Harbor containers are not running. Installation/start is required.")
	return false, nil
}

func (s *InstallAndStartHarborStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	installScriptPath := filepath.Join(s.RemoteInstallDir, "install.sh")
	if exists, _ := runner.Exists(ctx.GoContext(), conn, installScriptPath); !exists {
		return fmt.Errorf("Harbor install script not found at '%s', ensure previous steps ran successfully", installScriptPath)
	}

	installCmd := fmt.Sprintf("cd %s && ./install.sh", s.RemoteInstallDir)
	logger.Infof("Executing Harbor installation script on remote host: %s", installCmd)

	output, err := runner.Run(ctx.GoContext(), conn, installCmd, s.Sudo)
	if err != nil {
		logger.Errorf("Harbor installation failed. Full output:\n%s", output)
		return fmt.Errorf("failed to execute Harbor install script: %w", err)
	}

	logger.Info("Harbor installation script executed successfully.")
	logger.Debugf("Harbor installation output:\n%s", output)
	return nil
}

func (s *InstallAndStartHarborStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	uninstallScriptPath := filepath.Join(s.RemoteInstallDir, "install.sh")
	if exists, _ := runner.Exists(ctx.GoContext(), conn, uninstallScriptPath); !exists {
		logger.Warnf("Harbor install script not found at '%s', cannot run uninstall.", uninstallScriptPath)
		return nil
	}

	uninstallCmd := fmt.Sprintf("cd %s && docker-compose down -v", s.RemoteInstallDir)

	logger.Warnf("Rolling back by running: %s", uninstallCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, uninstallCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to run docker-compose down during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*InstallAndStartHarborStep)(nil)
