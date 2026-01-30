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

func (r *defaultRunner) Download(ctx context.Context, conn connector.Connector, facts *Facts, url, destPath string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	cmd := fmt.Sprintf("curl -sSL -o %s %s", destPath, url)
	curlPath, errLookPathCurl := r.LookPath(ctx, conn, "curl")

	if errLookPathCurl != nil {
		wgetPath, errLookPathWget := r.LookPath(ctx, conn, "wget")
		if errLookPathWget != nil {
			return fmt.Errorf("neither curl nor wget found on the remote host: curl_err=%v, wget_err=%v", errLookPathCurl, errLookPathWget)
		}
		cmd = fmt.Sprintf("wget -qO %s %s", destPath, url)
		_ = wgetPath
	} else {
		_ = curlPath
	}

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to download %s to %s using command '%s': %w", url, destPath, cmd, err)
	}
	return nil
}

func (r *defaultRunner) Extract(ctx context.Context, conn connector.Connector, facts *Facts, archivePath, destDir string, sudo bool, preserveOriginalArchive bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	var cmd string
	archiveFilename := filepath.Base(archivePath)

	switch {
	case strings.HasSuffix(archiveFilename, ".tar.gz") || strings.HasSuffix(archiveFilename, ".tgz"):
		cmd = fmt.Sprintf("tar -xzf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".tar.bz2") || strings.HasSuffix(archiveFilename, ".tbz2"):
		cmd = fmt.Sprintf("tar -xjf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".tar.xz") || strings.HasSuffix(archiveFilename, ".txz"):
		cmd = fmt.Sprintf("tar -xJf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".tar"):
		cmd = fmt.Sprintf("tar -xf %s -C %s", archivePath, destDir)
	case strings.HasSuffix(archiveFilename, ".zip"):
		if _, errLk := r.LookPath(ctx, conn, "unzip"); errLk != nil {
			return fmt.Errorf("unzip command not found on remote host, required for .zip files: %w", errLk)
		}
		cmd = fmt.Sprintf("unzip -o %s -d %s", archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format for file: %s (supported: .tar, .tar.gz, .tgz, .tar.bz2, .tbz2, .tar.xz, .txz, .zip)", archiveFilename)
	}

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to extract %s to %s using command '%s': %w", archivePath, destDir, cmd, err)
	}
	if !preserveOriginalArchive {
		cmd := fmt.Sprintf("rm -f %s", archivePath)
		_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
		if err != nil {
			r.logger.Warn("Extraction successful, but failed to remove original archive.", "file", archivePath, "error", err)
		}
	}
	return nil
}

func (r *defaultRunner) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *Facts, url, destDir string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	archiveName := filepath.Base(url)
	safeArchiveName := strings.ReplaceAll(archiveName, "/", "_")
	safeArchiveName = strings.ReplaceAll(safeArchiveName, "..", "_")

	remoteTempPath := filepath.Join("/tmp", safeArchiveName)
	if facts != nil && facts.OS != nil {
	}

	if err := r.Download(ctx, conn, facts, url, remoteTempPath, sudo); err != nil {
		return fmt.Errorf("download phase of DownloadAndExtract failed for URL %s to %s: %w", url, remoteTempPath, err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := r.Remove(cleanupCtx, conn, remoteTempPath, sudo, true); err != nil {
			r.logger.Errorf("%v Warning: failed to cleanup temporary archive %s: %v\n", os.Stderr, remoteTempPath, err)
		}
	}()

	if err := r.Mkdirp(ctx, conn, destDir, "0755", sudo); err != nil {
		return fmt.Errorf("failed to create destination directory %s for extraction: %w", destDir, err)
	}

	if err := r.Extract(ctx, conn, facts, remoteTempPath, destDir, sudo, false); err != nil {
		return fmt.Errorf("extraction phase of DownloadAndExtract failed for archive %s to %s: %w", remoteTempPath, destDir, err)
	}

	return nil
}

func (r *defaultRunner) Compress(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sources []string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if len(sources) == 0 {
		return fmt.Errorf("no source paths provided for compression")
	}
	baseDir, relativeSources, err := findCommonBase(sources)
	if err != nil {
		return fmt.Errorf("failed to process source paths for compression: %w", err)
	}
	sourcePaths := strings.Join(relativeSources, " ")

	tarChangeDirFlag := ""
	if baseDir != "." {
		tarChangeDirFlag = fmt.Sprintf("-C %s", baseDir)
	}

	archiveFilename := filepath.Base(archivePath)
	var cmd string

	switch {
	case strings.HasSuffix(archiveFilename, ".tar.gz") || strings.HasSuffix(archiveFilename, ".tgz"):
		cmd = fmt.Sprintf("tar %s -czf %s %s", tarChangeDirFlag, archivePath, sourcePaths)
	case strings.HasSuffix(archiveFilename, ".zip"):
		if _, errLk := r.LookPath(ctx, conn, "zip"); errLk != nil {
			return fmt.Errorf("zip command not found: %w", errLk)
		}
		cmd = fmt.Sprintf("zip -r %s %s", archivePath, strings.Join(sources, " "))
	default:
		return fmt.Errorf("unsupported archive format for compression: %s", archiveFilename)
	}

	cmd = strings.TrimSpace(strings.ReplaceAll(cmd, "  ", " "))

	archiveDir := filepath.Dir(archivePath)
	if err := r.Mkdirp(ctx, conn, archiveDir, "0755", sudo); err != nil {
		return fmt.Errorf("failed to create parent directory for archive '%s': %w", archiveDir, err)
	}

	_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if execErr != nil {
		return fmt.Errorf("failed to compress sources to '%s' using command '%s': %w", archivePath, cmd, execErr)
	}
	return nil
}

func (r *defaultRunner) ListArchiveContents(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sudo bool) ([]string, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}
	archiveFilename := filepath.Base(archivePath)
	var cmd string
	isZip := false

	switch {
	case strings.HasSuffix(archiveFilename, ".tar.gz") || strings.HasSuffix(archiveFilename, ".tgz") ||
		strings.HasSuffix(archiveFilename, ".tar.bz2") || strings.HasSuffix(archiveFilename, ".tbz2") ||
		strings.HasSuffix(archiveFilename, ".tar.xz") || strings.HasSuffix(archiveFilename, ".txz") ||
		strings.HasSuffix(archiveFilename, ".tar"):
		if _, errLk := r.LookPath(ctx, conn, "tar"); errLk != nil {
			return nil, fmt.Errorf("tar command not found on remote host, required to list contents: %w", errLk)
		}
		cmd = fmt.Sprintf("tar -tf %s", archivePath)
	case strings.HasSuffix(archiveFilename, ".zip"):
		if _, errLk := r.LookPath(ctx, conn, "unzip"); errLk != nil {
			return nil, fmt.Errorf("unzip command not found on remote host, required for .zip files: %w", errLk)
		}
		isZip = true
		cmd = fmt.Sprintf("unzip -Z1 %s", archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive format for listing contents: %s", archiveFilename)
	}

	stdout, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		if isZip {
			fmt.Fprintf(os.Stderr, "Warning: 'unzip -Z1' failed, falling back to 'unzip -l'. Error: %v\n", err)
			cmd = fmt.Sprintf("unzip -l %s", archivePath)
			stdout, _, err = r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
			if err != nil {
				return nil, fmt.Errorf("failed to list contents of '%s' using both 'unzip -Z1' and 'unzip -l': %w", archivePath, err)
			}
		} else {
			return nil, fmt.Errorf("failed to list contents of '%s' using command '%s': %w", archivePath, cmd, err)
		}
	}

	output := string(stdout)
	var contents []string
	lines := strings.Split(output, "\n")

	if isZip && strings.Contains(cmd, "unzip -l") {
		if len(lines) < 5 {
			if strings.Contains(output, "number of entries: 0") {
				return []string{}, nil
			}
			return nil, fmt.Errorf("unexpected output format from unzip -l for %s: %s", archivePath, output)
		}
		for i, line := range lines {
			if i < 3 || i >= len(lines)-2 {
				continue
			}
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}
			parts := strings.Fields(trimmedLine)
			if len(parts) >= 4 {
				filename := strings.Join(parts[3:], " ")
				contents = append(contents, filename)
			}
		}
	} else {
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine != "" {
				contents = append(contents, trimmedLine)
			}
		}
	}

	return contents, nil
}

func findCommonBase(paths []string) (base string, rels []string, err error) {
	if len(paths) == 0 {
		return ".", nil, nil
	}
	if len(paths) == 1 {
		p := paths[0]
		return filepath.Dir(p), []string{filepath.Base(p)}, nil
	}
	base = filepath.Dir(paths[0])
	for _, p := range paths[1:] {
		for !strings.HasPrefix(p, base+string(filepath.Separator)) && base != "." && base != "/" {
			base = filepath.Dir(base)
		}
	}

	if base == "." || base == "/" {
		return ".", paths, nil
	}

	rels = make([]string, len(paths))
	for i, p := range paths {
		rel, err := filepath.Rel(base, p)
		if err != nil {
			return "", nil, fmt.Errorf("cannot find relative path for '%s' from base '%s': %w", p, base, err)
		}
		rels[i] = rel
	}

	return base, rels, nil
}
