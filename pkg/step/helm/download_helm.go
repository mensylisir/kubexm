package helm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers" // 引入 helpers 包
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/schollz/progressbar/v3"
)

type DownloadHelmStep struct {
	step.Base
}

type DownloadHelmStepBuilder struct {
	step.Builder[DownloadHelmStepBuilder, *DownloadHelmStep]
}

func NewDownloadHelmStepBuilder(ctx runtime.Context, instanceName string) *DownloadHelmStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHelm, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &DownloadHelmStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download helm binaries for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadHelmStepBuilder).Init(s)
	return b
}

func (s *DownloadHelmStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadHelmStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	hosts := ctx.GetHostsByRole("control-plane")
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no control-plane hosts found to determine required helm architectures")
	}

	provider := binary.NewBinaryProvider(ctx)
	for _, host := range hosts {
		binaryInfo, err := provider.GetBinary(binary.ComponentHelm, host.GetArch())
		if err != nil {
			return nil, fmt.Errorf("error checking if helm is needed for host %s: %w", host.GetName(), err)
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

func (s *DownloadHelmStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		logger.Info("No hosts require helm in this cluster configuration. Step is complete.")
		return true, nil
	}

	provider := binary.NewBinaryProvider(ctx)
	allDone := true

	for arch := range requiredArchs {
		binaryInfo, _ := provider.GetBinary(binary.ComponentHelm, arch)
		if binaryInfo == nil {
			continue
		}

		destPath := binaryInfo.FilePath()
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			logger.Debugf("File for arch %s (%s) does not exist. Download is required.", arch, destPath)
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
		logger.Info("All required helm binaries for all architectures already exist and are valid.")
		return true, nil
	}
	return false, nil
}

func (s *DownloadHelmStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}

	provider := binary.NewBinaryProvider(ctx)

	for arch := range requiredArchs {
		binaryInfo, _ := provider.GetBinary(binary.ComponentHelm, arch)
		if binaryInfo == nil {
			continue
		}

		destPath := binaryInfo.FilePath()
		match, _ := helpers.VerifyLocalFileChecksum(destPath, binaryInfo.Checksum())
		if match {
			logger.Infof("Skipping download for arch %s, file already exists and is valid.", arch)
			continue
		}

		if err := s.downloadFile(ctx, binaryInfo); err != nil {
			return fmt.Errorf("failed to download helm for arch %s: %w", arch, err)
		}
	}

	logger.Info("All required helm binaries have been downloaded successfully.")
	return nil
}

func (s *DownloadHelmStep) downloadFile(ctx runtime.ExecutionContext, binaryInfo *binary.Binary) error {
	logger := ctx.GetLogger()
	destPath := binaryInfo.FilePath()
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	req, err := http.NewRequestWithContext(ctx.GoContext(), "GET", binaryInfo.URL(), nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	logger.Infof("Downloading helm (arch: %s, version: %s) from %s ...", binaryInfo.Arch, binaryInfo.Version, binaryInfo.URL())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", filepath.Base(destPath))),
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
		_ = os.Remove(destPath)
		return fmt.Errorf("failed to write to destination file: %w", err)
	}
	_ = bar.Finish()

	logger.Infof("Successfully downloaded to %s", destPath)

	match, err := helpers.VerifyLocalFileChecksum(destPath, binaryInfo.Checksum())
	if err != nil {
		return fmt.Errorf("failed to verify checksum after download: %w", err)
	}
	if !match {
		return fmt.Errorf("checksum mismatch after download for '%s'. Expected '%s'", destPath, binaryInfo.Checksum())
	}
	logger.Info("Checksum verification successful.")

	return nil
}

func (s *DownloadHelmStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Errorf("Failed to get required architectures during rollback: %v", err)
		return nil
	}

	provider := binary.NewBinaryProvider(ctx)
	for arch := range requiredArchs {
		binaryInfo, _ := provider.GetBinary(binary.ComponentHelm, arch)
		if binaryInfo == nil {
			continue
		}
		destFile := binaryInfo.FilePath()
		if _, statErr := os.Stat(destFile); statErr == nil {
			logger.Warnf("Rolling back by deleting downloaded file: %s", destFile)
			_ = os.Remove(destFile)
		}
	}
	return nil
}

var _ step.Step = (*DownloadHelmStep)(nil)
