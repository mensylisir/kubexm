package etcd

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

type ExtractEtcdStep struct {
	step.Base
}

type ExtractEtcdStepBuilder struct {
	step.Builder[ExtractEtcdStepBuilder, *ExtractEtcdStep]
}

func NewExtractEtcdStepBuilder(ctx runtime.Context, instanceName string) *ExtractEtcdStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentEtcd, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &ExtractEtcdStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract etcd archives for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ExtractEtcdStepBuilder).Init(s)
	return b
}

func (s *ExtractEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractEtcdStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	etcdHosts := ctx.GetHostsByRole("")
	if len(etcdHosts) == 0 {
		return nil, fmt.Errorf("no control-plane hosts found to determine required etcd architectures")
	}

	provider := binary.NewBinaryProvider(ctx)
	for _, host := range etcdHosts {
		binaryInfo, err := provider.GetBinary(binary.ComponentEtcd, host.GetArch())
		if err != nil {
			return nil, fmt.Errorf("error checking if etcd is needed for host %s: %w", host.GetName(), err)
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

func (s *ExtractEtcdStep) getPathsForArch(ctx runtime.ExecutionContext, arch string) (sourcePath, destPath, cacheKey string, err error) {
	provider := binary.NewBinaryProvider(ctx)
	binaryInfo, err := provider.GetBinary(binary.ComponentEtcd, arch)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get etcd binary info for arch %s: %w", arch, err)
	}
	if binaryInfo == nil {
		return "", "", "", fmt.Errorf("etcd is disabled for arch %s", arch)
	}

	sourcePath = binaryInfo.FilePath()
	innerDir := "etcd"
	destPath = filepath.Join(filepath.Dir(sourcePath), innerDir)
	cacheKey = sourcePath

	return sourcePath, destPath, cacheKey, nil
}

func (s *ExtractEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require etcd. Step is done.")
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

		keyFile := filepath.Join(destPath, "etcd")

		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			logger.Infof("Key file '%s' for arch %s does not exist. Extraction is required.", keyFile, arch)
			allDone = false
		} else {
			ctx.GetTaskCache().Set(cacheKey, destPath)
		}
	}

	if allDone {
		logger.Info("All required etcd archives for all architectures already extracted and are valid.")
	}

	return allDone, nil
}

func (s *ExtractEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require etcd. Skipping extraction.")
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

	logger.Info("All required etcd archives have been extracted successfully.")
	return nil
}

func (s *ExtractEtcdStep) extractFileForArch(ctx runtime.ExecutionContext, arch string) error {
	logger := ctx.GetLogger()
	sourcePath, destPath, cacheKey, err := s.getPathsForArch(ctx, arch)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Debugf("Skipping etcd extraction for arch %s as it's not required.", arch)
			return nil
		}
		return err
	}

	keyFile := filepath.Join(destPath, "etcd")
	if _, err := os.Stat(keyFile); err == nil {
		logger.Infof("Skipping extraction for arch %s, destination already exists and is valid.", arch)
		ctx.GetTaskCache().Set(cacheKey, destPath)
		return nil
	}

	tempExtractDir, err := os.MkdirTemp(filepath.Dir(destPath), "etcd-extract-")
	if err != nil {
		return fmt.Errorf("failed to create temp extraction directory: %w", err)
	}
	defer os.RemoveAll(tempExtractDir)

	logger.Infof("Extracting archive '%s' to temporary directory '%s'...", sourcePath, tempExtractDir)

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

	if err := ar.Extract(sourcePath, tempExtractDir); err != nil {
		_ = bar.Clear()
		return fmt.Errorf("failed to extract archive '%s' to temp dir: %w", sourcePath, err)
	}
	_ = bar.Finish()

	innerDirName := strings.TrimSuffix(filepath.Base(sourcePath), ".tar.gz")
	sourceOfMove := filepath.Join(tempExtractDir, innerDirName)

	filesToMove, err := os.ReadDir(sourceOfMove)
	if err != nil {
		return fmt.Errorf("failed to read temp extracted directory %s: %w", sourceOfMove, err)
	}

	for _, file := range filesToMove {
		src := filepath.Join(sourceOfMove, file.Name())
		dst := filepath.Join(destPath, file.Name())
		logger.Debugf("Moving extracted file from %s to %s", src, dst)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move extracted file %s: %w", file.Name(), err)
		}
	}

	logger.Infof("Successfully extracted archive for arch %s.", arch)
	ctx.GetTaskCache().Set(cacheKey, destPath)
	return nil
}

func (s *ExtractEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	for arch := range requiredArchs {
		sourcePath, destPath, _, err := s.getPathsForArch(ctx, arch)
		if err != nil {
			if strings.Contains(err.Error(), "disabled for arch") {
				continue
			}
			logger.Warnf("Could not get paths for arch %s during rollback: %v", arch, err)
			continue
		}

		filesToClean, err := helpers.ListFilesInEtcdArchive(sourcePath)
		if err != nil {
			logger.Errorf("Could not list files in archive %s for rollback, manual cleanup of %s may be required: %v", sourcePath, destPath, err)
			continue
		}

		logger.Warnf("Rolling back by deleting extracted etcd files in: %s", destPath)
		for _, fileName := range filesToClean {
			pathToRemove := filepath.Join(destPath, fileName)
			if err := os.RemoveAll(pathToRemove); err != nil && !os.IsNotExist(err) {
				logger.Errorf("Failed to remove path %s during rollback: %v", pathToRemove, err)
			}
		}
	}

	return nil
}

var _ step.Step = (*ExtractEtcdStep)(nil)
