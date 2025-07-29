package registry

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

type DownloadRegistryStep struct {
	step.Base
}

type DownloadRegistryStepBuilder struct {
	step.Builder[DownloadRegistryStepBuilder, *DownloadRegistryStep]
}

func NewDownloadRegistryStepBuilder(ctx runtime.Context, instanceName string) *DownloadRegistryStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, representativeArch)
	if err != nil || binaryInfo == nil {
		return nil
	}
	s := &DownloadRegistryStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download registry binaries for all required architectures", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	b := new(DownloadRegistryStepBuilder).Init(s)
	return b
}

func (s *DownloadRegistryStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadRegistryStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	registryHosts := ctx.GetHostsByRole("")
	if len(registryHosts) == 0 {
		return nil, fmt.Errorf("no hosts with role 'registry' found")
	}
	provider := binary.NewBinaryProvider(ctx)
	for _, host := range registryHosts {
		binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, host.GetArch())
		if err != nil {
			return nil, err
		}
		if binaryInfo != nil {
			archs[host.GetArch()] = true
		}
	}
	return archs, nil
}

func (s *DownloadRegistryStep) verifyChecksum(filePath string, binaryInfo *binary.Binary) (bool, error) {
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

func (s *DownloadRegistryStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")
	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return false, err
	}
	if len(requiredArchs) == 0 {
		return true, nil
	}
	provider := binary.NewBinaryProvider(ctx)
	allDone := true
	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, arch)
		if err != nil || binaryInfo == nil {
			continue
		}
		destPath := binaryInfo.FilePath()
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			allDone = false
			continue
		}
		match, _ := s.verifyChecksum(destPath, binaryInfo)
		if !match {
			allDone = false
		}
	}
	if allDone {
		logger.Info("All required registry binaries already exist and are valid.")
	}
	return allDone, nil
}

func (s *DownloadRegistryStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}
	if len(requiredArchs) == 0 {
		return nil
	}
	provider := binary.NewBinaryProvider(ctx)
	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, arch)
		if err != nil || binaryInfo == nil {
			continue
		}
		destPath := binaryInfo.FilePath()
		if _, err := os.Stat(destPath); err == nil {
			if match, _ := s.verifyChecksum(destPath, binaryInfo); match {
				logger.Infof("Skipping download for arch %s, file already exists and is valid.", arch)
				continue
			}
		}
		if err := s.downloadFile(ctx, binaryInfo); err != nil {
			return fmt.Errorf("failed to download registry for arch %s: %w", arch, err)
		}
	}
	logger.Info("All required registry binaries have been downloaded successfully.")
	return nil
}

func (s *DownloadRegistryStep) downloadFile(ctx runtime.ExecutionContext, binaryInfo *binary.Binary) error {
	logger := ctx.GetLogger()
	destDir := filepath.Dir(binaryInfo.FilePath())
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx.GoContext(), "GET", binaryInfo.URL(), nil)
	if err != nil {
		return err
	}
	logger.Infof("Downloading registry (arch: %s, version: %s) from %s ...", binaryInfo.Arch, binaryInfo.Version, binaryInfo.URL())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}
	out, err := os.Create(binaryInfo.FilePath())
	if err != nil {
		return err
	}
	defer out.Close()
	bar := progressbar.NewOptions64(resp.ContentLength, progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", filepath.Base(binaryInfo.FilePath()))), progressbar.OptionSetWriter(os.Stderr), progressbar.OptionShowBytes(true), progressbar.OptionSetWidth(40), progressbar.OptionThrottle(65*time.Millisecond), progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }), progressbar.OptionSpinnerType(14), progressbar.OptionFullWidth())
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		_ = bar.Clear()
		_ = out.Close()
		_ = os.Remove(binaryInfo.FilePath())
		return err
	}
	_ = bar.Finish()
	logger.Infof("Successfully downloaded to %s", binaryInfo.FilePath())
	match, err := s.verifyChecksum(binaryInfo.FilePath(), binaryInfo)
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("checksum mismatch after download")
	}
	logger.Info("Checksum verification successful.")
	return nil
}

func (s *DownloadRegistryStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return nil
	}
	provider := binary.NewBinaryProvider(ctx)
	for arch := range requiredArchs {
		binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, arch)
		if err != nil || binaryInfo == nil {
			continue
		}
		logger.Warnf("Rolling back by deleting downloaded file: %s", binaryInfo.FilePath())
		_ = os.Remove(binaryInfo.FilePath())
	}
	return nil
}

var _ step.Step = (*DownloadRegistryStep)(nil)
