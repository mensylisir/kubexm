package runtime

import (
	"fmt"
	"sync"
)

// ===================================================================
// DataBus V2 - 类型安全的数据传递机制
// 解决Step间数据传递不优雅的问题
// ===================================================================

// DataBus 数据总线，用于Step之间传递数据
type DataBus struct {
	ctx       ExecutionContext
	data      map[string]any
	listeners map[string][]chan any
	mu        sync.RWMutex
}

// NewDataBus 创建新的DataBus
func NewDataBus(ctx ExecutionContext) *DataBus {
	return &DataBus{
		ctx:       ctx,
		data:      make(map[string]any),
		listeners: make(map[string][]chan any),
	}
}

// Publish 发布数据
func (db *DataBus) Publish(key string, value any) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.data[key] = value

	// 通知监听者
	if chans, ok := db.listeners[key]; ok {
		for _, ch := range chans {
			ch <- value
		}
	}
}

// Subscribe 订阅数据（阻塞式）
func (db *DataBus) Subscribe(key string) <-chan any {
	db.mu.Lock()
	defer db.mu.Unlock()

	ch := make(chan any, 1)
	if _, ok := db.listeners[key]; !ok {
		db.listeners[key] = []chan any{}
	}
	db.listeners[key] = append(db.listeners[key], ch)

	// 如果已有数据，立即发送
	if val, ok := db.data[key]; ok {
		ch <- val
	}

	return ch
}

// Get 获取数据
func (db *DataBus) Get(key string) (any, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	val, ok := db.data[key]
	return val, ok
}

