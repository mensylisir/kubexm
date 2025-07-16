package util

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/mensylisir/kubexm/pkg/logger"
	"io"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	defaultDockerAddress = "/var/run/docker.sock"
)

type DockerClient struct {
	cli *client.Client
	log *logger.Logger
}

type RunContainerOptions struct {
	ImageName     string
	ContainerName string
	Cmd           []string
	Ports         map[string]string
	Volumes       []string
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerClient{
		cli: cli,
		log: logger.Get(),
	}, nil
}

func (dc *DockerClient) PullImage(ctx context.Context, imageName string, authConfig *registry.AuthConfig) error {
	opts := image.PullOptions{}

	if authConfig != nil {
		authStr, err := encodeAuthConfig(*authConfig)
		if err != nil {
			return fmt.Errorf("failed to encode auth config for pull: %w", err)
		}
		opts.RegistryAuth = authStr
	}

	out, err := dc.cli.ImagePull(ctx, imageName, opts)
	if err != nil {
		return fmt.Errorf("failed to pull image '%s': %w", imageName, err)
	}
	defer out.Close()
	_, err = io.Copy(os.Stdout, out)
	return err
}

func (dc *DockerClient) ListImages(ctx context.Context) ([]image.Summary, error) {
	images, err := dc.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	return images, nil
}

func (dc *DockerClient) ImageExists(ctx context.Context, imageName string) (bool, error) {
	filters1 := filters.NewArgs()
	filters1.Add("reference", imageName)

	images, err := dc.cli.ImageList(ctx, image.ListOptions{Filters: filters1})
	if err != nil {
		return false, fmt.Errorf("failed to check for image '%s': %w", imageName, err)
	}

	return len(images) > 0, nil
}

func (dc *DockerClient) RunContainer(ctx context.Context, opts RunContainerOptions) (container.CreateResponse, error) {
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}
	for hostPort, containerPort := range opts.Ports {
		var port nat.Port
		var err error
		port, err = nat.NewPort(nat.SplitProtoPort(containerPort))
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

	resp, err := dc.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, opts.ContainerName)
	if err != nil {
		return container.CreateResponse{}, fmt.Errorf("failed to create container '%s': %w", opts.ContainerName, err)
	}

	if err := dc.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = dc.RemoveContainer(ctx, resp.ID)
		return resp, fmt.Errorf("failed to start container '%s' (ID: %s): %w", opts.ContainerName, resp.ID, err)
	}

	return resp, nil
}

func (dc *DockerClient) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
	containers, err := dc.cli.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}

func (dc *DockerClient) StopContainer(ctx context.Context, containerID string) error {
	return dc.cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

func (dc *DockerClient) RemoveContainer(ctx context.Context, containerID string) error {
	if err := dc.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container '%s': %w", containerID, err)
	}
	return nil
}

func (dc *DockerClient) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	out, err := dc.cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
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

func (dc *DockerClient) SaveImagesToTar(ctx context.Context, imageNames []string, tarPath string) error {
	dc.log.Info("Saving images %v to %s", imageNames, tarPath)
	imageReader, err := dc.cli.ImageSave(ctx, imageNames)
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

	dc.log.Info("Successfully saved images to %s", tarPath)
	return nil
}

func (dc *DockerClient) LoadImageFromTar(ctx context.Context, tarPath string) error {
	dc.log.Info("Loading images from %s...", tarPath)
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar file %s: %w", tarPath, err)
	}
	defer file.Close()

	resp, err := dc.cli.ImageLoad(ctx, file, client.ImageLoadWithQuiet(false))
	if err != nil {
		return fmt.Errorf("failed to load images from tar file %s: %w", tarPath, err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read image load response: %w", err)
	}

	dc.log.Info("Successfully loaded images from %s", tarPath)
	return nil
}

func (dc *DockerClient) TagImage(ctx context.Context, sourceImage, targetImage string) error {
	dc.log.Info("Tagging image %s as %s", sourceImage, targetImage)
	err := dc.cli.ImageTag(ctx, sourceImage, targetImage)
	if err != nil {
		return fmt.Errorf("failed to tag image %s as %s: %w", sourceImage, targetImage, err)
	}
	return nil
}

func (dc *DockerClient) PushImage(ctx context.Context, imageName string, authConfig *registry.AuthConfig) error {
	opts := image.PushOptions{}
	if authConfig != nil {
		authStr, err := encodeAuthConfig(*authConfig)
		if err != nil {
			return fmt.Errorf("failed to encode auth config: %w", err)
		}
		opts.RegistryAuth = authStr
	}

	out, err := dc.cli.ImagePush(ctx, imageName, opts)
	if err != nil {
		return fmt.Errorf("failed to push image %s: %w", imageName, err)
	}
	defer out.Close()

	dc.log.Info("Pushing image %s...", imageName)
	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		return fmt.Errorf("failed to read push output for image %s: %w", imageName, err)
	}
	dc.log.Info("Successfully pushed image %s", imageName)
	return nil
}

func (dc *DockerClient) Login(ctx context.Context, username, password, serverAddress string) (string, error) {
	authConfig := registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	}

	response, err := dc.cli.RegistryLogin(ctx, authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to login to registry '%s': %w", serverAddress, err)
	}
	dc.log.Info("Login Succeeded to %s", serverAddress)
	return response.Status, nil
}

func RetagImageForRegistry(originalImage, newRegistry, newNamespace string) (string, error) {
	named, err := reference.ParseDockerRef(originalImage)
	if err != nil {
		return "", fmt.Errorf("failed to parse original image name '%s': %w", originalImage, err)
	}
	originalPath := reference.TrimNamed(named).Name()

	var newPath string
	if newNamespace != "" {
		newPath = path.Join(newNamespace, originalPath)
	} else {
		if !strings.Contains(originalPath, "/") {
			newPath = path.Join("library", originalPath)
		} else {
			newPath = originalPath
		}
	}

	finalNameWithRegistry := path.Join(newRegistry, newPath)

	var finalName string
	if tagged, ok := named.(reference.NamedTagged); ok {
		finalName = fmt.Sprintf("%s:%s", finalNameWithRegistry, tagged.Tag())
	} else if digested, ok := named.(reference.Digested); ok {
		finalName = fmt.Sprintf("%s@%s", finalNameWithRegistry, digested.Digest())
	} else {
		finalName = finalNameWithRegistry
	}
	return finalName, nil
}

func encodeAuthConfig(authConfig registry.AuthConfig) (string, error) {
	authBytes, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(authBytes), nil
}

func (dc *DockerClient) Close() error {
	return dc.cli.Close()
}
