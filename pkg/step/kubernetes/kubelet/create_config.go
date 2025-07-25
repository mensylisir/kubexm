package kubelet

import (
	"bytes"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type CreateKubeletConfigYAMLStep struct {
	step.Base
	ClientCAFile                     string
	ClusterDNSIP                     string
	ClusterDomain                    string
	ResolvConf                       string
	CgroupDriver                     string
	CpuManagerPolicy                 string
	KubeReserved                     map[string]string
	SystemReserved                   map[string]string
	EvictionHard                     map[string]string
	EvictionSoft                     map[string]string
	EvictionSoftGracePeriod          map[string]string
	EvictionMaxPodGracePeriod        int
	EvictionPressureTransitionPeriod string
	FeatureGates                     map[string]bool
	MaxPods                          int
	PodPidsLimit                     int64
	HairpinMode                      string
	ContainerLogMaxSize              string
	ContainerLogMaxFiles             int
	StaticPodPath                    string
	RemoteConfigYAMLFile             string
}

type CreateKubeletConfigYAMLStepBuilder struct {
	step.Builder[CreateKubeletConfigYAMLStepBuilder, *CreateKubeletConfigYAMLStep]
}

func NewCreateKubeletConfigYAMLStepBuilder(ctx runtime.Context, instanceName string) *CreateKubeletConfigYAMLStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes

	_, serviceNet, _ := net.ParseCIDR(clusterCfg.Spec.Network.KubeServiceCIDR)
	dnsIP, _ := helpers.GetIPAtIndex(serviceNet, 10)

	s := &CreateKubeletConfigYAMLStep{
		ClientCAFile:                     filepath.Join(common.KubernetesPKIDir, common.CACertFileName),
		ClusterDNSIP:                     dnsIP.String(),
		ClusterDomain:                    common.DefaultClusterLocal,
		CgroupDriver:                     common.CgroupDriverSystemd,
		CpuManagerPolicy:                 "none",
		KubeReserved:                     map[string]string{"cpu": "200m", "memory": "250Mi"},
		SystemReserved:                   map[string]string{"cpu": "200m", "memory": "250Mi"},
		EvictionHard:                     map[string]string{"memory.available": "5%", "pid.available": "10%"},
		EvictionMaxPodGracePeriod:        120,
		EvictionPressureTransitionPeriod: "30s",
		EvictionSoft:                     make(map[string]string),
		EvictionSoftGracePeriod:          make(map[string]string),
		FeatureGates:                     map[string]bool{"RotateKubeletServerCertificate": true},
		MaxPods:                          110,
		PodPidsLimit:                     10000,
		HairpinMode:                      common.DefaultKubeletHairpinMode,
		ContainerLogMaxSize:              "5Mi",
		ContainerLogMaxFiles:             3,
		StaticPodPath:                    "",
		RemoteConfigYAMLFile:             common.KubeletConfigYAMLPathTarget,
	}

	if k8sSpec.DNSDomain != "" {
		s.ClusterDomain = k8sSpec.DNSDomain
	}
	if k8sSpec.Kubelet != nil {
		kubeletCfg := k8sSpec.Kubelet
		if kubeletCfg.MaxPods != nil {
			s.MaxPods = *kubeletCfg.MaxPods
		}
		if kubeletCfg.CgroupDriver != "" {
			s.CgroupDriver = kubeletCfg.CgroupDriver
		}
		if kubeletCfg.CpuManagerPolicy != "" {
			s.CpuManagerPolicy = kubeletCfg.CpuManagerPolicy
		}
		if len(kubeletCfg.KubeReserved) > 0 {
			s.KubeReserved = kubeletCfg.KubeReserved
		}
		if len(kubeletCfg.SystemReserved) > 0 {
			s.SystemReserved = kubeletCfg.SystemReserved
		}
		if len(kubeletCfg.EvictionHard) > 0 {
			s.EvictionHard = kubeletCfg.EvictionHard
		}
		if len(kubeletCfg.EvictionSoft) > 0 {
			s.EvictionSoft = kubeletCfg.EvictionSoft
		}
		if len(kubeletCfg.EvictionSoftGracePeriod) > 0 {
			s.EvictionSoftGracePeriod = kubeletCfg.EvictionSoftGracePeriod
		}
		if kubeletCfg.EvictionMaxPodGracePeriod != nil {
			s.EvictionMaxPodGracePeriod = *kubeletCfg.EvictionMaxPodGracePeriod
		}
		if kubeletCfg.EvictionPressureTransitionPeriod != "" {
			s.EvictionPressureTransitionPeriod = kubeletCfg.EvictionPressureTransitionPeriod
		}
		if kubeletCfg.PodPidsLimit != nil {
			s.PodPidsLimit = *kubeletCfg.PodPidsLimit
		}
		if kubeletCfg.HairpinMode != "" {
			s.HairpinMode = kubeletCfg.HairpinMode
		}
		if kubeletCfg.ContainerLogMaxSize != "" {
			s.ContainerLogMaxSize = kubeletCfg.ContainerLogMaxSize
		}
		if kubeletCfg.ContainerLogMaxFiles != nil {
			s.ContainerLogMaxFiles = *kubeletCfg.ContainerLogMaxFiles
		}
		if len(kubeletCfg.FeatureGates) > 0 {
			for k, v := range kubeletCfg.FeatureGates {
				s.FeatureGates[k] = v
			}
		}
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create comprehensive kubelet configuration file (config.yaml)", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(CreateKubeletConfigYAMLStepBuilder).Init(s)
	return b
}

func (s *CreateKubeletConfigYAMLStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CreateKubeletConfigYAMLStep) detectResolvConf(ctx runtime.ExecutionContext) (string, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", err
	}
	const script = `
SYSTEMD_RESOLV_CONF="/run/systemd/resolve/resolv.conf"
STANDARD_RESOLV_CONF="/etc/resolv.conf"
RESOLV_CONF="$STANDARD_RESOLV_CONF"
if [ -r "$SYSTEMD_RESOLV_CONF" ]; then
    if ! grep -q "nameserver 127.0.0.53" "$SYSTEMD_RESOLV_CONF"; then
        RESOLV_CONF="$SYSTEMD_RESOLV_CONF"
    fi
fi
echo "$RESOLV_CONF"
`
	logger.Info("Detecting best resolv.conf path on remote host...")
	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, script, false)
	if err != nil {
		logger.Warnf("Failed to run resolv.conf detection script, falling back to /etc/resolv.conf. Error: %v, Stderr: %s", err, stderr)
		return "/etc/resolv.conf", nil
	}
	detectedPath := strings.TrimSpace(stdout)
	if detectedPath == "" {
		logger.Warn("Resolv.conf detection script returned empty, falling back to /etc/resolv.conf.")
		return "/etc/resolv.conf", nil
	}
	logger.Infof("Detected '%s' as the optimal resolv.conf path for kubelet.", detectedPath)
	return detectedPath, nil
}

