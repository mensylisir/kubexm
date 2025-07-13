package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util/images"
)

type PullImagesStep struct {
	meta     spec.StepMeta
	Resolver *images.Resolver
	Sudo     bool
}

func NewPullImagesStep(resolver *images.Resolver) step.Step {
	return &PullImagesStep{
		meta: spec.StepMeta{
			Name:        "Pull Container Images",
			Description: "Pull all required container images on each node",
		},
		Resolver: resolver,
	}
}

func (s *PullImagesStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *PullImagesStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("precheck: failed to get connector for host %s: %w", host.GetName(), err)
	}

	containerManager := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type
	var imageListCmd string
	switch containerManager {
	case common.RuntimeTypeContainerd, common.RuntimeTypeCRIO:
		imageListCmd = "crictl images --output=json"
	case common.RuntimeTypeDocker:
		imageListCmd = `docker images --format "{{.Repository}}:{{.Tag}}"`
	case common.RuntimeTypeIsula:
		imageListCmd = `isulad images --format "{{.Repository}}:{{.Tag}}"`
	default:
		imageListCmd = `docker images --format "{{.Repository}}:{{.Tag}}"`
	}

	output, err := runnerSvc.Run(ctx.GoContext(), conn, imageListCmd, s.Sudo)
	if err != nil {
		logger.Warn("Failed to list existing images, assuming they need to be pulled.", "error", err)
		return false, nil
	}

	existingImages := string(output)
	allExist := true
	for _, img := range s.Images {
		fullImageName := s.Resolver.Full(img)
		if !strings.Contains(existingImages, fullImageName) {
			logger.Info("Image not found on host, needs to be pulled.", "image", fullImageName)
			allExist = false
			break
		}
	}

	if allExist {
		logger.Info("All required images already exist on the host. Step considered done.")
		return true, nil
	}

	logger.Info("Some required images are missing. Step needs to run.")
	return false, nil
}

func (s *PullImagesStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("run: failed to get connector for host %s: %w", host.GetName(), err)
	}

	containerManager := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type
	arch := host.GetArch()

	for _, img := range s.Images {
		fullImageName := s.Resolver.Full(img)

		logger.Info("Pulling image", "name", fullImageName)

		pullCmd, err := s.getPullCommand(fullImageName, arch, string(containerManager))
		if err != nil {
			return err
		}

		if _, err := runnerSvc.Run(ctx.GoContext(), conn, pullCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to pull image %s on host %s: %w", fullImageName, host.GetName(), err)
		}
	}
	logger.Info("All required images for this host pulled successfully.")
	return nil
}

func (s *PullImagesStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for PullImagesStep is a no-op (no action taken).")
	return nil
}

func (s *PullImagesStep) getPullCommand(imageName, arch, manager string) (string, error) {
	var pullCmd string
	switch manager {
	case "crio", "containerd":
		pullCmd = fmt.Sprintf("crictl pull %s", imageName)
	case "isula":
		pullCmd = fmt.Sprintf("isula pull --platform %s %s", arch, imageName)
	case "docker":
		pullCmd = fmt.Sprintf("docker pull --platform %s %s", arch, imageName)
	default:
		return "", fmt.Errorf("unsupported container manager: %s", manager)
	}
	return fmt.Sprintf("env PATH=$PATH:/usr/local/bin %s", pullCmd), nil
}

var _ step.Step = (*PullImagesStep)(nil)
