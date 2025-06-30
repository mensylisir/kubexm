package connector

import (
	"context"
	// "crypto/rand" // Marked as unused by compiler
	// "crypto/rsa"   // Marked as unused by compiler
	"errors"
	"fmt"
	// "golang.org/x/crypto/ssh" // Also marked as unused by compiler
	// "net"          // Marked as unused by compiler
	"strings"
	// "sync"         // Marked as unused by compiler
	"testing"
	"time"
	// Removed redundant import of "golang.org/x/crypto/ssh"
)

// --- Test Cases ---
// TestConnectionPool_GetPut_BasicReuse_Mocked and other _Mocked tests from the fixed version.
// TestConnectionPool_NewPoolDefaults is also kept as it doesn't rely on complex mocks.

func TestConnectionPool_GetPut_BasicReuse_Mocked(t *testing.T) {
	resetActualDialCount()
	serverAddr, cleanup := createMockSSHServer(t)
	defer cleanup()

	mockDialer := newMockDialer(serverAddr)
	restore := SetTestDialerOverride(mockDialer)
	defer restore()

	pool := NewConnectionPool(DefaultPoolConfig())
	defer pool.Shutdown()
	cfg := ConnectionCfg{Host: "mockhost", User: "mockuser", Port: 22}

	mc1, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("First Get failed: %v", err)
	}
	if mc1 == nil || mc1.Client() == nil {
		t.Fatal("First Get returned a nil ManagedConnection or client")
	}
	client1 := mc1.Client() // Keep using client1 for actual SSH operations
	if getActualDialCount() != 1 {
		t.Errorf("Expected 1 dial for the first Get, but got %d", getActualDialCount())
	}

	isHealthy := false
	var healthCheckErr error
	if _, _, errS := client1.SendRequest("keepalive@openssh.com", true, nil); errS == nil {
		isHealthy = true
	} else {
		healthCheckErr = errS // Store first error
		session, sessionErr := client1.NewSession()
		if sessionErr == nil {
			session.Close()
			isHealthy = true
			healthCheckErr = nil // Clear error if NewSession succeeded
		} else {
			healthCheckErr = fmt.Errorf("SendRequest error: %v, NewSession error: %v", errS, sessionErr)
		}
	}
	if !isHealthy {
		t.Fatalf("Mocked client should be healthy (passed SendRequest or NewSession). Last error: %v", healthCheckErr)
	}

	pool.Put(mc1, true) // Pass ManagedConnection and health status

	mc2, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}
	if mc2 == nil || mc2.Client() == nil {
		t.Fatal("Second Get returned a nil ManagedConnection or client")
	}
	client2 := mc2.Client() // Keep using client2 for actual SSH operations
	if getActualDialCount() != 1 { // Dial count should still be 1
		t.Errorf("Expected dial count to remain 1 after reuse, but got %d", getActualDialCount())
	}

	if client1 != client2 { // Comparing the underlying *ssh.Client instances
		t.Errorf("Expected to get the same client instance back from the pool (%p), but got a different one (%p).", client1, client2)
	}
	// It's also good to check if mc1 and mc2 are the same ManagedConnection instance
	if mc1 != mc2 {
		t.Errorf("Expected to get the same ManagedConnection instance back, mc1: %p vs mc2: %p", mc1, mc2)
	}
}

func TestConnectionPool_MaxPerKeyLimit_Mocked(t *testing.T) {
	resetActualDialCount()
	serverAddr, cleanup := createMockSSHServer(t)
	defer cleanup()

	mockDialer := newMockDialer(serverAddr)
	restore := SetTestDialerOverride(mockDialer)
	defer restore()

	poolCfg := DefaultPoolConfig()
	poolCfg.MaxPerKey = 1
	pool := NewConnectionPool(poolCfg)
	defer pool.Shutdown()
	cfg := ConnectionCfg{Host: "mockhost", User: "mockuser", Port: 22}

	mc1, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("First Get failed: %v", err)
	}
	if mc1 == nil || mc1.Client() == nil {
		t.Fatal("First Get returned nil ManagedConnection or client")
	}

	if getActualDialCount() != 1 {
		t.Fatalf("Expected 1 dial, got %d", getActualDialCount())
	}

	// Attempt to Get another - should fail due to MaxPerKey=1
	_, err = pool.Get(context.Background(), cfg)
	if err == nil {
		t.Fatal("Second Get should have failed due to MaxPerKey limit, but it succeeded.")
	}
	if !errors.Is(err, ErrPoolExhausted) {
		t.Errorf("Expected error to be ErrPoolExhausted, but got %v", err)
	}
	if getActualDialCount() != 1 { // Dial count should still be 1, as pool was exhausted
		t.Errorf("Dial count should not increase when pool is exhausted. Expected 1, got %d", getActualDialCount())
	}

	// Put the first client back
	pool.Put(mc1, true) // mc1 must be healthy to be put back for reuse

	// Get again - should succeed by reusing the one just Put
	mc2, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Third Get failed after Put: %v", err)
	}
	if mc2 == nil || mc2.Client() == nil {
		t.Fatal("Third Get returned nil ManagedConnection or client")
	}
	if getActualDialCount() != 1 { // Dial count should still be 1 (reuse)
		t.Errorf("Expected 1 dial for third Get (reuse), got %d", getActualDialCount())
	}
	if mc1.Client() != mc2.Client() { // Check underlying client
		t.Errorf("Expected to get the same client instance back, but they were different.")
	}
	if mc1 != mc2 { // Check ManagedConnection instance
		t.Errorf("Expected to get the same ManagedConnection instance back, mc1: %p vs mc2: %p", mc1, mc2)
	}
}

