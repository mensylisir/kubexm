package connector

import (
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/ssh"
	"strings"
	"time"
)

type Factory interface {
	NewSSHConnector(pool *ConnectionPool) Connector
	NewLocalConnector() Connector
	NewConnectionCfg(host Host, globalTimeout time.Duration) (ConnectionCfg, error)
	NewConnectorForHost(host Host, pool *ConnectionPool) (Connector, error)
}
type defaultFactory struct{}

func NewFactory() Factory {
	return &defaultFactory{}
}

func (f *defaultFactory) NewSSHConnector(pool *ConnectionPool) Connector {
	return NewSSHConnector(pool)
}

func (f *defaultFactory) NewLocalConnector() Connector {
	return &LocalConnector{}
}

func (f *defaultFactory) NewConnectorForHost(host Host, pool *ConnectionPool) (Connector, error) {
	address := host.GetAddress()
	isLocal := strings.ToLower(address) == "localhost" || address == "127.0.0.1"

	if isLocal {
		return f.NewLocalConnector(), nil
	}

	if pool == nil {
		return nil, fmt.Errorf("connection pool is required for non-local host %s", host.GetName())
	}
	return f.NewSSHConnector(pool), nil
}

func (f *defaultFactory) NewConnectionCfg(host Host, globalTimeout time.Duration) (ConnectionCfg, error) {
	connCfg := ConnectionCfg{
		Host:            host.GetAddress(),
		Port:            host.GetPort(),
		User:            host.GetUser(),
		Password:        host.GetPassword(),
		PrivateKey:      []byte(host.GetPrivateKey()),
		PrivateKeyPath:  host.GetPrivateKeyPath(),
		Timeout:         time.Duration(host.GetTimeout()) * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if globalTimeout > 0 {
		connCfg.Timeout = globalTimeout
	} else if connCfg.Timeout <= 0 {
		connCfg.Timeout = 30 * time.Second
	}

	if host.GetPrivateKey() != "" {
		decodedKey, err := base64.StdEncoding.DecodeString(host.GetPrivateKey())
		if err != nil {
			return ConnectionCfg{}, fmt.Errorf("host %s: failed to decode base64 private key: %w", host.GetName(), err)
		}
		connCfg.PrivateKey = decodedKey
	}

	return connCfg, nil
}

var _ Factory = (*defaultFactory)(nil)
