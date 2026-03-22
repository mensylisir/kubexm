package harbor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
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

func NewConfigureHarborStepBuilder(ctx runtime.ExecutionContext, instanceName string) *ConfigureHarborStepBuilder {
	provider := binary.NewBinaryProvider(ctx)
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
		LocalCertsDir:      ctx.GetHarborCertsDir(),
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

func (s *ConfigureHarborStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	files := s.filesToDistribute()

	for localPath, remotePath := range files {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			err := fmt.Errorf("local source file '%s' not found, ensure previous generate steps ran successfully", localPath)
			result.MarkFailed(err, err.Error())
			return result, err
		}

		remoteDir := filepath.Dir(remotePath)
		if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
			err := fmt.Errorf("failed to create remote directory '%s': %w", remoteDir, err)
			result.MarkFailed(err, err.Error())
			return result, err
		}

		logger.Infof("Uploading %s to %s", filepath.Base(localPath), remotePath)
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			err := fmt.Errorf("failed to upload '%s' to '%s': %w", localPath, remotePath, err)
			result.MarkFailed(err, err.Error())
			return result, err
		}
	}

	logger.Info("Successfully distributed Harbor configuration and certificates.")
	result.MarkCompleted("Harbor configuration distributed successfully")
	return result, nil
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
