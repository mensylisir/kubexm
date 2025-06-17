package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path
)

// Download a file from a URL to a specified destination on the remote host.
func (r *Runner) Download(ctx context.Context, url, destPath string, sudo bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}

	// Prefer curl, fallback to wget.
	// A more robust solution might involve checking for command existence using LookPath first.
	// For now, try curl and if it fails in a way that suggests it's not found, consider wget.
	// However, differentiating "command not found" from other errors via Exec output alone is tricky.
	// The current connector.CommandError doesn't explicitly give a "not found" type.
	// We rely on LookPath being used by higher level logic if specific tool selection is critical.

	// Default to curl
	// -sSL: silent, show error, follow redirects, insecure (for self-signed certs, common in internal nets)
	// Consider making -k (insecure) an option if strict SSL is needed.
	cmd := fmt.Sprintf("curl -sSL -o %s %s", destPath, url)
	curlPath, errLookPathCurl := r.LookPath(ctx, "curl")

	if errLookPathCurl != nil { // curl not found, try wget
		wgetPath, errLookPathWget := r.LookPath(ctx, "wget")
		if errLookPathWget != nil {
			return fmt.Errorf("neither curl nor wget found on the remote host: curl_err=%v, wget_err=%v", errLookPathCurl, errLookPathWget)
		}
		cmd = fmt.Sprintf("wget -qO %s %s", destPath, url) // -q: quiet, -O: output to file
		t_wget_path := wgetPath // To satisfy linter if a variable is not used, though it is implicitly.
		_ = t_wget_path
	} else {
		t_curl_path := curlPath // To satisfy linter
		_ = t_curl_path
	}


	// Execute the download command
	// No specific timeout here, relies on context or default Exec behavior.
	// Could use RunWithOptions if specific timeout/retries for download are needed.
	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to download %s to %s using command '%s': %w (stderr: %s)", url, destPath, cmd, err, string(stderr))
	}
	return nil
}

// Extract an archive file on the remote host to a destination directory.
// Supports .tar, .tar.gz, .tgz, .zip.
func (r *Runner) Extract(ctx context.Context, archivePath, destDir string, sudo bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}

	var cmd string
	archiveFilename := filepath.Base(archivePath) // Get filename for suffix matching

	// Ensure destination directory exists (best effort, user might want to manage this)
	// Could use Mkdirp here if it's desired for Extract to also ensure destDir.
	// For now, assume destDir exists or tar/unzip can handle its creation if needed.

	switch {
	case strings.HasSuffix(archiveFilename, ".tar.gz") || strings.HasSuffix(archiveFilename, ".tgz"):
		// -x: extract, -z: gzip, -f: file, -C: change to directory
		cmd = fmt.Sprintf("tar -xzf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".tar"):
		cmd = fmt.Sprintf("tar -xf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".zip"):
		// -o: overwrite files without prompting
		// -d: extract files into exdir
		// Ensure unzip is available.
		if _, errLk := r.LookPath(ctx, "unzip"); errLk != nil {
			return fmt.Errorf("unzip command not found on remote host, required for .zip files: %w", errLk)
		}
		cmd = fmt.Sprintf("unzip -o %s -d %s", archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format for file: %s (supported: .tar, .tar.gz, .tgz, .zip)", archiveFilename)
	}

	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to extract %s to %s using command '%s': %w (stderr: %s)", archivePath, destDir, cmd, err, string(stderr))
	}
	return nil
}

// DownloadAndExtract combines downloading a file and then extracting it.
func (r *Runner) DownloadAndExtract(ctx context.Context, url, destDir string, sudo bool /* sudo for download and extract */) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}

	// Determine a temporary path for the downloaded archive on the remote host.
	// Using /tmp or a configurable temp directory in Runner's facts/config would be better.
	// For now, derive from URL base name and put in /tmp.
	archiveName := filepath.Base(url)
	// Ensure archiveName is somewhat safe for a filepath. This is a basic sanitization.
	safeArchiveName := strings.ReplaceAll(archiveName, "/", "_")
	safeArchiveName  = strings.ReplaceAll(safeArchiveName, "..", "_")


	remoteTempPath := filepath.Join("/tmp", safeArchiveName)


	// Download the file
	if err := r.Download(ctx, url, remoteTempPath, sudo); err != nil {
		return fmt.Errorf("download phase of DownloadAndExtract failed for URL %s to %s: %w", url, remoteTempPath, err)
	}

	// Defer cleanup of the downloaded archive.
	// The sudo for Remove should ideally match the sudo for Download,
	// as the temp file might be owned by root if downloaded with sudo.
	defer func() {
		// Use a new context for cleanup if the original one is done.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Short timeout for cleanup
		defer cancel()
		if err := r.Remove(cleanupCtx, remoteTempPath, sudo); err != nil {
			// Log cleanup error, but don't let it override the main function's error.
			// A logger associated with the runner would be good here.
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup temporary archive %s: %v\n", remoteTempPath, err)
		}
	}()

	// Ensure the destination directory exists before extraction.
	// This makes Extract more reliable.
	if err := r.Mkdirp(ctx, destDir, "0755", sudo); err != nil { // Default permissions for dest dir
		return fmt.Errorf("failed to create destination directory %s for extraction: %w", destDir, err)
	}


	// Extract the archive
	if err := r.Extract(ctx, remoteTempPath, destDir, sudo); err != nil {
		return fmt.Errorf("extraction phase of DownloadAndExtract failed for archive %s to %s: %w", remoteTempPath, destDir, err)
	}

	return nil
}
