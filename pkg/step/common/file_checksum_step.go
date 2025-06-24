package common

import (
	"crypto/md5" // Added for md5 support
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step" // For step.Step and step.StepContext
)

// FileChecksumStep verifies the checksum of a local file.
type FileChecksumStep struct {
	meta              spec.StepMeta
	FilePath          string
	ExpectedChecksum  string
	ChecksumAlgorithm string // e.g., "sha256", "md5"
}

// NewFileChecksumStep creates a new FileChecksumStep.
// instanceName is optional. Algorithm defaults to "sha256" if empty and checksum is provided.
func NewFileChecksumStep(instanceName, filePath, expectedChecksum, algorithm string) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = fmt.Sprintf("VerifyChecksum-%s", filepath.Base(filePath)) // filepath is not imported, using a simpler name
	}
	if algorithm == "" && expectedChecksum != "" {
		algorithm = "sha256" // Default algorithm
	}
	return &FileChecksumStep{
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Verifies %s checksum of file %s", algorithm, filePath),
		},
		FilePath:          filePath,
		ExpectedChecksum:  expectedChecksum,
		ChecksumAlgorithm: algorithm,
	}
}

func (s *FileChecksumStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck for FileChecksumStep:
// Returns true (skip Run) if the file does not exist, as checksum cannot be verified.
// Otherwise, returns false, indicating Run should proceed to verify.
func (s *FileChecksumStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if _, err := os.Stat(s.FilePath); os.IsNotExist(err) {
		logger.Warnf("File %s does not exist, cannot verify checksum. Skipping step.", s.FilePath)
		return true, nil // Skip Run if file doesn't exist
	} else if err != nil {
		logger.Errorf("Error stating file %s for checksum precheck: %v", s.FilePath, err)
		return false, fmt.Errorf("failed to stat file %s for checksum precheck: %w", s.FilePath, err)
	}
	logger.Infof("File %s exists, proceeding to checksum verification in Run phase.", s.FilePath)
	return false, nil // Proceed to Run
}

func (s *FileChecksumStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.ExpectedChecksum == "" {
		logger.Info("No expected checksum provided. Step considered successful.", "file", s.FilePath)
		return nil
	}
	if s.ChecksumAlgorithm == "" {
		return fmt.Errorf("checksum algorithm cannot be empty when expected checksum is provided for file %s", s.FilePath)
	}

	file, err := os.Open(s.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s for checksum: %w", s.FilePath, err)
	}
	defer file.Close()

	var h hash.Hash
	algoLower := strings.ToLower(s.ChecksumAlgorithm)
	switch algoLower {
	case "sha256":
		h = sha256.New()
	case "md5":
		h = md5.New()
	default:
		return fmt.Errorf("unsupported checksum algorithm '%s' for file %s", s.ChecksumAlgorithm, s.FilePath)
	}

	if _, err := io.Copy(h, file); err != nil {
		return fmt.Errorf("failed to read file %s for checksum calculation: %w", s.FilePath, err)
	}

	calculatedChecksum := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(calculatedChecksum, s.ExpectedChecksum) {
		return fmt.Errorf("checksum mismatch for %s: algorithm %s, expected %s, got %s",
			s.FilePath, s.ChecksumAlgorithm, s.ExpectedChecksum, calculatedChecksum)
	}

	logger.Infof("Checksum verified successfully for %s.", s.FilePath)
	return nil
}

// Rollback for FileChecksumStep is a no-op as it doesn't change system state.
func (s *FileChecksumStep) Rollback(ctx step.StepContext, host connector.Host) error {
	ctx.GetLogger().Info("FileChecksumStep has no rollback action.", "step", s.meta.Name, "host", host.GetName())
	return nil
}

var _ step.Step = (*FileChecksumStep)(nil)
