package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/mensylisir/kubexm/pkg/logger"
)

type PodmanClient struct {
	cli *client.Client
	log *logger.Logger
}

func getPodmanSocketPath() string {
	if socketPath := os.Getenv("PODMAN_SOCKET"); socketPath != "" {
		return socketPath
	}

	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		rootlessPath := filepath.Join(runtimeDir, "podman/podman.sock")
		if _, err := os.Stat(rootlessPath); err == nil {
			return rootlessPath
		}
	}

	uid := os.Getuid()
	rootlessPath := fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
	if _, err := os.Stat(rootlessPath); err == nil {
		return rootlessPath
	}

	rootfulPath := "/run/podman/podman.sock"
	if _, err := os.Stat(rootfulPath); err == nil {
		return rootfulPath
	}

	return fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
}

func NewPodmanClient() (*PodmanClient, error) {
	socketPath := getPodmanSocketPath()
	host := "unix://" + socketPath

	log := logger.Get()
	log.Info("Attempting to connect to Podman service at: %s", host)

	cli, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Podman client with host '%s': %w. Is 'podman system service' running?", host, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping Podman service at '%s': %w. Please ensure the service is active.", host, err)
	}

	log.Info("Successfully connected to Podman service.")
	return &PodmanClient{
		cli: cli,
		log: log,
	}, nil
}

func (pc *PodmanClient) Login(ctx context.Context, username, password, serverAddress string) (string, error) {
	authConfig := registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	}
	response, err := pc.cli.RegistryLogin(ctx, authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to login to registry '%s': %w", serverAddress, err)
	}
	pc.log.Info("Login Succeeded to %s", serverAddress)
	return response.Status, nil
}

func (pc *PodmanClient) PullImage(ctx context.Context, imageName string, authConfig *registry.AuthConfig) error {
	opts := image.PullOptions{}
	if authConfig != nil {
		authStr, err := encodeAuthConfig(*authConfig)
		if err != nil {
			return fmt.Errorf("failed to encode auth config for pull: %w", err)
		}
		opts.RegistryAuth = authStr
	}

	out, err := pc.cli.ImagePull(ctx, imageName, opts)
	if err != nil {
		return fmt.Errorf("failed to pull image '%s': %w", imageName, err)
	}
	defer out.Close()
	_, err = io.Copy(os.Stdout, out)
	return err
}

func (pc *PodmanClient) PushImage(ctx context.Context, imageName string, authConfig *registry.AuthConfig) error {
	opts := image.PushOptions{}
	if authConfig != nil {
		authStr, err := encodeAuthConfig(*authConfig)
		if err != nil {
			return fmt.Errorf("failed to encode auth config: %w", err)
		}
		opts.RegistryAuth = authStr
	}

	out, err := pc.cli.ImagePush(ctx, imageName, opts)
	if err != nil {
		return fmt.Errorf("failed to push image %s: %w", imageName, err)
	}
	defer out.Close()
	pc.log.Info("Pushing image %s...", imageName)
	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		return fmt.Errorf("failed to read push output for image %s: %w", imageName, err)
	}
	pc.log.Info("Successfully pushed image %s", imageName)
	return nil
}

func (pc *PodmanClient) ListImages(ctx context.Context) ([]image.Summary, error) {
	return pc.cli.ImageList(ctx, image.ListOptions{})
}

func (pc *PodmanClient) ImageExists(ctx context.Context, imageName string) (bool, error) {
	filters := filters.NewArgs()
	filters.Add("reference", imageName)
	images, err := pc.cli.ImageList(ctx, image.ListOptions{Filters: filters})
	if err != nil {
		return false, fmt.Errorf("failed to check for image '%s': %w", imageName, err)
	}
	return len(images) > 0, nil
}

