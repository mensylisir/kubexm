package connector

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	// "os" // Not directly used in the new pool.go logic itself, but good for generatePoolKey if it read files.

	"golang.org/x/crypto/ssh"
)

// ErrPoolExhausted is returned when Get is called and the pool has reached its capacity for a key.
var ErrPoolExhausted = errors.New("connection pool exhausted for key")

// ManagedConnection wraps an ssh.Client and its potential bastion client, making it a single manageable unit.
type ManagedConnection struct {
	client        *ssh.Client
	bastionClient *ssh.Client // Associated bastion client, if any.
	poolKey       string      // The key of the pool this connection belongs to.
	lastUsed      time.Time   // Timestamp of when the connection was last returned to the pool.
	createdAt     time.Time   // Timestamp of when the connection was created.
}

// PoolKey returns the pool key associated with this managed connection.
func (mc *ManagedConnection) PoolKey() string {
	return mc.poolKey
}

// Close closes both the target client and its bastion client.
func (mc *ManagedConnection) Close() {
	if mc.client != nil {
		mc.client.Close()
	}
	if mc.bastionClient != nil {
		mc.bastionClient.Close()
	}
}

// IsHealthy performs a lightweight check on the connection's health.
func (mc *ManagedConnection) IsHealthy() bool {
	if mc.client == nil {
		return false
	}
	// SendRequest is a low-overhead way to check if the connection is alive.
	_, _, err := mc.client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

// PoolConfig holds configuration for the ConnectionPool.
type PoolConfig struct {
	MaxPerKey           int           // Maximum number of connections (active + idle) per key.
	MaxIdlePerKey       int           // Maximum number of idle connections per key.
	MaxConnectionAge    time.Duration // Maximum age of any connection.
	IdleTimeout         time.Duration // Maximum time an idle connection can stay in the pool.
	HealthCheckInterval time.Duration // How often the background scrubber runs.
	ConnectTimeout      time.Duration // Timeout for establishing new SSH connections.
}

// DefaultPoolConfig returns a PoolConfig with sensible defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxPerKey:           10,
		MaxIdlePerKey:       5,
		MaxConnectionAge:    1 * time.Hour,
		IdleTimeout:         10 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		ConnectTimeout:      15 * time.Second,
	}
}

// hostConnectionPool holds idle connections and tracks active count for a specific host config.
type hostConnectionPool struct {
	sync.Mutex
	idle      []*ManagedConnection
	numActive int // Total connections associated with this key (idle + in-use)
}

// ConnectionPool manages pools of SSH connections.
type ConnectionPool struct {
	pools  map[string]*hostConnectionPool
	config PoolConfig
	mu     sync.RWMutex
	stopCh chan struct{} // Channel to signal the scrubber to stop.
	wg     sync.WaitGroup
}

// currentDialer is a package-level variable holding the function used to dial SSH connections.
// It defaults to the real dialSSH function but can be overridden for testing.
var currentDialer dialSSHFunc = dialSSH


// NewConnectionPool initializes a new ConnectionPool and starts its background scrubber.
func NewConnectionPool(config PoolConfig) *ConnectionPool {
	// Apply defaults for zero-value fields
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = DefaultPoolConfig().ConnectTimeout
	}
	if config.MaxPerKey == 0 {
		config.MaxPerKey = DefaultPoolConfig().MaxPerKey
	}
	if config.MaxIdlePerKey == 0 {
		config.MaxIdlePerKey = DefaultPoolConfig().MaxIdlePerKey
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = DefaultPoolConfig().IdleTimeout
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = DefaultPoolConfig().HealthCheckInterval
	}
	// MaxConnectionAge can be 0 for no limit.

	cp := &ConnectionPool{
		pools:  make(map[string]*hostConnectionPool),
		config: config,
		stopCh: make(chan struct{}),
	}

	if config.HealthCheckInterval > 0 {
		cp.wg.Add(1)
		go cp.scrubber()
	}

	return cp
}

