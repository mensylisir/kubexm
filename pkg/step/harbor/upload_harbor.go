package harbor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type UploadHarborInstallerStep struct {
	step.Base
	RemoteTempPath string
}

type UploadHarborInstallerStepBuilder struct {
	step.Builder[UploadHarborInstallerStepBuilder, *UploadHarborInstallerStep]
}

func NewUploadHarborInstallerStepBuilder(ctx runtime.Context, instanceName string) *UploadHarborInstallerStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &UploadHarborInstallerStep{
		RemoteTempPath: filepath.Join(ctx.GetUploadDir(), "harbor-installer.tgz"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Upload Harbor offline installer to registry node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	b := new(UploadHarborInstallerStepBuilder).Init(s)
	return b
}

func (s *UploadHarborInstallerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UploadHarborInstallerStep) getLocalSourcePath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get Harbor binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("Harbor is unexpectedly disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Upload Harbor installer (version %s, arch %s)", s.Base.Meta.Name, binaryInfo.Version, arch)

	return binaryInfo.FilePath(), nil
}

func (s *UploadHarborInstallerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteTempPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", s.RemoteTempPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Infof("Target file '%s' already exists on remote host. Skipping upload.", s.RemoteTempPath)
		return true, nil
	}

	logger.Infof("Target file '%s' does not exist. Upload is required.", s.RemoteTempPath)
	return false, nil
}

func (s *UploadHarborInstallerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localSourcePath, err := s.getLocalSourcePath(ctx)
	if err != nil {
		return err
	}
	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, ensure download step ran successfully", localSourcePath)
	}

	remoteDir := filepath.Dir(s.RemoteTempPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteDir, err)
	}

	logger.Infof("Uploading %s to %s:%s", filepath.Base(localSourcePath), ctx.GetHost().GetName(), s.RemoteTempPath)
	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, s.RemoteTempPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, s.RemoteTempPath, err)
	}

	logger.Infof("Successfully uploaded Harbor installer to %s", s.RemoteTempPath)
	return nil
}

func (s *UploadHarborInstallerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing uploaded installer: %s", s.RemoteTempPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteTempPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.RemoteTempPath, err)
	}
	return nil
}

var _ step.Step = (*UploadHarborInstallerStep)(nil)
