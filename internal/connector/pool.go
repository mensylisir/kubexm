package connector

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrPoolExhausted = errors.New("connection pool exhausted for key")

type ManagedConnection struct {
	client        *ssh.Client
	bastionClient *ssh.Client
	poolKey       string
	lastUsed      time.Time
	createdAt     time.Time
}

func (mc *ManagedConnection) Client() *ssh.Client {
	return mc.client
}

func (mc *ManagedConnection) PoolKey() string {
	return mc.poolKey
}

func (mc *ManagedConnection) Close() {
	if mc.client != nil {
		mc.client.Close()
	}
	if mc.bastionClient != nil {
		mc.bastionClient.Close()
	}
}

func (mc *ManagedConnection) IsHealthy() bool {
	if mc.client == nil {
		return false
	}
	_, _, err := mc.client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}

type PoolConfig struct {
	MaxPerKey           int
	MaxIdlePerKey       int
	MaxConnectionAge    time.Duration
	IdleTimeout         time.Duration
	HealthCheckInterval time.Duration
	ConnectTimeout      time.Duration
}

func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxPerKey:           10,
		MaxIdlePerKey:       5,
		MaxConnectionAge:    1 * time.Hour,
		IdleTimeout:         10 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		ConnectTimeout:      15 * time.Second,
	}
}

type hostConnectionPool struct {
	sync.Mutex
	idle      []*ManagedConnection
	numActive int
}

type ConnectionPool struct {
	pools  map[string]*hostConnectionPool
	config PoolConfig
	mu     sync.RWMutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

var currentDialer dialSSHFunc = dialSSH

func NewConnectionPool(config *PoolConfig) *ConnectionPool {
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

	cp := &ConnectionPool{
		pools:  make(map[string]*hostConnectionPool),
		config: *config,
		stopCh: make(chan struct{}),
	}

	if config.HealthCheckInterval > 0 {
		cp.wg.Add(1)
		go cp.scrubber()
	}

	return cp
}

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
		bastionConnCfg := ConnectionCfg{
			Host:           cfg.BastionCfg.Host,
			Port:           cfg.BastionCfg.Port,
			User:           cfg.BastionCfg.User,
			Password:       cfg.BastionCfg.Password,
			PrivateKeyPath: cfg.BastionCfg.PrivateKeyPath,
		}
		keyParts = append(keyParts, "bastion:"+generatePoolKey(bastionConnCfg))
	}
	sort.Strings(keyParts)
	return strings.Join(keyParts, "|")
}

func (cp *ConnectionPool) getOrCreateHostPool(poolKey string) *hostConnectionPool {
	cp.mu.RLock()
	hcp, ok := cp.pools[poolKey]
	cp.mu.RUnlock()
	if ok {
		return hcp
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()
	hcp, ok = cp.pools[poolKey]
	if !ok {
		hcp = &hostConnectionPool{}
		cp.pools[poolKey] = hcp
	}
	return hcp
}

func (cp *ConnectionPool) Get(ctx context.Context, cfg ConnectionCfg) (*ManagedConnection, error) {
	poolKey := generatePoolKey(cfg)
	hcp := cp.getOrCreateHostPool(poolKey)

	hcp.Lock()

	for len(hcp.idle) > 0 {
		mc := hcp.idle[0]
		hcp.idle = hcp.idle[1:]

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

	mc := &ManagedConnection{
		client:        targetClient,
		bastionClient: bastionClient,
		poolKey:       poolKey,
		lastUsed:      time.Now(),
		createdAt:     time.Now(),
	}

	return mc, nil
}

func (cp *ConnectionPool) Put(mc *ManagedConnection, isHealthy bool) {
	if mc == nil || mc.client == nil {
		// If mc is nil or its client is nil, there's nothing to pool or close explicitly here.
		// If mc exists but mc.client is nil, it implies it was likely already closed or invalid.
		// If the intention was to decrement numActive for a connection that was taken from pool
		// but then found to be unusable before even trying to "Put" it back healthy,
		// that decrement should happen in Get or the calling code should use CloseConnection.
		return
	}

	if mc.poolKey == "" {
		mc.Close()
		return
	}

	hcp := cp.getOrCreateHostPool(mc.poolKey)

	hcp.Lock()
	defer hcp.Unlock()

	if !isHealthy || len(hcp.idle) >= cp.config.MaxIdlePerKey {
		mc.Close()
		hcp.numActive--
		if hcp.numActive < 0 {
			hcp.numActive = 0
		}
		return
	}

	mc.lastUsed = time.Now()
	hcp.idle = append(hcp.idle, mc)
}

func (cp *ConnectionPool) CloseConnection(cfg ConnectionCfg, client *ssh.Client) {
	if client == nil {
		return
	}
	poolKey := generatePoolKey(cfg)
	hcp := cp.getOrCreateHostPool(poolKey)

	hcp.Lock()
	defer hcp.Unlock()

	for i, mc := range hcp.idle {
		if mc.client == client {
			hcp.idle = append(hcp.idle[:i], hcp.idle[i+1:]...)
			break
		}
	}
	hcp.numActive--
	if hcp.numActive < 0 {
		hcp.numActive = 0
	}
}

func (cp *ConnectionPool) scrubber() {
	defer cp.wg.Done()
	ticker := time.NewTicker(cp.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.mu.RLock()
			for _, hcp := range cp.pools {
				hcp.Lock()
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
				hcp.numActive -= cleanedCount
				if hcp.numActive < 0 {
					hcp.numActive = 0
				}
				hcp.Unlock()
			}
			cp.mu.RUnlock()
		case <-cp.stopCh:
			return
		}
	}
}

func (cp *ConnectionPool) Shutdown() {
	if cp.stopCh != nil {
		close(cp.stopCh)
	}
	cp.wg.Wait()

	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, hcp := range cp.pools {
		hcp.Lock()
		for _, mc := range hcp.idle {
			mc.Close()
		}
		hcp.idle = nil
		hcp.numActive = 0
		hcp.Unlock()
	}
	cp.pools = make(map[string]*hostConnectionPool)
}
