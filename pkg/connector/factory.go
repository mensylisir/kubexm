package connector

type defaultFactory struct{}

func NewFactory() Factory {
	return &defaultFactory{}
}

func (f *defaultFactory) NewSSHConnector(pool *ConnectionPool) Connector {
	return NewSSHConnector(pool) // 调用你已有的 NewSSHConnector 函数
}

func (f *defaultFactory) NewLocalConnector() Connector {
	return &LocalConnector{}
}

var _ Factory = (*defaultFactory)(nil)
