package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestReboot_Success tests the successful reboot sequence.
func TestReboot_Success(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{} // The Reboot method is on defaultRunner

	rebootCmd := "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'"
	uptimeCmd := "uptime"

	// Mock for issuing the reboot command (via r.RunWithOptions -> conn.Exec)
	// Simulate connection dropping, which is an acceptable error for the reboot command.
	mockConn.On("Exec", mock.Anything, rebootCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
		return opts.Sudo && opts.Timeout == 10*time.Second
	})).Return(nil, nil, errors.New("session channel closed")).Once()

	// Mock for uptime check: fail a few times, then succeed.
	mockConn.On("Exec", mock.Anything, uptimeCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
		return !opts.Sudo && opts.Timeout == 5*time.Second
	})).Return(nil, nil, errors.New("connection refused")).Times(2) // Fail twice

	mockConn.On("Exec", mock.Anything, uptimeCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
		return !opts.Sudo && opts.Timeout == 5*time.Second
	})).Return([]byte("uptime output"), nil, nil).Once() // Then succeed

	// Short timeout for the test itself to not wait too long for retries.
	// The polling interval in Reboot is 5s, grace period is 10s.
	// So, initial reboot cmd + grace (10s) + 2 failed polls (2*5s=10s) + 1 successful poll (5s)
	// Needs at least ~25s, but internal timeouts in Reboot() are shorter.
	// Let's set overall test timeout generously.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Generous for test
	defer cancel()

	// The timeout passed to Reboot() function itself.
	// Make it shorter than ctx timeout to test Reboot's own timeout if needed,
	// but long enough for the success path.
	rebootFunctionTimeout := 20 * time.Second // Enough for 10s grace + 2*5s polls = 20s for first success

	err := r.Reboot(ctx, mockConn, rebootFunctionTimeout)
	assert.NoError(t, err)

	mockConn.AssertExpectations(t)
}

// TestReboot_IssueCommand_Fails tests when issuing the reboot command itself fails hard.
func TestReboot_IssueCommand_Fails(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	rebootCmd := "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'"

	// Mock for issuing the reboot command to fail with a non-connection related error
	mockConn.On("Exec", mock.Anything, rebootCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, errors.New("command not found")).Once()

	err := r.Reboot(context.Background(), mockConn, 5*time.Second) // Short timeout, won't be reached
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue reboot command")
	assert.Contains(t, err.Error(), "command not found")
}

// TestReboot_TimeoutWaitingForHost tests when the host never becomes responsive.
func TestReboot_TimeoutWaitingForHost(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	rebootCmd := "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'"
	uptimeCmd := "uptime"

	// Mock for issuing the reboot command (can succeed or simulate connection drop)
	mockConn.On("Exec", mock.Anything, rebootCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, errors.New("session channel closed")).Once()

	// Mock for uptime check to always fail
	mockConn.On("Exec", mock.Anything, uptimeCmd, mock.AnythingOfType("*connector.ExecOptions")).Return(nil, nil, errors.New("connection still refused")) // Called multiple times

	// The timeout for the Reboot function itself.
	// The internal polling is 5s. Let's make the timeout allow for one poll after grace.
	// Grace period is 10s. First poll at ~15s.
	rebootFunctionTimeout := 16 * time.Second // Allows grace + one poll attempt

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // Test context slightly longer
	defer cancel()

	err := r.Reboot(ctx, mockConn, rebootFunctionTimeout)

	assert.Error(t, err)
	fmt.Println(err.Error())
	assert.True(t, strings.Contains(err.Error(), "timed out waiting for host to become responsive after reboot"), "Error message mismatch: "+err.Error())
}

func TestReboot_NilConnector(t *testing.T) {
	r := &defaultRunner{}
	err := r.Reboot(context.Background(), nil, 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector cannot be nil for Reboot")
}

// TestReboot_IssueCommand_Succeeds_NoConnectionDrop tests when reboot command exec returns nil error
func TestReboot_IssueCommand_Succeeds_NoConnectionDrop(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	rebootCmd := "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'"
	uptimeCmd := "uptime"

	// Mock for issuing the reboot command - succeeds without error (e.g. if shell doesn't close session immediately)
	mockConn.On("Exec", mock.Anything, rebootCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
		return opts.Sudo && opts.Timeout == 10*time.Second
	})).Return(nil, nil, nil).Once() // Succeeds

	// Uptime check
	mockConn.On("Exec", mock.Anything, uptimeCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool {
		return !opts.Sudo && opts.Timeout == 5*time.Second
	})).Return([]byte("uptime output"), nil, nil).Once() // Succeed on first real check

	rebootFunctionTimeout := 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	err := r.Reboot(ctx, mockConn, rebootFunctionTimeout)
	assert.NoError(t, err)
	mockConn.AssertExpectations(t)
}
