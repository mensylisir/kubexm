package crio

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

type InstallCniStep struct {
	step.Base
}

type InstallCniStepBuilder struct {
	step.Builder[InstallCniStepBuilder, *InstallCniStep]
}

func NewInstallCniStepBuilder(ctx runtime.Context, instanceName string) *InstallCniStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(ComponentCrio, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &InstallCniStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI plugins from CRI-O package", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCniStepBuilder).Init(s)
	return b
}

func (s *InstallCniStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCniStep) getExtractedSourceDir(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(ComponentCrio, arch)
	if err != nil {
		return "", err
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("CRI-O binary info not found for arch %s", arch)
	}

	sourcePath := binaryInfo.FilePath()
	destPath := filepath.Join(filepath.Dir(sourcePath), "cri-o", "bin")
	return destPath, nil
}

func (s *InstallCniStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	targetPath := filepath.Join(common.DefaultCNIBin, "bridge")
	exists, err := runner.Exists(ctx.GoContext(), conn, targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for CNI plugin '%s': %w", targetPath, err)
	}

	if exists {
		logger.Infof("Key CNI plugin '%s' found on target host. Assuming all plugins are installed.", targetPath)
		return true, nil
	}

	logger.Info("Key CNI plugin not found. Installation is required.")
	return false, nil
}

func (s *InstallCniStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	sourceDir, err := s.getExtractedSourceDir(ctx)
	if err != nil {
		return err
	}

	targetDir := common.DefaultCNIBin

	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target directory '%s': %w", targetDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("cni-plugins-%d", time.Now().UnixNano()))
	logger.Debugf("Uploading local CNI plugins directory %s to remote temp dir %s", sourceDir, remoteUploadTmpDir)
	if err := runner.Upload(ctx.GoContext(), conn, sourceDir, remoteUploadTmpDir, false); err != nil {
		return fmt.Errorf("failed to upload CNI plugins directory: %w", err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	moveAndChmodCmd := fmt.Sprintf("cp -f %s/* %s && chmod +x %s/*", remoteUploadTmpDir, targetDir, targetDir)
	logger.Infof("Copying CNI plugins to %s and setting executable permissions", targetDir)
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, moveAndChmodCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to copy and set permissions for CNI plugins: %w", err)
	}

	logger.Info("CNI plugins installed successfully.")
	return nil
}

func (s *InstallCniStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rollback for 'install_cni' is not implemented to avoid deleting shared files. Manual cleanup of '%s' may be required.", common.DefaultCNIBin)

	return nil
}

var _ step.Step = (*InstallCniStep)(nil)
