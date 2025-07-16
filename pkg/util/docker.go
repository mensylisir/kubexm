package util

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/image"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerClient struct {
	cli *client.Client
	ctx context.Context
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
		ctx: context.Background(),
	}, nil
}

func (dc *DockerClient) PullImage(imageName string) error {
	out, err := dc.cli.ImagePull(dc.ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image '%s': %w", imageName, err)
	}
	defer out.Close()
	_, err = io.Copy(os.Stdout, out)
	return err
}

func (dc *DockerClient) ListImages() ([]image.Summary, error) {
	images, err := dc.cli.ImageList(dc.ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	return images, nil
}

func (dc *DockerClient) ImageExists(imageName string) (bool, error) {
	filters := filters.NewArgs()
	filters.Add("reference", imageName)

	images, err := dc.cli.ImageList(dc.ctx, image.ListOptions{Filters: filters})
	if err != nil {
		return false, fmt.Errorf("failed to check for image '%s': %w", imageName, err)
	}

	return len(images) > 0, nil
}

func (dc *DockerClient) RunContainer(opts RunContainerOptions) (container.CreateResponse, error) {
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}
	for hostPort, containerPort := range opts.Ports {
		var port nat.Port
		var err error
		if strings.Contains(containerPort, "/") {
			parts := strings.Split(containerPort, "/")
			port, err = nat.NewPort(parts[1], parts[0])
		} else {
			port, err = nat.NewPort("tcp", containerPort)
		}
		if err != nil {
			return container.CreateResponse{}, fmt.Errorf("invalid port format '%s': %w", containerPort, err)
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

	resp, err := dc.cli.ContainerCreate(dc.ctx, config, hostConfig, nil, nil, opts.ContainerName)
	if err != nil {
		return container.CreateResponse{}, fmt.Errorf("failed to create container '%s': %w", opts.ContainerName, err)
	}

	if err := dc.cli.ContainerStart(dc.ctx, resp.ID, container.StartOptions{}); err != nil {
		return resp, fmt.Errorf("failed to start container '%s' (ID: %s): %w", opts.ContainerName, resp.ID, err)
	}

	return resp, nil
}

func (dc *DockerClient) ListContainers(all bool) ([]types.Container, error) {
	containers, err := dc.cli.ContainerList(dc.ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}

func (dc *DockerClient) StopContainer(containerID string) error {
	if err := dc.cli.ContainerStop(dc.ctx, containerID, container.StopOptions{}); err != nil {
		return fmt.Errorf("failed to stop container '%s': %w", containerID, err)
	}
	return nil
}

func (dc *DockerClient) RemoveContainer(containerID string) error {
	if err := dc.cli.ContainerRemove(dc.ctx, containerID, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove container '%s': %w", containerID, err)
	}
	return nil
}

func (dc *DockerClient) GetContainerLogs(containerID string) (string, error) {
	out, err := dc.cli.ContainerLogs(dc.ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
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

func (dc *DockerClient) Close() error {
	return dc.cli.Close()
}
