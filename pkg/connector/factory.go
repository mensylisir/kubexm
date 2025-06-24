// in pkg/connector/factory.go
package connector

// defaultFactory implements the Factory interface with the standard connectors.
type defaultFactory struct{}

// NewFactory creates a new instance of the default connector factory.
func NewFactory() Factory {
	return &defaultFactory{}
}

// NewSSHConnector returns a new standard SSHConnector.
func (f *defaultFactory) NewSSHConnector(pool *ConnectionPool) Connector {
	return NewSSHConnector(pool) // 调用你已有的 NewSSHConnector 函数
}

// NewLocalConnector returns a new standard LocalConnector.
func (f *defaultFactory) NewLocalConnector() Connector {
	return &LocalConnector{}
}

// 确保它实现了接口
var _ Factory = (*defaultFactory)(nil)
