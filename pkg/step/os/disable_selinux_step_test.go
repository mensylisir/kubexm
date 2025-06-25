package os

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime" // Using full runtime for mocks
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/task" // For task.TaskContext if needed by underlying mocks

	// Mocks for runner and potentially connector if not using real ones
	connectormocks "github.com/mensylisir/kubexm/pkg/connector/mocks"
	loggermocks "github.com/mensylisir/kubexm/pkg/logger/mocks"
	runnermocks "github.com/mensylisir/kubexm/pkg/runner/mocks"
	stepmocks "github.com/mensylisir/kubexm/pkg/step/mocks" // For step.StepContext mock
)

func TestDisableSelinuxStep_NewDisableSelinuxStep(t *testing.T) {
	s := NewDisableSelinuxStep("TestDisableSelinux", true)
	require.NotNil(t, s)
	assert.Equal(t, "TestDisableSelinux", s.Meta().Name)
	assert.True(t, s.(*DisableSelinuxStep).Sudo)

	sDefault := NewDisableSelinuxStep("", false)
	require.NotNil(t, sDefault)
	assert.Equal(t, "DisableSelinux", sDefault.Meta().Name)
	assert.False(t, sDefault.(*DisableSelinuxStep).Sudo)
}

func TestDisableSelinuxStep_Precheck(t *testing.T) {
	tests := []struct {
		name                string
		getenforceOutput    string
		getenforceErr       error
		readFileContent     string
		readFileErr         error
		expectedIsDone      bool
		expectedErrContains string
	}{
		{
			name:             "SELinux already disabled by getenforce and config",
			getenforceOutput: "Disabled",
			readFileContent:  "SELINUX=disabled\nSOMETHING_ELSE=true",
			expectedIsDone:   true,
		},
		{
			name:             "SELinux permissive by getenforce, needs config change",
			getenforceOutput: "Permissive",
			readFileContent:  "SELINUX=permissive",
			expectedIsDone:   false,
		},
		{
			name:             "SELinux enforcing by getenforce",
			getenforceOutput: "Enforcing",
			expectedIsDone:   false,
		},
		{
			name:          "getenforce fails (e.g., not installed)",
			getenforceErr: errors.New("getenforce command failed"),
			// In this case, the step assumes SELinux is not an issue and skips.
			expectedIsDone: true,
		},
		{
			name:             "SELinux disabled by getenforce, but config still enforcing",
			getenforceOutput: "Disabled",
			readFileContent:  "SELINUX=enforcing",
			expectedIsDone:   false, // Needs to run to fix config
		},
		{
			name:             "SELinux disabled by getenforce, config file read error",
			getenforceOutput: "Disabled",
			readFileErr:      errors.New("permission denied"),
			expectedIsDone:   false, // Needs to run as config state is unknown
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := new(runnermocks.MockRunner)
			mockConnector := new(connectormocks.MockConnector)
			mockLogger := new(loggermocks.MockLogger)
			mockHost := new(connectormocks.MockHost)

			mockHost.On("GetName").Return("test-host")
			mockLogger.On("With", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockLogger) // Handle logger.With
			mockLogger.On("Debug", mock.AnythingOfType("string")).Maybe()
			mockLogger.On("Info", mock.AnythingOfType("string")).Maybe()
			mockLogger.On("Warn", mock.AnythingOfType("string")).Maybe()


			// Mock getenforce
			var getenforceCmdErr error
			var getenforceCmdExitCode int
			if tt.getenforceErr != nil {
				if strings.Contains(tt.getenforceErr.Error(), "exit code") { // Simple way to simulate CommandError
					// Parse exit code if provided in error for more specific CommandError mocking
					// For now, just a generic error.
					getenforceCmdErr = &connector.CommandError{Underlying: tt.getenforceErr, ExitCode: 1, Stderr: "simulated error"}
				} else {
					getenforceCmdErr = tt.getenforceErr // Generic error
				}
			}
			mockRunner.On("RunWithOptions", mock.Anything, mockConnector, "getenforce", mock.AnythingOfType("*connector.ExecOptions")).
				Return([]byte(tt.getenforceOutput), []byte(""), getenforceCmdErr).Once()

			// Mock ReadFile for /etc/selinux/config if getenforce doesn't make it skip
			if tt.getenforceOutput == "Disabled" { // Only mock ReadFile if getenforce is "Disabled"
				mockRunner.On("ReadFile", mock.Anything, mockConnector, selinuxConfigFile).
					Return([]byte(tt.readFileContent), tt.readFileErr).Once()
			}

			// Use a simplified mock for StepContext
			mockStepCtx := new(stepmocks.MockStepContext)
			mockStepCtx.On("GetLogger").Return(mockLogger)
			mockStepCtx.On("GetRunner").Return(mockRunner)
			mockStepCtx.On("GetConnectorForHost", mockHost).Return(mockConnector, nil)
			mockStepCtx.On("GoContext").Return(context.Background())


			s := NewDisableSelinuxStep("", true).(*DisableSelinuxStep)
			isDone, err := s.Precheck(mockStepCtx, mockHost)

			assert.Equal(t, tt.expectedIsDone, isDone)
			if tt.expectedErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrContains)
			} else {
				assert.NoError(t, err)
			}
			mockRunner.AssertExpectations(t)
		})
	}
}


