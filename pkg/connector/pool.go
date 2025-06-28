package connector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os" // Ensured os package is imported
	"sort"
	"strings"
	"sync"
	"time"

	"errors"
	"golang.org/x/crypto/ssh"
)

// currentDialer is a package-level variable holding the function used to dial SSH connections.
// It defaults to the real dialSSH function but can be overridden for testing.
var currentDialer dialSSHFunc = dialSSH

// ErrPoolExhausted is returned when Get is called and the pool has reached MaxPerKey for that key.
var ErrPoolExhausted = errors.New("connection pool exhausted for key")

// ManagedConnection wraps an ssh.Client with additional metadata for pooling.
type ManagedConnection struct {
	client        *ssh.Client
	bastionClient *ssh.Client // Bastion client, if this is a connection via bastion
	poolKey       string      // The key of the pool this connection belongs to
	lastUsed      time.Time   // Timestamp of when the connection was last returned to the pool or used
	createdAt     time.Time   // Timestamp of when the connection was created (when client was established)
}

// Client returns the underlying *ssh.Client.
func (mc *ManagedConnection) Client() *ssh.Client {
	return mc.client
}

// PoolConfig holds configuration settings for the ConnectionPool.
type PoolConfig struct {
	MaxTotalConnections int           // Maximum total connections allowed across all keys. (0 for unlimited)
	MaxPerKey           int           // Maximum number of active connections per pool key. (0 for default, e.g., 5)
	MinIdlePerKey       int           // Minimum number of idle connections to keep per pool key. (0 for default, e.g., 1)
	MaxIdlePerKey       int           // Maximum number of idle connections allowed per pool key. (0 for default, e.g., 3)
	MaxConnectionAge    time.Duration // Maximum age of a connection before it's closed (even if active/idle). (0 for no limit)
	IdleTimeout         time.Duration // Maximum time an idle connection can stay in the pool. (0 for no limit)
	HealthCheckInterval time.Duration // How often to check health of idle connections. (0 to disable periodic checks)
	ConnectTimeout      time.Duration // Timeout for establishing new SSH connections. (Defaults to e.g. 15s if not set)
}

// DefaultPoolConfig returns a PoolConfig with sensible default values.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxTotalConnections: 100,
		MaxPerKey:           5,
		MinIdlePerKey:       1,
		MaxIdlePerKey:       3,
		MaxConnectionAge:    1 * time.Hour,
		IdleTimeout:         10 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		ConnectTimeout:      15 * time.Second,
	}
}

// hostConnectionPool holds a list (acting as a queue) of managed connections for a specific host configuration.
type hostConnectionPool struct {
	sync.Mutex
	connections []*ManagedConnection
	numActive   int
}

// ConnectionPool manages pools of SSH connections for various host configurations.
type ConnectionPool struct {
	pools            map[string]*hostConnectionPool
	config           PoolConfig
	mu               sync.RWMutex
	tempBastionMap   map[*ssh.Client]*ssh.Client
	tempBastionMapMu sync.Mutex
}

// NewConnectionPool initializes and returns a new *ConnectionPool.
func NewConnectionPool(config PoolConfig) *ConnectionPool {
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

	return &ConnectionPool{
		pools:          make(map[string]*hostConnectionPool),
		config:         config,
		tempBastionMap: make(map[*ssh.Client]*ssh.Client),
	}
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
			PrivateKey:     cfg.BastionCfg.PrivateKey,
			PrivateKeyPath: cfg.BastionCfg.PrivateKeyPath,
			Timeout:        cfg.BastionCfg.Timeout,
		}
		keyParts = append(keyParts, "bastion:"+generatePoolKey(bastionConnCfg))
	}
	sort.Strings(keyParts)
	return strings.Join(keyParts, "|")
}

