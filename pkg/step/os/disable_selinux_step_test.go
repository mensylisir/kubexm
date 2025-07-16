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
	// "github.com/mensylisir/kubexm/pkg/plan" // Not directly used in this version of the test
	"github.com/mensylisir/kubexm/pkg/runner" // For runner.Facts
	"github.com/mensylisir/kubexm/pkg/runtime"
	// "github.com/mensylisir/kubexm/pkg/step" // Not directly used
	// "github.com/mensylisir/kubexm/pkg/task"

	// loggermocks "github.com/mensylisir/kubexm/pkg/logger/mocks" // Temporarily commented out
	// stepmocks "github.com/mensylisir/kubexm/pkg/step/mocks" // Temporarily commented out
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
			name:           "getenforce fails (e.g., not installed)",
			getenforceErr:  errors.New("getenforce command failed"),
			expectedIsDone: true,
		},
		{
			name:             "SELinux disabled by getenforce, but config still enforcing",
			getenforceOutput: "Disabled",
			readFileContent:  "SELINUX=enforcing",
			expectedIsDone:   false,
		},
		{
			name:             "SELinux disabled by getenforce, config file read error",
			getenforceOutput: "Disabled",
			readFileErr:      errors.New("permission denied"),
			expectedIsDone:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := new(runnermocks.MockRunner)
			mockConnector := new(connectormocks.MockConnector)
			mockHost := new(connectormocks.MockHost)

			mockHost.On("GetName").Return("test-host")

			var getenforceCmdErr error
			if tt.getenforceErr != nil {
				if strings.Contains(tt.getenforceErr.Error(), "exit code") {
					getenforceCmdErr = &connector.CommandError{Underlying: tt.getenforceErr, ExitCode: 1, Stderr: "simulated error"}
				} else {
					getenforceCmdErr = tt.getenforceErr
				}
			}
			mockRunner.On("RunWithOptions", mock.Anything, mockConnector, "getenforce", mock.AnythingOfType("*connector.ExecOptions")).
				Return([]byte(tt.getenforceOutput), []byte(""), getenforceCmdErr).Once()

			if tt.getenforceOutput == "Disabled" {
				mockRunner.On("ReadFile", mock.Anything, mockConnector, selinuxConfigFile).
					Return([]byte(tt.readFileContent), tt.readFileErr).Once()
			}

			dummyCtx := &runtime.Context{
				GoCtx:         context.Background(),
				Runner:        mockRunner,
				ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster"}},
				hostInfoMap: map[string]*runtime.HostRuntimeInfo{
					"test-host": {Host: mockHost, Conn: mockConnector, Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}}},
				},
			}
			dummyCtx.SetCurrentHost(mockHost)

			s := NewDisableSelinuxStep("", true).(*DisableSelinuxStep)
			isDone, err := s.Precheck(dummyCtx, mockHost)

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
	mockHost := new(connectormocks.MockHost)

	mockHost.On("GetName").Return("test-host")

	dummyCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Runner:        mockRunner,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster"}},
		hostInfoMap: map[string]*runtime.HostRuntimeInfo{
			"test-host": {Host: mockHost, Conn: mockConnector, Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}}},
		},
	}
	dummyCtx.SetCurrentHost(mockHost)

	s := NewDisableSelinuxStep("TestRunDisableSelinux", true).(*DisableSelinuxStep)
	s.originalSelinuxValue = "enforcing"

	mockRunner.On("RunWithOptions", mock.Anything, mockConnector, "setenforce 0", mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
		Return([]byte(""), []byte(""), nil).Once()

	expectedSedCmd := fmt.Sprintf("sed -i.bak -e 's/^SELINUX=enforcing/SELINUX=disabled/' -e 's/^SELINUX=permissive/SELINUX=disabled/' %s", selinuxConfigFile)
	mockRunner.On("RunWithOptions", mock.Anything, mockConnector, expectedSedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
		Return([]byte(""), []byte(""), nil).Once()

	err := s.Run(dummyCtx, mockHost)
	assert.NoError(t, err)
	assert.True(t, s.rebootRequired)

	mockRunner.AssertExpectations(t)
}

func TestDisableSelinuxStep_Rollback_RestoresFromBackup(t *testing.T) {
	mockRunner := new(runnermocks.MockRunner)
	mockConnector := new(connectormocks.MockConnector)
	mockHost := new(connectormocks.MockHost)

	mockHost.On("GetName").Return("test-host")

	dummyCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Runner:        mockRunner,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster"}},
		hostInfoMap: map[string]*runtime.HostRuntimeInfo{
			"test-host": {Host: mockHost, Conn: mockConnector, Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}}},
		},
	}
	dummyCtx.SetCurrentHost(mockHost)

	s := NewDisableSelinuxStep("", true).(*DisableSelinuxStep)
	s.originalSelinuxValue = "enforcing"
	s.fstabBackupPath = selinuxConfigFile + ".bak"

	mockRunner.On("Exists", mock.Anything, mockConnector, s.fstabBackupPath).Return(true, nil).Once()
	restoreCmd := fmt.Sprintf("mv %s %s", s.fstabBackupPath, selinuxConfigFile)
	mockRunner.On("RunWithOptions", mock.Anything, mockConnector, restoreCmd, mock.AnythingOfType("*connector.ExecOptions")).Return([]byte{}, []byte{}, nil).Once()
	mockRunner.On("RunWithOptions", mock.Anything, mockConnector, "setenforce 1", mock.AnythingOfType("*connector.ExecOptions")).Return([]byte{}, []byte{}, nil).Once()

	err := s.Rollback(dummyCtx, mockHost)
	assert.NoError(t, err)
	mockRunner.AssertExpectations(t)
}
