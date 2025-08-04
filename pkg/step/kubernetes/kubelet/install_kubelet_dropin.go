package kubelet

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type InstallKubeletDropInStep struct {
	step.Base
	KubeconfigArgs           string
	ConfigYAMLPath           string
	CgroupDriver             string
	ContainerRuntimeEndpoint string
	PodInfraContainerImage   string
	NodeIP                   string
	RemoteDropInDir          string
	RemoteDropInFile         string
}

type InstallKubeletDropInStepBuilder struct {
	step.Builder[InstallKubeletDropInStepBuilder, *InstallKubeletDropInStep]
}

func NewInstallKubeletDropInStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeletDropInStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes

	s := &InstallKubeletDropInStep{
		ConfigYAMLPath:           common.KubeletConfigYAMLPathTarget,
		CgroupDriver:             common.CgroupDriverSystemd,
		ContainerRuntimeEndpoint: common.ContainerdDefaultEndpoint,
		RemoteDropInDir:          common.KubeletSystemdDropinDirTarget,
		RemoteDropInFile:         filepath.Join(common.KubeletSystemdDropinDirTarget, "10-kubexm.conf"),
	}

	if s.PodInfraContainerImage == "" {
		pauseImage := util.GetImage(ctx, "pause")
		s.PodInfraContainerImage = pauseImage.ImageName()
	}

	if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeContainerd {
		s.CgroupDriver = *k8sSpec.ContainerRuntime.Containerd.CgroupDriver
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Containerd.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Containerd.Pause
	} else if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeDocker {
		s.CgroupDriver = *k8sSpec.ContainerRuntime.Docker.CgroupDriver
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Docker.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Docker.Pause
	} else if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeCRIO {
		s.CgroupDriver = *k8sSpec.ContainerRuntime.Crio.CgroupDriver
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Crio.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Crio.Pause
	} else if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeIsula {
		s.CgroupDriver = *k8sSpec.ContainerRuntime.Isulad.CgroupDriver
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Crio.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Crio.Pause
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kubelet systemd drop-in file with args", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(InstallKubeletDropInStepBuilder).Init(s)
	return b
}

func (s *InstallKubeletDropInStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeletDropInStep) render(ctx runtime.ExecutionContext) (string, error) {
	host := ctx.GetHost()

	s.NodeIP = host.GetInternalAddress()
	if s.NodeIP == "" {
		s.NodeIP = host.GetAddress()
	}

	if host.IsRole(common.RoleMaster) {
		s.KubeconfigArgs = fmt.Sprintf("--kubeconfig=%s", common.KubeletKubeconfigPathTarget)
	} else {
		s.KubeconfigArgs = fmt.Sprintf("--bootstrap-kubeconfig=%s --kubeconfig=%s", common.KubeletBootstrapKubeconfigPathTarget, common.KubeletKubeconfigPathTarget)
	}

	tmplContent, err := templates.Get("kubernetes/kubelet-dropin-10-kubexm.conf.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kubelet drop-in template: %w", err)
	}

	tmpl, err := template.New("kubelet-dropin").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buffer.String(), nil
}

func (s *InstallKubeletDropInStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteDropInFile)
	if err != nil {
		return false, err
	}
	if !exists {
		logger.Info("Kubelet drop-in file does not exist. Installation is required.")
		return false, nil
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteDropInFile)
	if err != nil {
		return false, err
	}

	expectedContent, err := s.render(ctx)
	if err != nil {
		return false, err
	}

	if string(remoteContent) == expectedContent {
		logger.Info("Kubelet drop-in file is up to date. Step is done.")
		return true, nil
	}

	logger.Warn("Kubelet drop-in file content mismatch. Re-installation is required.")
	return false, nil
}

func (s *InstallKubeletDropInStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteDropInDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory for kubelet drop-in file: %w", err)
	}

	content, err := s.render(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Writing kubelet drop-in file to %s", s.RemoteDropInFile)
	return runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.RemoteDropInFile, "0644", s.Sudo)
}

func (s *InstallKubeletDropInStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}
	logger.Warnf("Rolling back by removing %s", s.RemoteDropInFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteDropInFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove kubelet drop-in file during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*InstallKubeletDropInStep)(nil)