func (cp *ConnectionPool) Get(ctx context.Context, cfg ConnectionCfg) (*ssh.Client, error) {
	poolKey := generatePoolKey(cfg)

	cp.mu.RLock()
	hcp, ok := cp.pools[poolKey]
	cp.mu.RUnlock()

	if !ok {
		cp.mu.Lock()
		hcp, ok = cp.pools[poolKey]
		if !ok {
			hcp = &hostConnectionPool{connections: make([]*ManagedConnection, 0)}
			cp.pools[poolKey] = hcp
		}
		cp.mu.Unlock()
	}

	hcp.Lock()
	for i := len(hcp.connections) - 1; i >= 0; i-- {
		mc := hcp.connections[i]
		hcp.connections = append(hcp.connections[:i], hcp.connections[i+1:]...)

		if cp.config.IdleTimeout > 0 && mc.lastUsed.Add(cp.config.IdleTimeout).Before(time.Now()) {
			mc.client.Close()
			if mc.bastionClient != nil {
				mc.bastionClient.Close()
			}
			hcp.numActive--
			continue
		}

		session, err := mc.client.NewSession()
		if err == nil {
			session.Close()
			mc.lastUsed = time.Now()
			hcp.Unlock()
			return mc.client, nil
		}
		mc.client.Close()
		if mc.bastionClient != nil {
			mc.bastionClient.Close()
		}
		hcp.numActive--
	}

	if hcp.numActive < cp.config.MaxPerKey {
		hcp.numActive++
		hcp.Unlock()

		// The new dialSSH infers timeout from cfg or uses a default.
	targetClient, bastionClient, err := currentDialer(ctx, cfg)
		if err != nil {
			hcp.Lock()
			hcp.numActive--
			hcp.Unlock()
			return nil, err
		}

		session, testErr := targetClient.NewSession()
		if testErr != nil {
			targetClient.Close()
			if bastionClient != nil {
				bastionClient.Close()
			}
			hcp.Lock()
			hcp.numActive--
			hcp.Unlock()
			return nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("newly dialed pooled connection failed test session: %w", testErr)}
		}
		session.Close()

		if bastionClient != nil {
			cp.tempBastionMapMu.Lock()
			cp.tempBastionMap[targetClient] = bastionClient
			cp.tempBastionMapMu.Unlock()
		}
		return targetClient, nil
	}
	hcp.Unlock()
	return nil, fmt.Errorf("%w: %s (max %d reached)", ErrPoolExhausted, poolKey, cp.config.MaxPerKey)
}

func (cp *ConnectionPool) Put(cfg ConnectionCfg, client *ssh.Client, isHealthy bool) {
	if client == nil {
		return
	}
	poolKey := generatePoolKey(cfg)

	cp.mu.RLock()
	hcp, ok := cp.pools[poolKey]
	cp.mu.RUnlock()

	if !ok {
		client.Close()
		cp.tempBastionMapMu.Lock()
		if bastionClient, bcOK := cp.tempBastionMap[client]; bcOK {
			bastionClient.Close()
			delete(cp.tempBastionMap, client)
		}
		cp.tempBastionMapMu.Unlock()
		return
	}

	hcp.Lock()
	defer hcp.Unlock()

	if !isHealthy || len(hcp.connections) >= cp.config.MaxIdlePerKey {
		cp.tempBastionMapMu.Lock()
		associatedBastion := cp.tempBastionMap[client]
		delete(cp.tempBastionMap, client)
		cp.tempBastionMapMu.Unlock()

		client.Close()
		if associatedBastion != nil {
			associatedBastion.Close()
		}
		hcp.numActive--
		return
	}

	cp.tempBastionMapMu.Lock()
	retrievedBastionClient := cp.tempBastionMap[client]
	delete(cp.tempBastionMap, client)
	cp.tempBastionMapMu.Unlock()

	mc := &ManagedConnection{
		client:        client,
		bastionClient: retrievedBastionClient,
		poolKey:       poolKey,
		lastUsed:      time.Now(),
		createdAt:     time.Now(),
	}
	hcp.connections = append(hcp.connections, mc)
}

func (cp *ConnectionPool) CloseConnection(cfg ConnectionCfg, client *ssh.Client) {
	if client == nil {
		return
	}

	cp.tempBastionMapMu.Lock()
	associatedBastion := cp.tempBastionMap[client]
	delete(cp.tempBastionMap, client)
	cp.tempBastionMapMu.Unlock()

	poolKey := generatePoolKey(cfg)
	cp.mu.RLock()
	hcp, ok := cp.pools[poolKey]
	cp.mu.RUnlock()

	if ok {
		hcp.Lock()
		hcp.numActive--
		if hcp.numActive < 0 {
			hcp.numActive = 0
		}
		// Remove from idle connections if present
		for i, mc := range hcp.connections {
			if mc.client == client {
				hcp.connections = append(hcp.connections[:i], hcp.connections[i+1:]...)
				if mc.bastionClient != nil {
					mc.bastionClient.Close()
				}
				break
			}
		}
		hcp.Unlock()
	}

	client.Close()
	if associatedBastion != nil {
		associatedBastion.Close()
	}
}

func (cp *ConnectionPool) Shutdown() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for _, hcp := range cp.pools {
		hcp.Lock()
		for _, mc := range hcp.connections {
			mc.client.Close()
			if mc.bastionClient != nil {
				mc.bastionClient.Close()
			}
		}
		hcp.connections = make([]*ManagedConnection, 0)
		hcp.numActive = 0
		hcp.Unlock()
	}
	cp.pools = make(map[string]*hostConnectionPool)

	cp.tempBastionMapMu.Lock()
	for client, bastionClient := range cp.tempBastionMap {
		client.Close()
		if bastionClient != nil {
			bastionClient.Close()
		}
	}
	cp.tempBastionMap = make(map[*ssh.Client]*ssh.Client)
	cp.tempBastionMapMu.Unlock()
}

