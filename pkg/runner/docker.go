package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// --- Docker Methods ---

func (r *defaultRunner) PullImage(ctx context.Context, conn connector.Connector, imageName string) error {
	return fmt.Errorf("not implemented: PullImage")
}
func (r *defaultRunner) ImageExists(ctx context.Context, conn connector.Connector, imageName string) (bool, error) {
	return false, fmt.Errorf("not implemented: ImageExists")
}
func (r *defaultRunner) ListImages(ctx context.Context, conn connector.Connector, all bool) ([]ImageInfo, error) {
	return nil, fmt.Errorf("not implemented: ListImages")
}
func (r *defaultRunner) RemoveImage(ctx context.Context, conn connector.Connector, imageName string, force bool) error {
	return fmt.Errorf("not implemented: RemoveImage")
}
func (r *defaultRunner) BuildImage(ctx context.Context, conn connector.Connector, dockerfilePath string, imageNameAndTag string, contextPath string, buildArgs map[string]string) error {
	return fmt.Errorf("not implemented: BuildImage")
}
func (r *defaultRunner) CreateContainer(ctx context.Context, conn connector.Connector, options ContainerCreateOptions) (string, error) {
	return "", fmt.Errorf("not implemented: CreateContainer")
}
func (r *defaultRunner) ContainerExists(ctx context.Context, conn connector.Connector, containerNameOrID string) (bool, error) {
	return false, fmt.Errorf("not implemented: ContainerExists")
}
func (r *defaultRunner) StartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error {
	return fmt.Errorf("not implemented: StartContainer")
}
func (r *defaultRunner) StopContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error {
	return fmt.Errorf("not implemented: StopContainer")
}
func (r *defaultRunner) RestartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error {
	return fmt.Errorf("not implemented: RestartContainer")
}
func (r *defaultRunner) RemoveContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, force bool, removeVolumes bool) error {
	return fmt.Errorf("not implemented: RemoveContainer")
}
func (r *defaultRunner) ListContainers(ctx context.Context, conn connector.Connector, all bool, filters map[string]string) ([]ContainerInfo, error) {
	return nil, fmt.Errorf("not implemented: ListContainers")
}
func (r *defaultRunner) GetContainerLogs(ctx context.Context, conn connector.Connector, containerNameOrID string, options ContainerLogOptions) (string, error) {
	return "", fmt.Errorf("not implemented: GetContainerLogs")
}
func (r *defaultRunner) GetContainerStats(ctx context.Context, conn connector.Connector, containerNameOrID string, stream bool) (<-chan ContainerStats, error) {
	// For a stub, returning a closed channel or nil with error is appropriate.
	// A closed channel signals no data immediately.
	// ch := make(chan ContainerStats)
	// close(ch)
	// return ch, fmt.Errorf("not implemented: GetContainerStats")
	return nil, fmt.Errorf("not implemented: GetContainerStats")
}
func (r *defaultRunner) InspectContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) (*ContainerDetails, error) {
	return nil, fmt.Errorf("not implemented: InspectContainer")
}
func (r *defaultRunner) PauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error {
	return fmt.Errorf("not implemented: PauseContainer")
}
func (r *defaultRunner) UnpauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error {
	return fmt.Errorf("not implemented: UnpauseContainer")
}
func (r *defaultRunner) ExecInContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, cmd []string, user string, workDir string, tty bool) (string, error) {
	return "", fmt.Errorf("not implemented: ExecInContainer")
}
func (r *defaultRunner) CreateDockerNetwork(ctx context.Context, conn connector.Connector, name string, driver string, subnet string, gateway string, options map[string]string) error {
	return fmt.Errorf("not implemented: CreateDockerNetwork")
}
func (r *defaultRunner) RemoveDockerNetwork(ctx context.Context, conn connector.Connector, networkNameOrID string) error {
	return fmt.Errorf("not implemented: RemoveDockerNetwork")
}
func (r *defaultRunner) ListDockerNetworks(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerNetworkInfo, error) {
	return nil, fmt.Errorf("not implemented: ListDockerNetworks")
}
func (r *defaultRunner) ConnectContainerToNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, ipAddress string) error {
	return fmt.Errorf("not implemented: ConnectContainerToNetwork")
}
func (r *defaultRunner) DisconnectContainerFromNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, force bool) error {
	return fmt.Errorf("not implemented: DisconnectContainerFromNetwork")
}
func (r *defaultRunner) CreateDockerVolume(ctx context.Context, conn connector.Connector, name string, driver string, driverOpts map[string]string, labels map[string]string) error {
	return fmt.Errorf("not implemented: CreateDockerVolume")
}
func (r *defaultRunner) RemoveDockerVolume(ctx context.Context, conn connector.Connector, volumeName string, force bool) error {
	return fmt.Errorf("not implemented: RemoveDockerVolume")
}
func (r *defaultRunner) ListDockerVolumes(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerVolumeInfo, error) {
	return nil, fmt.Errorf("not implemented: ListDockerVolumes")
}
func (r *defaultRunner) InspectDockerVolume(ctx context.Context, conn connector.Connector, volumeName string) (*DockerVolumeDetails, error) {
	return nil, fmt.Errorf("not implemented: InspectDockerVolume")
}
func (r *defaultRunner) DockerInfo(ctx context.Context, conn connector.Connector) (*DockerSystemInfo, error) {
	return nil, fmt.Errorf("not implemented: DockerInfo")
}
func (r *defaultRunner) DockerPrune(ctx context.Context, conn connector.Connector, pruneType string, filters map[string]string, all bool) (string, error) {
	return "", fmt.Errorf("not implemented: DockerPrune")
}
