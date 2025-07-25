package kubeadm

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type CreateKubeletEnvFileStep struct {
	step.Base
	CgroupDriver             string
	ContainerRuntimeEndpoint string
	PodInfraContainerImage   string
	RemoteEnvFile            string
}

type CreateKubeletEnvFileStepBuilder struct {
	step.Builder[CreateKubeletEnvFileStepBuilder, *CreateKubeletEnvFileStep]
}

func NewCreateKubeletEnvFileStepBuilder(ctx runtime.Context, instanceName string) *CreateKubeletEnvFileStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes

	s := &CreateKubeletEnvFileStep{
		CgroupDriver:             k8sSpec.Kubelet.CgroupDriver,
		ContainerRuntimeEndpoint: common.ContainerdDefaultEndpoint,
		RemoteEnvFile:            common.KubeletFlagsEnvPathTarget,
	}

	if s.PodInfraContainerImage == "" {
		pauseImage := helpers.GetImage(ctx, "pause")
		s.PodInfraContainerImage = pauseImage.ImageName()
	}

	if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeContainerd {
		if *k8sSpec.ContainerRuntime.Containerd.UseSystemdCgroup {
			s.CgroupDriver = common.CgroupDriverSystemd
		} else {
			s.CgroupDriver = common.CgroupDriverCgroupfs
		}
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Containerd.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Containerd.Pause
	} else if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeDocker {
		s.CgroupDriver = *k8sSpec.ContainerRuntime.Docker.CgroupDriver
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Docker.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Docker.Pause
	} else if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeCRIO {
		s.CgroupDriver = *k8sSpec.ContainerRuntime.Crio.CgroupManager
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Crio.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Crio.Pause
	} else if k8sSpec.ContainerRuntime.Type == common.RuntimeTypeIsula {
		s.CgroupDriver = *k8sSpec.ContainerRuntime.Isulad.CgroupManager
		s.ContainerRuntimeEndpoint = k8sSpec.ContainerRuntime.Crio.Endpoint
		s.PodInfraContainerImage = k8sSpec.ContainerRuntime.Crio.Pause
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create kubelet environment file for dynamic flags", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(CreateKubeletEnvFileStepBuilder).Init(s)
	return b
}

func (s *CreateKubeletEnvFileStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CreateKubeletEnvFileStep) render() (string, error) {
	tmplContent, err := templates.Get("kubernetes/kubelet-kubeadm-flags.env.tmpl")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("kubelet-env").Parse(tmplContent)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (s *CreateKubeletEnvFileStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteEnvFile)
	if err != nil {
		return false, err
	}
	if !exists {
		logger.Info("Kubelet env file does not exist. Creation is required.")
		return false, nil
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteEnvFile)
	if err != nil {
		return false, err
	}

	expectedContent, err := s.render()
	if err != nil {
		return false, err
	}

	if string(remoteContent) == expectedContent {
		logger.Info("Kubelet env file is up to date. Step is done.")
		return true, nil
	}

	logger.Warn("Kubelet env file content mismatch. Re-creation is required.")
	return false, nil
}

func (s *CreateKubeletEnvFileStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, filepath.Dir(s.RemoteEnvFile), "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory for kubelet env file: %w", err)
	}

	content, err := s.render()
	if err != nil {
		return err
	}

	logger.Infof("Writing kubelet env file to %s", s.RemoteEnvFile)
	return runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.RemoteEnvFile, "0644", s.Sudo)
}

func (s *CreateKubeletEnvFileStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}
	logger.Warnf("Rolling back by removing %s", s.RemoteEnvFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteEnvFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove kubelet env file during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*CreateKubeletEnvFileStep)(nil)
