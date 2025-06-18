package utils

import (
	"fmt"
	"path/filepath"
	"strings"
	// Assuming runtime.Context is the correct way to pass logger & runner
	"github.com/kubexms/kubexms/pkg/runtime"
)

// DownloadFile downloads a file from a given URL to a target directory with a specific filename.
// It can also optionally verify the checksum of the downloaded file.
func DownloadFile(ctx *runtime.Context, // Assuming runtime.Context is appropriate for logging & runner access
	urlStr string,
	targetDir string,
	targetFilename string,
	sudoDownload bool, // Whether sudo is needed for the download location or Mkdirp
	checksum string,    // Expected checksum string, e.g., "sha256:abcdef123..." or just "abcdef123..."
	checksumType string) (downloadedPath string, err error) {

	if ctx.Host.Runner == nil {
		return "", fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	// Derive logger from context, specific to the host if context provides it.
	// Using SugaredLogger for easier structured logging if available.
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "util", "DownloadFile")

	// Ensure target directory exists
	// The sudo flag for Mkdirp should depend on the permissions of targetDir's parent.
	// If targetDir is /tmp/something, sudo might not be needed for Mkdirp.
	// If targetDir is /opt/kubexms/downloads, sudo might be needed.
	// Assuming sudoDownload implies sudo for Mkdirp as well if needed.
	logger.Infof("Ensuring target directory %s exists.", targetDir)
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, targetDir, "0755", sudoDownload); err != nil {
		return "", fmt.Errorf("failed to create target directory %s on host %s: %w", targetDir, ctx.Host.Name, err)
	}

	fullPath := filepath.Join(targetDir, targetFilename)
	logger.Infof("Attempting to download %s to %s on host %s", urlStr, fullPath, ctx.Host.Name)

	// Assuming runner.Download handles HTTP/HTTPS and writes to fullPath.
	// The sudoDownload flag here applies to the write permission for fullPath.
	if err := ctx.Host.Runner.Download(ctx.GoContext, urlStr, fullPath, sudoDownload); err != nil {
		return "", fmt.Errorf("failed to download from %s to %s on host %s: %w", urlStr, fullPath, ctx.Host.Name, err)
	}
	logger.Successf("Successfully downloaded %s to %s on host %s", urlStr, fullPath, ctx.Host.Name)

	// Checksum verification
	if checksum != "" {
		logger.Infof("Verifying checksum for %s on host %s", fullPath, ctx.Host.Name)

		actualChecksumValue := checksum
		effectiveChecksumType := strings.ToLower(checksumType)

		// Allow "type:value" in checksum string to override/provide type
		if strings.Contains(checksum, ":") {
			parts := strings.SplitN(checksum, ":", 2)
			parsedType := strings.ToLower(parts[0])
			parsedValue := parts[1]

			if effectiveChecksumType != "" && effectiveChecksumType != parsedType {
				return fullPath, fmt.Errorf(
					"checksum type mismatch for %s on host %s: parameter specifies '%s', but checksum string implies '%s'",
					fullPath, ctx.Host.Name, effectiveChecksumType, parsedType)
			}
			effectiveChecksumType = parsedType
			actualChecksumValue = parsedValue
		}

		if effectiveChecksumType == "" { // Default to sha256 if not specified anywhere
			effectiveChecksumType = "sha256"
			logger.Debugf("Checksum type not specified, defaulting to %s for %s", effectiveChecksumType, fullPath)
		}

		var fileHash string
		var verifyErr error

		// This part assumes runner has methods like GetSHA256, GetMD5, etc.
		// Or a generic GetFileChecksum(path, type).
		// As per plan, only sha256 is supported via a specific GetSHA256 for now.
		switch effectiveChecksumType {
		case "sha256":
			// Assuming Runner.GetSHA256(ctx, path) (string, error) exists.
			fileHash, verifyErr = ctx.Host.Runner.GetSHA256(ctx.GoContext, fullPath)
		// Example for future expansion:
		// case "md5":
		// 	fileHash, verifyErr = ctx.Host.Runner.GetMD5(ctx.GoContext, fullPath)
		// case "sha512":
		// 	fileHash, verifyErr = ctx.Host.Runner.GetSHA512(ctx.GoContext, fullPath)
		default:
			verifyErr = fmt.Errorf("unsupported checksum type '%s' for file %s on host %s. Only 'sha256' is currently implemented in this utility",
				effectiveChecksumType, fullPath, ctx.Host.Name)
		}

		if verifyErr != nil {
			// Attempt to remove the downloaded file if checksum verification setup failed.
			ctx.Host.Runner.Remove(ctx.GoContext, fullPath, sudoDownload) // Use sudoDownload for consistency if download needed it
			return fullPath, fmt.Errorf("failed to get %s checksum for %s on host %s: %w. File removed.",
				effectiveChecksumType, fullPath, ctx.Host.Name, verifyErr)
		}

		if !strings.EqualFold(fileHash, actualChecksumValue) { // Case-insensitive compare for hashes
			// Attempt to remove the downloaded file due to checksum mismatch.
			ctx.Host.Runner.Remove(ctx.GoContext, fullPath, sudoDownload)
			return fullPath, fmt.Errorf(
				"checksum mismatch for %s on host %s: expected %s (type %s), got %s. File removed.",
				fullPath, ctx.Host.Name, actualChecksumValue, effectiveChecksumType, fileHash)
		}
		logger.Successf("Checksum verified for %s (type %s) on host %s", fullPath, effectiveChecksumType, ctx.Host.Name)
	}
	return fullPath, nil
}
