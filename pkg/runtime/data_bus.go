package runtime

import (
	"fmt"
)

// DataPublisher is an interface for publishing data to different scopes
type DataPublisher interface {
	// Publish publishes data to the specified scope with a simple key
	Publish(scope string, key string, value interface{}) error

	// PublishWithTTL publishes data with a time-to-live
	PublishWithTTL(scope string, key string, value interface{}, ttl int64) error
}

// DataSubscriber is an interface for subscribing to data
type DataSubscriber interface {
	// Subscribe retrieves data by key, searching through scopes hierarchically
	Subscribe(key string) (interface{}, bool)

	// SubscribeFromScope retrieves data from a specific scope
	SubscribeFromScope(scope string, key string) (interface{}, bool)

	// SubscribeString retrieves data as a string
	SubscribeString(key string) (string, bool)

	// SubscribeInt retrieves data as an int
	SubscribeInt(key string) (int, bool)

	// SubscribeBool retrieves data as a bool
	SubscribeBool(key string) (bool, bool)
}

// DataContext combines publisher and subscriber interfaces
type DataContext interface {
	DataPublisher
	DataSubscriber
	ExecutionContext
}

// SimpleDataBus provides simplified data sharing mechanisms
type SimpleDataBus struct {
	ctx ExecutionContext
}

// NewSimpleDataBus creates a new SimpleDataBus
func NewSimpleDataBus(ctx ExecutionContext) *SimpleDataBus {
	return &SimpleDataBus{ctx: ctx}
}

// Publish publishes data to the specified scope with a simple key
func (db *SimpleDataBus) Publish(scope string, key string, value interface{}) error {
	fullKey := db.buildFullKey(key)
	return db.ctx.Export(scope, fullKey, value)
}

// PublishWithTTL publishes data with a time-to-live (not implemented in current cache)
func (db *SimpleDataBus) PublishWithTTL(scope string, key string, value interface{}, ttl int64) error {
	// For now, we ignore TTL as the current cache implementation doesn't support it
	return db.Publish(scope, key, value)
}

// Subscribe retrieves data by key, searching through scopes hierarchically
func (db *SimpleDataBus) Subscribe(key string) (interface{}, bool) {
	fullKey := db.buildFullKey(key)
	return db.ctx.Import("", fullKey)
}

// SubscribeFromScope retrieves data from a specific scope
func (db *SimpleDataBus) SubscribeFromScope(scope string, key string) (interface{}, bool) {
	fullKey := db.buildFullKey(key)
	return db.ctx.Import(scope, fullKey)
}

// SubscribeString retrieves data as a string
func (db *SimpleDataBus) SubscribeString(key string) (string, bool) {
	value, found := db.Subscribe(key)
	if !found {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

// SubscribeInt retrieves data as an int
func (db *SimpleDataBus) SubscribeInt(key string) (int, bool) {
	value, found := db.Subscribe(key)
	if !found {
		return 0, false
	}
	i, ok := value.(int)
	return i, ok
}

// SubscribeBool retrieves data as a bool
func (db *SimpleDataBus) SubscribeBool(key string) (bool, bool) {
	value, found := db.Subscribe(key)
	if !found {
		return false, false
	}
	b, ok := value.(bool)
	return b, ok
}

// buildFullKey constructs a full key with context information
func (db *SimpleDataBus) buildFullKey(key string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s",
		db.ctx.GetRunID(),
		db.ctx.GetPipelineName(),
		db.ctx.GetModuleName(),
		db.ctx.GetTaskName(),
		key)
}

// BuildCacheKey builds a cache key using the common format
func (db *SimpleDataBus) BuildCacheKey(template string, args ...interface{}) string {
	// Prepend context information to the args
	contextArgs := []interface{}{
		db.ctx.GetRunID(),
		db.ctx.GetPipelineName(),
		db.ctx.GetModuleName(),
		db.ctx.GetTaskName(),
	}
	contextArgs = append(contextArgs, args...)
	return fmt.Sprintf(template, contextArgs...)
}

// Helper functions for common data operations

// PublishKubeadmInitData publishes the kubeadm init data (token, cert key, CA cert hash)
func (db *SimpleDataBus) PublishKubeadmInitData(token, certKey, caCertHash string) error {
	if err := db.Publish("task", "kubeadm.init.token", token); err != nil {
		return err
	}
	if err := db.Publish("task", "kubeadm.init.certkey", certKey); err != nil {
		return err
	}
	if err := db.Publish("task", "kubeadm.init.cacerthash", caCertHash); err != nil {
		return err
	}
	return nil
}

// SubscribeKubeadmInitToken subscribes to the kubeadm init token
func (db *SimpleDataBus) SubscribeKubeadmInitToken() (string, bool) {
	return db.SubscribeString("kubeadm.init.token")
}

// SubscribeKubeadmInitCertKey subscribes to the kubeadm init certificate key
func (db *SimpleDataBus) SubscribeKubeadmInitCertKey() (string, bool) {
	return db.SubscribeString("kubeadm.init.certkey")
}

// SubscribeKubeadmInitCACertHash subscribes to the kubeadm init CA certificate hash
func (db *SimpleDataBus) SubscribeKubeadmInitCACertHash() (string, bool) {
	return db.SubscribeString("kubeadm.init.cacerthash")
}
