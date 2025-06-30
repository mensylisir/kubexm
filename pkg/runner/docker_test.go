package runner

import (
	"context"
	"testing"
	// "time" // Not used by current Docker stubs tests

	"github.com/mensylisir/kubexm/pkg/connector/mocks"
	"github.com/stretchr/testify/assert"
)

// TestDockerMethodStubs asserts that all Docker methods currently return "not implemented".
func TestDockerMethodStubs(t *testing.T) {
	r := &defaultRunner{}
	mockConn := mocks.NewConnector(t)
	ctx := context.Background()

	// Helper to check for "not implemented" error
	assertNotImplemented := func(t *testing.T, err error, methodName string) {
		assert.Error(t, err, "Expected an error for %s", methodName)
		if err != nil {
			assert.Contains(t, err.Error(), "not implemented: "+methodName, "Error message should indicate 'not implemented'")
		}
	}
	assertNotImplementedBool := func(t *testing.T, val bool, err error, methodName string) {
		assert.False(t, val, "Expected bool value to be false for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedString := func(t *testing.T, val string, err error, methodName string) {
		assert.Empty(t, val, "Expected string value to be empty for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedSlice := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected slice value to be nil for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedChan := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected chan value to be nil for %s", methodName) // For stubs, returning nil channel is fine.
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedPtr := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected pointer value to be nil for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}

	t.Run("PullImage", func(t *testing.T) {
		err := r.PullImage(ctx, mockConn, "")
		assertNotImplemented(t, err, "PullImage")
	})
	t.Run("ImageExists", func(t *testing.T) {
		val, err := r.ImageExists(ctx, mockConn, "")
		assertNotImplementedBool(t, val, err, "ImageExists")
	})
	t.Run("ListImages", func(t *testing.T) {
		val, err := r.ListImages(ctx, mockConn, false)
		assertNotImplementedSlice(t, val, err, "ListImages")
	})
	t.Run("RemoveImage", func(t *testing.T) {
		err := r.RemoveImage(ctx, mockConn, "", false)
		assertNotImplemented(t, err, "RemoveImage")
	})
	t.Run("BuildImage", func(t *testing.T) {
		err := r.BuildImage(ctx, mockConn, "", "", "", nil)
		assertNotImplemented(t, err, "BuildImage")
	})
	t.Run("CreateContainer", func(t *testing.T) {
		val, err := r.CreateContainer(ctx, mockConn, ContainerCreateOptions{})
		assertNotImplementedString(t, val, err, "CreateContainer")
	})
	t.Run("ContainerExists", func(t *testing.T) {
		val, err := r.ContainerExists(ctx, mockConn, "")
		assertNotImplementedBool(t, val, err, "ContainerExists")
	})
	t.Run("StartContainer", func(t *testing.T) {
		err := r.StartContainer(ctx, mockConn, "")
		assertNotImplemented(t, err, "StartContainer")
	})
	t.Run("StopContainer", func(t *testing.T) {
		err := r.StopContainer(ctx, mockConn, "", nil)
		assertNotImplemented(t, err, "StopContainer")
	})
	t.Run("RestartContainer", func(t *testing.T) {
		err := r.RestartContainer(ctx, mockConn, "", nil)
		assertNotImplemented(t, err, "RestartContainer")
	})
	t.Run("RemoveContainer", func(t *testing.T) {
		err := r.RemoveContainer(ctx, mockConn, "", false, false)
		assertNotImplemented(t, err, "RemoveContainer")
	})
	t.Run("ListContainers", func(t *testing.T) {
		val, err := r.ListContainers(ctx, mockConn, false, nil)
		assertNotImplementedSlice(t, val, err, "ListContainers")
	})
	t.Run("GetContainerLogs", func(t *testing.T) {
		val, err := r.GetContainerLogs(ctx, mockConn, "", ContainerLogOptions{})
		assertNotImplementedString(t, val, err, "GetContainerLogs")
	})
	t.Run("GetContainerStats", func(t *testing.T) {
		val, err := r.GetContainerStats(ctx, mockConn, "", false)
		assertNotImplementedChan(t, val, err, "GetContainerStats")
	})
	t.Run("InspectContainer", func(t *testing.T) {
		val, err := r.InspectContainer(ctx, mockConn, "")
		assertNotImplementedPtr(t, val, err, "InspectContainer")
	})
	t.Run("PauseContainer", func(t *testing.T) {
		err := r.PauseContainer(ctx, mockConn, "")
		assertNotImplemented(t, err, "PauseContainer")
	})
	t.Run("UnpauseContainer", func(t *testing.T) {
		err := r.UnpauseContainer(ctx, mockConn, "")
		assertNotImplemented(t, err, "UnpauseContainer")
	})
	t.Run("ExecInContainer", func(t *testing.T) {
		val, err := r.ExecInContainer(ctx, mockConn, "", nil, "", "", false)
		assertNotImplementedString(t, val, err, "ExecInContainer")
	})
	t.Run("CreateDockerNetwork", func(t *testing.T) {
		err := r.CreateDockerNetwork(ctx, mockConn, "", "", "", "", nil)
		assertNotImplemented(t, err, "CreateDockerNetwork")
	})
	t.Run("RemoveDockerNetwork", func(t *testing.T) {
		err := r.RemoveDockerNetwork(ctx, mockConn, "")
		assertNotImplemented(t, err, "RemoveDockerNetwork")
	})
	t.Run("ListDockerNetworks", func(t *testing.T) {
		val, err := r.ListDockerNetworks(ctx, mockConn, nil)
		assertNotImplementedSlice(t, val, err, "ListDockerNetworks")
	})
	t.Run("ConnectContainerToNetwork", func(t *testing.T) {
		err := r.ConnectContainerToNetwork(ctx, mockConn, "", "", "")
		assertNotImplemented(t, err, "ConnectContainerToNetwork")
	})
	t.Run("DisconnectContainerFromNetwork", func(t *testing.T) {
		err := r.DisconnectContainerFromNetwork(ctx, mockConn, "", "", false)
		assertNotImplemented(t, err, "DisconnectContainerFromNetwork")
	})
	t.Run("CreateDockerVolume", func(t *testing.T) {
		err := r.CreateDockerVolume(ctx, mockConn, "", "", nil, nil)
		assertNotImplemented(t, err, "CreateDockerVolume")
	})
	t.Run("RemoveDockerVolume", func(t *testing.T) {
		err := r.RemoveDockerVolume(ctx, mockConn, "", false)
		assertNotImplemented(t, err, "RemoveDockerVolume")
	})
	t.Run("ListDockerVolumes", func(t *testing.T) {
		val, err := r.ListDockerVolumes(ctx, mockConn, nil)
		assertNotImplementedSlice(t, val, err, "ListDockerVolumes")
	})
	t.Run("InspectDockerVolume", func(t *testing.T) {
		val, err := r.InspectDockerVolume(ctx, mockConn, "")
		assertNotImplementedPtr(t, val, err, "InspectDockerVolume")
	})
	t.Run("DockerInfo", func(t *testing.T) {
		val, err := r.DockerInfo(ctx, mockConn)
		assertNotImplementedPtr(t, val, err, "DockerInfo")
	})
	t.Run("DockerPrune", func(t *testing.T) {
		val, err := r.DockerPrune(ctx, mockConn, "", nil, false)
		assertNotImplementedString(t, val, err, "DockerPrune")
	})
}
