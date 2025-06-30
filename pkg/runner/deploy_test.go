package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockRunnerForDeploy is a helper struct to mock specific runner methods
// called by DeployAndEnableService.
type mockRunnerForDeploy struct {
	mock.Mock
	// Store the real defaultRunner if we want to call through to some of its methods,
	// but for this test, we'll mock all its dependencies.
	// realRunner *defaultRunner
}

func (m *mockRunnerForDeploy) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	args := m.Called(ctx, conn, content, destPath, permissions, sudo)
	return args.Error(0)
}

func (m *mockRunnerForDeploy) DaemonReload(ctx context.Context, conn connector.Connector, facts *Facts) error {
	args := m.Called(ctx, conn, facts)
	return args.Error(0)
}

func (m *mockRunnerForDeploy) EnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	args := m.Called(ctx, conn, facts, serviceName)
	return args.Error(0)
}

func (m *mockRunnerForDeploy) RestartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	args := m.Called(ctx, conn, facts, serviceName)
	return args.Error(0)
}

// TestDeployAndEnableService_Success tests the successful deployment and enabling of a service.
func TestDeployAndEnableService_Success(t *testing.T) {
	mockConn := mocks.NewConnector(t) // This is the connector mock

	// We need a *defaultRunner instance to call DeployAndEnableService on.
	// Its internal methods (WriteFile, DaemonReload etc.) will be mocked using a different approach
	// if we cannot directly mock methods of defaultRunner itself with testify/mock easily for self-calls.
	// Alternative: Make DeployAndEnableService a standalone function that takes a Runner interface,
	// then we can pass a mock implementing Runner.
	// For now, let's assume DeployAndEnableService is a method on defaultRunner, and its sub-calls
	// to other defaultRunner methods (like r.WriteFile) will be tested via those methods' own tests.
	// The test for DeployAndEnableService will focus on the orchestration logic and error handling.

	// To test DeployAndEnableService as a method of defaultRunner, we need to mock its *dependencies*,
	// which are other methods of defaultRunner. This is tricky with testify/mock for self-method calls.
	// A common pattern is to refactor so dependencies are interfaces, or use a real object and mock
	// its external dependencies (like the connector).

	// Let's use a real defaultRunner and mock the calls its methods make to the *connector*.
	// This means the defaultRunner's WriteFile, DaemonReload etc. will be executed, and *their* calls
	// to conn.Exec or conn.WriteFile will be mocked. This is more of an integration test for DeployAndEnableService.

	// OR, we can redefine DeployAndEnableService to take a specific interface for its internal calls:
	/*
	type serviceDeployer interface {
		WriteFile(...) error
		DaemonReload(...) error
		EnableService(...) error
		RestartService(...) error
	}
	func (r *defaultRunner) DeployAndEnableService(..., srvDeployer serviceDeployer) { ... }
	*/
	// This is a larger refactor.

	// Sticking to the current structure: defaultRunner calls its own methods.
	// We will test the sequence and that errors propagate.
	// The individual methods (WriteFile, DaemonReload, etc.) should have their own unit tests
	// that mock the connector.

	// For this test, we'll create a mock that *behaves* like defaultRunner for the sub-methods.
	// This is not testing defaultRunner.DeployAndEnableService directly, but a mocked version.
	// The most straightforward way is to test defaultRunner as a whole and mock its *external* calls.

	// Let's create a test defaultRunner.
	// The methods called by DeployAndEnableService (WriteFile, DaemonReload, EnableService, RestartService)
	// are already on defaultRunner. We can't directly use testify/mock's .On() for methods
	// of the object being tested when they call each other.
	// So, we'll test the overall flow by ensuring that if any of these sub-steps fail, the error propagates.
	// And if they all succeed, it succeeds.

	// This test will assume the *implementations* of WriteFile, DaemonReload, EnableService, RestartService
	// are correct and tested elsewhere. We are testing their orchestration by DeployAndEnableService.

	// To properly unit test DeployAndEnableService, its dependencies (WriteFile, etc.) should be mockable.
	// This means they should be part of an interface that defaultRunner uses.
	// Since they are methods of defaultRunner itself, we can't easily mock them for a single test of another method.

	// Let's use a simplified approach for now:
	// We will mock the connector calls that *would* be made by the sub-methods.
	// This makes it an integration test of DeployAndEnableService with its sub-methods.

	r := &defaultRunner{}
	facts := &Facts{
		InitSystem: &ServiceInfo{
			Type:            InitSystemSystemd,
			DaemonReloadCmd: "systemctl daemon-reload",
			EnableCmd:       "systemctl enable %s",
			RestartCmd:      "systemctl restart %s",
			// Add other necessary fields to ServiceInfo if used by sub-methods
		},
		// Add other necessary fields to Facts if used by sub-methods
	}
	serviceName := "test-service"
	// configContent := "key=value" // This was unused as the template string is passed directly
	configPath := "/etc/test-service.conf"
	permissions := "0600"
	templateData := map[string]string{"item": "foo"}
	renderedConfigContent := "key=foo" // Assuming template renders "value" to "foo"

	ctx := context.Background()

	// Expectations for WriteFile (which calls conn.WriteFile)
	mockConn.On("WriteFile", ctx, []byte(renderedConfigContent), configPath, mock.AnythingOfType("*connector.FileTransferOptions")).Return(nil).Once()

	// Expectations for DaemonReload (which calls conn.Exec for systemctl daemon-reload)
	mockConn.On("Exec", ctx, "systemctl daemon-reload", mock.AnythingOfType("*connector.ExecOptions")).Return([]byte{}, []byte{}, nil).Once()

	// Expectations for EnableService (which calls conn.Exec for systemctl enable)
	enableCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, serviceName) // Assuming this is how EnableService forms its command
	mockConn.On("Exec", ctx, enableCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte{}, []byte{}, nil).Once()

	// Expectations for RestartService (which calls conn.Exec for systemctl restart)
	restartCmd := fmt.Sprintf(facts.InitSystem.RestartCmd, serviceName) // Assuming this is how RestartService forms its command
	mockConn.On("Exec", ctx, restartCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte{}, []byte{}, nil).Once()


	err := r.DeployAndEnableService(ctx, mockConn, facts, serviceName, "key={{.item}}", configPath, permissions, templateData)
	assert.NoError(t, err)

	mockConn.AssertExpectations(t)
}


