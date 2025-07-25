package containerd

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
	SystemdCgroup   bool
	SandboxImage    string
	Cni             CniConfig
	RegistryMirrors map[v1alpha1.ServerAddress]v1alpha1.MirrorConfig
	RegistryConfigs map[v1alpha1.ServerAddress]v1alpha1.AuthConfig
}

type ConfigureContainerdStepBuilder struct {
	step.Builder[ConfigureContainerdStepBuilder, *ConfigureContainerdStep]
}

func NewConfigureContainerdStepBuilder(ctx runtime.Context, instanceName string) *ConfigureContainerdStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	containerdCfg := cfg.Kubernetes.ContainerRuntime.Containerd

	grpcAddress := strings.TrimPrefix(common.ContainerdDefaultEndpoint, "unix://")

	s := &ConfigureContainerdStep{
		TargetPath:      common.ContainerdDefaultConfigFile,
		Root:            common.ContainerdDefaultRoot,
		State:           common.ContainerdDefaultState,
		Grpc:            GrpcConfig{Address: grpcAddress},
		SystemdCgroup:   true,
		Cni:             CniConfig{BinDir: common.DefaultCNIBin, ConfDir: common.DefaultCNIConfDirTarget},
		RegistryMirrors: make(map[v1alpha1.ServerAddress]v1alpha1.MirrorConfig),
		RegistryConfigs: make(map[v1alpha1.ServerAddress]v1alpha1.AuthConfig),
	}

	if s.SandboxImage == "" {
		pauseImage := helpers.GetImage(ctx, "pause")
		s.SandboxImage = pauseImage.ImageName()
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
		if containerdCfg.UseSystemdCgroup != nil {
			s.SystemdCgroup = *containerdCfg.UseSystemdCgroup
		}
		if containerdCfg.Pause != "" {
			s.SandboxImage = containerdCfg.Pause
		}
	}

	if cfg.Registry != nil {
		if cfg.Registry.MirroringAndRewriting != nil && cfg.Registry.MirroringAndRewriting.PrivateRegistry != "" {
			s.RegistryMirrors["docker.io"] = v1alpha1.MirrorConfig{
				Endpoints: []string{cfg.Registry.MirroringAndRewriting.PrivateRegistry},
			}
		}
		if len(cfg.Registry.Auths) > 0 {
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
	s.Base.Sudo = true
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
			logger.Warnf("Config file '%s' exists but failed to read, will overwrite. Error: %v", s.TargetPath, err)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Infof("Containerd config file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("Containerd config file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("Containerd config file '%s' does not exist. Configuration is required.", s.TargetPath)
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

	logger.Infof("Writing containerd config file to %s", s.TargetPath)
	return runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.TargetPath, "0644", s.Sudo)
}

func (s *ConfigureContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s' during rollback: %v", s.TargetPath, err)
		}
	}

	return nil
}

var _ step.Step = (*ConfigureContainerdStep)(nil)
