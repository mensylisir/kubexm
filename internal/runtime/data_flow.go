package runtime

import (
	"fmt"
)

// ===================================================================
// 数据管理器实现 (统一所有数据传递)
// ===================================================================

// DataManager 统一的数据管理器，解决step间数据传递问题
type DataManager struct {
	ctx      ExecutionContext
	stateBag StateBag
}

// NewDataManager 创建新的数据管理器
func NewDataManager(ctx ExecutionContext) *DataManager {
	return &DataManager{
		ctx:      ctx,
		stateBag: ctx.GetTaskState(),
	}
}

// BuildKey 构建完整键 (统一格式: run.pipeline.module.task.key)
// 解决键格式不一致问题
func (dm *DataManager) BuildKey(key string) string {
	return fmt.Sprintf("%s.%s.%s.%s.%s",
		dm.ctx.GetRunID(),
		dm.ctx.GetPipelineName(),
		dm.ctx.GetModuleName(),
		dm.ctx.GetTaskName(),
		key,
	)
}

// Publish 发布数据到StateBag (推荐使用StateBag而非Cache)
func (dm *DataManager) Publish(key string, value interface{}) error {
	fullKey := dm.BuildKey(key)
	dm.stateBag.Set(fullKey, value)
	return nil
}

// Subscribe 订阅数据
func (dm *DataManager) Subscribe(key string) (interface{}, bool) {
	fullKey := dm.BuildKey(key)
	return dm.stateBag.Get(fullKey)
}

// SubscribeString 订阅字符串类型数据
func (dm *DataManager) SubscribeString(key string) (string, bool) {
	val, ok := dm.Subscribe(key)
	if !ok {
		return "", false
	}
	str, isStr := val.(string)
	if !isStr {
		return "", false
	}
	return str, true
}

// SubscribeInt 订阅整数类型数据
func (dm *DataManager) SubscribeInt(key string) (int, bool) {
	val, ok := dm.Subscribe(key)
	if !ok {
		return 0, false
	}
	i, isInt := val.(int)
	if !isInt {
		return 0, false
	}
	return i, true
}

// SubscribeBool 订阅布尔类型数据
func (dm *DataManager) SubscribeBool(key string) (bool, bool) {
	val, ok := dm.Subscribe(key)
	if !ok {
		return false, false
	}
	b, isBool := val.(bool)
	if !isBool {
		return false, false
	}
	return b, true
}

// ===================================================================
// 预定义的数据类型 (解决结构体传递问题)
// ===================================================================

// KubeadmInitData kubeadm初始化数据
type KubeadmInitData struct {
	Token      string
	CACertHash string
	JoinURL    string
	APIServer  string
}

// KubeadmJoinData kubeadm加入数据
type KubeadmJoinData struct {
	JoinURL    string
	Token      string
	CACertHash string
	APIServer  string
}

// EtcdClusterData etcd集群数据
type EtcdClusterData struct {
	Endpoints  []string
	ClientCert string
	ClientKey  string
	CACert     string
}

// PKIInfo 证书信息
type PKIInfo struct {
	CACert     string
	ClientCert string
	ClientKey  string
	ServerCert string
	ServerKey  string
}

// ===================================================================
// 类型安全的数据发布器 (使用反射)
// ===================================================================

// KubeadmPublisher kubeadm数据发布器
type KubeadmPublisher struct {
	dm *DataManager
}

// NewKubeadmPublisher 创建kubeadm发布器
func NewKubeadmPublisher(dm *DataManager) *KubeadmPublisher {
	return &KubeadmPublisher{dm: dm}
}

// PublishInitData 发布kubeadm初始化数据
func (p *KubeadmPublisher) PublishInitData(data KubeadmInitData) error {
	return p.dm.Publish("kubeadm.init", data)
}

// PublishJoinData 发布kubeadm加入数据
func (p *KubeadmPublisher) PublishJoinData(data KubeadmJoinData) error {
	return p.dm.Publish("kubeadm.join", data)
}

// EtcdPublisher etcd数据发布器
type EtcdPublisher struct {
	dm *DataManager
}

// NewEtcdPublisher 创建etcd发布器
func NewEtcdPublisher(dm *DataManager) *EtcdPublisher {
	return &EtcdPublisher{dm: dm}
}

