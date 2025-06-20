package runner

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector" // Ensure this import is correct
)

// Render a Go template with data and write the result to a remote file.
func (r *defaultRunner) Render(
	ctx context.Context,
	conn connector.Connector, // Added conn parameter
	tmpl *template.Template,
	data interface{},
	destPath string,
	permissions string,
	sudo bool,
) error {
	if conn == nil { // Check conn, not r.Conn
		return fmt.Errorf("connector cannot be nil")
	}
	if tmpl == nil {
		return fmt.Errorf("template cannot be nil")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Use r.WriteFile, passing the conn
	return r.WriteFile(ctx, conn, buf.Bytes(), destPath, permissions, sudo)
}
