package docker

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
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

type DownloadCriDockerdStep struct {
	step.Base
	URL         string
	Version     string
	Arch        string
	WorkDir     string
	ClusterName string
	Zone        string
}

type DownloadCriDockerdStepBuilder struct {
	step.Builder[DownloadCriDockerdStepBuilder, *DownloadCriDockerdStep]
}

func NewDownloadCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *DownloadCriDockerdStepBuilder {
	s := &DownloadCriDockerdStep{
		Version:     common.DefaultCriDockerdVersion,
		Arch:        "",
		WorkDir:     ctx.GetGlobalWorkDir(),
		ClusterName: ctx.GetClusterConfig().ObjectMeta.Name,
		Zone:        helpers.GetZone(),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download cri-dockerd version %s", s.Base.Meta.Name, s.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	info, err := s.getBinaryInfo()
	if err != nil {
		return nil
	}
	s.URL = info.URL

	b := new(DownloadCriDockerdStepBuilder).Init(s)
	return b
}

func (b *DownloadCriDockerdStepBuilder) WithURL(url string) *DownloadCriDockerdStepBuilder {
	b.Step.URL = url
	return b
}

func (s *DownloadCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadCriDockerdStep) getBinaryInfo() (*helpers.BinaryInfo, error) {
	provider := helpers.NewBinaryProvider()
	return provider.GetBinaryInfo(
		helpers.ComponentCriDockerd,
		s.Version,
		s.Arch,
		s.Zone,
		s.WorkDir,
		s.ClusterName,
	)
}

func (s *DownloadCriDockerdStep) verifyChecksum(filePath, expectedChecksum string) (bool, error) {
	if expectedChecksum == "" || strings.HasPrefix(expectedChecksum, "dummy-") {
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

func (s *DownloadCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")
	binaryInfo, err := s.getBinaryInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get cri-dockerd binary info: %w", err)
	}
	destPath := binaryInfo.FilePath
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
	match, err := s.verifyChecksum(destPath, binaryInfo.ExpectedChecksum)
	if err != nil {
		logger.Warnf("Failed to verify checksum for '%s', will re-download. Error: %v", destPath, err)
		return false, nil
	}
	if !match {
		logger.Warnf("Checksum mismatch for '%s'. Re-downloading.", destPath)
		return false, nil
	}
	logger.Info("File exists and checksum matches. Step is done.")
	return true, nil
}

func (s *DownloadCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	binaryInfo, err := s.getBinaryInfo()
	if err != nil {
		return fmt.Errorf("failed to get cri-dockerd binary info for run: %w", err)
	}

	destDir := filepath.Dir(binaryInfo.FilePath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	req, err := http.NewRequestWithContext(ctx.GoContext(), "GET", binaryInfo.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	logger.Infof("Downloading cri-dockerd from %s ...", binaryInfo.URL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code %d from url %s", resp.StatusCode, binaryInfo.URL)
	}

	out, err := os.Create(binaryInfo.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", binaryInfo.FilePath, err)
	}
	defer out.Close()

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", filepath.Base(binaryInfo.FilePath))),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)

	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		bar.Clear()
		os.Remove(binaryInfo.FilePath)
		return fmt.Errorf("failed to write to destination file '%s': %w", binaryInfo.FilePath, err)
	}
	bar.Finish()

	logger.Infof("Successfully downloaded to %s", binaryInfo.FilePath)

	match, err := s.verifyChecksum(binaryInfo.FilePath, binaryInfo.ExpectedChecksum)
	if err != nil {
		return fmt.Errorf("failed to verify checksum after download: %w", err)
	}
	if !match {
		return fmt.Errorf("checksum mismatch after download for '%s'", binaryInfo.FilePath)
	}
	logger.Info("Checksum verification successful after download.")

	return nil
}

func (s *DownloadCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	binaryInfo, err := s.getBinaryInfo()
	if err != nil {
		logger.Errorf("Failed to get binary info during rollback, cannot determine file to delete. Error: %v", err)
		return nil
	}
	logger.Warnf("Rolling back by deleting downloaded file: %s", binaryInfo.FilePath)
	if err := os.Remove(binaryInfo.FilePath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to delete file '%s' during rollback: %v", binaryInfo.FilePath, err)
	}
	return nil
}
