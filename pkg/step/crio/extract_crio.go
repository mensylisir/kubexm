package crio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/schollz/progressbar/v3"
)

type ExtractCrioStep struct {
	step.Base
}

type ExtractCrioStepBuilder struct {
	step.Builder[ExtractCrioStepBuilder, *ExtractCrioStep]
}

func NewExtractCrioStepBuilder(ctx runtime.Context, instanceName string) *ExtractCrioStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(ComponentCrio, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &ExtractCrioStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract CRI-O archives for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ExtractCrioStepBuilder).Init(s)
	return b
}

func (s *ExtractCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractCrioStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration")
	}

	provider := binary.NewBinaryProvider(ctx)
	for _, host := range allHosts {
		binaryInfo, err := provider.GetBinary(ComponentCrio, host.GetArch())
		if err != nil {
			return nil, fmt.Errorf("error checking if CRI-O is needed for host %s: %w", host.GetName(), err)
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

func (s *ExtractCrioStep) getPathsForArch(ctx runtime.ExecutionContext, arch string) (sourcePath, destPath, cacheKey string, err error) {
	provider := binary.NewBinaryProvider(ctx)
	binaryInfo, err := provider.GetBinary(ComponentCrio, arch)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get CRI-O binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", "", "", fmt.Errorf("CRI-O is disabled for arch %s", arch)
	}

	sourcePath = binaryInfo.FilePath()
	destPath = filepath.Dir(sourcePath)
	cacheKey = sourcePath

	return sourcePath, destPath, cacheKey, nil
}

func (s *ExtractCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require CRI-O. Step is done.")
		return true, nil
	}

	allDone := true
	for arch := range requiredArchs {
		sourcePath, destPath, cacheKey, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			if strings.Contains(err.Error(), "disabled for arch") {
				continue
			}
			return false, err
		}

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return false, fmt.Errorf("source archive '%s' for arch %s not found, ensure download step ran successfully", sourcePath, arch)
		}

		keyDir := destPath
		if _, err := os.Stat(keyDir); os.IsNotExist(err) {
			logger.Infof("Key directory '%s' for arch %s does not exist. Extraction is required.", keyDir, arch)
			allDone = false
		} else {
			ctx.GetTaskCache().Set(cacheKey, keyDir)
		}
	}

	if allDone {
		logger.Info("All required CRI-O archives for all architectures already extracted.")
	}
	return allDone, nil
}

func (s *ExtractCrioStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require CRI-O. Skipping extraction.")
		return nil
	}

	for arch := range requiredArchs {
		if err := s.extractFileForArch(ctx, arch); err != nil {
			return err
		}
	}

	logger.Info("All required CRI-O archives have been extracted successfully.")
	return nil
}

func (s *ExtractCrioStep) extractFileForArch(ctx runtime.ExecutionContext, arch string) error {
	logger := ctx.GetLogger()
	sourcePath, destPath, cacheKey, err := s.getPathsForArch(ctx, arch)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Debugf("Skipping CRI-O extraction for arch %s as it's not required.", arch)
			return nil
		}
		return err
	}

	keyDir := filepath.Join(destPath, "crio")
	if _, err := os.Stat(keyDir); err == nil {
		logger.Infof("Skipping extraction, destination directory '%s' already exists.", keyDir)
		ctx.GetTaskCache().Set(cacheKey, keyDir)
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
	progressFunc := func(fileName string, totalBytes int64) { bar.Add64(totalBytes) }
	ar := helpers.NewArchiver(helpers.WithOverwrite(true), helpers.WithProgress(progressFunc))

	if err := ar.Extract(sourcePath, destPath); err != nil {
		_ = bar.Clear()
		_ = os.RemoveAll(keyDir)
		return fmt.Errorf("failed to extract archive '%s': %w", sourcePath, err)
	}

	_ = bar.Finish()
	logger.Infof("Successfully extracted archive for arch %s.", arch)
	ctx.GetTaskCache().Set(cacheKey, keyDir)
	return nil
}

func (s *ExtractCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	for arch := range requiredArchs {
		_, destPath, _, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			if strings.Contains(err.Error(), "disabled for arch") {
				continue
			}
			logger.Warnf("Could not get paths for arch %s during rollback: %v", arch, err)
			continue
		}

		dirToRemove := filepath.Join(destPath, "crio")
		logger.Warnf("Rolling back by deleting extracted directory: %s", dirToRemove)
		_ = os.RemoveAll(dirToRemove)
	}
	return nil
}

var _ step.Step = (*ExtractCrioStep)(nil)
