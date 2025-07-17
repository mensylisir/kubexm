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
		// Stderr is included in the error by RunWithOptions if it's a CommandError
		return fmt.Errorf("failed to download %s to %s using command '%s': %w", url, destPath, cmd, err)
	}
	return nil
}

func (r *defaultRunner) Extract(ctx context.Context, conn connector.Connector, facts *Facts, archivePath, destDir string, sudo bool) error {
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
		if _, errLk := r.LookPath(ctx, conn, "unzip"); errLk != nil { // Use r.LookPath
			return fmt.Errorf("unzip command not found on remote host, required for .zip files: %w", errLk)
		}
		cmd = fmt.Sprintf("unzip -o %s -d %s", archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format for file: %s (supported: .tar, .tar.gz, .tgz, .tar.bz2, .tbz2, .tar.xz, .txz, .zip)", archiveFilename)
	}

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo}) // Use r.RunWithOptions
	if err != nil {
		return fmt.Errorf("failed to extract %s to %s using command '%s': %w", archivePath, destDir, cmd, err)
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

	if err := r.Download(ctx, conn, facts, url, remoteTempPath, sudo); err != nil { // Pass facts
		return fmt.Errorf("download phase of DownloadAndExtract failed for URL %s to %s: %w", url, remoteTempPath, err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := r.Remove(cleanupCtx, conn, remoteTempPath, sudo); err != nil { // Use r.Remove
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

func (r *defaultRunner) Compress(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sources []string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if len(sources) == 0 {
		return fmt.Errorf("no source paths provided for compression")
	}

	archiveFilename := filepath.Base(archivePath)
	var cmd string
	sourcePaths := strings.Join(sources, " ")
	if len(sources) > 0 {
		isAbsolute := filepath.IsAbs(sources[0])
		if !isAbsolute {
			// If relative, try to get CWD or assume paths are relative to user's home or a known dir.
			// For simplicity, we'll try to make paths relative to a common parent if possible,
			// or rely on tar's behavior when -C is not specified (usually current remote working dir).
			// A more robust solution might involve ensuring sources share a common prefix or running commands from a specific directory.
		}
		// For tar, using -C <directory> changes the directory before adding files.
		// This is useful if you want to archive 'file1' from '/tmp/mydir/file1' as 'file1' instead of 'tmp/mydir/file1'.
		// We need to calculate the paths relative to the -C directory.
		// Example: sources are ["/tmp/data/file1", "/tmp/data/dir1"], archivePath is "/opt/backup.tar.gz"
		// We could use: tar -czf /opt/backup.tar.gz -C /tmp/data file1 dir1
		// This requires stripping the common base path from sourcePaths.

		// Simplified approach: if all paths share a common parent, use it with -C.
		// This logic can be complex if paths are diverse. For now, assume paths are prepared correctly by the caller
		// or handle common base directory logic.
		// For this initial implementation, we'll keep it simpler and might require sources to be in the CWD or specified with appropriate relative/absolute paths.
	}

	switch {
	case strings.HasSuffix(archiveFilename, ".tar.gz") || strings.HasSuffix(archiveFilename, ".tgz"):
		// Ensure tar is available
		if _, errLk := r.LookPath(ctx, conn, "tar"); errLk != nil {
			return fmt.Errorf("tar command not found on remote host, required for .tar.gz/.tgz files: %w", errLk)
		}
		// If sources are in different directories, and you want to preserve paths relative to a common base,
		// you might need to use -C. For example, tar -czf archive.tar.gz -C /base/path dir1 file2
		// For simplicity, this example assumes paths are either absolute or relative to the remote CWD.
		// A more advanced version could calculate the common base directory and adjust sourcePaths.
		// Example: `tar -czf /path/to/archive.tar.gz -C /base/directory file1 dir2`
		// If sources are `/data/file1` and `/data/dir2`, and you want them as `file1` and `dir2` in archive,
		// use `-C /data file1 dir2`.
		// If `sources` contains `/foo/bar` and `/app/config` and `archivePath` is `/tmp/myarchive.tar.gz`
		// then `baseDir` might be `/` and `sourcePaths` would be `foo/bar app/config`
		// Let's assume for now the caller provides paths that make sense relative to remote CWD or provides absolute paths.
		// A common strategy is to cd into the parent directory of the items to be archived.
		// However, executing `cd` within a single RunWithOptions is tricky.
		// Using `tar -C` is the correct way.
		// Let's assume for now that if relative paths are given, they are relative to the user's home or current directory on the remote.
		// If absolute paths, tar handles them.
		// A simple approach without complex base path calculation:
		cmd = fmt.Sprintf("tar -czf %s %s", archivePath, sourcePaths)
	case strings.HasSuffix(archiveFilename, ".zip"):
		if _, errLk := r.LookPath(ctx, conn, "zip"); errLk != nil {
			return fmt.Errorf("zip command not found on remote host, required for .zip files: %w", errLk)
		}
		// zip -r archive.zip path1 path2 ...
		// Zip typically stores paths as specified. If you cd into a dir first, then paths are relative to that.
		// `zip -j` junks paths. `zip -r` recurses.
		// Assuming paths are correct as is (either full paths or relative to remote CWD).
		cmd = fmt.Sprintf("zip -r %s %s", archivePath, sourcePaths)

	// Add .tar.bz2 and .tar.xz compression support
	case strings.HasSuffix(archiveFilename, ".tar.bz2") || strings.HasSuffix(archiveFilename, ".tbz2"):
		if _, errLk := r.LookPath(ctx, conn, "tar"); errLk != nil {
			return fmt.Errorf("tar command not found on remote host, required for .tar.bz2/.tbz2 files: %w", errLk)
		}
		cmd = fmt.Sprintf("tar -cjf %s %s", archivePath, sourcePaths)
	case strings.HasSuffix(archiveFilename, ".tar.xz") || strings.HasSuffix(archiveFilename, ".txz"):
		if _, errLk := r.LookPath(ctx, conn, "tar"); errLk != nil {
			return fmt.Errorf("tar command not found on remote host, required for .tar.xz/.txz files: %w", errLk)
		}
		cmd = fmt.Sprintf("tar -cJf %s %s", archivePath, sourcePaths)

	default:
		return fmt.Errorf("unsupported archive format for compression: %s (supported: .tar.gz, .tgz, .zip, .tar.bz2, .tbz2, .tar.xz, .txz)", archiveFilename)
	}

	// Create parent directory for the archive if it doesn't exist
	archiveDir := filepath.Dir(archivePath)
	if err := r.Mkdirp(ctx, conn, archiveDir, "0755", sudo); err != nil {
		return fmt.Errorf("failed to create parent directory %s for archive: %w", archiveDir, err)
	}

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to compress sources to %s using command '%s': %w", archivePath, cmd, err)
	}
	return nil
}

// ListArchiveContents lists the contents of an archive without extracting it.
// Returns a slice of strings, where each string is a path within the archive.
func (r *defaultRunner) ListArchiveContents(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sudo bool) ([]string, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}

	archiveFilename := filepath.Base(archivePath)
	var cmd string

	switch {
	case strings.HasSuffix(archiveFilename, ".tar.gz") || strings.HasSuffix(archiveFilename, ".tgz") ||
		strings.HasSuffix(archiveFilename, ".tar.bz2") || strings.HasSuffix(archiveFilename, ".tbz2") ||
		strings.HasSuffix(archiveFilename, ".tar.xz") || strings.HasSuffix(archiveFilename, ".txz") ||
		strings.HasSuffix(archiveFilename, ".tar"):
		if _, errLk := r.LookPath(ctx, conn, "tar"); errLk != nil {
			return nil, fmt.Errorf("tar command not found on remote host, required to list contents: %w", errLk)
		}
		cmd = fmt.Sprintf("tar -tf %s", archivePath) // -t lists contents
	case strings.HasSuffix(archiveFilename, ".zip"):
		if _, errLk := r.LookPath(ctx, conn, "unzip"); errLk != nil {
			return nil, fmt.Errorf("unzip command not found on remote host, required for .zip files: %w", errLk)
		}
		// unzip -l lists contents. Output needs parsing.
		// Example output line: `  234  01-29-2024 10:00   path/to/file.txt`
		// We only need the path part.
		cmd = fmt.Sprintf("unzip -l %s | awk 'NR>3 {print $4}' | head -n -2", archivePath) // More complex to parse unzip -l robustly. Simpler: use zipinfo or another tool if available
		// A more robust way for zip, if `zipinfo` is available: `zipinfo -1 archive.zip`
		// Let's try with `unzip -Z -1 archive.zip` which is specifically for listing filenames
		// Check if unzip supports -Z -1 (macOS unzip does, GNU unzip might use `unzip -qql` or similar)
		// `unzip -Z -1` is not standard. `unzip -lqq` might be better, then parse.
		// `unzip -l` and parse:
		// Header lines, then file lines, then footer lines.
		// Example:
		// Archive:  test.zip
		//   Length      Date    Time    Name
		// ---------  ---------- -----   ----
		//         0  2024-03-15 11:22   d1/
		//         6  2024-03-15 11:22   d1/f1.txt
		// ---------                     -------
		//         6                     2 files
		// We need to skip first 3 lines and last 2 lines, then awk for the 4th field.
		// This is fragile. Using `tar -tf` for tarballs is much cleaner.
		// For zip, `zipinfo -1 file.zip` is the best if available.
		// If not, `unzip -qql file.zip | awk '{print $NF}'` might work (NF is last field).
		// Let's stick to a common `unzip -l` parsing strategy.
		// The command `unzip -l %s | awk 'NR>3 {for(i=4;i<=NF;i++) printf $i (i==NF?"":" ")} {if (NF>=4) print ""}' | head -n -2` attempts to grab all parts of filename
		cmd = fmt.Sprintf("unzip -l %s", archivePath)

	default:
		return nil, fmt.Errorf("unsupported archive format for listing contents: %s", archiveFilename)
	}

	stdout, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return nil, fmt.Errorf("failed to list contents of %s using command '%s': %w", archivePath, cmd, err)
	}

	output := string(stdout)
	var contents []string
	lines := strings.Split(output, "\n")

	if strings.HasSuffix(archiveFilename, ".zip") {
		// Parse unzip -l output
		// Skip header (usually 3 lines: Archive:, Length Date Time Name, --------- ...)
		// Skip footer (usually 2 lines: --------- ..., total ... files)
		if len(lines) < 5 { // Not enough lines for header, content, and footer
			// It might be an empty archive or very small.
			// Empty archive `unzip -l empty.zip`:
			// Archive:  empty.zip
			// Zip file size: 22 bytes, number of entries: 0
			if strings.Contains(output, "number of entries: 0") {
				return []string{}, nil
			}
			return nil, fmt.Errorf("unexpected output format from unzip -l for %s: %s", archivePath, output)
		}
		for i, line := range lines {
			if i < 3 || i >= len(lines)-2 { // Skip header and footer
				continue
			}
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}
			parts := strings.Fields(trimmedLine)
			if len(parts) >= 4 {
				// The filename starts from the 4th part and can contain spaces.
				filename := strings.Join(parts[3:], " ")
				contents = append(contents, filename)
			}
		}
	} else { // tar output is one file per line
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine != "" {
				contents = append(contents, trimmedLine)
			}
		}
	}

	return contents, nil
}
