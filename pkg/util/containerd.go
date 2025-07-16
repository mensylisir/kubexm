package util

import (
	"bytes"
	"context"
	"fmt"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/remotes"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/docker/api/types/registry"
	"github.com/mensylisir/kubexm/pkg/logger"
	ocispec "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	defaultContainerdAddress   = "/run/containerd/containerd.sock"
	defaultContainerdNamespace = "k8s.io"
)

type ContainerdClient struct {
	cli          *containerd.Client
	log          *logger.Logger
	auth         *registry.AuthConfig
	logBuffers   map[string]*bytes.Buffer
	logBuffersMu sync.RWMutex
}

func NewContainerdClient() (*ContainerdClient, error) {
	cli, err := containerd.New(defaultContainerdAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}

	return &ContainerdClient{
		cli: cli,
		log: logger.Get(),
		//ctx:          namespaces.WithNamespace(context.Background(), defaultContainerdNamespace),
		logBuffers:   make(map[string]*bytes.Buffer),
		logBuffersMu: sync.RWMutex{},
	}, nil
}

func (cc *ContainerdClient) getResolver(auth *registry.AuthConfig) remotes.Resolver {
	resolverOpts := docker.ResolverOptions{}
	if auth != nil {
		credsFunc := func(host string) (string, string, error) {
			if host == auth.ServerAddress || (auth.ServerAddress == "docker.io" && (host == "registry-1.docker.io" || host == "auth.docker.io")) {
				return auth.Username, auth.Password, nil
			}
			return "", "", nil
		}
		resolverOpts.Authorizer = docker.NewDockerAuthorizer(docker.WithAuthCreds(credsFunc))
	}
	return docker.NewResolver(resolverOpts)
}

func (cc *ContainerdClient) Login(_ context.Context, username, password, serverAddress string) (string, error) {
	if serverAddress == "" {
		serverAddress = "docker.io"
	}
	cc.auth = &registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	}
	cc.log.Info("Credentials stored in-memory for registry: %s", serverAddress)
	return "Login Succeeded (in-memory)", nil
}

func (cc *ContainerdClient) PullImage(ctx context.Context, imageName string, authConfig *registry.AuthConfig) error {
	auth := authConfig
	if auth == nil {
		auth = cc.auth
	}

	cc.log.Info("Pulling image %s...", imageName)
	img, err := cc.cli.Pull(ctx, imageName, containerd.WithResolver(cc.getResolver(auth)))
	if err != nil {
		return fmt.Errorf("failed to pull image '%s': %w", imageName, err)
	}
	cc.log.Info("Successfully pulled image %s (%s)", img.Name(), img.Target().Digest)
	return nil
}

func (cc *ContainerdClient) PushImage(ctx context.Context, imageName string, authConfig *registry.AuthConfig) error {
	auth := authConfig
	if auth == nil {
		auth = cc.auth
	}

	cc.log.Info("Pushing image %s...", imageName)
	img, err := cc.cli.ImageService().Get(ctx, imageName)
	if err != nil {
		return fmt.Errorf("failed to find image '%s' to push: %w", imageName, err)
	}
	descriptor := img.Target
	err = cc.cli.Push(ctx, imageName, descriptor, containerd.WithResolver(cc.getResolver(auth)))
	if err != nil {
		return fmt.Errorf("failed to push image '%s': %w", imageName, err)
	}
	cc.log.Info("Successfully pushed image %s", imageName)
	return nil
}

func (cc *ContainerdClient) ListImages(ctx context.Context) ([]images.Image, error) {
	return cc.cli.ImageService().List(ctx)
}

func (cc *ContainerdClient) ImageExists(ctx context.Context, imageName string) (bool, error) {
	_, err := cc.cli.ImageService().Get(ctx, imageName)
	if err == nil {
		return true, nil
	}
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check for image '%s': %w", imageName, err)
}

func (cc *ContainerdClient) SaveImagesToTar(ctx context.Context, imageNames []string, tarPath string) error {
	if len(imageNames) != 1 {
		return fmt.Errorf("containerd exporter currently supports exporting a single image at a time")
	}
	imageName := imageNames[0]

	cc.log.Info("Exporting image %s to %s", imageName, tarPath)
	file, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("failed to create tar file at %s: %w", tarPath, err)
	}
	defer file.Close()

	img, err := cc.cli.ImageService().Get(ctx, imageName)
	if err != nil {
		return fmt.Errorf("failed to get image '%s' for export: %w", imageName, err)
	}

	return cc.cli.Export(ctx, file, archive.WithManifest(img.Target))
}

func (cc *ContainerdClient) LoadImageFromTar(ctx context.Context, tarPath string) error {
	cc.log.Info("Importing images from %s...", tarPath)
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar file %s: %w", tarPath, err)
	}
	defer file.Close()

	_, err = cc.cli.Import(ctx, file)
	if err != nil {
		return fmt.Errorf("failed to import images from tar file %s: %w", tarPath, err)
	}
	cc.log.Info("Successfully imported images from %s", tarPath)
	return nil
}

func (cc *ContainerdClient) TagImage(ctx context.Context, sourceImage, targetImage string) error {
	cc.log.Info("Tagging image %s as %s", sourceImage, targetImage)
	is := cc.cli.ImageService()

	img, err := is.Get(ctx, sourceImage)
	if err != nil {
		return fmt.Errorf("failed to find source image '%s': %w", sourceImage, err)
	}

	img.Name = targetImage
	_, err = is.Create(ctx, img)
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			_, err = is.Update(ctx, img, "target")
			if err != nil {
				return fmt.Errorf("failed to update existing tag '%s': %w", targetImage, err)
			}
		} else {
			return fmt.Errorf("failed to create tag '%s': %w", targetImage, err)
		}
	}
	return nil
}

