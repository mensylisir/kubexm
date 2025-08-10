package containerd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/schollz/progressbar/v3"
)

type DownloadContainerdStep struct {
	step.Base
}

type DownloadContainerdStepBuilder struct {
	step.Builder[DownloadContainerdStepBuilder, *DownloadContainerdStep]
}

func NewDownloadContainerdStepBuilder(ctx runtime.Context, instanceName string) *DownloadContainerdStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentContainerd, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &DownloadContainerdStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download containerd binaries for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadContainerdStepBuilder).Init(s)
	return b
}

func (s *DownloadContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadContainerdStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration")
	}

	provider := binary.NewBinaryProvider(ctx)

	for _, host := range allHosts {
		binaryInfo, err := provider.GetBinary(binary.ComponentContainerd, host.GetArch())
		if err != nil {
			return nil, fmt.Errorf("error checking if containerd is needed for host %s: %w", host.GetName(), err)
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

func (s *DownloadContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}

	if len(requiredArchs) == 0 {
		logger.Info("No hosts require containerd in this cluster configuration. Step is done.")
		return true, nil
	}

	provider := binary.NewBinaryProvider(ctx)
	allDone := true

	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentContainerd, arch)
		if err != nil {
			return false, fmt.Errorf("failed to get containerd info for arch %s: %w", arch, err)
		}
		if binaryInfo == nil {
			continue
		}

		destPath := binaryInfo.FilePath()
		logger.Infof("Checking for containerd (arch: %s) at: %s", arch, destPath)

		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			logger.Infof("File for arch %s does not exist. Download is required.", arch)
			allDone = false
			continue
		}

		match, err := helpers.VerifyLocalFileChecksum(destPath, binaryInfo.Checksum())
		if err != nil {
			logger.Warnf("Failed to verify checksum for arch %s file (%s): %v. Re-download is required.", arch, destPath, err)
			allDone = false
			continue
		}
		if !match {
			logger.Warnf("Checksum mismatch for arch %s file (%s). Re-download is required.", arch, destPath)
			allDone = false
		}
	}

	if allDone {
		logger.Info("All required containerd binaries for all architectures already exist and are valid.")
		return true, nil
	}
	return false, nil
}

func (s *DownloadContainerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}

	if len(requiredArchs) == 0 {
		logger.Info("No hosts require containerd in this cluster configuration. Skipping download.")
		return nil
	}

	provider := binary.NewBinaryProvider(ctx)

	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentContainerd, arch)
		if err != nil {
			return fmt.Errorf("failed to get containerd info for arch %s: %w", arch, err)
		}
		if binaryInfo == nil {
			continue
		}

		destPath := binaryInfo.FilePath()
		if _, err := os.Stat(destPath); err == nil {
			match, _ := helpers.VerifyLocalFileChecksum(destPath, binaryInfo.Checksum())
			if match {
				logger.Infof("Skipping download for arch %s, file already exists and is valid.", arch)
				continue
			}
		}

		if err := s.downloadFile(ctx, binaryInfo); err != nil {
			return fmt.Errorf("failed to download containerd for arch %s: %w", arch, err)
		}
	}

	logger.Info("All required containerd binaries have been downloaded successfully.")
	return nil
}

func (s *DownloadContainerdStep) downloadFile(ctx runtime.ExecutionContext, binaryInfo *binary.Binary) error {
	logger := ctx.GetLogger()

	destDir := filepath.Dir(binaryInfo.FilePath())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	req, err := http.NewRequestWithContext(ctx.GoContext(), "GET", binaryInfo.URL(), nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	logger.Infof("Downloading containerd (arch: %s, version: %s) from %s ...", binaryInfo.Arch, binaryInfo.Version, binaryInfo.URL())
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

	match, err := helpers.VerifyLocalFileChecksum(binaryInfo.FilePath(), binaryInfo.Checksum())
	if err != nil {
		return fmt.Errorf("failed to verify checksum after download: %w", err)
	}
	if !match {
		return fmt.Errorf("checksum mismatch after download for '%s'. Expected '%s'", binaryInfo.FilePath(), binaryInfo.Checksum())
	}
	logger.Info("Checksum verification successful.")

	return nil
}

func (s *DownloadContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	provider := binary.NewBinaryProvider(ctx)
	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentContainerd, arch)
		if err != nil || binaryInfo == nil {
			continue
		}
		logger.Warnf("Rolling back by deleting downloaded file: %s", binaryInfo.FilePath())
		_ = os.Remove(binaryInfo.FilePath())
	}
	return nil
}

var _ step.Step = (*DownloadContainerdStep)(nil)