// PublishClusterData 发布etcd集群数据
func (p *EtcdPublisher) PublishClusterData(data EtcdClusterData) error {
	return p.dm.Publish("etcd.cluster", data)
}

// PKIPublisher 证书数据发布器
type PKIPublisher struct {
	dm *DataManager
}

// NewPKIPublisher 创建证书发布器
func NewPKIPublisher(dm *DataManager) *PKIPublisher {
	return &PKIPublisher{dm: dm}
}

// PublishPKIInfo 发布证书信息
func (p *PKIPublisher) PublishPKIInfo(data PKIInfo) error {
	return p.dm.Publish("pki.info", data)
}

// ===================================================================
// 类型安全的数据订阅器 (使用反射)
// ===================================================================

// KubeadmSubscriber kubeadm数据订阅器
type KubeadmSubscriber struct {
	dm *DataManager
}

// NewKubeadmSubscriber 创建kubeadm订阅器
func NewKubeadmSubscriber(dm *DataManager) *KubeadmSubscriber {
	return &KubeadmSubscriber{dm: dm}
}

// GetInitData 获取kubeadm初始化数据
func (s *KubeadmSubscriber) GetInitData() (*KubeadmInitData, bool) {
	val, ok := s.dm.Subscribe("kubeadm.init")
	if !ok {
		return nil, false
	}
	data, ok := val.(*KubeadmInitData)
	return data, ok
}

// GetJoinData 获取kubeadm加入数据
func (s *KubeadmSubscriber) GetJoinData() (*KubeadmJoinData, bool) {
	val, ok := s.dm.Subscribe("kubeadm.join")
	if !ok {
		return nil, false
	}
	data, ok := val.(*KubeadmJoinData)
	return data, ok
}

// EtcdSubscriber etcd数据订阅器
type EtcdSubscriber struct {
	dm *DataManager
}

// NewEtcdSubscriber 创建etcd订阅器
func NewEtcdSubscriber(dm *DataManager) *EtcdSubscriber {
	return &EtcdSubscriber{dm: dm}
}

// GetClusterData 获取etcd集群数据
func (s *EtcdSubscriber) GetClusterData() (*EtcdClusterData, bool) {
	val, ok := s.dm.Subscribe("etcd.cluster")
	if !ok {
		return nil, false
	}
	data, ok := val.(*EtcdClusterData)
	return data, ok
}

// PKISubscriber 证书数据订阅器
type PKISubscriber struct {
	dm *DataManager
}

// NewPKISubscriber 创建证书订阅器
func NewPKISubscriber(dm *DataManager) *PKISubscriber {
	return &PKISubscriber{dm: dm}
}

// GetPKIInfo 获取证书信息
func (s *PKISubscriber) GetPKIInfo() (*PKIInfo, bool) {
	val, ok := s.dm.Subscribe("pki.info")
	if !ok {
		return nil, false
	}
	data, ok := val.(*PKIInfo)
	return data, ok
}

// ===================================================================
// 使用示例
// ===================================================================

/*
// 旧的混乱方式:
// cacheKey := fmt.Sprintf("kubeadm.init.token.%s.%s.%s.%s",
//     ctx.GetRunID(), ctx.GetPipelineName(),
//     ctx.GetModuleName(), ctx.GetTaskName())
// ctx.GetTaskCache().Set(cacheKey, token)
//
// // 消费数据
// tokenVal, found := ctx.GetTaskCache().Get(cacheKey)
// token, ok := tokenVal.(string)  // 运行时类型断言

// 新的优雅方式:
// dm := runtime.NewDataManager(ctx)
// kubeadmPub := runtime.NewKubeadmPublisher(dm)
// err := kubeadmPub.PublishInitData(runtime.KubeadmInitData{
//     Token:      token,
//     CACertHash: caCertHash,
//     JoinURL:    joinURL,
// })
//
// // 消费数据
// dm := runtime.NewDataManager(ctx)
// kubeadmSub := runtime.NewKubeadmSubscriber(dm)
// data, ok := kubeadmSub.GetInitData()
// if ok {
//     fmt.Printf("Token: %s\n", data.Token)
//     fmt.Printf("CACertHash: %s\n", data.CACertHash)
// }
*/