type SSHConnectorWithPool struct {
	BaseConnector SSHConnector
	Pool          *ConnectionPool
}

func (s *SSHConnectorWithPool) Connect(ctx context.Context, cfg ConnectionCfg) error {
	if cfg.BastionCfg == nil {
		client, err := s.Pool.Get(ctx, cfg)
		if err == nil {
			s.BaseConnector.client = client
			s.BaseConnector.connCfg = cfg
			s.BaseConnector.isConnected = true
			return nil
		}
		return fmt.Errorf("failed to get connection from pool for %s: %w", cfg.Host, err)
	}

	originalConnector := NewSSHConnector(nil)
	err := originalConnector.Connect(ctx, cfg)
	if err == nil {
		s.BaseConnector.client = originalConnector.client
		s.BaseConnector.bastionClient = originalConnector.bastionClient
		s.BaseConnector.connCfg = originalConnector.connCfg
		s.BaseConnector.isConnected = true
	}
	return err
}

func (s *SSHConnectorWithPool) Close() error {
	if s.BaseConnector.client == nil {
		return nil
	}

	if s.BaseConnector.isFromPool && s.Pool != nil { // Check if it was from pool
		s.Pool.Put(s.BaseConnector.connCfg, s.BaseConnector.client, s.BaseConnector.IsConnected())
		s.BaseConnector.client = nil
		s.BaseConnector.isConnected = false
		s.BaseConnector.isFromPool = false // Reset flag
		return nil
	}
	// If not from pool (e.g. bastion, or direct dial if pool was nil in SSHConnector)
	return s.BaseConnector.Close()
}

func (s *SSHConnectorWithPool) Exec(ctx context.Context, cmd string, opts *ExecOptions) ([]byte, []byte, error) {
	if !s.BaseConnector.IsConnected() {
		return nil, nil, &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.Exec(ctx, cmd, opts)
}

func (s *SSHConnectorWithPool) CopyContent(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error {
	if !s.BaseConnector.IsConnected() {
		return &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.CopyContent(ctx, content, destPath, options)
}

func (s *SSHConnectorWithPool) Stat(ctx context.Context, path string) (*FileStat, error) {
	if !s.BaseConnector.IsConnected() {
		return nil, &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.Stat(ctx, path)
}

func (s *SSHConnectorWithPool) LookPath(ctx context.Context, file string) (string, error) {
	if !s.BaseConnector.IsConnected() {
		return "", &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.LookPath(ctx, file)
}

func (s *SSHConnectorWithPool) IsConnected() bool {
	return s.BaseConnector.IsConnected()
}

func (s *SSHConnectorWithPool) GetOS(ctx context.Context) (*OS, error) {
	if !s.BaseConnector.IsConnected() {
		return nil, &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.GetOS(ctx)
}
func (s *SSHConnectorWithPool) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if !s.BaseConnector.IsConnected() {
		return nil, &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.ReadFile(ctx, path)
}
func (s *SSHConnectorWithPool) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	if !s.BaseConnector.IsConnected() {
		return &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.WriteFile(ctx, content, destPath, permissions, sudo)
}
func (s *SSHConnectorWithPool) Mkdir(ctx context.Context, path string, perm string) error {
	if !s.BaseConnector.IsConnected() {
		return &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.Mkdir(ctx, path, perm)
}
func (s *SSHConnectorWithPool) Remove(ctx context.Context, path string, opts RemoveOptions) error {
	if !s.BaseConnector.IsConnected() {
		return &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.Remove(ctx, path, opts)
}
func (s *SSHConnectorWithPool) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	if !s.BaseConnector.IsConnected() {
		return "", &ConnectionError{Host: s.BaseConnector.connCfg.Host, Err: fmt.Errorf("not connected")}
	}
	return s.BaseConnector.GetFileChecksum(ctx, path, checksumType)
}

var _ Connector = &SSHConnectorWithPool{}

func (cfg *ConnectionCfg) ToSSHClientConfig() (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if len(cfg.PrivateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if cfg.PrivateKeyPath != "" {
		keyBytes, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file %s: %w", cfg.PrivateKeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key from file %s: %w", cfg.PrivateKeyPath, err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method available (password or private key required)")
	}

	hostKeyCallback := cfg.HostKeyCallback
	if hostKeyCallback == nil {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	return &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}, nil
}
