package docker

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
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/schollz/progressbar/v3"
)

type ExtractCriDockerdStep struct {
	step.Base
}

type ExtractCriDockerdStepBuilder struct {
	step.Builder[ExtractCriDockerdStepBuilder, *ExtractCriDockerdStep]
}

func NewExtractCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *ExtractCriDockerdStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentCriDockerd, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &ExtractCriDockerdStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract cri-dockerd archives for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ExtractCriDockerdStepBuilder).Init(s)
	return b
}

func (s *ExtractCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractCriDockerdStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration")
	}

	provider := binary.NewBinaryProvider(ctx)
	for _, host := range allHosts {
		binaryInfo, err := provider.GetBinary(binary.ComponentCriDockerd, host.GetArch())
		if err != nil {
			return nil, fmt.Errorf("error checking if cri-dockerd is needed for host %s: %w", host.GetName(), err)
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

func (s *ExtractCriDockerdStep) getPathsForArch(ctx runtime.ExecutionContext, arch string) (sourcePath, destPath string, err error) {
	provider := binary.NewBinaryProvider(ctx)
	binaryInfo, err := provider.GetBinary(binary.ComponentCriDockerd, arch)
	if err != nil {
		return "", "", fmt.Errorf("failed to get cri-dockerd binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", "", fmt.Errorf("cri-dockerd is unexpectedly disabled for arch %s", arch)
	}

	sourcePath = binaryInfo.FilePath()
	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tgz")
	destPath = filepath.Join(common.DefaultExtractTmpDir, destDirName)

	return sourcePath, destPath, nil
}

func (s *ExtractCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require cri-dockerd. Step is done.")
		return true, nil
	}

	allDone := true
	for arch := range requiredArchs {
		sourcePath, destPath, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			return false, err
		}

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return false, fmt.Errorf("source archive '%s' for arch %s not found, ensure download step ran successfully", sourcePath, arch)
		}

		_, err = os.Stat(destPath)
		if os.IsNotExist(err) {
			logger.Infof("Extraction destination '%s' for arch %s does not exist. Extraction is required.", destPath, arch)
			allDone = false
			continue
		}

		keyFile := filepath.Join(destPath, "cri-dockerd", "cri-dockerd")
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			logger.Warnf("Destination directory for arch %s exists, but key file '%s' is missing. Re-extracting.", arch, keyFile)
			allDone = false
		}
	}

	if allDone {
		logger.Info("All required cri-dockerd archives for all architectures already extracted and are valid.")
	}

	return allDone, nil
}

func (s *ExtractCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require cri-dockerd. Skipping extraction.")
		return nil
	}

	if err := os.MkdirAll(common.DefaultExtractTmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create global extract directory '%s': %w", common.DefaultExtractTmpDir, err)
	}

	for arch := range requiredArchs {
		if err := s.extractFileForArch(ctx, arch); err != nil {
			return err
		}
	}

	logger.Info("All required cri-dockerd archives have been extracted successfully.")
	return nil
}

func (s *ExtractCriDockerdStep) extractFileForArch(ctx runtime.ExecutionContext, arch string) error {
	logger := ctx.GetLogger()
	sourcePath, destPath, err := s.getPathsForArch(ctx, arch)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(destPath, "cri-dockerd", "cri-dockerd")); err == nil {
		logger.Infof("Skipping extraction for arch %s, destination already exists and is valid.", arch)
		return nil
	}

	logger.Infof("Extracting archive '%s' to '%s'...", sourcePath, destPath)

	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to get info for source file %s: %w", sourcePath, err)
	}

	bar := progressbar.NewOptions64(
		fileInfo.Size(),
		progressbar.OptionSetDescription(fmt.Sprintf("Extracting %s", filepath.Base(sourcePath))),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	progressFunc := func(fileName string, totalBytes int64) {
		bar.Add64(totalBytes)
	}

	ar := helpers.NewArchiver(
		helpers.WithOverwrite(true),
		helpers.WithProgress(progressFunc),
	)

	if err := ar.Extract(sourcePath, destPath); err != nil {
		_ = bar.Clear()
		_ = os.RemoveAll(destPath)
		return fmt.Errorf("failed to extract archive '%s': %w", sourcePath, err)
	}

	_ = bar.Finish()
	logger.Infof("Successfully extracted archive for arch %s.", arch)

	return nil
}

func (s *ExtractCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	for arch := range requiredArchs {
		_, destPath, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			continue
		}
		logger.Warnf("Rolling back by deleting extracted directory: %s", destPath)
		_ = os.RemoveAll(destPath)
	}

	return nil
}

var _ step.Step = (*ExtractCriDockerdStep)(nil)
