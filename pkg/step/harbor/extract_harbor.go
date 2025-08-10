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
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/schollz/progressbar/v3"
)

type ExtractHarborStep struct {
	step.Base
}

type ExtractHarborStepBuilder struct {
	step.Builder[ExtractHarborStepBuilder, *ExtractHarborStep]
}

func NewExtractHarborStepBuilder(ctx runtime.Context, instanceName string) *ExtractHarborStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &ExtractHarborStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract Harbor installers for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(ExtractHarborStepBuilder).Init(s)
	return b
}

func (s *ExtractHarborStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractHarborStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(registryHosts) == 0 {
		return nil, fmt.Errorf("no hosts with role '%s' found to determine required Harbor architectures", common.RoleRegistry)
	}

	provider := binary.NewBinaryProvider(ctx)
	for _, host := range registryHosts {
		binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, host.GetArch())
		if err != nil {
			return nil, fmt.Errorf("error checking if Harbor is needed for host %s: %w", host.GetName(), err)
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

func (s *ExtractHarborStep) getPathsForArch(ctx runtime.ExecutionContext, arch string) (sourcePath, destPath, cacheKey string, err error) {
	provider := binary.NewBinaryProvider(ctx)
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, arch)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get Harbor binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", "", "", fmt.Errorf("Harbor is disabled for arch %s", arch)
	}

	sourcePath = binaryInfo.FilePath()
	destPath = filepath.Dir(sourcePath)
	cacheKey = sourcePath

	return sourcePath, destPath, cacheKey, nil
}

func (s *ExtractHarborStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require Harbor. Step is done.")
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

		innerDir := "harbor"
		keyFile := filepath.Join(destPath, innerDir, "install.sh")

		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			logger.Infof("Key file '%s' for arch %s does not exist. Extraction is required.", keyFile, arch)
			allDone = false
		} else {
			ctx.GetTaskCache().Set(cacheKey, filepath.Dir(keyFile))
		}
	}

	if allDone {
		logger.Info("All required Harbor installers for all architectures already extracted and are valid.")
	}

	return allDone, nil
}

func (s *ExtractHarborStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require Harbor. Skipping extraction.")
		return nil
	}

	for arch := range requiredArchs {
		if err := s.extractFileForArch(ctx, arch); err != nil {
			return err
		}
	}

	logger.Info("All required Harbor installer archives have been extracted successfully.")
	return nil
}

func (s *ExtractHarborStep) extractFileForArch(ctx runtime.ExecutionContext, arch string) error {
	logger := ctx.GetLogger().With("arch", arch)
	sourcePath, destPath, cacheKey, err := s.getPathsForArch(ctx, arch)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Debugf("Skipping Harbor extraction for arch %s as it's not required.", arch)
			return nil
		}
		return err
	}

	innerDir := "harbor"
	keyFile := filepath.Join(destPath, innerDir, "install.sh")
	if _, err := os.Stat(keyFile); err == nil {
		logger.Infof("Skipping extraction for arch %s, destination already exists and is valid.", arch)
		ctx.GetTaskCache().Set(cacheKey, filepath.Dir(keyFile))
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
		_ = os.RemoveAll(filepath.Join(destPath, innerDir))
		return fmt.Errorf("failed to extract archive '%s': %w", sourcePath, err)
	}

	_ = bar.Finish()
	logger.Infof("Successfully extracted archive for arch %s.", arch)

	ctx.GetTaskCache().Set(cacheKey, filepath.Dir(keyFile))
	return nil
}

func (s *ExtractHarborStep) Rollback(ctx runtime.ExecutionContext) error {
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

		dirToRemove := filepath.Join(destPath, "harbor")
		logger.Warnf("Rolling back by deleting extracted directory: %s", dirToRemove)
		_ = os.RemoveAll(dirToRemove)
	}

	return nil
}

var _ step.Step = (*ExtractHarborStep)(nil)