func (pc *PodmanClient) SaveImagesToTar(ctx context.Context, imageNames []string, tarPath string) error {
	pc.log.Info("Saving images %v to %s", imageNames, tarPath)
	imageReader, err := pc.cli.ImageSave(ctx, imageNames)
	if err != nil {
		return fmt.Errorf("failed to save images %v: %w", imageNames, err)
	}
	defer imageReader.Close()

	file, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("failed to create tar file at %s: %w", tarPath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, imageReader)
	if err != nil {
		return fmt.Errorf("failed to write images to tar file %s: %w", tarPath, err)
	}
	pc.log.Info("Successfully saved images to %s", tarPath)
	return nil
}

func (pc *PodmanClient) LoadImageFromTar(ctx context.Context, tarPath string) error {
	pc.log.Info("Loading images from %s...", tarPath)
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar file %s: %w", tarPath, err)
	}
	defer file.Close()

	resp, err := pc.cli.ImageLoad(ctx, file, client.ImageLoadWithQuiet(true))
	if err != nil {
		return fmt.Errorf("failed to load images from tar file %s: %w", tarPath, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	pc.log.Info("Successfully loaded images from %s", tarPath)
	return nil
}

func (pc *PodmanClient) TagImage(ctx context.Context, sourceImage, targetImage string) error {
	pc.log.Info("Tagging image %s as %s", sourceImage, targetImage)
	return pc.cli.ImageTag(ctx, sourceImage, targetImage)
}

func (pc *PodmanClient) RunContainer(ctx context.Context, opts RunContainerOptions) (container.CreateResponse, error) {
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}
	for hostPort, containerPort := range opts.Ports {
		port, err := nat.NewPort(nat.SplitProtoPort(containerPort))
		if err != nil {
			return container.CreateResponse{}, fmt.Errorf("invalid container port format '%s': %w", containerPort, err)
		}
		exposedPorts[port] = struct{}{}
		hostBindingPort := hostPort
		if strings.Contains(hostPort, ":") {
			parts := strings.Split(hostPort, ":")
			hostBindingPort = parts[len(parts)-1]
		}
		portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostBindingPort}}
	}

	config := &container.Config{
		Image:        opts.ImageName,
		Cmd:          opts.Cmd,
		ExposedPorts: exposedPorts,
	}
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        opts.Volumes,
	}

	resp, err := pc.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, opts.ContainerName)
	if err != nil {
		return container.CreateResponse{}, fmt.Errorf("failed to create container '%s': %w", opts.ContainerName, err)
	}

	if err := pc.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = pc.RemoveContainer(ctx, resp.ID)
		return resp, fmt.Errorf("failed to start container '%s' (ID: %s): %w", opts.ContainerName, resp.ID, err)
	}

	pc.log.Info("Successfully started container '%s' (ID: %s)", opts.ContainerName, resp.ID)
	return resp, nil
}

func (pc *PodmanClient) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
	return pc.cli.ContainerList(ctx, container.ListOptions{All: all})
}

func (pc *PodmanClient) StopContainer(ctx context.Context, containerID string) error {
	pc.log.Info("Stopping container %s...", containerID)
	if err := pc.cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		return fmt.Errorf("failed to stop container '%s': %w", containerID, err)
	}
	pc.log.Info("Successfully stopped container %s", containerID)
	return nil
}

func (pc *PodmanClient) RemoveContainer(ctx context.Context, containerID string) error {
	pc.log.Info("Removing container %s...", containerID)
	if err := pc.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container '%s': %w", containerID, err)
	}
	pc.log.Info("Successfully removed container %s", containerID)
	return nil
}

func (pc *PodmanClient) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	out, err := pc.cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", fmt.Errorf("failed to get logs for container '%s': %w", containerID, err)
	}
	defer out.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(out)
	if err != nil {
		return "", fmt.Errorf("failed to read logs from container '%s': %w", containerID, err)
	}
	return buf.String(), nil
}

func (pc *PodmanClient) Close() error {
	pc.log.Info("Closing connection to Podman service.")
	return pc.cli.Close()
}