func (cc *ContainerdClient) RunContainer(ctx context.Context, opts RunContainerOptions) (containerd.Container, error) {
	if len(opts.Ports) > 0 {
		cc.log.Warn("Port mapping is not handled by containerd and the 'Ports' option will be ignored.")
	}

	image, err := cc.cli.GetImage(ctx, opts.ImageName)
	if err != nil {
		return nil, fmt.Errorf("failed to get image '%s': %w", opts.ImageName, err)
	}

	specOpts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithImageConfig(image),
	}
	if len(opts.Cmd) > 0 {
		specOpts = append(specOpts, oci.WithProcessArgs(opts.Cmd...))
	}
	if len(opts.Volumes) > 0 {
		var mounts []ocispec.Mount
		for _, v := range opts.Volumes {
			parts := strings.Split(v, ":")
			if len(parts) != 2 {
				cc.log.Warn("Invalid volume format '%s', skipping. Expected 'host-path:container-path'", v)
				continue
			}
			mounts = append(mounts, ocispec.Mount{
				Source:      parts[0],
				Destination: parts[1],
				Type:        "bind",
				Options:     []string{"rbind", "ro"},
			})
		}
		specOpts = append(specOpts, oci.WithMounts(mounts))
	}

	snapshotID := fmt.Sprintf("%s-snapshot", opts.ContainerName)
	containerOpts := []containerd.NewContainerOpts{
		containerd.WithSnapshotter(containerd.DefaultSnapshotter),
		containerd.WithNewSnapshot(snapshotID, image),
		containerd.WithNewSpec(specOpts...),
	}
	container, err := cc.cli.NewContainer(
		ctx,
		opts.ContainerName,
		containerOpts...,
	)
	if err != nil {
		snapshotter := cc.cli.SnapshotService(containerd.DefaultSnapshotter)
		_ = snapshotter.Remove(ctx, snapshotID)
		return nil, fmt.Errorf("failed to create container '%s': %w", opts.ContainerName, err)
	}

	logBuffer := new(bytes.Buffer)
	creator := cio.NewCreator(cio.WithStreams(nil, logBuffer, logBuffer))

	task, err := container.NewTask(ctx, creator)
	if err != nil {
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		return nil, fmt.Errorf("failed to create task for container '%s': %w", opts.ContainerName, err)
	}

	if err := task.Start(ctx); err != nil {
		_, _ = task.Delete(ctx)
		_ = container.Delete(ctx, containerd.WithSnapshotCleanup)
		return nil, fmt.Errorf("failed to start task for container '%s': %w", opts.ContainerName, err)
	}

	cc.logBuffersMu.Lock()
	cc.logBuffers[opts.ContainerName] = logBuffer
	cc.logBuffersMu.Unlock()

	cc.log.Info("Successfully started container '%s' (Task PID: %d)", opts.ContainerName, task.Pid())
	return container, nil
}

func (cc *ContainerdClient) ListContainers(ctx context.Context, _ bool) ([]containerd.Container, error) {
	return cc.cli.Containers(ctx)
}

func (cc *ContainerdClient) StopContainer(ctx context.Context, containerID string) error {
	cc.log.Info("Stopping container %s...", containerID)
	container, err := cc.cli.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container '%s': %w", containerID, err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			cc.log.Warn("Container '%s' has no running task, considering it stopped.", containerID)
			return nil
		}
		return fmt.Errorf("failed to get task for container '%s': %w", containerID, err)
	}

	status, err := task.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get task status for '%s': %w", containerID, err)
	}

	if status.Status != containerd.Running {
		cc.log.Info("Task for container '%s' is already in %s state.", containerID, status.Status)
		_, _ = task.Delete(ctx)
		return nil
	}

	if err := task.Kill(ctx, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to task for '%s': %w", containerID, err)
	}

	exitStatusC, err := task.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for task exit on '%s': %w", containerID, err)
	}
	<-exitStatusC
	_, err = task.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete task for container '%s': %w", containerID, err)
	}

	cc.log.Info("Successfully stopped container '%s'", containerID)
	return nil
}

func (cc *ContainerdClient) RemoveContainer(ctx context.Context, containerID string) error {
	if err := cc.StopContainer(ctx, containerID); err != nil {
		cc.log.Warn("Could not definitively stop container '%s' before removal: %v", containerID, err)
	}

	cc.log.Info("Removing container %s...", containerID)
	container, err := cc.cli.LoadContainer(ctx, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			cc.log.Warn("Container '%s' not found, assuming already removed.", containerID)
			return nil
		}
		return fmt.Errorf("failed to load container '%s' for removal: %w", containerID, err)
	}

	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		return fmt.Errorf("failed to delete container '%s': %w", containerID, err)
	}

	cc.logBuffersMu.Lock()
	delete(cc.logBuffers, containerID)
	cc.logBuffersMu.Unlock()

	cc.log.Info("Successfully removed container '%s'", containerID)
	return nil
}

func (cc *ContainerdClient) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	cc.logBuffersMu.RLock()
	defer cc.logBuffersMu.RUnlock()

	buffer, ok := cc.logBuffers[containerID]
	if !ok {
		_, err := cc.cli.LoadContainer(ctx, containerID)
		if err == nil {
			return "", fmt.Errorf("logs not available for container '%s' (it may not have been started by this client instance)", containerID)
		}
		return "", fmt.Errorf("container '%s' not found or logs not available", containerID)
	}
	return buffer.String(), nil
}

func (cc *ContainerdClient) Close() error {
	cc.log.Info("Closing connection to containerd.")
	return cc.cli.Close()
}
