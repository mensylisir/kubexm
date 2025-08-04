package registry

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

// ExtractRegistryStep 是一个无状态的编排步骤。
type ExtractRegistryStep struct {
	step.Base
}

type ExtractRegistryStepBuilder struct {
	step.Builder[ExtractRegistryStepBuilder, *ExtractRegistryStep]
}

// NewExtractRegistryStepBuilder 在创建前会检查 Registry 是否被启用。
func NewExtractRegistryStepBuilder(ctx runtime.Context, instanceName string) *ExtractRegistryStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil // 如果禁用或无法获取信息，则不创建此步骤
	}

	s := &ExtractRegistryStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract registry archives for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ExtractRegistryStepBuilder).Init(s)
	return b
}

func (s *ExtractRegistryStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// getRequiredArchs 辅助函数，精确找出集群中所有需要 registry 的节点架构。
func (s *ExtractRegistryStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(registryHosts) == 0 {
		return nil, fmt.Errorf("no hosts with role '%s' found to determine required registry architectures", common.RoleRegistry)
	}

	provider := binary.NewBinaryProvider(ctx)
	for _, host := range registryHosts {
		binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, host.GetArch())
		if err != nil {
			return nil, fmt.Errorf("error checking if registry is needed for host %s: %w", host.GetName(), err)
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

// getPathsForArch 辅助函数，为特定架构计算路径。
func (s *ExtractRegistryStep) getPathsForArch(ctx runtime.ExecutionContext, arch string) (sourcePath, destPath, cacheKey string, err error) {
	provider := binary.NewBinaryProvider(ctx)
	binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, arch)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get registry binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", "", "", fmt.Errorf("registry is unexpectedly disabled for arch %s", arch)
	}

	sourcePath = binaryInfo.FilePath()
	destDirName := strings.TrimSuffix(binaryInfo.FileName(), ".tar.gz")
	destPath = filepath.Join(ctx.GetExtractDir(), destDirName)
	cacheKey = sourcePath

	return sourcePath, destPath, cacheKey, nil
}

// Precheck 现在会检查所有需要的架构对应的文件是否已解压。
func (s *ExtractRegistryStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require registry. Step is done.")
		return true, nil
	}

	allDone := true
	for arch := range requiredArchs {
		sourcePath, destPath, cacheKey, err := s.getPathsForArch(ctx, arch)
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

		// 检查一个关键文件 'registry' 来判断解压是否完整
		keyFile := filepath.Join(destPath, "registry")
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			logger.Warnf("Destination directory for arch %s exists, but key file '%s' is missing. Re-extracting.", arch, keyFile)
			allDone = false
		} else {
			// 如果目录和关键文件都存在，我们就缓存这个路径
			ctx.GetTaskCache().Set(cacheKey, destPath)
		}
	}

	if allDone {
		logger.Info("All required registry archives for all architectures already extracted and are valid.")
	}

	return allDone, nil
}

// Run 方法现在会遍历所有需要的架构并进行解压。
func (s *ExtractRegistryStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require registry. Skipping extraction.")
		return nil
	}

	if err := os.MkdirAll(ctx.GetExtractDir(), 0755); err != nil {
		return fmt.Errorf("failed to create global extract directory '%s': %w", ctx.GetExtractDir(), err)
	}

	for arch := range requiredArchs {
		if err := s.extractFileForArch(ctx, arch); err != nil {
			return err
		}
	}

	logger.Info("All required registry archives have been extracted successfully.")
	return nil
}

// extractFileForArch 是实际的解压逻辑，被 Run 方法循环调用。
func (s *ExtractRegistryStep) extractFileForArch(ctx runtime.ExecutionContext, arch string) error {
	logger := ctx.GetLogger()
	sourcePath, destPath, cacheKey, err := s.getPathsForArch(ctx, arch)
	if err != nil {
		return err
	}

	// 在执行前再次检查，如果已解压且有效，则跳过
	if _, err := os.Stat(filepath.Join(destPath, "registry")); err == nil {
		logger.Infof("Skipping extraction for arch %s, destination already exists and is valid.", arch)
		ctx.GetTaskCache().Set(cacheKey, destPath)
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

	ctx.GetTaskCache().Set(cacheKey, destPath)
	return nil
}

func (s *ExtractRegistryStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	for arch := range requiredArchs {
		_, destPath, _, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			continue
		}
		logger.Warnf("Rolling back by deleting extracted directory: %s", destPath)
		_ = os.RemoveAll(destPath)
	}

	return nil
}

var _ step.Step = (*ExtractRegistryStep)(nil)
