package harbor

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type ExtractHarborInstallerStep struct {
	step.Base
	RemoteTempPath    string
	RemoteInstallRoot string
}

type ExtractHarborInstallerStepBuilder struct {
	step.Builder[ExtractHarborInstallerStepBuilder, *ExtractHarborInstallerStep]
}

func NewExtractHarborInstallerStepBuilder(ctx runtime.Context, instanceName string) *ExtractHarborInstallerStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	localCfg := cfg.Registry.LocalDeployment

	installRoot := "/opt"
	if localCfg != nil && localCfg.DataRoot != "" {
		installRoot = localCfg.DataRoot
	}

	s := &ExtractHarborInstallerStep{
		RemoteTempPath:    filepath.Join(ctx.GetUploadDir(), "harbor-installer.tgz"),
		RemoteInstallRoot: installRoot,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract Harbor offline installer on registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(ExtractHarborInstallerStepBuilder).Init(s)
	return b
}

func (s *ExtractHarborInstallerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractHarborInstallerStep) getRemoteInstallPath() string {
	return filepath.Join(s.RemoteInstallRoot, "harbor")
}

func (s *ExtractHarborInstallerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	installPath := s.getRemoteInstallPath()
	keyFile := filepath.Join(installPath, "install.sh")

	exists, err := runner.Exists(ctx.GoContext(), conn, keyFile)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", keyFile, ctx.GetHost().GetName(), err)
	}

	if exists {
		logger.Infof("Harbor installation directory '%s' and key file already exist. Skipping extraction.", installPath)
		return true, nil
	}

	logger.Infof("Harbor installation directory or key file not found. Extraction is required.")
	return false, nil
}

func (s *ExtractHarborInstallerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if exists, _ := runner.Exists(ctx.GoContext(), conn, s.RemoteTempPath); !exists {
		return fmt.Errorf("source archive '%s' not found on remote host, ensure upload step ran successfully", s.RemoteTempPath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteInstallRoot, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote install root directory '%s': %w", s.RemoteInstallRoot, err)
	}

	extractCmd := fmt.Sprintf("tar -xzvf %s -C %s", s.RemoteTempPath, s.RemoteInstallRoot)
	logger.Infof("Extracting Harbor installer on remote host with command: %s", extractCmd)

	if _, err := runner.Run(ctx.GoContext(), conn, extractCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to extract archive '%s' on remote host: %w", s.RemoteTempPath, err)
	}

	logger.Infof("Cleaning up temporary archive: %s", s.RemoteTempPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteTempPath, false, false); err != nil {
		logger.Warnf("Failed to remove temporary archive '%s': %v", s.RemoteTempPath, err)
	}

	logger.Infof("Successfully extracted Harbor installer to %s", s.getRemoteInstallPath())
	return nil
}

func (s *ExtractHarborInstallerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	installPath := s.getRemoteInstallPath()
	logger.Warnf("Rolling back by removing extracted directory: %s", installPath)
	if err := runner.Remove(ctx.GoContext(), conn, installPath, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove directory '%s' during rollback: %v", installPath, err)
	}

	logger.Warnf("Rolling back by removing temporary installer: %s", s.RemoteTempPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteTempPath, false, false); err != nil {
		logger.Errorf("Failed to remove temporary file '%s' during rollback: %v", s.RemoteTempPath, err)
	}

	return nil
}

var _ step.Step = (*ExtractHarborInstallerStep)(nil)
