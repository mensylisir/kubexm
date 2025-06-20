package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils"
)

// WriteFileStepSpec defines parameters for writing content to a file on a target host.
type WriteFileStepSpec struct {
	spec.StepMeta `json:",inline"`

	Destination string `json:"destination,omitempty"` // Full path to the destination file
	Content     string `json:"content,omitempty"`     // Content to write
	Permissions string `json:"permissions,omitempty"` // e.g., "0644"
	Owner       string `json:"owner,omitempty"`       // e.g., "user:group"
	Encoding    string `json:"encoding,omitempty"`    // Optional: e.g., "base64". If set, content is decoded before writing.
	// Checksum    string `json:"checksum,omitempty"` // Optional: sha256 checksum of the content to verify after write or in precheck.
}

// NewWriteFileStepSpec creates a new WriteFileStepSpec.
func NewWriteFileStepSpec(name, description, destination, content, permissions, owner, encoding string) *WriteFileStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Write file %s", destination)
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Writes content to file %s", destination)
		if permissions != "" {
			finalDescription += fmt.Sprintf(" with permissions %s", permissions)
		}
		if owner != "" {
			finalDescription += fmt.Sprintf(" and owner %s", owner)
		}
		if encoding != "" {
			finalDescription += fmt.Sprintf(" (encoding: %s)", encoding)
		}
	}

	return &WriteFileStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Destination: destination,
		Content:     content,
		Permissions: permissions,
		Owner:       owner,
		Encoding:    encoding,
	}
}

// Name returns the step's name.
func (s *WriteFileStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *WriteFileStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *WriteFileStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *WriteFileStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *WriteFileStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *WriteFileStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Precheck checks if the file exists and optionally if its content matches.
func (s *WriteFileStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.Destination == "" {
		return false, fmt.Errorf("Destination path must be specified for WriteFileStep: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.Destination)
	if err != nil {
		logger.Warn("Failed to check if destination file exists, will attempt write.", "path", s.Destination, "error", err)
		return false, nil
	}

	if !exists {
		logger.Info("Destination file does not exist. Write needed.", "path", s.Destination)
		return false, nil
	}

	// File exists, check content if we want to be thorough.
	// This requires reading the remote file and comparing its hash to the hash of s.Content.
	// For simplicity, if the file exists, we could assume it's correct or let Run overwrite.
	// A more robust check would compare content hash.
	logger.Info("Destination file already exists. WriteFileStep will overwrite.", "path", s.Destination)
	// To make it idempotent based on content, we would calculate hash of s.Content,
	// then calculate hash of remote s.Destination, and if they match, return true.
	// Example:
	// currentContentSHA256, err := utils.CalculateRemoteFileChecksum(ctx.GoContext(), conn, s.Destination, "sha256")
	// if err == nil {
	//    hasher := sha256.New()
	//    hasher.Write([]byte(s.Content)) // Consider decoding if s.Encoding is base64
	//    expectedSHA256 := hex.EncodeToString(hasher.Sum(nil))
	//    if currentContentSHA256 == expectedSHA256 {
	//        logger.Info("Destination file exists and content matches. Skipping write.")
	//        return true, nil
	//    }
	//    logger.Info("Destination file exists but content differs. Write needed.")
	// } else {
	//    logger.Warn("Could not calculate checksum of remote file, will proceed with write.", "error", err)
	// }

	return false, nil // Default to re-running to ensure content and permissions.
}

// Run executes writing the file.
func (s *WriteFileStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.Destination == "" {
		return fmt.Errorf("Destination path must be specified for WriteFileStep: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	contentToWrite := s.Content
	if strings.ToLower(s.Encoding) == "base64" {
		decodedBytes, err := utils.DecodeBase64(s.Content)
		if err != nil {
			return fmt.Errorf("failed to decode base64 content for %s: %w", s.Destination, err)
		}
		contentToWrite = string(decodedBytes)
	}

	// Determine sudo requirement for writing file (based on destination dir usually)
	// The connector.CopyContent should ideally handle sudo internally if needed for the temp path and final move.
	// For now, assume path requires sudo if utils.PathRequiresSudo says so.
	// This might influence temp locations chosen by CopyContent or if direct write is attempted.
	// However, CopyContent typically writes to a temp file then uses 'mv' with sudo.

	logger.Info("Writing file.", "destination", s.Destination)
	err = conn.CopyContent(ctx.GoContext(), contentToWrite, s.Destination, connector.FileStat{
		Permissions: s.Permissions, // Let CopyContent handle chmod
		Owner:       s.Owner,       // Let CopyContent handle chown
		Sudo:        utils.PathRequiresSudo(s.Destination), // Hint for CopyContent
	})

	if err != nil {
		return fmt.Errorf("failed to write content to %s on host %s: %w", s.Destination, host.GetName(), err)
	}

	// conn.CopyContent should handle permissions and owner if its FileStat is comprehensive.
	// If not, or for more explicit control:
	// execOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.Destination)}
	// if s.Permissions != "" { ... chmod ... }
	// if s.Owner != "" { ... chown ... }

	logger.Info("File written successfully.", "destination", s.Destination)
	return nil
}

// Rollback removes the written file.
func (s *WriteFileStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	if s.Destination == "" {
		logger.Info("Destination path is empty, nothing to roll back.")
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove written file.", "path", s.Destination)
	rmCmd := fmt.Sprintf("rm -f %s", s.Destination)
	rmOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.Destination)}
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, rmOpts)

	if errRm != nil {
		logger.Error("Failed to remove written file during rollback (best effort).", "path", s.Destination, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Written file removed successfully.", "path", s.Destination)
	}
	return nil
}

var _ step.Step = (*WriteFileStepSpec)(nil)

// Helper for sha256 - kept for reference if Precheck content check is implemented
func calculateSHA256(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}
