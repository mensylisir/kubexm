package docker

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallCriCtlStep struct {
	step.Base
	Version                string
	Arch                   string
	WorkDir                string
	ClusterName            string
	Zone                   string
	RemoteCriCtlTargetPath string
	CrictlPermissions      string
}

type InstallCriCtlStepBuilder struct {
	step.Builder[InstallCriCtlStepBuilder, *InstallCriCtlStep]
}

func NewInstallCriCtlStepBuilder(ctx runtime.Context, instanceName string) *InstallCriCtlStepBuilder {
	s := &InstallCriCtlStep{
		Version:                common.DefaultCrictlVersion,
		Arch:                   "",
		WorkDir:                ctx.GetGlobalWorkDir(),
		ClusterName:            ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:                   util.GetZone(),
		RemoteCriCtlTargetPath: filepath.Join(common.DefaultBinDir, "crictl"),
		CrictlPermissions:      "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install crictl binary for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallCriCtlStepBuilder).Init(s)
	return b
}

func (b *InstallCriCtlStepBuilder) WithRemoteRuncTargetPath(remoteRuncTargetPath string) *InstallCriCtlStepBuilder {
	b.Step.RemoteCriCtlTargetPath = remoteRuncTargetPath
	return b
}

func (b *InstallCriCtlStepBuilder) WithCrictlPermissions(permission string) *InstallCriCtlStepBuilder {
	b.Step.CrictlPermissions = permission
	return b
}

func (s *InstallCriCtlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCriCtlStep) getLocalExtractedPath() (string, error) {
	provider := util.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(
		util.ComponentCriCtl,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get crictl binary info: %w", err)
	}
	destDirName := strings.TrimSuffix(binaryInfo.FileName, ".tar.gz")
	return filepath.Join(common.DefaultExtractTmpDir, destDirName), nil
}

func (s *InstallCriCtlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteCriCtlTargetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", s.RemoteCriCtlTargetPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Infof("Target file '%s' already exists. Step is done.", s.RemoteCriCtlTargetPath)
		return true, nil
	}

	logger.Infof("Target file '%s' does not exist. Installation is required.", s.RemoteCriCtlTargetPath)
	return false, nil
}

func (s *InstallCriCtlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localExtractedPath, err := s.getLocalExtractedPath()
	if err != nil {
		return err
	}

	localSourcePath := filepath.Join(localExtractedPath, "crictl")

	if _, err := os.Stat(localSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("local source file '%s' not found, please run extract step first", localSourcePath)
	}

	remoteUploadTmpDir := common.DefaultUploadTmpDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}

	remoteTempPath := filepath.Join(remoteUploadTmpDir, "crictl")
	logger.Infof("Uploading %s to %s:%s", localSourcePath, ctx.GetHost().GetName(), remoteTempPath)

	if err := runner.Upload(ctx.GoContext(), conn, localSourcePath, remoteTempPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload '%s' to '%s': %w", localSourcePath, remoteTempPath, err)
	}

	installCmd := fmt.Sprintf("install -o root -g root -m %s %s %s", s.CrictlPermissions, remoteTempPath, s.RemoteCriCtlTargetPath)
	logger.Infof("Installing file to %s on remote host", s.RemoteCriCtlTargetPath)

	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		runner.Remove(ctx.GoContext(), conn, remoteTempPath, false, false)
		return fmt.Errorf("failed to install file '%s' on remote host: %w", s.RemoteCriCtlTargetPath, err)
	}

	return nil
}

func (s *InstallCriCtlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.RemoteCriCtlTargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteCriCtlTargetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.RemoteCriCtlTargetPath, err)
	}

	return nil
}

var _ step.Step = (*InstallCriCtlStep)(nil)
