package containerd

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
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/schollz/progressbar/v3"
)

type ExtractContainerdStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
}

type ExtractContainerdStepBuilder struct {
	step.Builder[ExtractContainerdStepBuilder, *ExtractContainerdStep]
}

func NewExtractContainerdStepBuilder(ctx runtime.Context, instanceName string) *ExtractContainerdStepBuilder {
	cfg := ctx.GetClusterConfig().Spec

	s := &ExtractContainerdStep{
		Version:     cfg.Kubernetes.ContainerRuntime.Containerd.Version,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        util.GetZone(),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract containerd archive for version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ExtractContainerdStepBuilder).Init(s)
	return b
}

func (s *ExtractContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ExtractContainerdStep) getPaths() (sourcePath, destPath, cacheKey string, err error) {
	provider := util.NewBinaryProvider()
	binaryInfo, err := provider.GetBinaryInfo(
		util.ComponentContainerd,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get containerd binary info: %w", err)
	}

	sourcePath = binaryInfo.FilePath

	destDirName := strings.TrimSuffix(binaryInfo.FileName, ".tar.gz")
	destPath = filepath.Join(common.DefaultExtractTmpDir, destDirName)
	cacheKey = sourcePath

	return sourcePath, destPath, cacheKey, nil
}

func (s *ExtractContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	sourcePath, destPath, cacheKey, err := s.getPaths()
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return false, fmt.Errorf("source archive '%s' not found, please run download step first", sourcePath)
	}

	info, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		logger.Infof("Extraction destination '%s' does not exist. Extraction is required.", destPath)
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat destination directory '%s': %w", destPath, err)
	}
	if !info.IsDir() {
		logger.Warnf("Destination '%s' exists but is not a directory. Removing and re-extracting.", destPath)
		if err := os.RemoveAll(destPath); err != nil {
			return false, fmt.Errorf("failed to remove invalid destination '%s': %w", destPath, err)
		}
		return false, nil
	}

	keyFile := filepath.Join(destPath, "bin", "containerd")
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		logger.Warnf("Destination directory '%s' exists, but key file '%s' is missing. Re-extracting.", destPath, keyFile)
		return false, nil
	}

	logger.Info("Destination directory exists and seems valid. Step is done.")
	ctx.GetTaskCache().Set(cacheKey, destPath)
	return true, nil
}

func (s *ExtractContainerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	sourcePath, destPath, cacheKey, err := s.getPaths()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(common.DefaultExtractTmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create global extract directory '%s': %w", common.DefaultExtractTmpDir, err)
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
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	progressFunc := func(fileName string, totalBytes int64) {
		bar.Describe(fmt.Sprintf("Extracting: %s", fileName))
	}

	ar := helpers.NewArchiver(
		helpers.WithOverwrite(true),
		helpers.WithProgress(progressFunc),
	)

	if err := ar.Extract(sourcePath, destPath); err != nil {
		bar.Clear()
		os.RemoveAll(destPath)
		return fmt.Errorf("failed to extract archive '%s': %w", sourcePath, err)
	}

	bar.Finish()

	logger.Info("Successfully extracted archive.")
	ctx.GetTaskCache().Set(cacheKey, destPath)
	return nil
}

func (s *ExtractContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	_, destPath, _, err := s.getPaths()
	if err != nil {
		logger.Errorf("Failed to get destination path during rollback, cannot determine directory to delete. Error: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by deleting extracted directory: %s", destPath)
	if err := os.RemoveAll(destPath); err != nil {
		logger.Errorf("Failed to delete directory '%s' during rollback: %v", destPath, err)
	}

	return nil
}

var _ step.Step = (*ExtractContainerdStep)(nil)
