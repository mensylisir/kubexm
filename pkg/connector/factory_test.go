package connector

import (
	// "reflect" // No longer needed after switching to type assertions
	"testing"
)

func TestDefaultFactory_NewSSHConnector(t *testing.T) {
	factory := NewFactory()
	// Test with a nil pool, as the factory itself doesn't use the pool,
	// it just passes it to NewSSHConnector.
	// The actual functionality of NewSSHConnector with a pool is tested in ssh_test.go or pool_test.go.
	connector := factory.NewSSHConnector(nil)

	if connector == nil {
		t.Fatal("NewSSHConnector returned nil")
	}

	// Check the type using type assertion
	if _, ok := connector.(*SSHConnector); !ok {
		t.Errorf("NewSSHConnector returned type %T, want *SSHConnector", connector)
	}
}

func TestDefaultFactory_NewLocalConnector(t *testing.T) {
	factory := NewFactory()
	connector := factory.NewLocalConnector()

	if connector == nil {
		t.Fatal("NewLocalConnector returned nil")
	}

	// Check the type using type assertion
	if _, ok := connector.(*LocalConnector); !ok {
		t.Errorf("NewLocalConnector returned type %T, want *LocalConnector", connector)
	}
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	if factory == nil {
		t.Fatal("NewFactory returned nil")
	}
	// Check if it's the expected type
	if _, ok := factory.(*defaultFactory); !ok {
		t.Errorf("NewFactory returned type %T, want *defaultFactory", factory)
	}
}
