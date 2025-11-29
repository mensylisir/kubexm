package connector

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// MockHost implements Host interface for testing
type MockHost struct {
	Name           string
	Address        string
	Port           int
	User           string
	Password       string
	PrivateKey     string
	PrivateKeyPath string
	Timeout        int64
}

func (m *MockHost) GetName() string                 { return m.Name }
func (m *MockHost) SetName(name string)             { m.Name = name }
func (m *MockHost) GetAddress() string              { return m.Address }
func (m *MockHost) SetAddress(str string)           { m.Address = str }
func (m *MockHost) GetInternalAddress() string      { return "" }
func (m *MockHost) GetInternalIPv4Address() string  { return "" }
func (m *MockHost) GetInternalIPv6Address() string  { return "" }
func (m *MockHost) SetInternalAddress(str string)   {}
func (m *MockHost) GetPort() int                    { return m.Port }
func (m *MockHost) SetPort(port int)                { m.Port = port }
func (m *MockHost) GetUser() string                 { return m.User }
func (m *MockHost) SetUser(u string)                { m.User = u }
func (m *MockHost) GetPassword() string             { return m.Password }
func (m *MockHost) SetPassword(password string)     { m.Password = password }
func (m *MockHost) GetPrivateKey() string           { return m.PrivateKey }
func (m *MockHost) SetPrivateKey(privateKey string) { m.PrivateKey = privateKey }
func (m *MockHost) GetPrivateKeyPath() string       { return m.PrivateKeyPath }
func (m *MockHost) SetPrivateKeyPath(path string)   { m.PrivateKeyPath = path }
func (m *MockHost) GetArch() string                 { return "amd64" }
func (m *MockHost) SetArch(arch string)             {}
func (m *MockHost) GetTimeout() int64               { return m.Timeout }
func (m *MockHost) SetTimeout(timeout int64)        { m.Timeout = timeout }
func (m *MockHost) GetRoles() []string              { return nil }
func (m *MockHost) SetRoles(roles []string)         {}
func (m *MockHost) IsRole(role string) bool         { return false }
func (m *MockHost) GetHostSpec() v1alpha1.HostSpec  { return v1alpha1.HostSpec{} }

func TestDefaultFactory_NewConnectorForHost(t *testing.T) {
	f := NewFactory()

	t.Run("LocalHost", func(t *testing.T) {
		host := &MockHost{Address: "localhost"}
		conn, err := f.NewConnectorForHost(host, nil)
		if err != nil {
			t.Fatalf("Failed to create local connector: %v", err)
		}
		if _, ok := conn.(*LocalConnector); !ok {
			t.Errorf("Expected *LocalConnector, got %T", conn)
		}
	})

	t.Run("RemoteHost", func(t *testing.T) {
		host := &MockHost{Address: "192.168.1.100"}
		conn, err := f.NewConnectorForHost(host, nil)
		if err != nil {
			t.Fatalf("Failed to create ssh connector: %v", err)
		}
		if _, ok := conn.(*SSHConnector); !ok {
			t.Errorf("Expected *SSHConnector, got %T", conn)
		}
	})
}

func TestDefaultFactory_NewConnectionCfg(t *testing.T) {
	f := NewFactory()

	t.Run("BasicConfig", func(t *testing.T) {
		host := &MockHost{
			Address:  "192.168.1.100",
			Port:     2222,
			User:     "user",
			Password: "password",
			Timeout:  10,
		}
		cfg, err := f.NewConnectionCfg(host, 0)
		if err != nil {
			t.Fatalf("NewConnectionCfg failed: %v", err)
		}
		if cfg.Host != "192.168.1.100" {
			t.Errorf("Expected host 192.168.1.100, got %s", cfg.Host)
		}
		if cfg.Port != 2222 {
			t.Errorf("Expected port 2222, got %d", cfg.Port)
		}
		if cfg.Timeout != 10*time.Second {
			t.Errorf("Expected timeout 10s, got %v", cfg.Timeout)
		}
	})

	t.Run("PrivateKeyBase64", func(t *testing.T) {
		keyContent := "test-private-key"
		encodedKey := base64.StdEncoding.EncodeToString([]byte(keyContent))
		host := &MockHost{
			PrivateKey: encodedKey,
		}
		cfg, err := f.NewConnectionCfg(host, 0)
		if err != nil {
			t.Fatalf("NewConnectionCfg failed: %v", err)
		}
		if string(cfg.PrivateKey) != keyContent {
			t.Errorf("Expected private key %s, got %s", keyContent, string(cfg.PrivateKey))
		}
	})
}
