package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Helper to quickly get a runner with a mock connector for template tests
func newTestRunnerForTemplate(t *testing.T) (*Runner, *MockConnector) {
	mockConn := NewMockConnector()
	// Default GetOS for NewRunner
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	// Default Exec for NewRunner fact gathering
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, "hostname") { return []byte("test-host"), nil, nil }
		if strings.Contains(cmd, "uname -r") { return []byte("test-kernel"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("1024"), nil, nil } // 1MB
		if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1"), nil, nil }
		if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
		return []byte("default exec output"), nil, nil // Fallback for other commands
	}
	r, err := NewRunner(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("Failed to create runner for template tests: %v", err)
	}
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

	mockConn.CopyContentFunc = func(ctx context.Context, content []byte, dPath string, opts *connector.FileTransferOptions) error {
		capturedContent = content
		capturedPath = dPath
		if opts != nil {
			capturedPerms = opts.Permissions
			capturedSudo = opts.Sudo
		}
		return nil
	}

	err = r.Render(context.Background(), tmpl, data, destPath, permissions, useSudo)
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
	r, _ := newTestRunnerForTemplate(t) // mockConn not directly needed here as error is pre-CopyContent

	tmplString := "Hello {{.NonExistentField}}"
	tmpl, err := template.New("errorTmpl").Parse(tmplString)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	data := struct{ Name string }{Name: "Test"}

	err = r.Render(context.Background(), tmpl, data, "/test/anypath.txt", "0644", false)
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
	expectedErr := errors.New("failed to write file via CopyContent")

	mockConn.CopyContentFunc = func(ctx context.Context, content []byte, dPath string, opts *connector.FileTransferOptions) error {
		return expectedErr
	}

	err = r.Render(context.Background(), tmpl, data, "/test/remote.txt", "0600", false)
	if !errors.Is(err, expectedErr) { // Check if the error is the one we returned or wraps it
		t.Fatalf("Render() error = %v, want %v", err, expectedErr)
	}
}

func TestRunner_Render_NilTemplate(t *testing.T) {
	r, _ := newTestRunnerForTemplate(t)
	err := r.Render(context.Background(), nil, nil, "path", "perms", false)
	if err == nil {
		t.Fatal("Render() with nil template expected error, got nil")
	}
	if !strings.Contains(err.Error(), "template cannot be nil") {
		t.Errorf("Error message mismatch: got %v", err)
	}
}