// generatePoolKey creates a unique, sorted key for a given connection configuration.
// This function needs to be in this file or accessible if it's used by the pool.
// The user-provided code for pool.go includes this function.
func generatePoolKey(cfg ConnectionCfg) string {
	var keyParts []string
	keyParts = append(keyParts, fmt.Sprintf("%s@%s:%d", cfg.User, cfg.Host, cfg.Port))

	if len(cfg.PrivateKey) > 0 {
		h := sha256.New()
		h.Write(cfg.PrivateKey)
		keyParts = append(keyParts, fmt.Sprintf("pksha256:%x", h.Sum(nil)))
	} else if cfg.PrivateKeyPath != "" {
		keyParts = append(keyParts, "pkpath:"+cfg.PrivateKeyPath)
	}
	if cfg.Password != "" {
		keyParts = append(keyParts, "pwd:true")
	}

	if cfg.BastionCfg != nil {
		// Create a ConnectionCfg for the bastion to generate its part of the key.
		// This ensures all relevant bastion details contribute to the key.
		bastionConnCfg := ConnectionCfg{
			Host:           cfg.BastionCfg.Host,
			Port:           cfg.BastionCfg.Port,
			User:           cfg.BastionCfg.User,
			Password:       cfg.BastionCfg.Password, // Consider if bastion password should be part of key
			PrivateKeyPath: cfg.BastionCfg.PrivateKeyPath, // Or hash of private key
			// Timeout and HostKeyCallback are usually not part of the identity for pooling key itself.
		}
		keyParts = append(keyParts, "bastion:"+generatePoolKey(bastionConnCfg)) // Recursive call
	}
	sort.Strings(keyParts)
	return strings.Join(keyParts, "|")
}


