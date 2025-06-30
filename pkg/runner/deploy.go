package runner

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// DeployAndEnableService renders a configuration file, writes it to the host,
// reloads the service daemon, enables the service, and then restarts it.
// This is a high-level orchestration function.
// receiver `r` is of type *defaultRunner.
func (r *defaultRunner) DeployAndEnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName, configContent, configPath, permissions string, templateData interface{}) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for DeployAndEnableService")
	}
	if facts == nil {
		return fmt.Errorf("facts cannot be nil for DeployAndEnableService")
	}
	if serviceName == "" {
		return fmt.Errorf("serviceName cannot be empty")
	}
	if configPath == "" {
		return fmt.Errorf("configPath cannot be empty")
	}

	var contentBytes []byte

	// 1. Render configuration if templateData is provided
	if templateData != nil {
		if configContent == "" {
			// configContent must be the template string if templateData is used.
			return fmt.Errorf("configContent (template string) cannot be empty if templateData is provided for service %s", serviceName)
		}
		tmpl, err := template.New(serviceName + "-config").Parse(configContent)
		if err != nil {
			return fmt.Errorf("failed to parse config content as template for service %s: %w", serviceName, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, templateData); err != nil {
			return fmt.Errorf("failed to execute template for service %s with data: %w", serviceName, err)
		}
		contentBytes = buf.Bytes()
	} else {
		// If no templateData, configContent is used as is.
		contentBytes = []byte(configContent)
	}

	// 2. Write configuration file
	effectivePermissions := permissions
	if effectivePermissions == "" {
		effectivePermissions = "0644" // Default permissions for a config file
	}
	// Assuming sudo is required for writing service configuration files.
	// The WriteFile method on the runner should handle sudo.
	if err := r.WriteFile(ctx, conn, contentBytes, configPath, effectivePermissions, true); err != nil {
		return fmt.Errorf("failed to write configuration file %s for service %s: %w", configPath, serviceName, err)
	}

	// 3. Daemon Reload (important after changing service unit files or some configs)
	// The DaemonReload method on the runner should handle different init systems based on facts.
	if err := r.DaemonReload(ctx, conn, facts); err != nil {
		return fmt.Errorf("failed to perform daemon-reload after writing config for service %s: %w", serviceName, err)
	}

	// 4. Enable Service
	// The EnableService method on the runner handles different init systems.
	if err := r.EnableService(ctx, conn, facts, serviceName); err != nil {
		return fmt.Errorf("failed to enable service %s: %w", serviceName, err)
	}

	// 5. Restart Service (or Start if preferred, Restart is often safer for config changes)
	// The RestartService method on the runner handles different init systems.
	if err := r.RestartService(ctx, conn, facts, serviceName); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}

	return nil
}
