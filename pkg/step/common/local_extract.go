package common

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExtractArchiveStep struct {
	step.Base
	SourceArchivePath         string
	DestinationDir            string
	RemoveArchiveAfterExtract bool
	ExpectedFiles             []string
}

type ExtractArchiveStepBuilder struct {
	step.Builder[ExtractArchiveStepBuilder, *ExtractArchiveStep]
}

func NewExtractArchiveStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceArchivePath, destinationDir string) *ExtractArchiveStepBuilder {
	cs := &ExtractArchiveStep{
		DestinationDir:    destinationDir,
		SourceArchivePath: sourceArchivePath,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Extract %s to [%s]", instanceName, sourceArchivePath, destinationDir)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(ExtractArchiveStepBuilder).Init(cs)
}

func (b *ExtractArchiveStepBuilder) WithSourceArchivePath(sourceArchivePath string) *ExtractArchiveStepBuilder {
	b.Step.SourceArchivePath = sourceArchivePath
	return b
}

func (b *ExtractArchiveStepBuilder) WithDestinationDir(destinationDir string) *ExtractArchiveStepBuilder {
	b.Step.DestinationDir = destinationDir
	return b
}

func (b *ExtractArchiveStepBuilder) WithRemoveArchiveAfterExtract(removeArchiveAfterExtract bool) *ExtractArchiveStepBuilder {
	b.Step.RemoveArchiveAfterExtract = removeArchiveAfterExtract
	return b
}

func (b *ExtractArchiveStepBuilder) WithExpectedFiles(expectedFiles []string) *ExtractArchiveStepBuilder {
	b.Step.ExpectedFiles = expectedFiles
	return b
}

func (s *ExtractArchiveStep) Meta() *spec.StepMeta {
	return &s.GetBase().Meta
}

func (s *ExtractArchiveStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.GetBase().Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if _, err := os.Stat(s.DestinationDir); os.IsNotExist(err) {
		logger.Info("Destination directory does not exist, extraction required.", "path", s.DestinationDir)
		return false, nil
	} else if err != nil {
		logger.Error(err, "Failed to stat destination directory, extraction will be attempted.", "path", s.DestinationDir)
		return false, nil
	}

	if len(s.ExpectedFiles) > 0 {
		for _, relPath := range s.ExpectedFiles {
			expectedFilePath := filepath.Join(s.DestinationDir, relPath)
			if _, err := os.Stat(expectedFilePath); os.IsNotExist(err) {
				logger.Info("Expected file not found in destination directory, extraction required.", "path", expectedFilePath)
				return false, nil
			} else if err != nil {
				logger.Warn("Failed to stat expected file, assuming extraction needed.", "path", expectedFilePath, "error", err)
				return false, nil
			}
		}
		logger.Info("All expected files found in destination directory. Extraction step will be skipped.")
		return true, nil
	}
	logger.Info("Destination directory exists, but no specific files checked. Assuming extraction might be needed or re-extraction is safe.", "path", s.DestinationDir)
	return false, nil
}

func (s *ExtractArchiveStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.GetBase().Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if s.Sudo {
		logger.Warn("Sudo is true for ExtractArchiveStep. This is unusual for control-node work_dir operations.")
	}

	logger.Info("Ensuring destination directory exists.", "path", s.DestinationDir)
	if err := os.MkdirAll(s.DestinationDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", s.DestinationDir, err)
	}

	logger.Info("Opening archive file.", "path", s.SourceArchivePath)
	file, err := os.Open(s.SourceArchivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", s.SourceArchivePath, err)
	}
	defer file.Close()

	archiveLower := strings.ToLower(s.SourceArchivePath)

	var reader io.Reader = file
	if strings.HasSuffix(archiveLower, ".tar.gz") || strings.HasSuffix(archiveLower, ".tgz") {
		gzr, errGzip := gzip.NewReader(file)
		if errGzip != nil {
			return fmt.Errorf("failed to create gzip reader for %s: %w", s.SourceArchivePath, errGzip)
		}
		defer gzr.Close()
		reader = gzr
	}

	if strings.Contains(archiveLower, ".tar") {
		return s.extractTar(ctx, tar.NewReader(reader))
	}
	if strings.HasSuffix(archiveLower, ".zip") {
		fi, errStat := file.Stat()
		if errStat != nil {
			return fmt.Errorf("failed to stat archive file %s for zip processing: %w", s.SourceArchivePath, errStat)
		}
		return s.extractZip(ctx, file, fi.Size())
	}
	return fmt.Errorf("unsupported archive format for %s", s.SourceArchivePath)
}

func (s *ExtractArchiveStep) extractTar(ctx runtime.ExecutionContext, tarReader *tar.Reader) error {
	ctx.GetLogger().Info("Extracting tar archive contents.", "source", s.SourceArchivePath, "destination", s.DestinationDir)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header from %s: %w", s.SourceArchivePath, err)
		}

		targetPath := filepath.Join(s.DestinationDir, header.Name)
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(s.DestinationDir)+string(os.PathSeparator)) && targetPath != filepath.Clean(s.DestinationDir) {
			return fmt.Errorf("tar entry %s attempts to escape destination directory %s", header.Name, s.DestinationDir)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s during tar extraction: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for tar file %s: %w", targetPath, err)
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s during tar extraction: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write to file %s during tar extraction: %w", targetPath, err)
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s during tar extraction: %w", targetPath, header.Linkname, err)
			}
		default:
			ctx.GetLogger().Debug("Skipping unsupported tar entry type.", "name", header.Name, "type", header.Typeflag)
		}
	}
	ctx.GetLogger().Info("Tar archive extracted successfully.", "destination", s.DestinationDir)
	if s.RemoveArchiveAfterExtract {
		ctx.GetLogger().Info("Removing source tar archive after extraction.", "path", s.SourceArchivePath)
		if err := os.Remove(s.SourceArchivePath); err != nil {
			ctx.GetLogger().Warn("Failed to remove source tar archive post-extraction.", "path", s.SourceArchivePath, "error", err)
		}
	}
	return nil
}

