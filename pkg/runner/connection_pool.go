package runner

//
//import (
//	"context"
//	"fmt"
//	"sync"
//	"time"
//
//	"github.com/mensylisir/kubexm/pkg/connector"
//)
//
//// ConnectionPool manages a pool of connector connections for better resource management
//type ConnectionPool struct {
//	mu           sync.RWMutex
//	connections  map[string]*PooledConnection
//	maxIdleTime  time.Duration
//	maxPoolSize  int
//	factory      ConnectionFactory
//	cleanupDone  chan struct{}
//	cleanupOnce  sync.Once
//}
//
//// PooledConnection represents a connection in the pool
//type PooledConnection struct {
//	conn     connector.Connector
//	lastUsed time.Time
//	inUse    bool
//}
//
//// ConnectionFactory creates new connections
//type ConnectionFactory func(ctx context.Context, key string) (connector.Connector, error)
//
//// NewConnectionPool creates a new connection pool
//func NewConnectionPool(factory ConnectionFactory, maxPoolSize int, maxIdleTime time.Duration) *ConnectionPool {
//	pool := &ConnectionPool{
//		connections:  make(map[string]*PooledConnection),
//		maxIdleTime:  maxIdleTime,
//		maxPoolSize:  maxPoolSize,
//		factory:      factory,
//		cleanupDone:  make(chan struct{}),
//	}
//
//	// Start cleanup goroutine
//	go pool.cleanup()
//
//	return pool
//}
//
//// GetConnection gets a connection from the pool or creates a new one
//func (p *ConnectionPool) GetConnection(ctx context.Context, key string) (connector.Connector, error) {
//	p.mu.Lock()
//	defer p.mu.Unlock()
//
//	// Check if connection exists and is not in use
//	if pooled, exists := p.connections[key]; exists && !pooled.inUse {
//		// Check if connection is still valid
//		if pooled.conn.IsConnected() {
//			pooled.inUse = true
//			pooled.lastUsed = time.Now()
//			return pooled.conn, nil
//		}
//		// Connection is stale, remove it
//		delete(p.connections, key)
//	}
//
//	// Check pool size limit
//	if len(p.connections) >= p.maxPoolSize {
//		return nil, fmt.Errorf("connection pool is full (max: %d)", p.maxPoolSize)
//	}
//
//	// Create new connection
//	conn, err := p.factory(ctx, key)
//	if err != nil {
//		return nil, fmt.Errorf("failed to create connection: %w", err)
//	}
//
//	// Add to pool
//	p.connections[key] = &PooledConnection{
//		conn:     conn,
//		lastUsed: time.Now(),
//		inUse:    true,
//	}
//
//	return conn, nil
//}
//
//// ReleaseConnection releases a connection back to the pool
//func (p *ConnectionPool) ReleaseConnection(key string) {
//	p.mu.Lock()
//	defer p.mu.Unlock()
//
//	if pooled, exists := p.connections[key]; exists {
//		pooled.inUse = false
//		pooled.lastUsed = time.Now()
//	}
//}
//
//// RemoveConnection removes a connection from the pool
//func (p *ConnectionPool) RemoveConnection(key string) {
//	p.mu.Lock()
//	defer p.mu.Unlock()
//
//	if pooled, exists := p.connections[key]; exists {
//		if pooled.conn.IsConnected() {
//			pooled.conn.Close()
//		}
//		delete(p.connections, key)
//	}
//}
//
//// Close closes all connections and shuts down the pool
//func (p *ConnectionPool) Close() {
//	p.mu.Lock()
//	defer p.mu.Unlock()
//
//	// Close all connections
//	for key, pooled := range p.connections {
//		if pooled.conn.IsConnected() {
//			pooled.conn.Close()
//		}
//		delete(p.connections, key)
//	}
//
//	// Stop cleanup goroutine
//	p.cleanupOnce.Do(func() {
//		close(p.cleanupDone)
//	})
//}
//
//// cleanup removes idle connections
//func (p *ConnectionPool) cleanup() {
//	ticker := time.NewTicker(p.maxIdleTime / 2)
//	defer ticker.Stop()
//
//	for {
//		select {
//		case <-ticker.C:
//			p.cleanupIdleConnections()
//		case <-p.cleanupDone:
//			return
//		}
//	}
//}
//
//// cleanupIdleConnections removes connections that have been idle too long
//func (p *ConnectionPool) cleanupIdleConnections() {
//	p.mu.Lock()
//	defer p.mu.Unlock()
//
//	now := time.Now()
//	for key, pooled := range p.connections {
//		if !pooled.inUse && now.Sub(pooled.lastUsed) > p.maxIdleTime {
//			if pooled.conn.IsConnected() {
//				pooled.conn.Close()
//			}
//			delete(p.connections, key)
//		}
//	}
//}
//
//// Stats returns connection pool statistics
//func (p *ConnectionPool) Stats() map[string]interface{} {
//	p.mu.RLock()
//	defer p.mu.RUnlock()
//
//	inUse := 0
//	idle := 0
//
//	for _, pooled := range p.connections {
//		if pooled.inUse {
//			inUse++
//		} else {
//			idle++
//		}
//	}
//
//	return map[string]interface{}{
//		"total":       len(p.connections),
//		"in_use":      inUse,
//		"idle":        idle,
//		"max_size":    p.maxPoolSize,
//		"max_idle_time": p.maxIdleTime.String(),
//	}
//}
//
//// Enhanced runner with connection pooling
//type pooledRunner struct {
//	*defaultRunner
//	pool *ConnectionPool
//}
//
//// NewPooledRunner creates a new runner with connection pooling
//func NewPooledRunner(factory ConnectionFactory, maxPoolSize int, maxIdleTime time.Duration) Runner {
//	return &pooledRunner{
//		defaultRunner: &defaultRunner{},
//		pool:          NewConnectionPool(factory, maxPoolSize, maxIdleTime),
//	}
//}
//
//// WithConnection executes a function with a pooled connection
//func (r *pooledRunner) WithConnection(ctx context.Context, key string, fn func(connector.Connector) error) error {
//	conn, err := r.pool.GetConnection(ctx, key)
//	if err != nil {
//		return err
//	}
//	defer r.pool.ReleaseConnection(key)
//
//	return fn(conn)
//}
//
//// Close closes the connection pool
//func (r *pooledRunner) Close() {
//	r.pool.Close()
//}
//
//// GetConnectionStats returns connection pool statistics
//func (r *pooledRunner) GetConnectionStats() map[string]interface{} {
//	return r.pool.Stats()
//}
