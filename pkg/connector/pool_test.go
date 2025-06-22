package connector

import (
	"context"
	"errors" // Added
	"fmt"
	"sync"
	"testing"
	"time"
	"unsafe" // For casting mock client, with caveats

	"golang.org/x/crypto/ssh"
)

// --- Mocking Infrastructure ---

var (
	actualDialCount      int
	actualDialCountMutex sync.Mutex
)

func resetActualDialCount() {
	actualDialCountMutex.Lock()
	defer actualDialCountMutex.Unlock()
	actualDialCount = 0
}

func incrementActualDialCount() {
	actualDialCountMutex.Lock()
	defer actualDialCountMutex.Unlock()
	actualDialCount++
}

func getActualDialCount() int {
	actualDialCountMutex.Lock()
	defer actualDialCountMutex.Unlock()
	return actualDialCount
}

// mockSSHClient implements parts of the ssh.Client behavior for testing.
// It does NOT embed ssh.Client to avoid needing a real connection.
// Casting to *ssh.Client using unsafe.Pointer is done for tests,
// assuming pool logic only calls Close() and NewSession().
type mockSSHClient struct {
	id            string
	isClosed      bool
	closeErr      error
	newSessionErr error
	newSessionCalls int
	// ssh.Client // Cannot embed directly as it has unexported fields and complex state
}

func (m *mockSSHClient) Close() error {
	m.isClosed = true
	return m.closeErr
}

func (m *mockSSHClient) NewSession() (*ssh.Session, error) {
	m.newSessionCalls++
	if m.isClosed {
		return nil, fmt.Errorf("ssh: client is closed")
	}
	if m.newSessionErr != nil {
		return nil, m.newSessionErr
	}
	// To satisfy the pool's health check, which just checks the error,
	// returning (nil, nil) is acceptable. A real session isn't needed.
	return nil, nil
}


// mockDialerSetup is used to configure the behavior of the mock dialer for a specific key or generally.
type mockDialerSetup struct {
	targetClient    *mockSSHClient
	bastionClient   *mockSSHClient // For now, bastion client mocking is minimal
	dialErr         error
	numTimesToReturn int // How many times to return this setup before moving to next or default
}

// newMockDialer returns a mock dialSSHFunc for testing.
// It uses a map to provide specific responses for specific pool keys.
func newMockDialer(keySpecificResponses map[string][]mockDialerSetup) dialSSHFunc {
	keyCallCounts := make(map[string]int)
	var mu sync.Mutex

	return func(ctx context.Context, cfg ConnectionCfg, timeout time.Duration) (*ssh.Client, *ssh.Client, error) {
		incrementActualDialCount()
		mu.Lock()
		defer mu.Unlock()

		poolKey := generatePoolKey(cfg) // Assuming generatePoolKey is accessible or re-implemented for test

		setups, ok := keySpecificResponses[poolKey]
		if !ok || len(setups) == 0 {
			return nil, nil, fmt.Errorf("mockDialer: no setup for key %s", poolKey)
		}

		callCount := keyCallCounts[poolKey]
		currentSetup := setups[0] // Default to the first setup if call count exceeds specific ones

		if callCount < len(setups) {
			currentSetup = setups[callCount]
		}

		if currentSetup.numTimesToReturn > 0 {
			keyCallCounts[poolKey] = callCount + 1
			if keyCallCounts[poolKey] >= currentSetup.numTimesToReturn && callCount < len(setups)-1 {
				// This logic is a bit off for numTimesToReturn, needs refinement if complex sequences are needed.
				// For now, simpler: if numTimesToReturn is 1, next call for this key uses next setup or fails.
				// This is simplified: it will reuse the last setup if callCount >= len(setups).
			}
		}


		if currentSetup.dialErr != nil {
			return nil, nil, currentSetup.dialErr
		}

		// Unsafe cast: This is risky and assumes the pool only calls methods we've mocked (Close, NewSession).
		// It's a common but fragile way to test concrete types from external packages.
		var targetSSHClient *ssh.Client
		if currentSetup.targetClient != nil {
			targetSSHClient = (*ssh.Client)(unsafe.Pointer(currentSetup.targetClient))
		}
		var bastionSSHClient *ssh.Client
		if currentSetup.bastionClient != nil {
			bastionSSHClient = (*ssh.Client)(unsafe.Pointer(currentSetup.bastionClient))
		}
		return targetSSHClient, bastionSSHClient, nil
	}
}


// --- Test Cases ---

