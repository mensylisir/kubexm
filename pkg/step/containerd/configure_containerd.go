package containerd

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"net/url"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const (
	containerdConfigTemplatePath = "containerd/config.toml.tmpl"
)

type GrpcConfig struct {
	Address string
}
type CniConfig struct {
	BinDir  string
	ConfDir string
}

type ConfigureContainerdStep struct {
	step.Base
	TargetPath      string
	Root            string
	State           string
	Grpc            GrpcConfig
	SystemdCgroup   string
	SandboxImage    string
	Cni             CniConfig
	RegistryMirrors map[v1alpha1.ServerAddress]v1alpha1.MirrorConfig
	RegistryConfigs map[v1alpha1.ServerAddress]v1alpha1.AuthConfig
}

type ConfigureContainerdStepBuilder struct {
	step.Builder[ConfigureContainerdStepBuilder, *ConfigureContainerdStep]
}

func NewConfigureContainerdStepBuilder(ctx runtime.Context, instanceName string) *ConfigureContainerdStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentContainerd, representativeArch)
	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	containerdCfg := cfg.Kubernetes.ContainerRuntime.Containerd

	var sandboxImage string
	if containerdCfg != nil && containerdCfg.Pause != "" {
		sandboxImage = containerdCfg.Pause
	} else {
		imageProvider := images.NewImageProvider(&ctx)
		pauseImage := imageProvider.GetImage("pause")
		if pauseImage == nil {
			return nil
		}
		sandboxImage = pauseImage.FullName()
	}
	if sandboxImage == "" {
		return nil
	}

	s := &ConfigureContainerdStep{
		TargetPath:      common.ContainerdDefaultConfigFile,
		Root:            common.ContainerdDefaultRoot,
		State:           common.ContainerdDefaultState,
		Grpc:            GrpcConfig{Address: strings.TrimPrefix(common.ContainerdDefaultEndpoint, "unix://")},
		SystemdCgroup:   common.CgroupDriverSystemd,
		SandboxImage:    sandboxImage,
		Cni:             CniConfig{BinDir: common.DefaultCNIBin, ConfDir: common.DefaultCNIConfDirTarget},
		RegistryMirrors: make(map[v1alpha1.ServerAddress]v1alpha1.MirrorConfig),
		RegistryConfigs: make(map[v1alpha1.ServerAddress]v1alpha1.AuthConfig),
	}

	if containerdCfg != nil {
		if containerdCfg.ConfigPath != nil && *containerdCfg.ConfigPath != "" {
			s.TargetPath = *containerdCfg.ConfigPath
		}
		if containerdCfg.Root != nil && *containerdCfg.Root != "" {
			s.Root = *containerdCfg.Root
		}
		if containerdCfg.State != nil && *containerdCfg.State != "" {
			s.State = *containerdCfg.State
		}
		if containerdCfg.Endpoint != "" {
			s.Grpc.Address = strings.TrimPrefix(containerdCfg.Endpoint, "unix://")
		}
		if containerdCfg.CgroupDriver != nil {
			s.SystemdCgroup = *containerdCfg.CgroupDriver
		}
	}

	if cfg.Registry != nil && cfg.Registry.MirroringAndRewriting != nil && cfg.Registry.MirroringAndRewriting.PrivateRegistry != "" {
		privateRegistryHost := cfg.Registry.MirroringAndRewriting.PrivateRegistry
		if u, err := url.Parse("scheme://" + privateRegistryHost); err == nil {
			privateRegistryHost = u.Host
		}

		serverAddr := v1alpha1.ServerAddress(privateRegistryHost)
		_, hasExplicitAuth := cfg.Registry.Auths[string(serverAddr)]

		if !hasExplicitAuth {
			s.RegistryConfigs[serverAddr] = v1alpha1.AuthConfig{
				TLS: nil,
			}
		}
	}

	if cfg.Registry != nil && len(cfg.Registry.Auths) > 0 {
		for server, auth := range cfg.Registry.Auths {
			serverAddr := v1alpha1.ServerAddress(server)
			authConfig := v1alpha1.AuthConfig{
				Auth: &v1alpha1.ContainerdRegistryAuth{Username: auth.Username, Password: auth.Password, Auth: auth.Auth},
				TLS:  &v1alpha1.TLSConfig{InsecureSkipVerify: true},
			}
			if auth.SkipTLSVerify != nil {
				authConfig.TLS.InsecureSkipVerify = *auth.SkipTLSVerify
			}
			if auth.CertsPath != "" {
				authConfig.TLS.CAFile = filepath.Join(auth.CertsPath, "ca.crt")
				authConfig.TLS.CertFile = filepath.Join(auth.CertsPath, "tls.crt")
				authConfig.TLS.KeyFile = filepath.Join(auth.CertsPath, "tls.key")
			}
			if auth.PlainHTTP != nil && *auth.PlainHTTP {
				authConfig.TLS = nil
			}
			s.RegistryConfigs[serverAddr] = authConfig
		}
	}
	if containerdCfg != nil && containerdCfg.Registry != nil {
		for server, mirror := range containerdCfg.Registry.Mirrors {
			s.RegistryMirrors[server] = mirror
		}
		for server, config := range containerdCfg.Registry.Configs {
			s.RegistryConfigs[server] = config
		}
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure containerd", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureContainerdStepBuilder).Init(s)
	return b
}

func (s *ConfigureContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureContainerdStep) renderContent() (string, error) {
	tmplStr, err := templates.Get(containerdConfigTemplatePath)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("config.toml").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse containerd config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render containerd config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureContainerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderContent()
	if err != nil {
		return false, fmt.Errorf("failed to render expected content for precheck: %w", err)
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for config file '%s': %w", s.TargetPath, err)
	}

	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			logger.Warn(err, "Config file exists but failed to read, will overwrite.", "path", s.TargetPath)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Info("Containerd config file already exists and content matches. Step is done.", "path", s.TargetPath)
			return true, nil
		}
		logger.Info("Containerd config file exists but content differs. Step needs to run.", "path", s.TargetPath)
		return false, nil
	}

	logger.Info("Containerd config file does not exist. Configuration is required.", "path", s.TargetPath)
	return false, nil
}

func (s *ConfigureContainerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(s.TargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create containerd config directory '%s': %w", targetDir, err)
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	logger.Info("Writing containerd config file.", "path", s.TargetPath)
	return helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
}

func (s *ConfigureContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}

	logger.Warn("Rolling back by removing.", "path", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Error(err, "Failed to remove path during rollback.", "path", s.TargetPath)
		}
	}

	return nil
}

var _ step.Step = (*ConfigureContainerdStep)(nil)
