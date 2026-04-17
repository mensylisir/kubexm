package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/common"
	"gopkg.in/yaml.v3"
)

// LocalConfig represents the CLI local configuration
type LocalConfig struct {
	CurrentContext string            `yaml:"currentContext,omitempty"`
	Contexts       map[string]Context `yaml:"contexts,omitempty"`
	DefaultPackageDir string         `yaml:"defaultPackageDir,omitempty"`
	Verbose        bool              `yaml:"verbose,omitempty"`
}

// Context represents a cluster context in the local config
type Context struct {
	Name      string `yaml:"name,omitempty"`
	ClusterName string `yaml:"clusterName,omitempty"`
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
}

// GetConfigPath returns the path to the local config file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, common.KubexmRootDirName, "config.yaml"), nil
}

// LoadLocalConfig loads the local configuration from ~/.kubexm/config.yaml
func LoadLocalConfig() (*LocalConfig, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &LocalConfig{
				Contexts: make(map[string]Context),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config LocalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Contexts == nil {
		config.Contexts = make(map[string]Context)
	}

	return &config, nil
}

// SaveLocalConfig saves the local configuration to ~/.kubexm/config.yaml
func SaveLocalConfig(config *LocalConfig) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}