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

type DownloadCNIPluginsStep struct {
	step.Base
}

type DownloadCNIPluginsStepBuilder struct {
	step.Builder[DownloadCNIPluginsStepBuilder, *DownloadCNIPluginsStep]
}

func NewDownloadCNIPluginsStepBuilder(ctx runtime.Context, instanceName string) *DownloadCNIPluginsStepBuilder {
	s := &DownloadCNIPluginsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download CNI plugins for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadCNIPluginsStepBuilder).Init(s)
	return b
}

func (s *DownloadCNIPluginsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadCNIPluginsStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration")
	}

	for _, host := range allHosts {
		archs[host.GetArch()] = true
	}
	return archs, nil
}

func (s *DownloadCNIPluginsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}

	provider := binary.NewBinaryProvider(ctx)
	allDone := true

	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentKubeCNI, arch)
		if err != nil {
			return false, fmt.Errorf("failed to get CNI plugins info for arch %s: %w", arch, err)
		}
		if binaryInfo == nil {
			logger.Warn("Skipping check as no compatible CNI plugins version was found in BOM.", "arch", arch)
			continue
		}

		destPath := binaryInfo.FilePath()
		logger.Debug("Checking for CNI plugins.", "arch", arch, "path", destPath)

		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			logger.Info("File for arch does not exist. Download is required.", "arch", arch)
			allDone = false
			continue
		}

		match, err := helpers.VerifyLocalFileChecksum(destPath, binaryInfo.Checksum())
		if err != nil {
			logger.Warn(err, "Failed to verify checksum, re-download is required.", "arch", arch, "path", destPath)
			allDone = false
			continue
		}
		if !match {
			logger.Warn("Checksum mismatch, re-download is required.", "arch", arch, "path", destPath)
			allDone = false
		}
	}

	if allDone {
		logger.Info("All required CNI plugins for all architectures already exist and are valid.")
		return true, nil
	}
	return false, nil
}

func (s *DownloadCNIPluginsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}

	provider := binary.NewBinaryProvider(ctx)

	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentKubeCNI, arch)
		if err != nil {
			return fmt.Errorf("failed to get CNI plugins info for arch %s: %w", arch, err)
		}
		if binaryInfo == nil {
			logger.Warn("Skipping download as no compatible CNI plugins version was found in BOM.", "arch", arch)
			continue
		}

		destPath := binaryInfo.FilePath()
		if _, err := os.Stat(destPath); err == nil {
			match, _ := helpers.VerifyLocalFileChecksum(destPath, binaryInfo.Checksum())
			if match {
				logger.Info("Skipping download, file already exists and is valid.", "arch", arch)
				continue
			}
		}

		if err := s.downloadFile(ctx, binaryInfo); err != nil {
			return fmt.Errorf("failed to download CNI plugins for arch %s: %w", arch, err)
		}
	}

	logger.Info("All required CNI plugins have been downloaded successfully.")
	return nil
}

func (s *DownloadCNIPluginsStep) downloadFile(ctx runtime.ExecutionContext, binaryInfo *binary.Binary) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	destDir := filepath.Dir(binaryInfo.FilePath())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	req, err := http.NewRequestWithContext(ctx.GoContext(), "GET", binaryInfo.URL(), nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	logger.Info("Downloading CNI plugins.", "arch", binaryInfo.Arch, "version", binaryInfo.Version, "url", binaryInfo.URL())
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

	logger.Info("Successfully downloaded.", "path", binaryInfo.FilePath())

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

func (s *DownloadCNIPluginsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		logger.Error(err, "Failed to get required architectures during rollback.")
		return nil
	}

	provider := binary.NewBinaryProvider(ctx)
	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentKubeCNI, arch)
		if err != nil || binaryInfo == nil {
			continue
		}
		logger.Warn("Rolling back by deleting downloaded file.", "path", binaryInfo.FilePath())
		_ = os.Remove(binaryInfo.FilePath())
	}
	return nil
}

var _ step.Step = (*DownloadCNIPluginsStep)(nil)
