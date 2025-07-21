package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type PushImagesStep struct {
	step.Base
	Images []string
}

type PushImagesStepBuilder struct {
	step.Builder[PushImagesStepBuilder, *PushImagesStep]
}

func NewPushImagesStepBuilder(ctx runtime.ExecutionContext, instanceName string) *PushImagesStepBuilder {
	cs := &PushImagesStep{}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Pushing container images to remote registry...", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 15 * time.Minute
	return new(PushImagesStepBuilder).Init(cs)
}

func (b *PushImagesStepBuilder) WithImages(images []string) *PushImagesStepBuilder {
	b.Step.Images = images
	return b
}

func (s *PushImagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PushImagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("PushImagesStep will always attempt to run if executed.")
	return false, nil
}

func (s *PushImagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	containerManager := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type

	for _, img := range s.Images {
		logger.Infof("Pushing image: %s", img)

		pushCmd, err := s.getPushCommand(img, string(containerManager))
		if err != nil {
			return err
		}

		if _, err := runnerSvc.Run(ctx.GoContext(), conn, pushCmd, s.Sudo); err != nil {
			errMsg := fmt.Sprintf("failed to push image %s on host %s. "+
				"Please check registry credentials, network connectivity, and local image existence. Error: %v",
				img, ctx.GetHost().GetName(), err)
			return fmt.Errorf(errMsg)
		}
	}

	logger.Info("All required images for this host pushed successfully.")
	return nil
}

func (s *PushImagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for PushImagesStep is a no-op, images will not be removed from the remote registry.")
	return nil
}

func (s *PushImagesStep) getPushCommand(imageName, manager string) (string, error) {
	var pushCmd string
	switch manager {
	case "crio", "containerd":
		pushCmd = fmt.Sprintf("crictl push %s", imageName)
	case "isula":
		pushCmd = fmt.Sprintf("isula push %s", imageName)
	case "docker":
		pushCmd = fmt.Sprintf("docker push %s", imageName)
	default:
		return "", fmt.Errorf("unsupported container manager for push: %s", manager)
	}
	return fmt.Sprintf("env PATH=$PATH:/usr/local/bin %s", pushCmd), nil
}

var _ step.Step = (*PushImagesStep)(nil)
