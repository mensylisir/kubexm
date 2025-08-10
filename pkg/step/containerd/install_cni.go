package containerd

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
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type InstallCNIPluginsStep struct {
	step.Base
	RemoteCNIBinDir string
	Permission      string
	knownCniPlugins []string
}

type InstallCNIPluginsStepBuilder struct {
	step.Builder[InstallCNIPluginsStepBuilder, *InstallCNIPluginsStep]
}

func NewInstallCNIPluginsStepBuilder(ctx runtime.Context, instanceName string) *InstallCNIPluginsStepBuilder {
	s := &InstallCNIPluginsStep{
		RemoteCNIBinDir: common.DefaultCNIBinDirTarget,
		Permission:      "0755",
		knownCniPlugins: []string{
			"bridge", "dhcp", "dummy", "firewall", "host-device", "host-local",
			"ipvlan", "loopback", "macvlan", "portmap", "ptp", "sbr",
			"static", "tap", "tuning", "vlan", "vrf",
		},
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI plugins", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCNIPluginsStepBuilder).Init(s)
	return b
}

func (b *InstallCNIPluginsStepBuilder) WithRemoteCNIBinDir(remoteCNIBinDir string) *InstallCNIPluginsStepBuilder {
	if remoteCNIBinDir != "" {
		b.Step.RemoteCNIBinDir = remoteCNIBinDir
	}
	return b
}

func (b *InstallCNIPluginsStepBuilder) WithPermission(permission string) *InstallCNIPluginsStepBuilder {
	if permission != "" {
		b.Step.Permission = permission
	}
	return b
}

func (s *InstallCNIPluginsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCNIPluginsStep) getLocalExtractedPath(ctx runtime.ExecutionContext) (string, error) {
	provider := binary.NewBinaryProvider(ctx)
	arch := ctx.GetHost().GetArch()
	binaryInfo, err := provider.GetBinary(binary.ComponentKubeCNI, arch)
	if err != nil {
		return "", fmt.Errorf("failed to get CNI plugins binary info: %w", err)
	}
	if binaryInfo == nil {
		return "", fmt.Errorf("CNI plugins are disabled for arch %s", arch)
	}

	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI plugins (version %s)", s.Base.Meta.Name, binaryInfo.Version)

	sourceDir := filepath.Dir(binaryInfo.FilePath())
	innerDir := "cni-plugins"
	return filepath.Join(sourceDir, innerDir), nil
}

func (s *InstallCNIPluginsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	localExtractedPath, err := s.getLocalExtractedPath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("CNI plugins not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return true, nil
		}
		return false, err
	}
	if _, err := os.Stat(filepath.Join(localExtractedPath, "bridge")); os.IsNotExist(err) {
		return false, fmt.Errorf("local CNI extracted path '%s' or key file 'bridge' not found, ensure extract step ran successfully", localExtractedPath)
	}

	pluginsToCheck := []string{"bridge", "host-local", "loopback"}
	allExist := true
	for _, pluginName := range pluginsToCheck {
		remotePath := filepath.Join(s.RemoteCNIBinDir, pluginName)
		exists, err := runner.Exists(ctx.GoContext(), conn, remotePath)
		if err != nil {
			return false, fmt.Errorf("failed to check existence of remote plugin '%s': %w", remotePath, err)
		}
		if !exists {
			logger.Infof("Key CNI plugin '%s' does not exist. Installation is required.", remotePath)
			allExist = false
			break
		}
	}

	if allExist {
		logger.Info("Key CNI plugins already exist. Step is done.")
	}

	return allExist, nil
}

func (s *InstallCNIPluginsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localExtractedPath, err := s.getLocalExtractedPath(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "disabled for arch") {
			logger.Infof("CNI plugins not required for this host (arch: %s), skipping.", ctx.GetHost().GetArch())
			return nil
		}
		return err
	}
	if _, err := os.Stat(localExtractedPath); os.IsNotExist(err) {
		return fmt.Errorf("local CNI extracted path '%s' not found, ensure extract step ran successfully", localExtractedPath)
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteCNIBinDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote CNI bin directory '%s': %w", s.RemoteCNIBinDir, err)
	}

	remoteUploadTmpDir := filepath.Join(ctx.GetUploadDir(), fmt.Sprintf("cni-plugins-%d", time.Now().UnixNano()))
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteUploadTmpDir, "0755", false); err != nil {
		return fmt.Errorf("failed to create remote upload directory '%s': %w", remoteUploadTmpDir, err)
	}
	defer func() {
		_ = runner.Remove(ctx.GoContext(), conn, remoteUploadTmpDir, false, true)
	}()

	for _, pluginName := range s.knownCniPlugins {
		localPath := filepath.Join(localExtractedPath, pluginName)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			continue
		}

		remoteTempPath := filepath.Join(remoteUploadTmpDir, pluginName)
		remoteTargetPath := filepath.Join(s.RemoteCNIBinDir, pluginName)

		logger.Debugf("Uploading CNI plugin %s to %s:%s", pluginName, ctx.GetHost().GetName(), remoteTempPath)
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remoteTempPath, false); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", localPath, err)
		}

		moveCmd := fmt.Sprintf("mv %s %s", remoteTempPath, remoteTargetPath)
		logger.Debugf("Moving CNI plugin to %s on remote host", remoteTargetPath)
		if _, err := runner.Run(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to move CNI plugin '%s': %w", pluginName, err)
		}

		logger.Debugf("Setting permissions for %s to %s", remoteTargetPath, s.Permission)
		if err := runner.Chmod(ctx.GoContext(), conn, remoteTargetPath, s.Permission, s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on CNI plugin '%s': %w", pluginName, err)
		}
	}

	logger.Info("All required CNI plugins have been installed successfully.")
	return nil
}

func (s *InstallCNIPluginsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing installed CNI plugins from %s", s.RemoteCNIBinDir)
	for _, pluginName := range s.knownCniPlugins {
		remotePath := filepath.Join(s.RemoteCNIBinDir, pluginName)
		if err := runner.Remove(ctx.GoContext(), conn, remotePath, s.Sudo, false); err != nil {
			logger.Errorf("Failed to remove CNI plugin '%s' during rollback: %v", remotePath, err)
		}
	}
	return nil
}

var _ step.Step = (*InstallCNIPluginsStep)(nil)