// GetString 获取字符串类型数据
func (db *DataBus) GetString(key string) (string, bool) {
	val, ok := db.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt 获取整数类型数据
func (db *DataBus) GetInt(key string) (int, bool) {
	val, ok := db.Get(key)
	if !ok {
		return 0, false
	}
	i, ok := val.(int)
	return i, ok
}

// GetBool 获取布尔类型数据
func (db *DataBus) GetBool(key string) (bool, bool) {
	val, ok := db.Get(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// Has 检查数据是否存在
func (db *DataBus) Has(key string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	_, ok := db.data[key]
	return ok
}

// Remove 删除数据
func (db *DataBus) Remove(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()

	delete(db.data, key)
}

// Clear 清空所有数据
func (db *DataBus) Clear() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.data = make(map[string]any)
}

// Keys 获取所有键
func (db *DataBus) Keys() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	keys := make([]string, 0, len(db.data))
	for k := range db.data {
		keys = append(keys, k)
	}
	return keys
}

// ===================================================================
// TypedDataBus - 类型安全的数据总线
// ===================================================================

// TypedDataBus 泛型类型安全的数据总线
type TypedDataBus[T any] struct {
	bus *DataBus
	key string
}

// NewTypedDataBus 创建类型安全的数据总线
func NewTypedDataBus[T any](ctx ExecutionContext, key string) *TypedDataBus[T] {
	return &TypedDataBus[T]{
		bus: NewDataBus(ctx),
		key: key,
	}
}

// Publish 发布类型数据
func (tdb *TypedDataBus[T]) Publish(value T) {
	tdb.bus.Publish(tdb.key, value)
}

// Subscribe subscribes to typed data
func (tdb *TypedDataBus[T]) Subscribe() <-chan T {
	ch := make(chan T, 1)
	go func() {
		val := <-tdb.bus.Subscribe(tdb.key)
		if val != nil {
			ch <- val.(T)
		}
	}()
	return ch
}

// Get 获取类型数据
func (tdb *TypedDataBus[T]) Get() (T, bool) {
	val, ok := tdb.bus.Get(tdb.key)
	if !ok {
		var zero T
		return zero, false
	}
	return val.(T), true
}

// MustGet 获取类型数据，如果不存在则返回零值
func (tdb *TypedDataBus[T]) MustGet() T {
	val, ok := tdb.bus.Get(tdb.key)
	if !ok {
		var zero T
		return zero
	}
	return val.(T)
}

// Has 检查数据是否存在
func (tdb *TypedDataBus[T]) Has() bool {
	return tdb.bus.Has(tdb.key)
}

// ===================================================================
//预定义的DataBus键常量
// ===================================================================

const (
	// Kubeadm相关键
	KeyKubeadmInitData   = "kubeadm.init_data"
	KeyKubeadmJoinData   = "kubeadm.join_data"
	KeyKubeadmToken      = "kubeadm.token"
	KeyKubeadmCACertHash = "kubeadm.ca_cert_hash"

	// Etcd相关键
	KeyEtcdClusterData = "etcd.cluster_data"
	KeyEtcdEndpoints   = "etcd.endpoints"

	// 负载均衡相关键
	KeyLoadBalancerVIP    = "lb.vip"
	KeyLoadBalancerConfig = "lb.config"
	KeyLoadBalancerType   = "lb.type"

	// PKI相关键
	KeyPKICACert     = "pki.ca_cert"
	KeyPKICACertPath = "pki.ca_cert_path"
	KeyPKIClientCert = "pki.client_cert"
	KeyPKIClientKey  = "pki.client_key"

	// Facts相关键
	KeyHostFacts    = "host.facts"
	KeyAllHostFacts = "host.all_facts"

	// 任务状态键
	KeyTaskCompleted = "task.completed"
	KeyTaskResult    = "task.result"
)

// ===================================================================
// LoadBalancerData 负载均衡配置数据
// ===================================================================

// LoadBalancerData 负载均衡配置数据
type LoadBalancerData struct {
	Type           string   `json:"type"`            // haproxy, nginx, kube-vip
	Mode           string   `json:"mode"`            // external, internal
	DeploymentType string   `json:"deployment_type"` // systemd, static_pod
	VIP            string   `json:"vip"`
	Port           int      `json:"port"`
	Servers        []string `json:"servers"`
	ConfigPath     string   `json:"config_path"`
	ServiceName    string   `json:"service_name"`
}

// NewLoadBalancerData 创建负载均衡数据
func NewLoadBalancerData(lbType, mode, deploymentType, vip string, port int, servers []string) *LoadBalancerData {
	return &LoadBalancerData{
		Type:           lbType,
		Mode:           mode,
		DeploymentType: deploymentType,
		VIP:            vip,
		Port:           port,
		Servers:        servers,
	}
}

// ===================================================================
// 便捷构造函数
// ===================================================================

// KubeadmInitDataBuilder kubeadm初始化数据构建器
type KubeadmInitDataBuilder struct {
	data KubeadmInitData
}

// NewKubeadmInitDataBuilder 创建构建器
func NewKubeadmInitDataBuilder() *KubeadmInitDataBuilder {
	return &KubeadmInitDataBuilder{}
}

// WithToken 设置token
func (b *KubeadmInitDataBuilder) WithToken(token string) *KubeadmInitDataBuilder {
	b.data.Token = token
	return b
}

// WithCACertHash 设置CA证书哈希
func (b *KubeadmInitDataBuilder) WithCACertHash(hash string) *KubeadmInitDataBuilder {
	b.data.CACertHash = hash
	return b
}

// WithJoinURL 设置加入URL
func (b *KubeadmInitDataBuilder) WithJoinURL(url string) *KubeadmInitDataBuilder {
	b.data.JoinURL = url
	return b
}

// WithAPIServer 设置API服务器地址
func (b *KubeadmInitDataBuilder) WithAPIServer(server string) *KubeadmInitDataBuilder {
	b.data.APIServer = server
	return b
}

// Build 构建数据
func (b *KubeadmInitDataBuilder) Build() KubeadmInitData {
	return b.data
}

// PublishTo 发布到DataBus
func (b *KubeadmInitDataBuilder) PublishTo(ctx ExecutionContext) error {
	dm := NewDataManager(ctx)
	return dm.Publish(KeyKubeadmInitData, b.data)
}

// EtcdClusterDataBuilder etcd集群数据构建器
type EtcdClusterDataBuilder struct {
	data EtcdClusterData
}

// NewEtcdClusterDataBuilder 创建构建器
func NewEtcdClusterDataBuilder() *EtcdClusterDataBuilder {
	return &EtcdClusterDataBuilder{}
}

// WithEndpoints 设置端点
func (b *EtcdClusterDataBuilder) WithEndpoints(endpoints []string) *EtcdClusterDataBuilder {
	b.data.Endpoints = endpoints
	return b
}

// WithClientCert 设置客户端证书
func (b *EtcdClusterDataBuilder) WithClientCert(cert string) *EtcdClusterDataBuilder {
	b.data.ClientCert = cert
	return b
}

// WithClientKey 设置客户端密钥
func (b *EtcdClusterDataBuilder) WithClientKey(key string) *EtcdClusterDataBuilder {
	b.data.ClientKey = key
	return b
}

// WithCACert 设置CA证书
func (b *EtcdClusterDataBuilder) WithCACert(cert string) *EtcdClusterDataBuilder {
	b.data.CACert = cert
	return b
}

// Build 构建数据
func (b *EtcdClusterDataBuilder) Build() EtcdClusterData {
	return b.data
}

// PublishTo 发布到DataBus
func (b *EtcdClusterDataBuilder) PublishTo(ctx ExecutionContext) error {
	dm := NewDataManager(ctx)
	return dm.Publish(KeyEtcdClusterData, b.data)
}

// ===================================================================
// 数据订阅便捷函数
// ===================================================================

// MustGetKubeadmInitData 获取kubeadm初始化数据
func MustGetKubeadmInitData(ctx ExecutionContext) (KubeadmInitData, error) {
	dm := NewDataManager(ctx)
	val, ok := dm.Subscribe(KeyKubeadmInitData)
	if !ok {
		return KubeadmInitData{}, fmt.Errorf("kubeadm init data not found")
	}
	data, ok := val.(KubeadmInitData)
	if !ok {
		return KubeadmInitData{}, fmt.Errorf("invalid kubeadm init data type")
	}
	return data, nil
}

// MustGetKubeadmJoinData 获取kubeadm加入数据
func MustGetKubeadmJoinData(ctx ExecutionContext) (KubeadmJoinData, error) {
	dm := NewDataManager(ctx)
	val, ok := dm.Subscribe(KeyKubeadmJoinData)
	if !ok {
		return KubeadmJoinData{}, fmt.Errorf("kubeadm join data not found")
	}
	data, ok := val.(KubeadmJoinData)
	if !ok {
		return KubeadmJoinData{}, fmt.Errorf("invalid kubeadm join data type")
	}
	return data, nil
}

// MustGetEtcdClusterData 获取etcd集群数据
func MustGetEtcdClusterData(ctx ExecutionContext) (EtcdClusterData, error) {
	dm := NewDataManager(ctx)
	val, ok := dm.Subscribe(KeyEtcdClusterData)
	if !ok {
		return EtcdClusterData{}, fmt.Errorf("etcd cluster data not found")
	}
	data, ok := val.(EtcdClusterData)
	if !ok {
		return EtcdClusterData{}, fmt.Errorf("invalid etcd cluster data type")
	}
	return data, nil
}

// MustGetLoadBalancerData 获取负载均衡数据
func MustGetLoadBalancerData(ctx ExecutionContext) (*LoadBalancerData, error) {
	dm := NewDataManager(ctx)
	val, ok := dm.Subscribe(KeyLoadBalancerConfig)
	if !ok {
		return nil, fmt.Errorf("load balancer data not found")
	}
	data, ok := val.(*LoadBalancerData)
	if !ok {
		return nil, fmt.Errorf("invalid load balancer data type")
	}
	return data, nil
}