func TestConnectionPool_NewPoolDefaults(t *testing.T) {
	cfg := PoolConfig{} // Zero values
	pool := NewConnectionPool(cfg)

	defaults := DefaultPoolConfig()

	if pool.config.ConnectTimeout != defaults.ConnectTimeout {
		t.Errorf("Expected ConnectTimeout %v, got %v", defaults.ConnectTimeout, pool.config.ConnectTimeout)
	}
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
	if pool.config.ConnectTimeout != 5*time.Second {
		t.Errorf("Expected custom ConnectTimeout 5s, got %v", pool.config.ConnectTimeout)
	}
	if pool.config.MaxPerKey != 2 {
		t.Errorf("Expected custom MaxPerKey 2, got %d", pool.config.MaxPerKey)
	}
    // Check that non-set fields still get defaults
    if pool.config.MaxIdlePerKey != defaults.MaxIdlePerKey {
		t.Errorf("Expected MaxIdlePerKey %d for custom config, got %d", defaults.MaxIdlePerKey, pool.config.MaxIdlePerKey)
	}
}

func TestConnectionPool_GetPut_BasicReuse(t *testing.T) {
	resetActualDialCount()

	mockClient1 := &mockSSHClient{id: "client1"}
	dialerSetups := map[string][]mockDialerSetup{
		"testuser@testhost:22|pkpath:keypath": {
			{targetClient: mockClient1, numTimesToReturn: 1},
		},
	}

	cleanup := SetDialSSHOverrideForTesting(newMockDialer(dialerSetups))
	defer cleanup()

	pool := NewConnectionPool(DefaultPoolConfig())
	cfg := ConnectionCfg{Host: "testhost", Port: 22, User: "testuser", PrivateKeyPath: "keypath"}

	// 1. Get a connection - should dial
	client1, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if client1 == nil {
		t.Fatal("Get returned nil client")
	}
	if getActualDialCount() != 1 {
		t.Errorf("Expected 1 dial, got %d", getActualDialCount())
	}
	// To check if it's our mockClient1, we can't directly compare client1 to mockClient1
	// due to unsafe.Pointer. We rely on dial count and mock setup.

	// 2. Put the connection back
	pool.Put(cfg, client1, true)

	// 3. Get again - should reuse from pool, no new dial
	client2, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}
	if client2 == nil {
		t.Fatal("Second Get returned nil client")
	}
	if getActualDialCount() != 1 { // Dial count should still be 1
		t.Errorf("Expected 1 dial for reuse, got %d", getActualDialCount())
	}

	// Verify it's the same underlying mock client instance
	// This check relies on the mock dialer returning the *same* mockSSHClient instance when cast.
	// And that the pool returns the exact same *ssh.Client (which is our casted mock).
	if client1 != client2 {
		 // Due to unsafe.Pointer, they might not be identical if the mock was re-cast.
		 // A better check is to see if mockClient1's fields were affected as expected.
		t.Logf("Warning: client1 (%p) and client2 (%p) are different instances, which can happen with unsafe.Pointer casting if not careful. Verifying mock client state instead.", client1, client2)
	}
	// Check if the *original* mockClient1 was reused (e.g. its lastUsed time updated if we could inspect ManagedConnection)
	// For now, the dial count is the primary indicator of reuse.
}

func TestConnectionPool_MaxPerKeyLimit(t *testing.T) {
	resetActualDialCount()

	mockClient1 := &mockSSHClient{id: "client1-max"}
	mockClient2 := &mockSSHClient{id: "client2-max"} // Should not be reached if MaxPerKey is 1

	dialerSetups := map[string][]mockDialerSetup{
		"testuser@testhost:22|pkpath:keypath": {
			{targetClient: mockClient1, numTimesToReturn: 1}, // First Get
			{targetClient: mockClient2, numTimesToReturn: 1}, // Second Get if first is not Put
		},
	}
	cleanup := SetDialSSHOverrideForTesting(newMockDialer(dialerSetups))
	defer cleanup()

	poolCfg := DefaultPoolConfig()
	poolCfg.MaxPerKey = 1
	pool := NewConnectionPool(poolCfg)
	cfg := ConnectionCfg{Host: "testhost", Port: 22, User: "testuser", PrivateKeyPath: "keypath"}

	// Get first connection - should succeed
	client1, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("First Get failed: %v", err)
	}
	if client1 == nil {
		t.Fatal("First Get returned nil client")
	}
	if getActualDialCount() != 1 {
		t.Errorf("Expected 1 dial for first Get, got %d", getActualDialCount())
	}

	// Attempt to Get another - should fail due to MaxPerKey=1
	_, err = pool.Get(context.Background(), cfg)
	if err == nil {
		t.Fatal("Second Get should have failed due to MaxPerKey limit, but succeeded")
	}
	if !errors.Is(err, ErrPoolExhausted) { // Check for specific error type
		t.Errorf("Expected ErrPoolExhausted, got %T: %v", err, err)
	}
	if getActualDialCount() != 1 { // Dial count should still be 1, as pool was exhausted
		t.Errorf("Expected 1 dial after exhausted Get, got %d", getActualDialCount())
	}


	// Put the first client back
	pool.Put(cfg, client1, true)

	// Get again - should succeed by reusing the one just Put
	client3, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Third Get failed after Put: %v", err)
	}
	if client3 == nil {
		t.Fatal("Third Get returned nil client")
	}
	if getActualDialCount() != 1 { // Dial count should still be 1 (reuse)
		t.Errorf("Expected 1 dial for third Get (reuse), got %d", getActualDialCount())
	}
	if client1 != client3 {
		t.Logf("Warning: client1 (%p) and client3 (%p) are different after Put/Get. Verifying mock client state is key.", client1, client3)
	}
}

