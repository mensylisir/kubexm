package containerd

import (
	"crypto/sha256"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/schollz/progressbar/v3"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DownloadRuncStep struct {
	step.Base
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
}

type DownloadRuncStepBuilder struct {
	step.Builder[DownloadRuncStepBuilder, *DownloadRuncStep]
}

func NewDownloadRuncStepBuilder(ctx runtime.Context, instanceName string) *DownloadRuncStepBuilder {
	s := &DownloadRuncStep{
		Version:     common.DefaultRuncVersion,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        util.GetZone(),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download runc version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(DownloadRuncStepBuilder).Init(s)
	return b
}

func (b *DownloadRuncStepBuilder) WithVersion(version string) *DownloadRuncStepBuilder {
	b.Step.Version = version
	b.Step.Base.Meta.Description = fmt.Sprintf("[%s]>>Download runc version %s", b.Step.Base.Meta.Name, b.Step.Version)
	return b
}

func (b *DownloadRuncStepBuilder) WithArch(arch string) *DownloadRuncStepBuilder {
	if arch != "" {
		b.Step.Arch = arch
	}
	return b
}
func (s *DownloadRuncStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadRuncStep) getBinaryInfo(ctx runtime.ExecutionContext) (*binary.Binary, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentRunc, arch)
	if err != nil {
		return nil, err
	}
	if binaryInfo == nil {
		return nil, fmt.Errorf("Runc are disabled or no compatible version found")
	}
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Runc version %s", s.Base.Meta.Name, binaryInfo.Version)

	return binaryInfo, nil
}

func (s *DownloadRuncStep) verifyChecksum(filePath, expectedChecksum string) (bool, error) {
	if expectedChecksum == "" || expectedChecksum == "dummy-etcd-checksum-val" { // 同样忽略占位符
		return true, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file '%s' for checksum: %w", filePath, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, fmt.Errorf("failed to calculate checksum for '%s': %w", filePath, err)
	}

	calculatedSum := fmt.Sprintf("%x", h.Sum(nil))
	return calculatedSum == expectedChecksum, nil
}

func (s *DownloadRuncStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	binaryInfo, err := s.getBinaryInfo(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get runc binary info: %w", err)
	}

	destPath := binaryInfo.FilePath()
	logger.Infof("Checking for existing file at: %s", destPath)

	info, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		logger.Info("File does not exist. Download is required.")
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to stat destination file '%s': %w", destPath, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("destination '%s' is a directory, not a file", destPath)
	}

	logger.Infof("File '%s' exists. Verifying checksum...", destPath)
	match, err := s.verifyChecksum(destPath, binaryInfo.Checksum())
	if err != nil {
		logger.Warnf("Failed to verify checksum for '%s', will re-download. Error: %v", destPath, err)
		return false, nil
	}
	if !match {
		logger.Warnf("Checksum mismatch for '%s'. Expected '%s'. Re-downloading.", destPath, binaryInfo.Checksum())
		return false, nil
	}

	logger.Info("File exists and checksum matches. Step is done.")
	return true, nil
}

func (s *DownloadRuncStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	binaryInfo, err := s.getBinaryInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get runc binary info for run: %w", err)
	}
	destDir := filepath.Dir(binaryInfo.FilePath())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	req, err := http.NewRequestWithContext(ctx.GoContext(), "GET", binaryInfo.URL(), nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	logger.Infof("Downloading runc from %s ...", binaryInfo.URL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code %d from url %s", resp.StatusCode, binaryInfo.URL)
	}

	out, err := os.Create(binaryInfo.FilePath())
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", binaryInfo.FilePath, err)
	}
	defer out.Close()

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", filepath.Base(binaryInfo.FilePath()))),
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

	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		bar.Clear()
		os.Remove(binaryInfo.FilePath())
		return fmt.Errorf("failed to write to destination file '%s': %w", binaryInfo.FilePath, err)
	}

	bar.Finish()
	logger.Infof("Successfully downloaded to %s", binaryInfo.FilePath)

	match, err := s.verifyChecksum(binaryInfo.FilePath(), binaryInfo.Checksum())
	if err != nil {
		return fmt.Errorf("failed to verify checksum after download: %w", err)
	}
	if !match {
		return fmt.Errorf("checksum mismatch after download for '%s'. Expected '%s'", binaryInfo.FilePath, binaryInfo.Checksum())
	}
	logger.Info("Checksum verification successful after download.")

	return nil
}

func (s *DownloadRuncStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	binaryInfo, err := s.getBinaryInfo(ctx)
	if err != nil {
		logger.Errorf("Failed to get binary info during rollback, cannot determine file to delete. Error: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by deleting downloaded file: %s", binaryInfo.FilePath)
	if err := os.Remove(binaryInfo.FilePath()); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to delete file '%s' during rollback: %v", binaryInfo.FilePath, err)
	}
	return nil
}

var _ step.Step = (*DownloadRuncStep)(nil)
