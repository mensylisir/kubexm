package common

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type FileChecksumStep struct {
	step.Base
	FilePath          string
	ExpectedChecksum  string
	ChecksumAlgorithm string
}

type FileChecksumStepBuilder struct {
	step.Builder[FileChecksumStepBuilder, *FileChecksumStep]
}

func NewFileChecksumStepBuilder(ctx runtime.ExecutionContext, instanceName, filePath string) *FileChecksumStepBuilder {
	cs := &FileChecksumStep{
		FilePath: filePath,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Checksum [%s]", instanceName, filePath)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(FileChecksumStepBuilder).Init(cs)
}

func (b *FileChecksumStepBuilder) WithExpectedChecksum(expectedChecksum string) *FileChecksumStepBuilder {
	b.Step.ExpectedChecksum = expectedChecksum
	return b
}

func (b *FileChecksumStepBuilder) WithChecksumAlgorithm(checksumAlgorithm string) *FileChecksumStepBuilder {
	b.Step.ChecksumAlgorithm = checksumAlgorithm
	return b
}

func (s *FileChecksumStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *FileChecksumStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	if _, err := os.Stat(s.FilePath); os.IsNotExist(err) {
		logger.Warnf("File %s does not exist, cannot verify checksum. Skipping step.", s.FilePath)
		return false, fmt.Errorf("file to be checked does not exist: %s", s.FilePath)
	} else if err != nil {
		logger.Errorf("Error stating file %s for checksum precheck: %v", s.FilePath, err)
		return false, fmt.Errorf("failed to stat file %s for checksum precheck: %w", s.FilePath, err)
	}
	logger.Infof("File %s exists, proceeding to checksum verification in Run phase.", s.FilePath)
	return false, nil
}

func (s *FileChecksumStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	if s.ExpectedChecksum == "" {
		logger.Info("No expected checksum provided. Step considered successful.", "file", s.FilePath)
		result.MarkCompleted("No expected checksum provided, step skipped")
		return result, nil
	}
	if s.ChecksumAlgorithm == "" {
		err := fmt.Errorf("checksum algorithm cannot be empty when expected checksum is provided for file %s", s.FilePath)
		result.MarkFailed(err, err.Error())
		return result, err
	}

	file, err := os.Open(s.FilePath)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to open file %s for checksum: %v", s.FilePath, err))
		return result, err
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
		err := fmt.Errorf("unsupported checksum algorithm '%s' for file %s", s.ChecksumAlgorithm, s.FilePath)
		result.MarkFailed(err, err.Error())
		return result, err
	}
	if _, err := io.Copy(h, file); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to read file %s for checksum calculation: %v", s.FilePath, err))
		return result, err
	}

	calculatedChecksum := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(calculatedChecksum, s.ExpectedChecksum) {
		err := fmt.Errorf("checksum mismatch for %s: algorithm %s, expected %s, got %s",
			s.FilePath, s.ChecksumAlgorithm, s.ExpectedChecksum, calculatedChecksum)
		result.MarkFailed(err, err.Error())
		return result, err
	}

	logger.Infof("Checksum verified successfully for %s.", s.FilePath)
	result.MarkCompleted(fmt.Sprintf("Checksum verified for %s", s.FilePath))
	return result, nil
}

func (s *FileChecksumStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("FileChecksumStep has no rollback action.")
	return nil
}

var _ step.Step = (*FileChecksumStep)(nil)
