package docker

import (
	"encoding/json"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/tidwall/gjson"
)

const DefaultDockerDaemonJSONPath = common.DockerDefaultConfigFileTarget

type daemonConfigForRender struct {
	ExecOpts               []string                     `json:"exec-opts,omitempty"`
	LogDriver              string                       `json:"log-driver,omitempty"`
	LogOpts                map[string]string            `json:"log-opts,omitempty"`
	StorageDriver          string                       `json:"storage-driver,omitempty"`
	StorageOpts            []string                     `json:"storage-opts,omitempty"`
	RegistryMirrors        []string                     `json:"registry-mirrors,omitempty"`
	InsecureRegistries     []string                     `json:"insecure-registries,omitempty"`
	DataRoot               string                       `json:"data-root,omitempty"`
	Bridge                 string                       `json:"bridge,omitempty"`
	Bip                    string                       `json:"bip,omitempty"`
	LiveRestore            bool                         `json:"live-restore"`
	IPTables               bool                         `json:"iptables"`
	IPMasq                 bool                         `json:"ip-masq"`
	DefaultAddressPools    []v1alpha1.DockerAddressPool `json:"default-address-pools,omitempty"`
	MaxConcurrentDownloads int                          `json:"max-concurrent-downloads,omitempty"`
	MaxConcurrentUploads   int                          `json:"max-concurrent-uploads,omitempty"`
}

type ConfigureDockerStep struct {
	step.Base
	FinalConfig    daemonConfigForRender
	ConfigFilePath string
}

type ConfigureDockerStepBuilder struct {
	step.Builder[ConfigureDockerStepBuilder, *ConfigureDockerStep]
}

func NewConfigureDockerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureDockerStepBuilder {
	clusterCfgSpec := ctx.GetClusterConfig().Spec
	if clusterCfgSpec.Kubernetes.ContainerRuntime.Type != common.RuntimeTypeDocker {
		return nil
	}
	finalConfig := daemonConfigForRender{
		LogDriver:              common.DockerLogDriverJSONFile,
		LogOpts:                map[string]string{"max-size": common.DockerLogOptMaxSizeDefault},
		StorageDriver:          common.StorageDriverOverlay2,
		DataRoot:               common.DockerDefaultDataRoot,
		Bridge:                 common.DefaultDockerBridgeName,
		LiveRestore:            true,
		IPTables:               true,
		IPMasq:                 true,
		ExecOpts:               []string{fmt.Sprintf("native.cgroupdriver=%s", common.CgroupDriverSystemd)},
		MaxConcurrentDownloads: common.DockerMaxConcurrentDownloadsDefault,
		MaxConcurrentUploads:   common.DockerMaxConcurrentUploadsDefault,
	}

	mirrorSet := make(map[string]struct{})
	insecureSet := make(map[string]struct{})

	if clusterCfgSpec.Registry != nil {
		if clusterCfgSpec.Registry.MirroringAndRewriting != nil && clusterCfgSpec.Registry.MirroringAndRewriting.PrivateRegistry != "" {
			insecureSet[clusterCfgSpec.Registry.MirroringAndRewriting.PrivateRegistry] = struct{}{}
		}

		for server, auth := range clusterCfgSpec.Registry.Auths {
			if auth.PlainHTTP != nil && *auth.PlainHTTP {
				insecureSet[server] = struct{}{}
			}
		}
	}

	userDockerCfg := clusterCfgSpec.Kubernetes.ContainerRuntime.Docker
	if userDockerCfg != nil {
		for _, mirror := range userDockerCfg.RegistryMirrors {
			mirrorSet[mirror] = struct{}{}
		}
		for _, insecure := range userDockerCfg.InsecureRegistries {
			insecureSet[insecure] = struct{}{}
		}

		if userDockerCfg.CgroupDriver != nil && *userDockerCfg.CgroupDriver != "" {
			finalConfig.ExecOpts = []string{fmt.Sprintf("native.cgroupdriver=%s", *userDockerCfg.CgroupDriver)}
		}
		if userDockerCfg.LogDriver != nil && *userDockerCfg.LogDriver != "" {
			finalConfig.LogDriver = *userDockerCfg.LogDriver
		}
		if len(userDockerCfg.LogOpts) > 0 {
			finalConfig.LogOpts = userDockerCfg.LogOpts
		}
		if userDockerCfg.StorageDriver != nil && *userDockerCfg.StorageDriver != "" {
			finalConfig.StorageDriver = *userDockerCfg.StorageDriver
		}
		if len(userDockerCfg.StorageOpts) > 0 {
			finalConfig.StorageOpts = userDockerCfg.StorageOpts
		}
		if userDockerCfg.DataRoot != nil && *userDockerCfg.DataRoot != "" {
			finalConfig.DataRoot = *userDockerCfg.DataRoot
		}
		if userDockerCfg.Bridge != nil && *userDockerCfg.Bridge != "" {
			finalConfig.Bridge = *userDockerCfg.Bridge
		}
		if userDockerCfg.BIP != nil && *userDockerCfg.BIP != "" {
			finalConfig.Bip = *userDockerCfg.BIP
		}
		if userDockerCfg.LiveRestore != nil {
			finalConfig.LiveRestore = *userDockerCfg.LiveRestore
		}
		if userDockerCfg.IPTables != nil {
			finalConfig.IPTables = *userDockerCfg.IPTables
		}
		if userDockerCfg.IPMasq != nil {
			finalConfig.IPMasq = *userDockerCfg.IPMasq
		}
		if len(userDockerCfg.DefaultAddressPools) > 0 {
			finalConfig.DefaultAddressPools = userDockerCfg.DefaultAddressPools
		}
		if userDockerCfg.MaxConcurrentDownloads != nil {
			finalConfig.MaxConcurrentDownloads = *userDockerCfg.MaxConcurrentDownloads
		}
		if userDockerCfg.MaxConcurrentUploads != nil {
			finalConfig.MaxConcurrentUploads = *userDockerCfg.MaxConcurrentUploads
		}
	}

	if len(mirrorSet) > 0 {
		finalConfig.RegistryMirrors = make([]string, 0, len(mirrorSet))
		for mirror := range mirrorSet {
			finalConfig.RegistryMirrors = append(finalConfig.RegistryMirrors, mirror)
		}
	}
	if len(insecureSet) > 0 {
		finalConfig.InsecureRegistries = make([]string, 0, len(insecureSet))
		for insecure := range insecureSet {
			finalConfig.InsecureRegistries = append(finalConfig.InsecureRegistries, insecure)
		}
	}

	s := &ConfigureDockerStep{
		ConfigFilePath: DefaultDockerDaemonJSONPath,
		FinalConfig:    finalConfig,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure Docker's daemon.json", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureDockerStepBuilder).Init(s)
	return b
}

func (b *ConfigureDockerStepBuilder) WithCgroupDriver(driver string) *ConfigureDockerStepBuilder {
	if driver != "" {
		b.Step.FinalConfig.ExecOpts = []string{fmt.Sprintf("native.cgroupdriver=%s", driver)}
	}
	return b
}

func (s *ConfigureDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureDockerStep) renderExpectedConfig() ([]byte, error) {
	return json.MarshalIndent(s.FinalConfig, "", "  ")
}

func (s *ConfigureDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		return false, nil
	}

	expectedContentBytes, err := s.renderExpectedConfig()
	if err != nil {
		return false, fmt.Errorf("failed to render expected config for precheck: %w", err)
	}

	if gjson.ParseBytes(currentContentBytes).String() == gjson.ParseBytes(expectedContentBytes).String() {
		ctx.GetLogger().Info("Existing daemon.json content matches expected content. Step is done.")
		return true, nil
	}

	ctx.GetLogger().Info("Existing daemon.json content differs. Regeneration is required.")
	return false, nil
}

func (s *ConfigureDockerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configContent, err := s.renderExpectedConfig()
	if err != nil {
		return fmt.Errorf("failed to render final docker config: %w", err)
	}

	remoteDir := filepath.Dir(s.ConfigFilePath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
	}

	logger.Info("Writing Docker daemon.json file.", "path", s.ConfigFilePath)
	err = helpers.WriteContentToRemote(ctx, conn, string(configContent), s.ConfigFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write Docker daemon.json to %s: %w", s.ConfigFilePath, err)
	}

	logger.Info("Docker daemon.json file configured successfully. Restart Docker for changes to take effect.")
	return nil
}

func (s *ConfigureDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.ConfigFilePath)
	if err := runner.Remove(ctx.GoContext(), conn, s.ConfigFilePath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove Docker daemon.json during rollback.")
	}
	return nil
}

var _ step.Step = (*ConfigureDockerStep)(nil)
