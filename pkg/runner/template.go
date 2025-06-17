package runner

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	// Deliberately using text/template for general purpose templating.
	// If HTML escaping is ever needed, html/template could be an alternative for specific functions.
)

// Render a Go template with data and write the result to a remote file.
// This is a core function for configuration management. It performs rendering
// locally (within the Runner's execution context, which could be on a control node
// or wherever the kubexms binary is run) and then efficiently uploads the
// rendered content via the Connector's CopyContent method.
func (r *Runner) Render(
	ctx context.Context,
	tmpl *template.Template,
	data interface{},
	destPath string,
	permissions string,
	sudo bool,
) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if tmpl == nil {
		return fmt.Errorf("template cannot be nil")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Use the existing WriteFile method which internally uses Conn.CopyContent
	// This also handles permissions and sudo for the write operation.
	return r.WriteFile(ctx, buf.Bytes(), destPath, permissions, sudo)
}