func TestDeployAndEnableService_WriteFile_Fails(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	facts := &Facts{}
	ctx := context.Background()

	configContent := "content"
	configPath := "/etc/service.conf"

	// Mock WriteFile to fail (by making its underlying conn.WriteFile call fail)
	mockConn.On("WriteFile", ctx, []byte(configContent), configPath, mock.AnythingOfType("*connector.FileTransferOptions")).Return(errors.New("write failed")).Once()

	err := r.DeployAndEnableService(ctx, mockConn, facts, "svc", configContent, configPath, "0644", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write configuration file")
	assert.Contains(t, err.Error(), "write failed")
}

func TestDeployAndEnableService_DaemonReload_Fails(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	facts := &Facts{InitSystem: &ServiceInfo{Type: InitSystemSystemd, DaemonReloadCmd: "systemctl daemon-reload"}}
	ctx := context.Background()
	serviceName := "svc"
	configContent := "content"
	configPath := "/etc/svc.conf"

	mockConn.On("WriteFile", ctx, []byte(configContent), configPath, mock.AnythingOfType("*connector.FileTransferOptions")).Return(nil).Once()
	mockConn.On("Exec", ctx, "systemctl daemon-reload", mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, errors.New("daemon reload failed")).Once()

	err := r.DeployAndEnableService(ctx, mockConn, facts, serviceName, configContent, configPath, "0644", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to perform daemon-reload")
	assert.Contains(t, err.Error(), "daemon reload failed")
}


func TestDeployAndEnableService_EnableService_Fails(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	serviceName := "svc"
	facts := &Facts{InitSystem: &ServiceInfo{Type: InitSystemSystemd, DaemonReloadCmd: "systemctl daemon-reload", EnableCmd: "systemctl enable %s"}}
	ctx := context.Background()
	configContent := "content"
	configPath := "/etc/svc.conf"
	enableCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, serviceName)


	mockConn.On("WriteFile", ctx, []byte(configContent), configPath, mock.AnythingOfType("*connector.FileTransferOptions")).Return(nil).Once()
	mockConn.On("Exec", ctx, "systemctl daemon-reload", mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, nil).Once()
	mockConn.On("Exec", ctx, enableCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, errors.New("enable failed")).Once()

	err := r.DeployAndEnableService(ctx, mockConn, facts, serviceName, configContent, configPath, "0644", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to enable service")
	assert.Contains(t, err.Error(), "enable failed")
}

func TestDeployAndEnableService_RestartService_Fails(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	serviceName := "svc"
	facts := &Facts{InitSystem: &ServiceInfo{
		Type: InitSystemSystemd,
		DaemonReloadCmd: "systemctl daemon-reload",
		EnableCmd: "systemctl enable %s",
		RestartCmd: "systemctl restart %s",
	}}
	ctx := context.Background()
	configContent := "content"
	configPath := "/etc/svc.conf"
	enableCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, serviceName)
	restartCmd := fmt.Sprintf(facts.InitSystem.RestartCmd, serviceName)

	mockConn.On("WriteFile", ctx, []byte(configContent), configPath, mock.AnythingOfType("*connector.FileTransferOptions")).Return(nil).Once()
	mockConn.On("Exec", ctx, "systemctl daemon-reload", mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, nil).Once()
	mockConn.On("Exec", ctx, enableCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, nil).Once()
	mockConn.On("Exec", ctx, restartCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, errors.New("restart failed")).Once()


	err := r.DeployAndEnableService(ctx, mockConn, facts, serviceName, configContent, configPath, "0644", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to restart service")
	assert.Contains(t, err.Error(), "restart failed")
}

func TestDeployAndEnableService_InputValidation(t *testing.T) {
	r := &defaultRunner{}
	ctx := context.Background()
	mockConn := mocks.NewConnector(t) // Real connector, but won't be used if validation fails first
	facts := &Facts{}

	testCases := []struct {
		name          string
		serviceName   string
		configContent string
		configPath    string
		templateData  interface{}
		expectedError string
	}{
		{"nil connector", "svc", "content", "/path", nil, "connector cannot be nil"},
		{"nil facts", "svc", "content", "/path", nil, "facts cannot be nil"},
		{"empty serviceName", "", "content", "/path", nil, "serviceName cannot be empty"},
		{"empty configPath", "svc", "content", "", nil, "configPath cannot be empty"},
		{"templateData without configContent", "svc", "", "/path", map[string]string{}, "configContent (template string) cannot be empty"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var currentConn connector.Connector
			if tc.expectedError != "connector cannot be nil" {
				currentConn = mockConn
			}
			var currentFacts *Facts
			if tc.expectedError != "facts cannot be nil" {
				currentFacts = facts
			}


			err := r.DeployAndEnableService(ctx, currentConn, currentFacts, tc.serviceName, tc.configContent, tc.configPath, "0644", tc.templateData)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}