func TestConnectionPool_NewPoolDefaults(t *testing.T) {
	cfg := PoolConfig{}
	pool := NewConnectionPool(cfg)
	defer pool.Shutdown()

	defaults := DefaultPoolConfig()

	if pool.config.ConnectTimeout != defaults.ConnectTimeout {
		t.Errorf("Expected ConnectTimeout %v, got %v", defaults.ConnectTimeout, pool.config.ConnectTimeout)
	}
	// ... (other default checks as previously) ...
	if pool.config.MaxPerKey != defaults.MaxPerKey {
		t.Errorf("Expected MaxPerKey %d, got %d", defaults.MaxPerKey, pool.config.MaxPerKey)
	}
	if pool.config.MaxIdlePerKey != defaults.MaxIdlePerKey {
		t.Errorf("Expected MaxIdlePerKey %d, got %d", defaults.MaxIdlePerKey, pool.config.MaxIdlePerKey)
	}
    if pool.config.IdleTimeout != defaults.IdleTimeout {
		t.Errorf("Expected IdleTimeout %v, got %v", defaults.IdleTimeout, pool.config.IdleTimeout)
	}


	customCfg := PoolConfig{ConnectTimeout: 5 * time.Second, MaxPerKey: 2}
	pool = NewConnectionPool(customCfg)
	defer pool.Shutdown()
	if pool.config.ConnectTimeout != 5*time.Second {
		t.Errorf("Expected custom ConnectTimeout 5s, got %v", pool.config.ConnectTimeout)
	}
	if pool.config.MaxPerKey != 2 {
		t.Errorf("Expected custom MaxPerKey 2, got %d", pool.config.MaxPerKey)
	}
    if pool.config.MaxIdlePerKey != defaults.MaxIdlePerKey {
		t.Errorf("Expected MaxIdlePerKey %d for custom config, got %d", defaults.MaxIdlePerKey, pool.config.MaxIdlePerKey)
	}
}

func TestConnectionPool_IdleTimeout_Mocked(t *testing.T) {
	resetActualDialCount()
	serverAddr, serverCleanup := createMockSSHServer(t)
	defer serverCleanup()

	mockDialer := newMockDialer(serverAddr)
	restoreDialer := SetTestDialerOverride(mockDialer)
	defer restoreDialer()

	poolCfg := DefaultPoolConfig()
	poolCfg.IdleTimeout = 50 * time.Millisecond
	poolCfg.MaxPerKey = 2
	pool := NewConnectionPool(poolCfg)
	defer pool.Shutdown()

	cfg := ConnectionCfg{Host: "mockhost", User: "mockuser", Port: 22}

	mc1, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Get for IdleTimeout test failed: %v", err)
	}
	if mc1 == nil || mc1.Client() == nil {
		t.Fatal("mc1 or its client is nil")
	}
	client1 := mc1.Client() // Keep a reference to the underlying client for checks
	pool.Put(mc1, true) // Pass ManagedConnection
	if getActualDialCount() != 1 {
		t.Fatalf("Expected 1 dial for initial Get, got %d", getActualDialCount())
	}

	// Wait longer than IdleTimeout but less than typical network timeouts
	// Increased from 100ms to 150ms to give more buffer if the mock server or scheduler is slow.
	// The IdleTimeout is 50ms.
	time.Sleep(poolCfg.IdleTimeout + 100*time.Millisecond)


	mc2, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Get after IdleTimeout failed: %v", err)
	}
	if mc2 == nil || mc2.Client() == nil {
		t.Fatal("mc2 or its client is nil after idle timeout")
	}
	if getActualDialCount() != 2 {
		t.Errorf("Expected 2 dials (one stale, one new), got %d", getActualDialCount())
	}
	if mc1.Client() == mc2.Client() {
		t.Errorf("Expected a new client after idle timeout, but got the same client instance.")
	}


	// Check if the original client (client1) is actually closed.
	// SendRequest is a good way to check this.
	_, _, err = client1.SendRequest("keepalive@openssh.com", true, nil)
	expectedClosedErr := false
	if err != nil {
		// Check for common error messages indicating a closed connection
		if strings.Contains(strings.ToLower(err.Error()), "eof") ||
			strings.Contains(strings.ToLower(err.Error()), "closed") ||
			strings.Contains(strings.ToLower(err.Error()), "broken pipe") {
			expectedClosedErr = true
		}
	}

	if !expectedClosedErr {
		// If SendRequest didn't error as expected, try NewSession as a secondary check.
		// This is less direct for checking "closed" but can sometimes reveal issues.
		session, sessionErr := client1.NewSession()
		if sessionErr != nil {
			if strings.Contains(strings.ToLower(sessionErr.Error()), "eof") ||
				strings.Contains(strings.ToLower(sessionErr.Error()), "closed") ||
				strings.Contains(strings.ToLower(sessionErr.Error()), "broken pipe") {
				expectedClosedErr = true
			}
		} else {
			// If NewSession succeeded, the client is definitely not closed.
			session.Close() // Clean up the session
			t.Errorf("Expected client1 to be closed due to idle timeout, but SendRequest did not fail as expected (last err: %v) and NewSession succeeded.", err)
		}
	}

	if !expectedClosedErr {
		t.Errorf("Expected client1 to be closed (SendRequest or NewSession should fail with EOF/closed/broken pipe), last error from SendRequest: %v", err)
	}

	pool.Put(mc2, true) // Pass ManagedConnection
}

