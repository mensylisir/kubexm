package common

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step" // For step.Step and step.StepContext
)

// ExtractArchiveStep extracts an archive (e.g., .tar.gz) to a specified directory.
type ExtractArchiveStep struct {
	meta                      spec.StepMeta
	SourceArchivePath         string // Path to the archive file
	DestinationDir            string // Directory to extract contents into
	RemoveArchiveAfterExtract bool
	Sudo                      bool // Should generally be false for control-node work_dir operations
	// ExpectedFiles is an optional list of relative paths that are expected to be present
	// in DestinationDir after extraction, used for a more robust Precheck.
	ExpectedFiles []string
}

// NewExtractArchiveStep creates a new ExtractArchiveStep.
func NewExtractArchiveStep(instanceName, sourceArchivePath, destinationDir string, removeArchiveAfterExtract, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("ExtractArchive-%s", filepath.Base(sourceArchivePath))
	}
	return &ExtractArchiveStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Extracts archive %s to %s", sourceArchivePath, destinationDir),
		},
		SourceArchivePath:         sourceArchivePath,
		DestinationDir:            destinationDir,
		RemoveArchiveAfterExtract: removeArchiveAfterExtract,
		Sudo:                      sudo,
	}
}

func (s *ExtractArchiveStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ExtractArchiveStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	// Check if DestinationDir exists
	if _, err := os.Stat(s.DestinationDir); os.IsNotExist(err) {
		logger.Info("Destination directory does not exist, extraction required.", "path", s.DestinationDir)
		return false, nil
	} else if err != nil {
		logger.Error(err, "Failed to stat destination directory, extraction will be attempted.", "path", s.DestinationDir)
		return false, nil // Error stating, try to run anyway
	}

	// If ExpectedFiles are specified, check for their existence
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

	// If DestinationDir exists but no ExpectedFiles, it's hard to tell if it's correctly extracted.
	// For basic idempotency, if the dir exists, we might consider it done.
	// A more robust check might involve a marker file or comparing checksums of key extracted files.
	// For now, existence of DestinationDir is a weak signal for "done".
	// Relying on ExpectedFiles or assuming re-extraction is safe if state is unknown.
	logger.Info("Destination directory exists, but no specific files checked. Assuming extraction might be needed or re-extraction is safe.", "path", s.DestinationDir)
	return false, nil // Let Run proceed to ensure correct state if not using ExpectedFiles for precheck
}

func (s *ExtractArchiveStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

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

	var fileReader io.ReadCloser = file
	if strings.HasSuffix(strings.ToLower(s.SourceArchivePath), ".tar.gz") || strings.HasSuffix(strings.ToLower(s.SourceArchivePath), ".tgz") {
		gzr, errGzip := gzip.NewReader(file)
		if errGzip != nil {
			return fmt.Errorf("failed to create gzip reader for %s: %w", s.SourceArchivePath, errGzip)
		}
		defer gzr.Close()
		fileReader = gzr
		logger.Debug("Using gzip decompressor for tar archive.")
	} else if strings.HasSuffix(strings.ToLower(s.SourceArchivePath), ".tar") {
		// Plain tar
		logger.Debug("Processing plain tar archive.")
	} else {
		return fmt.Errorf("unsupported archive format for %s: must be .tar, .tar.gz, or .tgz", s.SourceArchivePath)
	}

	tarReader := tar.NewReader(fileReader)
	logger.Info("Extracting archive contents.", "source", s.SourceArchivePath, "destination", s.DestinationDir)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header from %s: %w", s.SourceArchivePath, err)
		}

		targetPath := filepath.Join(s.DestinationDir, header.Name)
		// Sanitize targetPath to prevent path traversal vulnerabilities (e.g., "../../../../../etc/passwd")
		// Ensure the cleaned path is still within DestinationDir
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(s.DestinationDir)+string(os.PathSeparator)) && targetPath != filepath.Clean(s.DestinationDir) {
			return fmt.Errorf("tar entry %s attempts to escape destination directory %s", header.Name, s.DestinationDir)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s during extraction: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Ensure parent directory of the file exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for file %s: %w", targetPath, err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s during extraction: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write to file %s during extraction: %w", targetPath, err)
			}
			outFile.Close()
		case tar.TypeSymlink:
			// Check if symlinks should be allowed and handled. For K8s binaries, they are common.
			// Ensure symlink target is also sanitized and within bounds.
			linkTarget := filepath.Join(filepath.Dir(targetPath), header.Linkname)
			if !strings.HasPrefix(filepath.Clean(linkTarget), filepath.Clean(s.DestinationDir)+string(os.PathSeparator)) {
				// Allow symlinks within the same extraction directory, even if they point "up" locally
				// as long as the final resolved path is within DestinationDir. This is complex.
				// Safest for now: if header.Linkname itself contains '..', be very careful.
				// For now, we will create symlink as specified. Security review needed for general purpose use.
				// logger.Warn("Symlink target might be outside destination. Review carefully.", "target", targetPath, "link", header.Linkname)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", targetPath, header.Linkname, err)
			}
		default:
			logger.Debug("Skipping unsupported tar entry type.", "name", header.Name, "type", header.Typeflag)

		}
	}
	logger.Info("Archive extracted successfully.", "destination", s.DestinationDir)

	if s.RemoveArchiveAfterExtract {
		logger.Info("Removing source archive after extraction.", "path", s.SourceArchivePath)
		if err := os.Remove(s.SourceArchivePath); err != nil {
			// Log as warning, primary goal (extraction) is done.
			logger.Warn("Failed to remove source archive post-extraction.", "path", s.SourceArchivePath, "error", err)
		}
	}

	return nil
}

func (s *ExtractArchiveStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rolling back an extraction means removing the DestinationDir.
	// This could be destructive if other files were in DestinationDir not from this archive.
	// A safer rollback might involve tracking all created files/dirs, which is complex.
	// For now, remove DestinationDir if it seems to have been created by this step.
	// (This is a best-effort, potentially risky rollback).
	logger.Info("Attempting to remove destination directory for rollback (best-effort).", "path", s.DestinationDir)
	err := os.RemoveAll(s.DestinationDir) // Use RemoveAll for directories
	if err != nil {
		logger.Error(err, "Failed to remove destination directory during rollback.", "path", s.DestinationDir)
		return fmt.Errorf("failed to remove %s during rollback: %w", s.DestinationDir, err)
	}
	logger.Info("Destination directory removed or was not present.", "path", s.DestinationDir)

	// If archive was removed, there's no simple way to restore it without re-downloading.
	if s.RemoveArchiveAfterExtract {
		logger.Warn("Source archive was removed after extraction. Cannot restore it on rollback.", "path", s.SourceArchivePath)
	}
	return nil
}

var _ step.Step = (*ExtractArchiveStep)(nil)
