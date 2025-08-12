package cni

import (
	"fmt"
	"github.com/schollz/progressbar/v3"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type ExtractCNIPluginsStep struct {
	step.Base
}

type ExtractCNIPluginsStepBuilder struct {
	step.Builder[ExtractCNIPluginsStepBuilder, *ExtractCNIPluginsStep]
}

func NewExtractCNIPluginsStepBuilder(ctx runtime.Context, instanceName string) *ExtractCNIPluginsStepBuilder {
	s := &ExtractCNIPluginsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract CNI plugins archives for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ExtractCNIPluginsStepBuilder).Init(s)

	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentKubeCNI, representativeArch)

	if err != nil || binaryInfo == nil {
		b.Error = fmt.Errorf("failed to get CNI binary info for pre-check: %w", err)
	}
	return b
}

func (s *ExtractCNIPluginsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractCNIPluginsStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration")
	}

	for _, host := range allHosts {
		archs[host.GetArch()] = true
	}
	return archs, nil
}

func (s *ExtractCNIPluginsStep) getPathsForArch(ctx runtime.ExecutionContext, arch string) (sourcePath, destPath, cacheKey, innerDir string, err error) {
	provider := binary.NewBinaryProvider(ctx)
	binaryInfo, err := provider.GetBinary(binary.ComponentKubeCNI, arch)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get CNI plugins binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", "", "", "", fmt.Errorf("CNI plugins are disabled for arch %s", arch)
	}

	sourcePath = binaryInfo.FilePath()
	innerDir = "cni-plugins"
	destPath = filepath.Join(filepath.Dir(sourcePath), innerDir)
	cacheKey = sourcePath

	return sourcePath, destPath, cacheKey, innerDir, nil
}

func (s *ExtractCNIPluginsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts found. Step is done.")
		return true, nil
	}

	allDone := true
	for arch := range requiredArchs {
		sourcePath, destPath, cacheKey, _, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			if strings.Contains(err.Error(), "disabled for arch") {
				continue
			}
			return false, err
		}

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return false, fmt.Errorf("source archive '%s' for arch %s not found, ensure download step ran successfully", sourcePath, arch)
		}

		keyFile := filepath.Join(destPath, "bridge")

		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			logger.Infof("Key file '%s' for arch %s does not exist. Extraction is required.", keyFile, arch)
			allDone = false
		} else {
			ctx.GetTaskCache().Set(cacheKey, destPath)
		}
	}

	if allDone {
		logger.Info("All required CNI plugins for all architectures already extracted and are valid.")
	}

	return allDone, nil
}

func (s *ExtractCNIPluginsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require CNI plugins. Skipping extraction.")
		return nil
	}

	for arch := range requiredArchs {
		if err := s.extractFileForArch(ctx, arch); err != nil {
			return err
		}
	}

	logger.Info("All required CNI plugins archives have been extracted successfully.")
	return nil
}

func (s *ExtractCNIPluginsStep) extractFileForArch(ctx runtime.ExecutionContext, arch string) error {
	logger := ctx.GetLogger().With("arch", arch)
	sourcePath, destPath, cacheKey, _, err := s.getPathsForArch(ctx, arch)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Debugf("Skipping CNI plugins extraction for arch %s as it's not required.", arch)
			return nil
		}
		return err
	}

	keyFile := filepath.Join(destPath, "bridge")
	if _, err := os.Stat(keyFile); err == nil {
		logger.Infof("Skipping extraction, destination already exists and is valid.")
		ctx.GetTaskCache().Set(cacheKey, destPath)
		return nil
	}

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destPath, err)
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
	progressFunc := func(fileName string, totalBytes int64) { bar.Add64(totalBytes) }
	ar := helpers.NewArchiver(helpers.WithOverwrite(true), helpers.WithProgress(progressFunc))

	if err := ar.Extract(sourcePath, destPath); err != nil {
		_ = bar.Clear()
		_ = os.RemoveAll(destPath)
		return fmt.Errorf("failed to extract archive '%s': %w", sourcePath, err)
	}

	_ = bar.Finish()
	logger.Infof("Successfully extracted archive for arch %s.", arch)

	ctx.GetTaskCache().Set(cacheKey, destPath)
	return nil
}

func (s *ExtractCNIPluginsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	for arch := range requiredArchs {
		_, destPath, _, _, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			if strings.Contains(err.Error(), "disabled for arch") {
				continue
			}
			logger.Warnf("Could not get paths for arch %s during rollback: %v", arch, err)
			continue
		}

		logger.Warnf("Rolling back by deleting extracted directory: %s", destPath)
		_ = os.RemoveAll(destPath)
	}

	return nil
}

var _ step.Step = (*ExtractCNIPluginsStep)(nil)
