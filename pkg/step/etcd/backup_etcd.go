package etcd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// BackupEtcdStepSpec defines parameters for backing up an etcd instance.
type BackupEtcdStepSpec struct {
	spec.StepMeta `json:",inline"`

	EtcdCtlEndpoint string `json:"etcdCtlEndpoint,omitempty"`
	BackupFilePath  string `json:"backupFilePath,omitempty"`
	EtcdCACertFile  string `json:"etcdCaCertFile,omitempty"` // Optional
	EtcdCertFile    string `json:"etcdCertFile,omitempty"`   // Optional
	EtcdKeyFile     string `json:"etcdKeyFile,omitempty"`    // Optional
	Sudo            bool   `json:"sudo,omitempty"`           // For etcdctl command if needed, and mkdir
}

// NewBackupEtcdStepSpec creates a new BackupEtcdStepSpec.
func NewBackupEtcdStepSpec(name, description, endpoint, backupPath, caFile, certFile, keyFile string) *BackupEtcdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Backup etcd to %s", backupPath)
	}
	finalDescription := description
	// Description will be refined in populateDefaults once endpoint is finalized.

	return &BackupEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		EtcdCtlEndpoint: endpoint,
		BackupFilePath:  backupPath,
		EtcdCACertFile:  caFile,
		EtcdCertFile:    certFile,
		EtcdKeyFile:     keyFile,
	}
}

// Name returns the step's name.
func (s *BackupEtcdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *BackupEtcdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *BackupEtcdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *BackupEtcdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *BackupEtcdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *BackupEtcdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *BackupEtcdStepSpec) populateDefaults(logger runtime.Logger) {
	if s.EtcdCtlEndpoint == "" {
		s.EtcdCtlEndpoint = "http://127.0.0.1:2379" // Default endpoint with http
		logger.Debug("EtcdCtlEndpoint defaulted.", "endpoint", s.EtcdCtlEndpoint)
	}
	// Ensure endpoint starts with http:// or https:// if not already
	if !strings.HasPrefix(s.EtcdCtlEndpoint, "http://") && !strings.HasPrefix(s.EtcdCtlEndpoint, "https://") {
		// Check if it looks like a hostname/IP without protocol (e.g. 127.0.0.1:2379 or etcd.server:2379)
		// A more robust check might involve trying to parse as URL.
		// For now, assume if no protocol, it's http.
		if strings.Contains(s.EtcdCtlEndpoint, ":") { // Basic check for port
			logger.Debug("EtcdCtlEndpoint seems to be missing protocol, prepending http://", "original", s.EtcdCtlEndpoint)
			s.EtcdCtlEndpoint = "http://" + s.EtcdCtlEndpoint
		} else {
			// If no port and no protocol, it's ambiguous. Log a warning.
			logger.Warn("EtcdCtlEndpoint does not specify a protocol (http/https) and is ambiguous.", "endpoint", s.EtcdCtlEndpoint)
		}
	}


	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Backs up etcd data from endpoint %s to %s",
			s.EtcdCtlEndpoint, s.BackupFilePath)
		authParts := []string{}
		if s.EtcdCACertFile != "" { authParts = append(authParts, "CA") }
		if s.EtcdCertFile != "" && s.EtcdKeyFile != "" { authParts = append(authParts, "Cert/Key") }
		if len(authParts) > 0 {
			s.StepMeta.Description += fmt.Sprintf(" using %s authentication.", strings.Join(authParts, " and "))
		}
	}
}

// Precheck ensures etcdctl is available and backup path is provided.
func (s *BackupEtcdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.BackupFilePath == "" {
		return false, fmt.Errorf("BackupFilePath must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), "etcdctl"); err != nil {
		return false, fmt.Errorf("etcdctl command not found on host %s: %w", host.GetName(), err)
	}
	logger.Debug("etcdctl command found on host.")

	// Precheck for backup usually returns false to allow new backup.
	// If BackupFilePath exists, etcdctl snapshot save will overwrite it.
	// We could check if parent dir of BackupFilePath is writable, but that's complex for precheck.
	return false, nil
}

// Run executes the etcdctl snapshot save command.
func (s *BackupEtcdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.BackupFilePath == "" {
		return fmt.Errorf("BackupFilePath must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure directory for BackupFilePath exists
	backupDir := filepath.Dir(s.BackupFilePath)
	// Sudo for mkdir based on path and spec's Sudo field for the overall operation
	mkdirSudo := s.Sudo || utils.PathRequiresSudo(backupDir)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", backupDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, &connector.ExecOptions{Sudo: mkdirSudo})
	if errMkdir != nil {
		return fmt.Errorf("failed to create backup directory %s (stderr: %s) on host %s: %w", backupDir, string(stderrMkdir), host.GetName(), errMkdir)
	}

	// Construct etcdctl command
	cmdArgs := []string{"etcdctl"}
	// Global etcdctl options should come before subcommand
	if s.EtcdCtlEndpoint != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--endpoints=%s", s.EtcdCtlEndpoint))
	}
	if s.EtcdCACertFile != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--cacert=%s", s.EtcdCACertFile))
	}
	if s.EtcdCertFile != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--cert=%s", s.EtcdCertFile))
	}
	if s.EtcdKeyFile != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--key=%s", s.EtcdKeyFile))
	}
	cmdArgs = append(cmdArgs, "snapshot", "save", s.BackupFilePath)

	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Executing etcd snapshot save.", "command", cmd)
	// etcdctl snapshot save might write to a path requiring sudo, or if etcdctl itself needs sudo due to cert access.
	execOpts := &connector.ExecOptions{Sudo: s.Sudo || utils.PathRequiresSudo(s.BackupFilePath)}
	_, stderrSave, errSave := conn.Exec(ctx.GoContext(), cmd, execOpts)
	if errSave != nil {
		return fmt.Errorf("failed to save etcd snapshot to %s (stderr: %s): %w", s.BackupFilePath, string(stderrSave), errSave)
	}

	logger.Info("Etcd snapshot saved successfully.", "path", s.BackupFilePath)
	return nil
}

// Rollback for etcd backup is typically a no-op to preserve backup data.
func (s *BackupEtcdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for etcd backup step is not performed to preserve backup data.")
	// Optionally, could remove s.BackupFilePath if Run failed and file is incomplete,
	// but generally safer to leave it.
	return nil
}

var _ step.Step = (*BackupEtcdStepSpec)(nil)