// getOrCreateHostPool retrieves or creates a host-specific pool safely.
func (cp *ConnectionPool) getOrCreateHostPool(poolKey string) *hostConnectionPool {
	cp.mu.RLock()
	hcp, ok := cp.pools[poolKey]
	cp.mu.RUnlock()
	if ok {
		return hcp
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()
	// Double-check in case it was created between RUnlock and Lock
	hcp, ok = cp.pools[poolKey]
	if !ok {
		hcp = &hostConnectionPool{}
		cp.pools[poolKey] = hcp
	}
	return hcp
}

// Get retrieves an active connection from the pool or creates a new one.
// It now returns a *ManagedConnection.
func (cp *ConnectionPool) Get(ctx context.Context, cfg ConnectionCfg) (*ManagedConnection, error) {
	poolKey := generatePoolKey(cfg)
	hcp := cp.getOrCreateHostPool(poolKey)

	hcp.Lock()

	// Try to get a healthy idle connection
	for len(hcp.idle) > 0 {
		mc := hcp.idle[0]
		hcp.idle = hcp.idle[1:] // Dequeue

		stale := false
		if cp.config.IdleTimeout > 0 && mc.lastUsed.Add(cp.config.IdleTimeout).Before(time.Now()) {
			stale = true
		}
		if !stale && cp.config.MaxConnectionAge > 0 && mc.createdAt.Add(cp.config.MaxConnectionAge).Before(time.Now()) {
			stale = true
		}

		if stale {
			mc.Close()
			hcp.numActive--
			continue
		}

		if mc.IsHealthy() {
			mc.lastUsed = time.Now()
			hcp.Unlock()
			return mc, nil
		}
		mc.Close()
		hcp.numActive--
	}

	if hcp.numActive >= cp.config.MaxPerKey {
		hcp.Unlock()
		return nil, fmt.Errorf("%w: %s (max %d reached, active %d)", ErrPoolExhausted, poolKey, cp.config.MaxPerKey, hcp.numActive)
	}

	hcp.numActive++
	hcp.Unlock()

	targetClient, bastionClient, err := currentDialer(ctx, cfg, cp.config.ConnectTimeout)
	if err != nil {
		hcp.Lock()
		hcp.numActive--
		hcp.Unlock()
		return nil, err
	}

	// Create ManagedConnection for the new connection.
	mc := &ManagedConnection{
		client:        targetClient,
		bastionClient: bastionClient,
		poolKey:       poolKey, // Set the poolKey
		lastUsed:      time.Now(), // lastUsed will be updated when Put back
		createdAt:     time.Now(), // This is the actual creation time
	}
	// Note: This new mc is not added to hcp.idle here. It's returned to the caller.
	// It will be added to idle when/if Put is called.
	return mc, nil
}

// Put returns a ManagedConnection to the pool.
func (cp *ConnectionPool) Put(mc *ManagedConnection, isHealthy bool) {
	if mc == nil || mc.client == nil {
		// If mc is nil or its client is nil, there's nothing to pool or close explicitly here.
		// If mc exists but mc.client is nil, it implies it was likely already closed or invalid.
		// If the intention was to decrement numActive for a connection that was taken from pool
		// but then found to be unusable before even trying to "Put" it back healthy,
		// that decrement should happen in Get or the calling code should use CloseConnection.
		return
	}

	// mc.poolKey should have been set by Get. If not, something is wrong.
	if mc.poolKey == "" {
		// This ManagedConnection doesn't know which pool it belongs to.
		// This indicates it wasn't properly obtained from this pool's Get method.
		// The safest action is to just close it to prevent leaks.
		// We cannot correctly adjust hcp.numActive without the key.
		mc.Close()
		// Consider logging this anomaly.
		// fmt.Fprintf(os.Stderr, "ConnectionPool: Put called with ManagedConnection lacking a poolKey. Connection closed.\n")
		return
	}

	hcp := cp.getOrCreateHostPool(mc.poolKey)

	hcp.Lock()
	defer hcp.Unlock()

	if !isHealthy || len(hcp.idle) >= cp.config.MaxIdlePerKey {
		mc.Close()
		hcp.numActive--
		if hcp.numActive < 0 { hcp.numActive = 0 } // Ensure numActive doesn't go negative
		return
	}

	mc.lastUsed = time.Now()
	hcp.idle = append(hcp.idle, mc) // Add to the end (LIFO behavior for Get)
}

// CloseConnection is called when SSHConnector knows a connection (potentially from the pool)
// is being definitively closed and should not be put back.
// This method primarily adjusts pool accounting. The actual closing of clients
// is handled by ManagedConnection.Close() or directly by SSHConnector if not from pool.
func (cp *ConnectionPool) CloseConnection(cfg ConnectionCfg, client *ssh.Client) {
	// This method is a bit tricky. If the client was from the pool and is now being
	// told to be closed (e.g. SSHConnector.Close called on a pooled connection that
	// was found unhealthy by IsConnected, or user explicitly closing),
	// the pool needs to know it's no longer active or idle.
	if client == nil {
		return
	}
	poolKey := generatePoolKey(cfg)
	hcp := cp.getOrCreateHostPool(poolKey)

	hcp.Lock()
	defer hcp.Unlock()

	// Try to find it in idle first (if it was Put back then found to be bad by scrubber, unlikely path here)
	// foundInIdle was declared but not used; removing it.
	for i, mc := range hcp.idle {
		if mc.client == client {
			hcp.idle = append(hcp.idle[:i], hcp.idle[i+1:]...)
			// mc.Close() is implicitly handled as this connection is being discarded
			break
		}
	}
	// Whether found in idle or was in-use, it's now gone, so decrement numActive.
	hcp.numActive--
	if hcp.numActive < 0 { // Safety
		hcp.numActive = 0
	}
}


// scrubber runs in the background to clean up stale idle connections.
func (cp *ConnectionPool) scrubber() {
	defer cp.wg.Done()
	ticker := time.NewTicker(cp.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.mu.RLock() // Lock for reading the pools map
			for _, hcp := range cp.pools {
				hcp.Lock() // Lock for modifying specific host pool
				var freshIdle []*ManagedConnection
				cleanedCount := 0
				for _, mc := range hcp.idle {
					stale := false
					if cp.config.IdleTimeout > 0 && mc.lastUsed.Add(cp.config.IdleTimeout).Before(time.Now()) {
						stale = true
					}
					if !stale && cp.config.MaxConnectionAge > 0 && mc.createdAt.Add(cp.config.MaxConnectionAge).Before(time.Now()) {
						stale = true
					}

					if stale {
						mc.Close()
						cleanedCount++
					} else {
						freshIdle = append(freshIdle, mc)
					}
				}
				hcp.idle = freshIdle
				hcp.numActive -= cleanedCount // Adjust numActive for cleaned connections
				if hcp.numActive < 0 { hcp.numActive = 0 } // Safety
				hcp.Unlock()
			}
			cp.mu.RUnlock()
		case <-cp.stopCh:
			return
		}
	}
}

// Shutdown closes all connections in the pool and stops the scrubber.
func (cp *ConnectionPool) Shutdown() {
	if cp.stopCh != nil {
		close(cp.stopCh)
	}
	cp.wg.Wait() // Wait for scrubber to finish if it was started

	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, hcp := range cp.pools {
		hcp.Lock()
		for _, mc := range hcp.idle {
			mc.Close()
		}
		hcp.idle = nil      // Clear idle list
		hcp.numActive = 0 // Reset active count, assuming in-use connections will be closed by users
		hcp.Unlock()
	}
	cp.pools = make(map[string]*hostConnectionPool) // Clear the pools map
}
