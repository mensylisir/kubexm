package harbor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type ConfigureHarborStep struct {
	step.Base
	LocalExtractedPath string
	LocalCertsDir      string
	RemoteInstallDir   string
}

type ConfigureHarborStepBuilder struct {
	step.Builder[ConfigureHarborStepBuilder, *ConfigureHarborStep]
}

func NewConfigureHarborStepBuilder(ctx runtime.Context, instanceName string) *ConfigureHarborStepBuilder {
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

	binaryInfoNoArch, _ := provider.GetBinary(binary.ComponentHarbor, "")
	extractedDirName := filepath.Base(strings.TrimSuffix(binaryInfoNoArch.FileName(), ".tgz"))

	s := &ConfigureHarborStep{
		LocalExtractedPath: filepath.Join(ctx.GetExtractDir(), extractedDirName, "harbor"),
		LocalCertsDir:      filepath.Join(ctx.GetCertsDir(), "harbor"),
		RemoteInstallDir:   filepath.Join(installRoot, "harbor"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute configuration and server certificates to Harbor node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(ConfigureHarborStepBuilder).Init(s)
	return b
}

func (s *ConfigureHarborStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureHarborStep) filesToDistribute() map[string]string {
	return map[string]string{
		filepath.Join(s.LocalExtractedPath, "harbor.yml"): filepath.Join(s.RemoteInstallDir, "harbor.yml"),
		filepath.Join(s.LocalCertsDir, "harbor.crt"):      filepath.Join(s.RemoteInstallDir, "common", "config", "core", "certificates", "harbor.crt"),
		filepath.Join(s.LocalCertsDir, "harbor.key"):      filepath.Join(s.RemoteInstallDir, "common", "config", "core", "certificates", "harbor.key"),
	}
}

func (s *ConfigureHarborStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *ConfigureHarborStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	files := s.filesToDistribute()

	for localPath, remotePath := range files {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return fmt.Errorf("local source file '%s' not found, ensure previous generate steps ran successfully", localPath)
		}

		remoteDir := filepath.Dir(remotePath)
		if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create remote directory '%s': %w", remoteDir, err)
		}

		logger.Infof("Uploading %s to %s", filepath.Base(localPath), remotePath)
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s' to '%s': %w", localPath, remotePath, err)
		}
	}

	logger.Info("Successfully distributed Harbor configuration and certificates.")
	return nil
}

func (s *ConfigureHarborStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	files := s.filesToDistribute()
	for _, remotePath := range files {
		logger.Warnf("Rolling back by removing: %s", remotePath)
		if err := runner.Remove(ctx.GoContext(), conn, remotePath, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", remotePath, err)
		}
	}

	return nil
}

var _ step.Step = (*ConfigureHarborStep)(nil)