func (s *CreateKubeletConfigYAMLStep) render(ctx runtime.ExecutionContext) (string, error) {
	resolvConfPath, err := s.detectResolvConf(ctx)
	if err != nil {
		return "", err
	}
	s.ResolvConf = resolvConfPath

	if ctx.GetHost().IsRole(common.RoleMaster) {
		s.StaticPodPath = common.KubernetesManifestsDir
	} else {
		s.StaticPodPath = ""
	}

	tmplContent, err := templates.Get("kubernetes/kubelet-config.yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kubelet-config.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("kubelet-config.yaml").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buffer.String(), nil
}

func (s *CreateKubeletConfigYAMLStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteConfigYAMLFile)
	if err != nil {
		return false, err
	}
	if !exists {
		logger.Info("Kubelet config.yaml does not exist. Configuration is required.")
		return false, nil
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteConfigYAMLFile)
	if err != nil {
		return false, err
	}

	expectedContent, err := s.render(ctx)
	if err != nil {
		return false, err
	}

	if string(remoteContent) == expectedContent {
		logger.Info("Kubelet config.yaml is up to date. Step is done.")
		return true, nil
	}

	logger.Warn("Kubelet config.yaml content mismatch. Re-configuration is required.")
	return false, nil
}

func (s *CreateKubeletConfigYAMLStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, filepath.Dir(s.RemoteConfigYAMLFile), "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory for kubelet config: %w", err)
	}

	content, err := s.render(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Writing kubelet config.yaml to %s", s.RemoteConfigYAMLFile)
	return runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.RemoteConfigYAMLFile, "0644", s.Sudo)
}

func (s *CreateKubeletConfigYAMLStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}
	logger.Warnf("Rolling back by removing %s", s.RemoteConfigYAMLFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteConfigYAMLFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove kubelet config file during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*CreateKubeletConfigYAMLStep)(nil)