func TestDisableSelinuxStep_Run(t *testing.T) {
	mockRunner := new(runnermocks.MockRunner)
	mockConnector := new(connectormocks.MockConnector)
	mockLogger := new(loggermocks.MockLogger)
	mockHost := new(connectormocks.MockHost)
	mockStepCtx := new(stepmocks.MockStepContext)

	mockHost.On("GetName").Return("test-host")
	mockLogger.On("With", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockLogger)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()


	mockStepCtx.On("GetLogger").Return(mockLogger)
	mockStepCtx.On("GetRunner").Return(mockRunner)
	mockStepCtx.On("GetConnectorForHost", mockHost).Return(mockConnector, nil)
	mockStepCtx.On("GoContext").Return(context.Background())

	s := NewDisableSelinuxStep("TestRunDisableSelinux", true).(*DisableSelinuxStep)
	s.originalSelinuxValue = "enforcing" // Simulate precheck found it enforcing

	// Mock setenforce 0
	mockRunner.On("RunWithOptions", mock.Anything, mockConnector, "setenforce 0", mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
		Return([]byte(""), []byte(""), nil).Once()

	// Mock sed command
	expectedSedCmd := fmt.Sprintf("sed -i.bak -e 's/^SELINUX=enforcing/SELINUX=disabled/' -e 's/^SELINUX=permissive/SELINUX=disabled/' %s", selinuxConfigFile)
	mockRunner.On("RunWithOptions", mock.Anything, mockConnector, expectedSedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
		Return([]byte(""), []byte(""), nil).Once()

	err := s.Run(mockStepCtx, mockHost)
	assert.NoError(t, err)
	assert.True(t, s.rebootRequired)

	mockRunner.AssertExpectations(t)
}

// TODO: Add TestDisableSelinuxStep_Rollback
// It would involve mocking Exists for backup file, and mv/sed commands for restore.

func TestDisableSelinuxStep_Rollback_RestoresFromBackup(t *testing.T) {
    mockRunner := new(runnermocks.MockRunner)
    mockConnector := new(connectormocks.MockConnector)
    mockLogger := new(loggermocks.MockLogger)
    mockHost := new(connectormocks.MockHost)
    mockStepCtx := new(stepmocks.MockStepContext)

    mockHost.On("GetName").Return("test-host")
    mockLogger.On("With", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockLogger)
    mockLogger.On("Info", mock.Anything).Maybe()
    mockLogger.On("Warn", mock.Anything).Maybe()
    mockLogger.On("Error", mock.Anything).Maybe()


    mockStepCtx.On("GetLogger").Return(mockLogger)
    mockStepCtx.On("GetRunner").Return(mockRunner)
    mockStepCtx.On("GetConnectorForHost", mockHost).Return(mockConnector, nil)
    mockStepCtx.On("GoContext").Return(context.Background())

    s := NewDisableSelinuxStep("", true).(*DisableSelinuxStep)
    s.originalSelinuxValue = "enforcing" // Original state was enforcing
    s.fstabBackupPath = selinuxConfigFile + ".bak" // Simulate backup was made with this name

    // Mock Exists for backup file -> true
    mockRunner.On("Exists", mock.Anything, mockConnector, s.fstabBackupPath).Return(true, nil).Once()
    // Mock mv command to restore backup
    restoreCmd := fmt.Sprintf("mv %s %s", s.fstabBackupPath, selinuxConfigFile)
    mockRunner.On("RunWithOptions", mock.Anything, mockConnector, restoreCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte{}, []byte{}, nil).Once()
    // Mock setenforce 1
    mockRunner.On("RunWithOptions", mock.Anything, mockConnector, "setenforce 1", mock.AnythingOfType("*connector.ExecOptions")).Return([]byte{}, []byte{}, nil).Once()


    err := s.Rollback(mockStepCtx, mockHost)
    assert.NoError(t, err)
    mockRunner.AssertExpectations(t)
}


[end of pkg/step/os/disable_selinux_step_test.go]
