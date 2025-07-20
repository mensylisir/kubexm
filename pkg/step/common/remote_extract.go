package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

type ExtractArchiverStep struct {
	step.Base
	ExtractionDir             string
	ArchiveType               string
	PreserveOriginalArchive   bool
	RemoveExtractedOnRollback bool
}

type ExtractArchiverStepBuilder struct {
	step.Builder[ExtractArchiverStepBuilder, *ExtractArchiverStep]
}

func NewExtractArchiverStepBuilder(instanceName, extractionDir string) *ExtractArchiverStepBuilder {
	cs := &ExtractArchiverStep{
		ExtractionDir: extractionDir,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract to [%s]", instanceName, extractionDir)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(ExtractArchiverStepBuilder).Init(cs)
}

func (b *ExtractArchiverStepBuilder) WithRecursive(archiveType string) *ExtractArchiverStepBuilder {
	b.Step.ArchiveType = archiveType
	return b
}

func (b *ExtractArchiverStepBuilder) WithPreserveOriginalArchive(preserveOriginalArchive bool) *ExtractArchiverStepBuilder {
	b.Step.PreserveOriginalArchive = preserveOriginalArchive
	return b
}

func (b *ExtractArchiverStepBuilder) WithRemoveExtractedOnRollback(removeExtractedOnRollback bool) *ExtractArchiverStepBuilder {
	b.Step.RemoveExtractedOnRollback = removeExtractedOnRollback
	return b
}

func (s *ExtractArchiverStep) Meta() *spec.StepMeta {
	return &s.GetBase().Meta
}

func (s *ExtractArchiverStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.GetBase().Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if s.ExtractionDir == "" {
		return false, fmt.Errorf("ExtractionDir not set for step %s on host %s", s.GetBase().Meta.Name, ctx.GetHost().GetName())
	}
	extractKeyVal, ok := ctx.GetFromRuntimeConfig("extract_path_key")
	if !ok {
		logger.Debug("No 'extract_path_key' in RuntimeConfig. Precheck assumes not done.")
		return false, nil
	}
	extractKey, isString := extractKeyVal.(string)
	if !isString || extractKey == "" {
		return false, nil
	}

	extractedPathVal, pathOk := ctx.GetTaskCache().Get(extractKey)
	if !pathOk {
		logger.Debug("Extracted path key not found in cache.", "key", extractKey)
		return false, nil
	}

	extractedPath, isString := extractedPathVal.(string)
	if !isString || extractedPath == "" {
		logger.Debug("Extracted path in cache is invalid.", "key", extractKey, "value", extractedPathVal)
		return false, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", ctx.GetHost().GetName(), s.GetBase().Meta.Name, err)
	}

	exists, errCheck := runnerSvc.Exists(ctx.GoContext(), conn, extractedPath)
	if errCheck != nil {
		logger.Warn("Failed to check existence of extracted path from cache.", "path", extractedPath, "error", errCheck)
		return false, nil
	}
	if exists {
		logger.Info("Extracted path found in cache and exists on disk. Step will be skipped.", "path", extractedPath)
		return true, nil
	}
	logger.Info("Extracted path was in cache but does not exist on disk. Re-extraction is needed.", "path", extractedPath)
	return false, nil
}

func (s *ExtractArchiverStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.GetBase().Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	inputKeyVal, ok := ctx.GetFromRuntimeConfig("download_path_key")
	if !ok {
		return fmt.Errorf("'download_path_key' is required but not provided in RuntimeConfig")
	}
	inputKey, isString := inputKeyVal.(string)
	if !isString || inputKey == "" {
		return fmt.Errorf("invalid 'download_path_key' in RuntimeConfig: expected a non-empty string, got %T", inputKeyVal)
	}

	archivePathVal, archiveOk := ctx.GetTaskCache().Get(inputKey)
	if !archiveOk {
		return fmt.Errorf("archive path not found in Task Cache using key '%s' for step %s on host %s", inputKey, s.GetBase().Meta.Name, ctx.GetHost().GetName())
	}
	archivePath, okStr := archivePathVal.(string)
	if !okStr || archivePath == "" {
		return fmt.Errorf("invalid or empty archive path in Task Cache using key '%s' for step %s on host %s", inputKey, s.GetBase().Meta.Name, ctx.GetHost().GetName())
	}

	if s.ExtractionDir == "" {
		return fmt.Errorf("ExtractionDir not set for step %s on host %s", s.GetBase().Meta.Name, ctx.GetHost().GetName())
	}
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", ctx.GetHost().GetName(), s.GetBase().Meta.Name, err)
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts for %s for step %s: %w", ctx.GetHost().GetName(), s.GetBase().Meta.Name, err)
	}

	logger.Info("Ensuring extraction directory exists", "path", s.ExtractionDir)
	if errMkdir := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.ExtractionDir, "0755", s.Sudo); errMkdir != nil {
		return fmt.Errorf("failed to create extraction directory %s for step %s on host %s: %w", s.ExtractionDir, s.GetBase().Meta.Name, ctx.GetHost().GetName(), errMkdir)
	}

	logger.Info("Extracting archive", "archive", archivePath, "destination", s.ExtractionDir)
	if errExtract := runnerSvc.Extract(ctx.GoContext(), conn, facts, archivePath, s.ExtractionDir, s.Sudo, s.PreserveOriginalArchive); errExtract != nil {
		return fmt.Errorf("failed to extract archive %s to %s for step %s on host %s: %w", archivePath, s.ExtractionDir, s.GetBase().Meta.Name, ctx.GetHost().GetName(), errExtract)
	}
	logger.Info("Archive extracted successfully.")
	determinedExtractedPath := s.ExtractionDir
	if outputKeyVal, ok := ctx.GetFromRuntimeConfig("extract_path_key"); ok {
		if outputKey, isString := outputKeyVal.(string); isString && outputKey != "" {
			ctx.GetTaskCache().Set(outputKey, determinedExtractedPath)
			logger.Info("Stored extracted path in Task Cache.", "key", outputKey, "path", determinedExtractedPath)
		} else {
			logger.Error(fmt.Errorf("invalid 'extract_path_key' in RuntimeConfig: not a non-empty string"), "configuration error")
		}
	}
	return nil
}

func (s *ExtractArchiverStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.GetBase().Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	if !s.RemoveExtractedOnRollback {
		logger.Info("Rollback requested, but RemoveExtractedOnRollback is false. No action taken.")
		return nil
	}
	if s.ExtractionDir == "" {
		logger.Warn("Cannot perform rollback: ExtractionDir is not set.")
		return nil
	}

	logger.Info("Attempting to remove extracted content directory for rollback.", "path", s.ExtractionDir)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", ctx.GetHost().GetName(), s.GetBase().Meta.Name, err)
	}

	if errRemove := runnerSvc.Remove(ctx.GoContext(), conn, s.ExtractionDir, s.Sudo, true); errRemove != nil {
		logger.Error("Failed to remove extraction directory during rollback.", "path", s.ExtractionDir, "error", errRemove)
	}

	logger.Info("Successfully removed extraction directory for rollback (or removal was skipped/failed non-critically).", "path", s.ExtractionDir)
	return nil
}

var _ step.Step = (*ExtractArchiverStep)(nil)