func (s *ExtractArchiveStep) extractZip(ctx runtime.ExecutionContext, zipFile *os.File, size int64) error {
	logger := ctx.GetLogger()
	logger.Info("Extracting zip archive contents.", "source", s.SourceArchivePath, "destination", s.DestinationDir)

	zipReader, err := zip.NewReader(zipFile, size)
	if err != nil {
		return fmt.Errorf("failed to open zip reader for %s: %w", s.SourceArchivePath, err)
	}
	for _, f := range zipReader.File {
		filePath := filepath.Join(s.DestinationDir, f.Name)
		if !strings.HasPrefix(filePath, filepath.Clean(s.DestinationDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry '%s' attempts to escape destination directory '%s'", f.Name, s.DestinationDir)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory %s during zip extraction: %w", filePath, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create parent directory for zip file %s: %w", filePath, err)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file %s during zip extraction: %w", filePath, err)
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return fmt.Errorf("failed to open file in zip archive '%s': %w", f.Name, err)
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()

		if err != nil {
			return fmt.Errorf("failed to write to file %s during zip extraction: %w", filePath, err)
		}
	}

	logger.Info("Zip archive extracted successfully.", "destination", s.DestinationDir)
	if s.RemoveArchiveAfterExtract {
		logger.Info("Removing source zip archive after extraction.", "path", s.SourceArchivePath)
		if err := os.Remove(s.SourceArchivePath); err != nil {
			logger.Warn("Failed to remove source zip archive post-extraction.", "path", s.SourceArchivePath, "error", err)
		}
	}

	return nil
}

func (s *ExtractArchiveStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.GetBase().Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove destination directory for rollback (best-effort).", "path", s.DestinationDir)
	err := os.RemoveAll(s.DestinationDir)
	if err != nil {
		logger.Error(err, "Failed to remove destination directory during rollback.", "path", s.DestinationDir)
		return fmt.Errorf("failed to remove %s during rollback: %w", s.DestinationDir, err)
	}
	logger.Info("Destination directory removed or was not present.", "path", s.DestinationDir)
	return nil
}

var _ step.Step = (*ExtractArchiveStep)(nil)
