package common

import (
	"encoding/json"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"strings"
	"time"
)

type PullImagesStep struct {
	step.Base
	Images []string
}

type PullImagesStepBuilder struct {
	step.Builder[PullImagesStepBuilder, *PullImagesStep]
}

func NewPullImagesStepBuilder(ctx runtime.ExecutionContext, instanceName string) *PullImagesStepBuilder {
	cs := &PullImagesStep{}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Pulling images...", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 15 * time.Minute
	return new(PullImagesStepBuilder).Init(cs)
}

func (b *PullImagesStepBuilder) WithRecursive(images []string) *PullImagesStepBuilder {
	b.Step.Images = images
	return b
}

func (s *PullImagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PullImagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	existingImages, err := s.listExistingImages(ctx)
	if err != nil {
		logger.Warnf("Failed to list existing images, assuming they need to be pulled. Error: %v", err)
		return false, nil
	}

	existingImagesSet := make(map[string]struct{})
	for _, img := range existingImages {
		existingImagesSet[img] = struct{}{}
	}

	for _, imgToCheck := range s.Images {
		if _, found := existingImagesSet[imgToCheck]; !found {
			logger.Infof("Image not found on host, step needs to run: %s", imgToCheck)
			return false, nil
		}
	}

	logger.Info("All required images already exist on the host. Step considered done.")
	return true, nil
}

func (s *PullImagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("run: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	containerManager := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type
	arch := ctx.GetHost().GetArch()

	for _, img := range s.Images {
		logger.Infof("Pulling image: %s", img)

		pullCmd, err := s.getPullCommand(img, arch, string(containerManager))
		if err != nil {
			return err
		}

		if _, err := runnerSvc.Run(ctx.GoContext(), conn, pullCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to pull image %s on host %s: %w", img, ctx.GetHost().GetName(), err)
		}
	}
	logger.Info("All required images for this host pulled successfully.")
	return nil
}

func (s *PullImagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for PullImagesStep is a no-op (images are not removed).")
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

func (s *PullImagesStep) listExistingImages(ctx runtime.ExecutionContext) ([]string, error) {
	var imageListCmd string
	isJSON := false
	manager := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil, fmt.Errorf("precheck: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	switch manager {
	case common.RuntimeTypeContainerd, common.RuntimeTypeCRIO:
		imageListCmd = "crictl images -o json"
		isJSON = true
	case common.RuntimeTypeDocker:
		imageListCmd = `docker images --format "{{.Repository}}:{{.Tag}}"`
	case common.RuntimeTypeIsula:
		imageListCmd = `isulad images --format "{{.Repository}}:{{.Tag}}"`
	default:
		return nil, fmt.Errorf("unsupported container manager for listing images: %s", manager)
	}

	output, err := runnerSvc.Run(ctx.GoContext(), conn, imageListCmd, s.Sudo)
	if err != nil {
		return nil, fmt.Errorf("command '%s' failed: %w", imageListCmd, err)
	}

	if isJSON {
		var crictlOutput struct {
			Images []struct {
				RepoTags []string `json:"repoTags"`
			} `json:"images"`
		}
		if err := json.Unmarshal([]byte(output), &crictlOutput); err != nil {
			return nil, fmt.Errorf("failed to parse crictl JSON output: %w", err)
		}

		var imageList []string
		for _, img := range crictlOutput.Images {
			for _, tag := range img.RepoTags {
				if tag != "<none>:<none>" {
					imageList = append(imageList, tag)
				}
			}
		}
		return imageList, nil
	}

	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

var _ step.Step = (*PullImagesStep)(nil)
