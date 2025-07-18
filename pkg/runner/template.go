package runner

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func (r *defaultRunner) Render(
	ctx context.Context,
	conn connector.Connector,
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

	return r.WriteFile(ctx, conn, buf.Bytes(), destPath, permissions, sudo)
}

func (r *defaultRunner) RenderToString(ctx context.Context, tmpl *template.Template, data interface{}) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("template cannot be nil for RenderToString")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template for RenderToString: %w", err)
	}
	return buf.String(), nil
}
