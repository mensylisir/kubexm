package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Download a file from a URL to a specified destination on the remote host.
func (r *defaultRunner) Download(ctx context.Context, conn connector.Connector, facts *Facts, url, destPath string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	// Prefer curl, fallback to wget.
	cmd := fmt.Sprintf("curl -sSL -o %s %s", destPath, url)
	curlPath, errLookPathCurl := r.LookPath(ctx, conn, "curl") // Use r.LookPath

	if errLookPathCurl != nil { // curl not found, try wget
		wgetPath, errLookPathWget := r.LookPath(ctx, conn, "wget") // Use r.LookPath
		if errLookPathWget != nil {
			return fmt.Errorf("neither curl nor wget found on the remote host: curl_err=%v, wget_err=%v", errLookPathCurl, errLookPathWget)
		}
		cmd = fmt.Sprintf("wget -qO %s %s", destPath, url)
		_ = wgetPath // Suppress unused variable warning if not further used
	} else {
		_ = curlPath // Suppress unused variable warning
	}

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo}) // Use r.RunWithOptions
	if err != nil {
		// Stderr is included in the error by RunWithOptions if it's a CommandError
		return fmt.Errorf("failed to download %s to %s using command '%s': %w", url, destPath, cmd, err)
	}
	return nil
}

// Extract an archive file on the remote host to a destination directory.
func (r *defaultRunner) Extract(ctx context.Context, conn connector.Connector, facts *Facts, archivePath, destDir string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	var cmd string
	archiveFilename := filepath.Base(archivePath)

	switch {
	case strings.HasSuffix(archiveFilename, ".tar.gz") || strings.HasSuffix(archiveFilename, ".tgz"):
		cmd = fmt.Sprintf("tar -xzf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".tar"):
		cmd = fmt.Sprintf("tar -xf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".zip"):
		if _, errLk := r.LookPath(ctx, conn, "unzip"); errLk != nil { // Use r.LookPath
			return fmt.Errorf("unzip command not found on remote host, required for .zip files: %w", errLk)
		}
		cmd = fmt.Sprintf("unzip -o %s -d %s", archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format for file: %s (supported: .tar, .tar.gz, .tgz, .zip)", archiveFilename)
	}

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo}) // Use r.RunWithOptions
	if err != nil {
		return fmt.Errorf("failed to extract %s to %s using command '%s': %w", archivePath, destDir, cmd, err)
	}
	return nil
}

// DownloadAndExtract combines downloading a file and then extracting it.
func (r *defaultRunner) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *Facts, url, destDir string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	archiveName := filepath.Base(url)
	safeArchiveName := strings.ReplaceAll(archiveName, "/", "_")
	safeArchiveName = strings.ReplaceAll(safeArchiveName, "..", "_")

	// Determine temp path based on facts if possible, otherwise default to /tmp
	// This requires facts to be non-nil. The interface implies facts would be passed.
	// If facts or facts.OS is nil, a sensible default like /tmp is used.
	// This part is simplified; a real implementation might use a more specific temp dir from facts or config.
	remoteTempPath := filepath.Join("/tmp", safeArchiveName)
	if facts != nil && facts.OS != nil {
		// Could use OS-specific temp dirs if needed, e.g. facts.OS.TempDir
	}


	if err := r.Download(ctx, conn, facts, url, remoteTempPath, sudo); err != nil { // Pass facts
		return fmt.Errorf("download phase of DownloadAndExtract failed for URL %s to %s: %w", url, remoteTempPath, err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := r.Remove(cleanupCtx, conn, remoteTempPath, sudo); err != nil { // Use r.Remove
			// Consider logging this error if a logger is available via ctx or r.
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup temporary archive %s: %v\n", remoteTempPath, err)
		}
	}()

	if err := r.Mkdirp(ctx, conn, destDir, "0755", sudo); err != nil { // Use r.Mkdirp
		return fmt.Errorf("failed to create destination directory %s for extraction: %w", destDir, err)
	}

	if err := r.Extract(ctx, conn, facts, remoteTempPath, destDir, sudo); err != nil { // Pass facts
		return fmt.Errorf("extraction phase of DownloadAndExtract failed for archive %s to %s: %w", remoteTempPath, destDir, err)
	}

	return nil
}