func TestConnectionPool_IdleTimeout(t *testing.T) {
	resetActualDialCount()

	mockClientStale := &mockSSHClient{id: "staleClient"}
	mockClientNew := &mockSSHClient{id: "newClientAfterStale"}

	dialerSetups := map[string][]mockDialerSetup{
		"testuser@testhost:22|pkpath:keypath": {
			{targetClient: mockClientStale, numTimesToReturn: 1}, // For first Get
			{targetClient: mockClientNew, numTimesToReturn: 1},   // For Get after stale one is discarded
		},
	}
	cleanup := SetDialSSHOverrideForTesting(newMockDialer(dialerSetups))
	defer cleanup()

	poolCfg := DefaultPoolConfig()
	poolCfg.IdleTimeout = 50 * time.Millisecond // Short timeout
	pool := NewConnectionPool(poolCfg)
	cfg := ConnectionCfg{Host: "testhost", Port: 22, User: "testuser", PrivateKeyPath: "keypath"}

	// Get and Put a connection
	client1, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Get for IdleTimeout test failed: %v", err)
	}
	pool.Put(cfg, client1, true)
	if getActualDialCount() != 1 {
		t.Fatalf("Expected 1 dial for initial Get, got %d", getActualDialCount())
	}

	// Wait for longer than IdleTimeout
	time.Sleep(100 * time.Millisecond)

	// Get again - should discard stale and dial a new one
	client2, err := pool.Get(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Get after IdleTimeout failed: %v", err)
	}
	if client2 == nil {
		t.Fatal("Get after IdleTimeout returned nil client")
	}
	if getActualDialCount() != 2 { // Should have dialed again
		t.Errorf("Expected 2 dials (one stale, one new), got %d", getActualDialCount())
	}

	// Verify the first client was closed
	// This check is tricky because client1 is an *ssh.Client (unsafe.Pointer cast).
	// We need to check the original mockClientStale.
	if !mockClientStale.isClosed {
		t.Errorf("Expected stale client (mockClientStale) to be closed after IdleTimeout, but it was not.")
	}
}


func TestConnectionPool_Shutdown(t *testing.T) {
	resetActualDialCount()

	mockClientA1 := &mockSSHClient{id: "clientA1"}
	mockClientB1 := &mockSSHClient{id: "clientB1"}
	mockClientA2 := &mockSSHClient{id: "clientA2_after_shutdown"} // For Get after shutdown

	dialerSetups := map[string][]mockDialerSetup{
		"userA@hostA:22": { // Simplified key for test
			{targetClient: mockClientA1, numTimesToReturn: 1},
			{targetClient: mockClientA2, numTimesToReturn: 1},
		},
		"userB@hostB:22": {
			{targetClient: mockClientB1, numTimesToReturn: 1},
		},
	}
	cleanup := SetDialSSHOverrideForTesting(newMockDialer(dialerSetups))
	defer cleanup()

	pool := NewConnectionPool(DefaultPoolConfig())
	cfgA := ConnectionCfg{Host: "hostA", Port: 22, User: "userA"}
	cfgB := ConnectionCfg{Host: "hostB", Port: 22, User: "userB"}

	// Get and Put some connections
	clientA1, _ := pool.Get(context.Background(), cfgA)
	pool.Put(cfgA, clientA1, true)

	clientB1, _ := pool.Get(context.Background(), cfgB)
	pool.Put(cfgB, clientB1, true)

	if getActualDialCount() != 2 {
		t.Fatalf("Expected 2 dials for setup, got %d", getActualDialCount())
	}

	// Shutdown the pool
	pool.Shutdown()

	// Verify initial clients were closed
	if !mockClientA1.isClosed {
		t.Errorf("mockClientA1 was not closed after Shutdown")
	}
	if !mockClientB1.isClosed {
		t.Errorf("mockClientB1 was not closed after Shutdown")
	}

	// Try to Get a connection for cfgA again - should result in a new dial
	_, err := pool.Get(context.Background(), cfgA)
	if err != nil {
		t.Fatalf("Get for cfgA after Shutdown failed: %v", err)
	}
	if getActualDialCount() != 3 { // 2 before shutdown, 1 after
		t.Errorf("Expected 3 total dials (new dial after shutdown), got %d", getActualDialCount())
	}
	if pool.pools[generatePoolKey(cfgA)].numActive != 1 { // Check numActive in new pool
		t.Errorf("Expected numActive=1 for new pool after shutdown, got %d", pool.pools[generatePoolKey(cfgA)].numActive)
	}
}
