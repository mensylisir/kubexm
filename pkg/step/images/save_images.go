package images

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type SaveImagesStep struct {
	step.Base
	ImagesDir   string
	Concurrency int
}

type SaveImagesStepBuilder struct {
	step.Builder[SaveImagesStepBuilder, *SaveImagesStep]
}

func NewSaveImagesStepBuilder(ctx runtime.Context, instanceName string) *SaveImagesStepBuilder {
	s := &SaveImagesStep{
		ImagesDir:   filepath.Join(ctx.GetClusterArtifactsDir(), "images"),
		Concurrency: 5,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Save all required container images to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 60 * time.Minute

	b := new(SaveImagesStepBuilder).Init(s)
	return b
}

func (b *SaveImagesStepBuilder) WithConcurrency(c int) *SaveImagesStepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

func (s *SaveImagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SaveImagesStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration")
	}

	for _, host := range allHosts {
		archs[host.GetArch()] = true
	}
	return archs, nil
}

func (s *SaveImagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if _, err := exec.LookPath("skopeo"); err != nil {
		return false, fmt.Errorf("skopeo command not found in PATH, please install it first")
	}

	return false, nil
}

func (s *SaveImagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	imageProvider := images.NewImageProvider(ctx)
	imagesToSave := imageProvider.GetImages()
	if len(imagesToSave) == 0 {
		logger.Info("No images need to be saved for this configuration.")
		return nil
	}

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.ImagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create local images directory '%s': %w", s.ImagesDir, err)
	}

	type copyJob struct {
		Image *images.Image
		Arch  string
	}

	totalJobs := len(imagesToSave) * len(requiredArchs)
	jobs := make(chan copyJob, totalJobs)
	errChan := make(chan error, totalJobs)

	var wg sync.WaitGroup
	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				logger.Infof("Worker %d: saving image %s for arch %s", workerID, job.Image.OriginalFullName(), job.Arch)
				err := s.copyImage(job.Image, job.Arch)
				if err != nil {
					logger.Errorf("Worker %d: failed to save image %s for arch %s: %v", workerID, job.Image.OriginalFullName(), job.Arch, err)
					errChan <- err
				}
			}
		}(i)
	}

	for _, image := range imagesToSave {
		for arch := range requiredArchs {
			jobs <- copyJob{Image: image, Arch: arch}
		}
	}
	close(jobs)

	wg.Wait()
	close(errChan)

	var allErrors []string
	for err := range errChan {
		allErrors = append(allErrors, err.Error())
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("failed to save some images:\n- %s", strings.Join(allErrors, "\n- "))
	}

	logger.Info("All required container images have been saved successfully.")
	return nil
}

func (s *SaveImagesStep) copyImage(img *images.Image, arch string) error {
	srcName := "docker://" + img.OriginalFullName()

	destTag := fmt.Sprintf("%s-%s", img.OriginalFullName(), arch)
	destName := fmt.Sprintf("oci:%s:%s", s.ImagesDir, destTag)

	args := []string{
		"copy",
		"--override-os=linux",
		"--override-arch=" + arch,
		"--dest-oci-accept-uncompressed-layers",
		srcName,
		destName,
	}

	cmd := exec.Command("skopeo", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy %s for arch %s: %w\nOutput: %s", srcName, arch, err, string(output))
	}

	return nil
}

func (s *SaveImagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for SaveImagesStep is a no-op, but you can manually delete the directory:", "path", s.ImagesDir)
	return nil
}

var _ step.Step = (*SaveImagesStep)(nil)
