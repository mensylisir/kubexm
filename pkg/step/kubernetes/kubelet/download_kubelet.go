package kubelet

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/schollz/progressbar/v3"
)

type DownloadKubeletStep struct {
	step.Base
}

type DownloadKubeletStepBuilder struct {
	step.Builder[DownloadKubeletStepBuilder, *DownloadKubeletStep]
}

func NewDownloadKubeletStepBuilder(ctx runtime.Context, instanceName string) *DownloadKubeletStepBuilder {
	s := &DownloadKubeletStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download kubelet binaries for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadKubeletStepBuilder).Init(s)
	return b
}

func (s *DownloadKubeletStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadKubeletStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration to determine required kubelet architectures")
	}

	for _, host := range allHosts {
		archs[host.GetArch()] = true
	}
	return archs, nil
}

func (s *DownloadKubeletStep) verifyChecksum(filePath string, binaryInfo *binary.Binary) (bool, error) {
	expectedChecksum := binaryInfo.Checksum()
	if expectedChecksum == "" || strings.HasPrefix(expectedChecksum, "dummy-") {
		return true, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return fmt.Sprintf("%x", h.Sum(nil)) == expectedChecksum, nil
}

func (s *DownloadKubeletStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}

	if len(requiredArchs) == 0 {
		logger.Info("No hosts require kubelet binaries in this cluster configuration. Step is done.")
		return true, nil
	}

	provider := binary.NewBinaryProvider(ctx)
	allDone := true

	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentKubelet, arch)
		if err != nil {
			return false, fmt.Errorf("failed to get kubelet info for arch %s: %w", arch, err)
		}
		if binaryInfo == nil {
			return false, fmt.Errorf("kubelet binary info not found for arch %s in BOM", arch)
		}

		destPath := binaryInfo.FilePath()
		logger.Infof("Checking for kubelet (arch: %s) at: %s", arch, destPath)

		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			logger.Infof("File for arch %s does not exist. Download is required.", arch)
			allDone = false
			continue
		}

		match, _ := s.verifyChecksum(destPath, binaryInfo)
		if !match {
			logger.Warnf("Checksum mismatch for arch %s file. Re-download is required.", arch)
			allDone = false
		}
	}

	if allDone {
		logger.Info("All required kubelet binaries for all architectures already exist and are valid.")
		return true, nil
	}
	return false, nil
}

func (s *DownloadKubeletStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}

	if len(requiredArchs) == 0 {
		logger.Info("No hosts require kubelet binaries in this cluster configuration. Skipping download.")
		return nil
	}

	provider := binary.NewBinaryProvider(ctx)

	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentKubelet, arch)
		if err != nil {
			return fmt.Errorf("failed to get kubelet info for arch %s: %w", arch, err)
		}
		if binaryInfo == nil {
			return fmt.Errorf("kubelet binary info not found for arch %s in BOM", arch)
		}

		destPath := binaryInfo.FilePath()
		if _, err := os.Stat(destPath); err == nil {
			match, _ := s.verifyChecksum(destPath, binaryInfo)
			if match {
				logger.Infof("Skipping download for arch %s, file already exists and is valid.", arch)
				continue
			}
		}

		if err := s.downloadFile(ctx, binaryInfo); err != nil {
			return fmt.Errorf("failed to download kubelet for arch %s: %w", arch, err)
		}
	}

	logger.Info("All required kubelet binaries have been downloaded successfully.")
	return nil
}

func (s *DownloadKubeletStep) downloadFile(ctx runtime.ExecutionContext, binaryInfo *binary.Binary) error {
	logger := ctx.GetLogger()

	destDir := filepath.Dir(binaryInfo.FilePath())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	req, err := http.NewRequestWithContext(ctx.GoContext(), "GET", binaryInfo.URL(), nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	logger.Infof("Downloading kubelet (arch: %s, version: %s) from %s ...", binaryInfo.Arch, binaryInfo.Version, binaryInfo.URL())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}

	out, err := os.Create(binaryInfo.FilePath())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", filepath.Base(binaryInfo.FilePath()))),
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
		_ = bar.Clear()
		_ = out.Close()
		_ = os.Remove(binaryInfo.FilePath())
		return fmt.Errorf("failed to write to destination file: %w", err)
	}
	_ = bar.Finish()

	logger.Infof("Successfully downloaded to %s", binaryInfo.FilePath())

	match, err := s.verifyChecksum(binaryInfo.FilePath(), binaryInfo)
	if err != nil {
		return fmt.Errorf("failed to verify checksum after download: %w", err)
	}
	if !match {
		return fmt.Errorf("checksum mismatch after download for '%s'. Expected '%s'", binaryInfo.FilePath(), binaryInfo.Checksum())
	}
	logger.Info("Checksum verification successful.")

	return nil
}

func (s *DownloadKubeletStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	provider := binary.NewBinaryProvider(ctx)
	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentKubelet, arch)
		if err != nil || binaryInfo == nil {
			continue
		}
		logger.Warnf("Rolling back by deleting downloaded file: %s", binaryInfo.FilePath())
		_ = os.Remove(binaryInfo.FilePath())
	}
	return nil
}

var _ step.Step = (*DownloadKubeletStep)(nil)