func TestConnectionPool_Shutdown_Mocked(t *testing.T) {
	resetActualDialCount()
	serverAddr, serverCleanup := createMockSSHServer(t)
	defer serverCleanup()

	mockDialer := newMockDialer(serverAddr)
	restoreDialer := SetTestDialerOverride(mockDialer)
	defer restoreDialer()

	pool := NewConnectionPool(DefaultPoolConfig())

	cfgA := ConnectionCfg{Host: "mockhostA", User: "mockuserA", Port: 22}

	mc1, err := pool.Get(context.Background(), cfgA)
	if err != nil {
		t.Fatalf("Setup Get for mc1 failed: %v", err)
	}
	if mc1 == nil || mc1.Client() == nil {
		t.Fatal("mc1 or its client is nil in shutdown test setup")
	}
	client1 := mc1.Client() // Keep ref to underlying client for checks
	pool.Put(mc1, true)    // Use new Put signature

	initialDialCount := getActualDialCount()
	if initialDialCount == 0 && err == nil { // Should be 1 if Get succeeded
		t.Fatalf("Expected at least 1 dial for setup, got %d", initialDialCount)
	}

	pool.Shutdown() // Call shutdown being tested

	// Check if the original client (client1 from mc1) is actually closed.
	_, _, err = client1.SendRequest("keepalive@openssh.com", true, nil)
	expectedClosedErr := false
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "eof") ||
			strings.Contains(strings.ToLower(err.Error()), "closed") ||
			strings.Contains(strings.ToLower(err.Error()), "broken pipe") {
			expectedClosedErr = true
		}
	}

	if !expectedClosedErr {
		session, sessionErr := client1.NewSession()
		if sessionErr != nil {
			if strings.Contains(strings.ToLower(sessionErr.Error()), "eof") ||
				strings.Contains(strings.ToLower(sessionErr.Error()), "closed") ||
				strings.Contains(strings.ToLower(sessionErr.Error()), "broken pipe") {
				expectedClosedErr = true
			}
		} else {
			session.Close()
			t.Errorf("Client1 should be closed after pool Shutdown, but SendRequest did not fail as expected (last err: %v) and NewSession succeeded.", err)
		}
	}
	if !expectedClosedErr {
		t.Errorf("Client1 after Shutdown: expected 'EOF' or 'closed' or 'broken pipe' from SendRequest/NewSession, last error from SendRequest: %v", err)
	}

	// Get a new connection after shutdown. Pool should re-initialize or create new.
	// The old pool instance is shut down, but the 'pool' variable still points to it.
	// A new Get should ideally still work by creating a new connection if the pool structure allows it,
	// or the test should reflect that the pool is unusable.
	// Current Shutdown clears internal maps. A Get should make a new one.
	mc2, err := pool.Get(context.Background(), cfgA)
	if err != nil {
		t.Fatalf("Get for cfgA after Shutdown failed: %v", err)
	}
	if mc2 == nil || mc2.Client() == nil {
		t.Fatal("mc2 or its client is nil after shutdown and Get")
	}
	// Defer Put for mc2. Since we called Shutdown, this mc2 is part of a "new" internal pool structure.
	defer pool.Put(mc2, true)
	// Note: Calling pool.Shutdown() again in defer might be redundant if the test expects the pool to be mostly inert after the first Shutdown.
	// However, for cleanup, it's fine.

	if getActualDialCount() != initialDialCount+1 {
		t.Errorf("Expected %d total dials (new dial after shutdown), got %d", initialDialCount+1, getActualDialCount())
	}
}

/*
// Original tests that relied on getRealSSHConfig and environment variables are commented out.
// They can be enabled for integration testing if desired, but the mocked tests above
// cover the pool's logic more reliably in a unit test context.
// func getRealSSHConfig(t *testing.T) (ConnectionCfg, bool) { ... }
// func TestConnectionPool_GetPut_BasicReuse(t *testing.T) { ... }
// func TestConnectionPool_MaxPerKeyLimit(t *testing.T) { ... }

// The following were adapted tests, ensure their original counterparts (if any) are handled or removed.
// func TestConnectionPool_IdleTimeout(t *testing.T) { ... }
// func TestConnectionPool_Shutdown(t *testing.T) { ... }
*/
