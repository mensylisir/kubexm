package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector"
	// "github.com/stretchr/testify/assert" // Removed
)

// Helper to quickly get a runner with a mock connector for template tests
func newTestRunnerForTemplate(t *testing.T) (Runner, *MockConnector) {
	mockConn := NewMockConnector()
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, "hostname") { return []byte("test-host"), nil, nil }
		if strings.Contains(cmd, "uname -r") { return []byte("test-kernel"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("1024"), nil, nil }
		if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1"), nil, nil }
		if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
		if strings.Contains(cmd, "command -v apt-get") { return []byte("/usr/bin/apt-get"), nil, nil }
		if strings.Contains(cmd, "command -v systemctl") { return []byte("/usr/bin/systemctl"), nil, nil }
		return []byte("default exec output"), nil, nil
	}
	r := NewRunner()
	return r, mockConn
}


func TestRunner_Render_Success(t *testing.T) {
	r, mockConn := newTestRunnerForTemplate(t)

	tmplString := "Hello {{.Name}} from {{.Location}}!"
	tmpl, err := template.New("testTmpl").Parse(tmplString)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := struct {
		Name     string
		Location string
	}{
		Name:     "TestUser",
		Location: "GoTest",
	}
	expectedRenderedContent := "Hello TestUser from GoTest!"
	destPath := "/test/rendered_template.conf"
	permissions := "0644"
	useSudo := true

	var capturedContent []byte
	var capturedPath string
	var capturedPerms string
	var capturedSudo bool

	// Render calls r.WriteFile, which calls conn.WriteFile. So, mock WriteFileFunc.
	mockConn.WriteFileFunc = func(ctx context.Context, content []byte, dPath string, opts *connector.FileTransferOptions) error {
		capturedContent = content
		capturedPath = dPath
		if opts != nil {
			capturedPerms = opts.Permissions
			capturedSudo = opts.Sudo
		}
		return nil
	}

	err = r.Render(context.Background(), mockConn, tmpl, data, destPath, permissions, useSudo)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if string(capturedContent) != expectedRenderedContent {
		t.Errorf("Rendered content = %q, want %q", string(capturedContent), expectedRenderedContent)
	}
	if capturedPath != destPath {
		t.Errorf("Render destination path = %q, want %q", capturedPath, destPath)
	}
	if capturedPerms != permissions {
		t.Errorf("Render permissions = %q, want %q", capturedPerms, permissions)
	}
	if capturedSudo != useSudo {
		t.Errorf("Render sudo = %v, want %v", capturedSudo, useSudo)
	}
}

func TestRunner_Render_TemplateExecuteError(t *testing.T) {
	r, mockConn := newTestRunnerForTemplate(t)

	tmplString := "Hello {{.NonExistentField}}"
	tmpl, err := template.New("errorTmpl").Parse(tmplString)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := struct{ Name string }{Name: "Test"}

	err = r.Render(context.Background(), mockConn, tmpl, data, "/test/anypath.txt", "0644", false)
	if err == nil {
		t.Fatal("Render() with template execution error expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to execute template") {
		t.Errorf("Render() error message = %q, expected to contain 'failed to execute template'", err.Error())
	}
}

func TestRunner_Render_WriteFileError(t *testing.T) {
	r, mockConn := newTestRunnerForTemplate(t)

	tmplString := "Valid template"
	tmpl, _ := template.New("validTmpl").Parse(tmplString)
	data := struct{}{}
	expectedErr := errors.New("mock WriteFile failed") // More generic error message

	mockConn.WriteFileFunc = func(ctx context.Context, content []byte, dPath string, opts *connector.FileTransferOptions) error {
		return expectedErr
	}

	err := r.Render(context.Background(), mockConn, tmpl, data, "/test/remote.txt", "0600", false)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Render() error = %v, want %v", err, expectedErr)
	}
}

func TestRunner_Render_NilTemplate(t *testing.T) {
	r, mockConn := newTestRunnerForTemplate(t)
	errRender := r.Render(context.Background(), mockConn, nil, nil, "path", "perms", false)
	if errRender == nil {
		t.Fatal("Render() with nil template expected error, got nil")
	}
	if !strings.Contains(errRender.Error(), "template cannot be nil") {
		t.Errorf("Error message mismatch: got %v, want to contain 'template cannot be nil'", errRender)
	}
}

func TestRunner_RenderToString_Success(t *testing.T) {
	r, _ := newTestRunnerForTemplate(t) // Connector not used by RenderToString

	tmplString := "Hello {{.Name}} from {{.Location}}!"
	tmpl, err := template.New("testRenderStrTmpl").Parse(tmplString)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := struct {
		Name     string
		Location string
	}{
		Name:     "Stringer",
		Location: "GoTest",
	}
	expectedRenderedContent := "Hello Stringer from GoTest!"

	renderedStr, err := r.RenderToString(context.Background(), tmpl, data)
	if err != nil {
		t.Fatalf("RenderToString() error = %v", err)
	}

	if renderedStr != expectedRenderedContent {
		t.Errorf("RenderToString() output = %q, want %q", renderedStr, expectedRenderedContent)
	}
}

func TestRunner_RenderToString_TemplateExecuteError(t *testing.T) {
	r, _ := newTestRunnerForTemplate(t)

	tmplString := "Hello {{.NonExistentField}}"
	tmpl, err := template.New("errorRenderStrTmpl").Parse(tmplString)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := struct{ Name string }{Name: "Test"}

	_, err = r.RenderToString(context.Background(), tmpl, data)
	if err == nil {
		t.Fatal("RenderToString() with template execution error expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to execute template for RenderToString") {
		t.Errorf("RenderToString() error message = %q, expected to contain 'failed to execute template for RenderToString'", err.Error())
	}
}

func TestRunner_RenderToString_NilTemplate(t *testing.T) {
	r, _ := newTestRunnerForTemplate(t)
	_, errRender := r.RenderToString(context.Background(), nil, nil)
	if errRender == nil {
		t.Fatal("RenderToString() with nil template expected error, got nil")
	}
	if !strings.Contains(errRender.Error(), "template cannot be nil for RenderToString") {
		t.Errorf("Error message mismatch: got %v, want to contain 'template cannot be nil for RenderToString'", errRender)
	}
}
