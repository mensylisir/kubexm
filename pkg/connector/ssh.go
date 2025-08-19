package connector

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/logger"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SSHConnector struct {
	client        *ssh.Client
	bastionClient *ssh.Client
	sftpClient    *sftp.Client
	connCfg       ConnectionCfg
	cachedOS      *OS
	isConnected   bool
	pool          *ConnectionPool
	isFromPool    bool
	managedConn   *ManagedConnection
}

func NewSSHConnector(pool *ConnectionPool) *SSHConnector {
	return &SSHConnector{
		pool: pool,
	}
}

func (s *SSHConnector) Connect(ctx context.Context, cfg ConnectionCfg) error {
	log := logger.Get()
	s.connCfg = cfg
	s.isFromPool = false

	if s.pool != nil && cfg.BastionCfg == nil {
		mc, err := s.pool.Get(ctx, cfg)
		if err == nil && mc != nil && mc.Client() != nil {
			s.managedConn = mc
			s.client = mc.Client()
			s.isFromPool = true
			s.isConnected = true
			return nil
		}
		if err != nil && !errors.Is(err, ErrPoolExhausted) {
			log.Error(os.Stderr, "SSHConnector: Failed to get connection from pool for %s (will attempt direct dial): %v\n", cfg.Host, err)
		}
	}

	s.isFromPool = false
	s.managedConn = nil
	client, bastionClient, err := currentDialer(ctx, cfg, cfg.Timeout)
	if err != nil {
		return err
	}
	s.client = client
	s.bastionClient = bastionClient
	s.isConnected = true
	return nil
}

func (s *SSHConnector) IsConnected() bool {
	if s.client == nil || !s.isConnected {
		return false
	}
	_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		s.isConnected = false
		return false
	}
	return true
}

func (s *SSHConnector) Close() error {
	s.isConnected = false
	var firstErr error
	log := logger.Get()
	if s.sftpClient != nil {
		if err := s.sftpClient.Close(); err != nil {
			firstErr = fmt.Errorf("failed to close SFTP client for %s: %w", s.connCfg.Host, err)
			log.Errorf("%v %v", os.Stderr, firstErr)
		}
		s.sftpClient = nil
	}

	if s.client != nil {
		if s.isFromPool && s.managedConn != nil && s.pool != nil {
			isHealthy := s.managedConn.IsHealthy()
			s.pool.Put(s.managedConn, isHealthy)
		} else {
			if err := s.client.Close(); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				log.Errorf("%v SSHConnector: Error closing direct client for %s: %v\n", os.Stderr, s.connCfg.Host, err)
			}
			if s.bastionClient != nil {
				if err := s.bastionClient.Close(); err != nil {
					if firstErr == nil {
						firstErr = err
					}
					log.Errorf("%v SSHConnector: Error closing direct bastion client for %s: %v\n", os.Stderr, s.connCfg.Host, err)
				}
			}
		}
	} else if s.bastionClient != nil {
		if err := s.bastionClient.Close(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Errorf("%v SSHConnector: Error closing orphaned direct bastion client for %s: %v\n", os.Stderr, s.connCfg.Host, err)
		}
	}

	s.client = nil
	s.bastionClient = nil
	s.managedConn = nil
	s.isFromPool = false
	return firstErr
}

func (s *SSHConnector) Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error) {
	if !s.IsConnected() {
		return nil, nil, &ConnectionError{Host: s.connCfg.Host, Err: fmt.Errorf("not connected")}
	}

	effectiveOptions := ExecOptions{}
	if options != nil {
		effectiveOptions = *options
	}

	runOnce := func(runCtx context.Context, stdinPipe io.Reader) ([]byte, []byte, error) {
		session, err := s.client.NewSession()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create session: %w", err)
		}
		defer session.Close()

		if len(effectiveOptions.Env) > 0 {
			for _, envVar := range effectiveOptions.Env {
				parts := strings.SplitN(envVar, "=", 2)
				if len(parts) == 2 {
					_ = session.Setenv(parts[0], parts[1])
				}
			}
		}

		finalCmd := cmd
		if effectiveOptions.Sudo {
			if s.connCfg.Password != "" {
				finalCmd = "sudo -S -p '' -E -- " + cmd
				session.Stdin = io.MultiReader(strings.NewReader(s.connCfg.Password+"\n"), stdinPipe)
			} else {
				finalCmd = "sudo -E -- " + cmd
				session.Stdin = stdinPipe
			}
		} else {
			session.Stdin = stdinPipe
		}

		var stdoutBuf, stderrBuf bytes.Buffer
		if effectiveOptions.Stream != nil {
			session.Stdout = io.MultiWriter(&stdoutBuf, effectiveOptions.Stream)
			session.Stderr = io.MultiWriter(&stderrBuf, effectiveOptions.Stream)
		} else {
			session.Stdout = &stdoutBuf
			session.Stderr = &stderrBuf
		}

		if err := session.Start(finalCmd); err != nil {
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf("failed to start command '%s': %w", finalCmd, err)
		}

		doneCh := make(chan error, 1)
		go func() {
			doneCh <- session.Wait()
		}()

		select {
		case <-runCtx.Done():
			_ = session.Signal(ssh.SIGKILL)
			select {
			case <-doneCh:
			case <-time.After(1 * time.Second):
			}
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), runCtx.Err()
		case err := <-doneCh:
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
		}
	}

	var stdinReader io.Reader = bytes.NewReader(nil)
	if roStream, ok := effectiveOptions.Stream.(readOnlyStream); ok {
		stdinReader = roStream.Reader
	} else if effectiveOptions.Stream != nil {
	}

	for i := 0; i <= effectiveOptions.Retries; i++ {
		attemptCtx := ctx
		var attemptCancel context.CancelFunc
		if effectiveOptions.Timeout > 0 {
			attemptCtx, attemptCancel = context.WithTimeout(ctx, effectiveOptions.Timeout)
		}

		stdout, stderr, err = runOnce(attemptCtx, stdinReader)

		if attemptCancel != nil {
			attemptCancel()
		}

		if err == nil {
			return stdout, stderr, nil // Success
		}
		if attemptCancel != nil {
			attemptCancel()
		}

		if err == nil { // Success on this attempt
			return stdout, stderr, nil
		}

		if ctx.Err() != nil {
			break
		}

		if i < effectiveOptions.Retries {
			if effectiveOptions.RetryDelay > 0 {
				select {
				case <-ctx.Done():
					return stdout, stderr, err
				case <-time.After(effectiveOptions.RetryDelay):
				}
			}
		} else {
			break
		}
	}

	exitCode := -1
	if exitErr, ok := err.(*ssh.ExitError); ok {
		exitCode = exitErr.ExitStatus()
	}
	return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: err}
}

func (s *SSHConnector) ensureSftp() error {
	if s.sftpClient == nil {
		if !s.IsConnected() {
			return &ConnectionError{Host: s.connCfg.Host, Err: fmt.Errorf("not connected, cannot initialize SFTP")}
		}
		var err error
		s.sftpClient, err = sftp.NewClient(s.client)
		if err != nil {
			return fmt.Errorf("failed to create SFTP client: %w", err)
		}
	}
	return nil
}

func (s *SSHConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	return s.writeFileFromReader(ctx, bytes.NewReader(content), dstPath, options)
}

func (s *SSHConnector) WriteFile(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error {
	opts := options
	if opts == nil {
		opts = &FileTransferOptions{}
	}
	return s.writeFileFromReader(ctx, bytes.NewReader(content), destPath, opts)
}

func (s *SSHConnector) writeFileFromReader(ctx context.Context, content io.Reader, dstPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	if err := s.ensureSftp(); err != nil {
		return err
	}

	if opts.Sudo {
		return s.sudoWrite(ctx, content, dstPath, opts)
	}
	return s.nonSudoWrite(ctx, content, dstPath, opts)
}

func (s *SSHConnector) sudoWrite(ctx context.Context, content io.Reader, dstPath string, opts FileTransferOptions) error {
	tmpPath := filepath.Join("/tmp", fmt.Sprintf("connector-tmp-%d-%s", time.Now().UnixNano(), filepath.Base(dstPath)))
	log := logger.Get()
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _, err := s.Exec(cleanupCtx, fmt.Sprintf("rm -f %s", shellEscape(tmpPath)), nil)
		if err != nil {
			log.Errorf("%v Warning: failed to remove temporary file %s on host %s: %v\n", os.Stderr, tmpPath, s.connCfg.Host, err)
		}
	}()

	err := s.writeFileViaSFTP(ctx, content, tmpPath, "0600")
	if err != nil {
		return fmt.Errorf("failed to upload to temporary path %s for sudo write: %w", tmpPath, err)
	}

	destDir := filepath.Dir(dstPath)
	if destDir != "." && destDir != "/" && destDir != "" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destDir))
		_, stderr, mkdirErr := s.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
		if mkdirErr != nil {
			return fmt.Errorf("failed to create destination directory %s with sudo: %s (underlying error %w)", destDir, string(stderr), mkdirErr)
		}
	}

	mvCmd := fmt.Sprintf("mv %s %s", shellEscape(tmpPath), shellEscape(dstPath))
	_, stderr, mvErr := s.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
	if mvErr != nil {
		return fmt.Errorf("failed to move file to %s with sudo: %s (underlying error %w)", dstPath, string(stderr), mvErr)
	}

	if opts.Permissions != "" {
		if _, err := strconv.ParseUint(opts.Permissions, 8, 32); err != nil {
			return fmt.Errorf("invalid permissions format '%s': %w", opts.Permissions, err)
		}
		chmodCmd := fmt.Sprintf("chmod %s %s", shellEscape(opts.Permissions), shellEscape(dstPath))
		_, stderr, chmodErr := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: true})
		if chmodErr != nil {
			return fmt.Errorf("failed to set permissions on %s with sudo: %s (underlying error %w)", dstPath, string(stderr), chmodErr)
		}
	}

	if opts.Owner != "" {
		ownerAndGroup := opts.Owner
		if opts.Group != "" {
			ownerAndGroup = fmt.Sprintf("%s:%s", opts.Owner, opts.Group)
		}
		chownCmd := fmt.Sprintf("chown %s %s", shellEscape(ownerAndGroup), shellEscape(dstPath))
		_, stderr, chownErr := s.Exec(ctx, chownCmd, &ExecOptions{Sudo: true})
		if chownErr != nil {
			return fmt.Errorf("failed to set ownership on %s with sudo: %s (underlying error %w)", dstPath, string(stderr), chownErr)
		}
	}

	return nil
}

func (s *SSHConnector) nonSudoWrite(ctx context.Context, content io.Reader, dstPath string, opts FileTransferOptions) error {
	return s.writeFileViaSFTP(ctx, content, dstPath, opts.Permissions)
}

func (s *SSHConnector) writeFileViaSFTP(ctx context.Context, content io.Reader, destPath, permissions string) error {
	log := logger.Get()
	if err := s.ensureSftp(); err != nil {
		return fmt.Errorf("sftp client not available for writeFileViaSFTP on host %s: %w", s.connCfg.Host, err)
	}

	parentDir := filepath.Dir(destPath)
	if parentDir != "." && parentDir != "/" && parentDir != "" {
		if err := s.sftpClient.MkdirAll(parentDir); err != nil {
			return fmt.Errorf("failed to create parent directory %s via sftp: %w", parentDir, err)
		}
	}

	file, err := s.sftpClient.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s via sftp: %w", destPath, err)
	}
	defer file.Close()

	if _, err = io.Copy(file, content); err != nil {
		return fmt.Errorf("failed to write content to remote file %s via sftp: %w", destPath, err)
	}

	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr != nil {
			log.Errorf("%v, Warning: Invalid permissions format '%s' for SFTP write to %s, skipping chmod: %v\n", os.Stderr, permissions, destPath, parseErr)
		} else {
			if chmodErr := s.sftpClient.Chmod(destPath, os.FileMode(permVal)); chmodErr != nil {
				log.Errorf("%v Warning: Failed to chmod remote file %s to %s via SFTP: %v\n", os.Stderr, destPath, permissions, chmodErr)
			}
		}
	}
	return nil
}

func (s *SSHConnector) Stat(ctx context.Context, path string) (*FileStat, error) {
	if err := s.ensureSftp(); err != nil {
		return nil, err
	}
	fi, err := s.sftpClient.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}
		return nil, fmt.Errorf("failed to stat remote path %s: %w", path, err)
	}
	return &FileStat{
		Name:    fi.Name(),
		Size:    fi.Size(),
		Mode:    fi.Mode(),
		ModTime: fi.ModTime(),
		IsDir:   fi.IsDir(),
		IsExist: true,
	}, nil
}

func (s *SSHConnector) StatWithOptions(ctx context.Context, path string, opts *StatOptions) (*FileStat, error) {
	useSudo := opts != nil && opts.Sudo

	if err := s.ensureSftp(); err == nil {
		fi, err := s.sftpClient.Lstat(path)
		if err == nil {
			return &FileStat{
				Name:    fi.Name(),
				Size:    fi.Size(),
				Mode:    fi.Mode(),
				ModTime: fi.ModTime(),
				IsDir:   fi.IsDir(),
				IsExist: true,
			}, nil
		}
		if os.IsNotExist(err) {
			return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}
		if !useSudo {
			return nil, fmt.Errorf("failed to stat remote path %s: %w", path, err)
		}
	} else if !useSudo {
		return nil, fmt.Errorf("sftp client not available for Stat: %w", err)
	}

	if useSudo {

		cmdExists := fmt.Sprintf("test -e %s", path)
		_, _, errExists := s.Exec(ctx, cmdExists, &ExecOptions{Sudo: true})
		if errExists != nil {
			return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}

		cmdIsDir := fmt.Sprintf("test -d %s", path)
		_, _, errIsDir := s.Exec(ctx, cmdIsDir, &ExecOptions{Sudo: true})
		isDir := (errIsDir == nil)

		return &FileStat{
			Name:    filepath.Base(path),
			IsDir:   isDir,
			IsExist: true,
		}, nil
	}
	return nil, fmt.Errorf("failed to stat remote path %s: sftp unavailable and sudo not requested", path)
}

func (s *SSHConnector) LookPath(ctx context.Context, file string) (string, error) {
	if strings.ContainsAny(file, " \t\n\r`;&|$<>()!{}[]*?^~") {
		return "", fmt.Errorf("invalid characters in executable name for LookPath: %q", file)
	}

	cmd := fmt.Sprintf("command -v %s", shellEscape(file))
	stdout, stderr, err := s.Exec(ctx, cmd, nil)
	if err != nil {
		return "", fmt.Errorf("failed to find executable '%s': %s (underlying error: %w)", file, string(stderr), err)
	}
	path := strings.TrimSpace(string(stdout))
	if path == "" { // Should be redundant if Exec returns error on non-zero exit
		return "", fmt.Errorf("executable %s not found in PATH (stderr: %s)", file, string(stderr))
	}
	return path, nil
}

func (s *SSHConnector) LookPathWithOptions(ctx context.Context, file string, opts *LookPathOptions) (string, error) {
	if strings.ContainsAny(file, " \t\n\r`;&|$<>()!{}[]*?^~") {
		return "", fmt.Errorf("invalid characters in executable name for LookPath: %q", file)
	}

	cmd := fmt.Sprintf("command -v %s", shellEscape(file))

	useSudo := opts != nil && opts.Sudo

	execOpts := &ExecOptions{
		Sudo: useSudo,
	}

	stdout, stderr, err := s.Exec(ctx, cmd, execOpts)
	if err != nil {
		return "", fmt.Errorf("failed to find executable '%s' (sudo: %v): %s (underlying error: %w)", file, useSudo, string(stderr), err)
	}

	path := strings.TrimSpace(string(stdout))
	if path == "" {
		return "", fmt.Errorf("executable '%s' not found in PATH (sudo: %v, stderr: %s)", file, useSudo, string(stderr))
	}

	return path, nil
}

func (s *SSHConnector) GetOS(ctx context.Context) (*OS, error) {
	log := logger.Get()
	if s.cachedOS != nil {
		return s.cachedOS, nil
	}

	osInfo := &OS{}
	var errs []string

	infoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	content, _, err := s.Exec(infoCtx, "cat /etc/os-release", nil)
	if err == nil {
		vars := parseKeyValues(string(content), "=", "\"")
		osInfo.ID = vars["ID"]
		osInfo.VersionID = vars["VERSION_ID"]
		osInfo.PrettyName = vars["PRETTY_NAME"]
		osInfo.Codename = vars["VERSION_CODENAME"]
	} else {
		errs = append(errs, fmt.Sprintf("/etc/os-release failed: %v", err))
	}

	if osInfo.ID == "" {
		lsbCtx, lsbCancel := context.WithTimeout(ctx, 10*time.Second)
		defer lsbCancel()
		content, _, err = s.Exec(lsbCtx, "lsb_release -a", nil)
		if err == nil {
			vars := parseKeyValues(string(content), ":", "")
			osInfo.ID = strings.ToLower(vars["Distributor ID"])
			osInfo.VersionID = vars["Release"]
			osInfo.PrettyName = vars["Description"]
			osInfo.Codename = vars["Codename"]
		} else {
			errs = append(errs, fmt.Sprintf("lsb_release -a failed: %v", err))
		}
	}

	if osInfo.ID == "" {
		unameCtx, unameCancel := context.WithTimeout(ctx, 5*time.Second)
		defer unameCancel()
		unameS, _, unameErr := s.Exec(unameCtx, "uname -s", nil)
		if unameErr == nil {
			osName := strings.ToLower(strings.TrimSpace(string(unameS)))
			if strings.HasPrefix(osName, "linux") {
				osInfo.ID = "linux"
			} else if strings.HasPrefix(osName, "darwin") {
				osInfo.ID = "darwin"
				swVerCtx, swVerCancel := context.WithTimeout(ctx, 10*time.Second)
				defer swVerCancel()
				pn, _, _ := s.Exec(swVerCtx, "sw_vers -productName", nil)
				pv, _, _ := s.Exec(swVerCtx, "sw_vers -productVersion", nil)
				osInfo.PrettyName = strings.TrimSpace(string(pn))
				osInfo.VersionID = strings.TrimSpace(string(pv))
			}
		} else {
			errs = append(errs, fmt.Sprintf("uname -s failed: %v", unameErr))
		}
	}

	archCtx, archCancel := context.WithTimeout(ctx, 5*time.Second)
	defer archCancel()
	arch, _, archErr := s.Exec(archCtx, "uname -m", nil)
	if archErr == nil {
		osInfo.Arch = strings.TrimSpace(string(arch))
	} else {
		errs = append(errs, fmt.Sprintf("uname -m failed: %v", archErr))
	}

	kernelCtx, kernelCancel := context.WithTimeout(ctx, 5*time.Second)
	defer kernelCancel()
	kernel, _, kernelErr := s.Exec(kernelCtx, "uname -r", nil)
	if kernelErr == nil {
		osInfo.Kernel = strings.TrimSpace(string(kernel))
	} else {
		errs = append(errs, fmt.Sprintf("uname -r failed: %v", kernelErr))
	}

	if osInfo.ID == "" {
		if osInfo.Kernel != "" && strings.Contains(strings.ToLower(osInfo.Kernel), "linux") {
			osInfo.ID = "linux"
		} else if osInfo.Arch != "" {
			return nil, fmt.Errorf("failed to determine OS ID, errors: %s", strings.Join(errs, "; "))
		}
	}

	osInfo.ID = strings.ToLower(strings.TrimSpace(osInfo.ID))
	osInfo.VersionID = strings.TrimSpace(osInfo.VersionID)
	osInfo.PrettyName = strings.TrimSpace(osInfo.PrettyName)
	osInfo.Codename = strings.TrimSpace(osInfo.Codename)

	if osInfo.ID == "" && osInfo.Arch == "" && osInfo.Kernel == "" {
		return nil, fmt.Errorf("failed to determine any OS info, errors: %s", strings.Join(errs, "; "))
	}

	if osInfo.ID != "" {
		s.cachedOS = osInfo
		if osInfo.VersionID == "" {
			log.Errorf("%v Warning: Could not determine OS VersionID for host %s\n", os.Stderr, s.connCfg.Host)
		}
		if osInfo.PrettyName == "" {
			log.Errorf("%v Warning: Could not determine OS PrettyName for host %s\n", os.Stderr, s.connCfg.Host)
		}
	}
	return s.cachedOS, nil
}

func (s *SSHConnector) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := s.ensureSftp(); err != nil {
		return nil, fmt.Errorf("sftp client not available for ReadFile on host %s: %w", s.connCfg.Host, err)
	}
	file, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file %s via sftp on host %s: %w", path, s.connCfg.Host, err)
	}
	defer file.Close()

	contentBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file %s via sftp on host %s: %w", path, s.connCfg.Host, err)
	}
	return contentBytes, nil
}

func (s *SSHConnector) ReadFileWithOptions(ctx context.Context, path string, opts *FileTransferOptions) ([]byte, error) {
	useSudo := false
	if opts != nil && opts.Sudo {
		useSudo = true
	}
	if !useSudo {
		if err := s.ensureSftp(); err != nil {
			return nil, fmt.Errorf("sftp client not available for ReadFile on host %s: %w", s.connCfg.Host, err)
		}
		file, err := s.sftpClient.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open remote file '%s' via sftp: %w", path, err)
		}
		defer file.Close()
		return io.ReadAll(file)
	}
	cmd := fmt.Sprintf("cat %s", path)
	execOpts := &ExecOptions{
		Sudo: true,
	}
	if opts != nil && opts.Timeout > 0 {
		execOpts.Timeout = opts.Timeout
	}

	stdout, _, err := s.Exec(ctx, cmd, execOpts)
	if err != nil {
		return stdout, fmt.Errorf("failed to read file '%s' with sudo cat: %w", path, err)
	}
	return stdout, nil
}

func (s *SSHConnector) Mkdir(ctx context.Context, path string, perm string) error {
	escapedPath := shellEscape(path)

	cmd := fmt.Sprintf("mkdir -p %s", escapedPath)
	_, stderr, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: false})
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %s (underlying error: %w)", path, string(stderr), err)
	}

	if perm != "" {
		if _, errP := strconv.ParseUint(perm, 8, 32); errP != nil {
			return fmt.Errorf("invalid permission format '%s' for Mkdir: %w", perm, errP)
		}
		chmodCmd := fmt.Sprintf("chmod %s %s", shellEscape(perm), escapedPath)
		_, chmodStderr, chmodErr := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: false})
		if chmodErr != nil {
			return fmt.Errorf("failed to chmod directory %s to %s: %s (underlying error: %w)", path, perm, string(chmodStderr), chmodErr)
		}
	}
	return nil
}

func (s *SSHConnector) Remove(ctx context.Context, path string, opts RemoveOptions) error {
	if opts.IgnoreNotExist {
		statCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		stat, err := s.Stat(statCtx, path)
		if err == nil && !stat.IsExist { // Stat succeeded and file does not exist
			return nil
		}
	}

	flags := "-f"
	if opts.Recursive {
		flags += "r"
	}
	cmd := fmt.Sprintf("rm %s %s", flags, shellEscape(path))

	_, stderr, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: opts.Sudo})
	if err != nil {
		return fmt.Errorf("failed to remove %s (sudo: %t): %s (underlying error: %w)", path, opts.Sudo, string(stderr), err)
	}
	return nil
}

func (s *SSHConnector) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	var checksumCmd string
	escapedPath := shellEscape(path)

	switch strings.ToLower(checksumType) {
	case "sha256":
		checksumCmd = fmt.Sprintf("sha256sum -b %s", escapedPath)
	case "md5":
		checksumCmd = fmt.Sprintf("md5sum -b %s", escapedPath)
	default:
		return "", fmt.Errorf("unsupported checksum type '%s' for remote file %s on host %s", checksumType, path, s.connCfg.Host)
	}

	stdoutBytes, stderrBytes, err := s.Exec(ctx, checksumCmd, &ExecOptions{Sudo: false})
	if err != nil {
		return "", fmt.Errorf("failed to execute checksum command '%s' on %s for %s: %s (underlying error: %w)", checksumCmd, s.connCfg.Host, path, string(stderrBytes), err)
	}

	parts := strings.Fields(string(stdoutBytes))
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("failed to parse checksum from command output for %s on host %s: '%s'", path, s.connCfg.Host, string(stdoutBytes))
}

type readOnlyStream struct {
	io.Reader
}

func (s readOnlyStream) Write(p []byte) (int, error) { return len(p), nil }

func parseKeyValues(content, delimiter, quoteChar string) map[string]string {
	vars := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, delimiter, 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if quoteChar != "" {
				val = strings.Trim(val, quoteChar)
			}
			vars[key] = val
		}
	}
	return vars
}

func dialSSH(ctx context.Context, cfg ConnectionCfg, connectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
	log := logger.Get()
	targetAuthMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("target auth error: %w", err)}
	}

	effectiveTimeout := connectTimeout
	if effectiveTimeout == 0 {
		effectiveTimeout = cfg.Timeout
	}
	if effectiveTimeout == 0 {
		effectiveTimeout = 30 * time.Second
	}

	targetSSHConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            targetAuthMethods,
		HostKeyCallback: cfg.HostKeyCallback,
		Timeout:         effectiveTimeout,
	}

	if targetSSHConfig.HostKeyCallback == nil {
		log.Errorf("%v Warning: HostKeyCallback is not set for target host %s. Using InsecureIgnoreHostKey(). This is NOT recommended for production.\n", os.Stderr, cfg.Host)
		targetSSHConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	targetDialAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	if cfg.BastionCfg != nil {
		bastionFullCfg := ConnectionCfg{
			Host:            cfg.BastionCfg.Host,
			Port:            cfg.BastionCfg.Port,
			User:            cfg.BastionCfg.User,
			Password:        cfg.BastionCfg.Password,
			PrivateKey:      cfg.BastionCfg.PrivateKey,
			PrivateKeyPath:  cfg.BastionCfg.PrivateKeyPath,
			Timeout:         cfg.BastionCfg.Timeout,
			HostKeyCallback: cfg.BastionCfg.HostKeyCallback,
		}
		return dialViaBastion(ctx, targetDialAddr, targetSSHConfig, bastionFullCfg, effectiveTimeout)
	}

	client, err := ssh.Dial("tcp", targetDialAddr, targetSSHConfig)
	if err != nil {
		return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("direct dial failed: %w", err)}
	}
	return client, nil, nil
}

func dialViaBastion(ctx context.Context, targetDialAddr string, targetSSHConfig *ssh.ClientConfig, bastionOverallCfg ConnectionCfg, bastionConnectTimeoutParam time.Duration) (*ssh.Client, *ssh.Client, error) {
	log := logger.Get()
	bastionAuthMethods, err := buildAuthMethods(bastionOverallCfg)
	if err != nil {
		return nil, nil, &ConnectionError{Host: bastionOverallCfg.Host, Err: fmt.Errorf("bastion auth error: %w", err)}
	}

	effectiveBastionTimeout := bastionConnectTimeoutParam
	if effectiveBastionTimeout == 0 {
		effectiveBastionTimeout = bastionOverallCfg.Timeout
	}
	if effectiveBastionTimeout == 0 {
		effectiveBastionTimeout = 30 * time.Second
	}

	bastionSSHConfig := &ssh.ClientConfig{
		User:            bastionOverallCfg.User,
		Auth:            bastionAuthMethods,
		HostKeyCallback: bastionOverallCfg.HostKeyCallback,
		Timeout:         effectiveBastionTimeout,
	}

	if bastionSSHConfig.HostKeyCallback == nil {
		log.Errorf("%v Warning: HostKeyCallback is not set for bastion host %s. Using InsecureIgnoreHostKey(). This is NOT recommended for production.\n", os.Stderr, bastionOverallCfg.Host)
		bastionSSHConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	bastionDialAddr := net.JoinHostPort(bastionOverallCfg.Host, strconv.Itoa(bastionOverallCfg.Port))

	bastionClient, err := ssh.Dial("tcp", bastionDialAddr, bastionSSHConfig)
	if err != nil {
		return nil, nil, &ConnectionError{Host: bastionOverallCfg.Host, Err: fmt.Errorf("bastion dial failed: %w", err)}
	}

	connToTarget, err := bastionClient.Dial("tcp", targetDialAddr)
	if err != nil {
		bastionClient.Close()
		return nil, nil, &ConnectionError{Host: targetDialAddr, Err: fmt.Errorf("dial target via bastion failed: %w", err)}
	}

	ncc, chans, reqs, err := ssh.NewClientConn(connToTarget, targetDialAddr, targetSSHConfig)
	if err != nil {
		connToTarget.Close()
		bastionClient.Close()
		return nil, nil, &ConnectionError{Host: targetDialAddr, Err: fmt.Errorf("SSH handshake to target via bastion failed: %w", err)}
	}
	targetClient := ssh.NewClient(ncc, chans, reqs)
	return targetClient, bastionClient, nil
}

func buildAuthMethods(cfg ConnectionCfg) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}

	if len(cfg.PrivateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key bytes: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	} else if cfg.PrivateKeyPath != "" {
		keyFileBytes, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file %s: %w", cfg.PrivateKeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyFileBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key from file %s: %w", cfg.PrivateKeyPath, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH authentication method provided (password or private key required for host %s)", cfg.Host)
	}
	return methods, nil
}

var _ Connector = &SSHConnector{}

func (s *SSHConnector) Upload(ctx context.Context, localPath, remotePath string, options *FileTransferOptions) error {
	return s.Copy(ctx, localPath, remotePath, options)
}

func (s *SSHConnector) Download(ctx context.Context, remotePath, localPath string, options *FileTransferOptions) error {
	if err := s.ensureSftp(); err != nil {
		return err
	}

	srcFile, err := s.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file %s for download: %w", remotePath, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for %s: %w", localPath, err)
	}
	dstFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s for download: %w", localPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy content from remote %s to local %s: %w", remotePath, localPath, err)
	}

	if options != nil && options.Permissions != "" {
		perm, _ := strconv.ParseUint(options.Permissions, 8, 32)
		os.Chmod(localPath, os.FileMode(perm))
	}

	return nil
}

func (s *SSHConnector) DownloadDir(ctx context.Context, remoteDir, localDir string, options *FileTransferOptions) error {
	log := logger.Get()

	if err := s.ensureSftp(); err != nil {
		return err
	}

	log.Infof("Starting recursive download from remote directory '%s' to local directory '%s'", remoteDir, localDir)

	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local root directory %s: %w", localDir, err)
	}

	walker := s.sftpClient.Walk(remoteDir)
	for walker.Step() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := walker.Err(); err != nil {
			log.Warnf("Skipping path due to walk error on remote directory '%s': %v", walker.Path(), err)
			continue
		}

		remotePath := walker.Path()
		info := walker.Stat()

		relPath, err := filepath.Rel(remoteDir, remotePath)
		if err != nil {
			return fmt.Errorf("internal error: failed to calculate relative path for '%s' against base '%s': %w", remotePath, remoteDir, err)
		}
		localPath := filepath.Join(localDir, relPath)

		if info.IsDir() {
			log.Debugf("Creating local directory: %s", localPath)
			if err := os.MkdirAll(localPath, info.Mode().Perm()); err != nil {
				return fmt.Errorf("failed to create local directory '%s': %w", localPath, err)
			}
		} else {
			log.Debugf("Downloading remote file: %s", remotePath)
			srcFile, err := s.sftpClient.Open(remotePath)
			if err != nil {
				return fmt.Errorf("failed to open remote file '%s' for download: %w", remotePath, err)
			}
			defer srcFile.Close()
			dstFile, err := os.Create(localPath)
			if err != nil {
				return fmt.Errorf("failed to create local file '%s' for download: %w", localPath, err)
			}
			defer dstFile.Close()

			bytesCopied, err := io.Copy(dstFile, srcFile)
			if err != nil {
				_ = os.Remove(localPath)
				return fmt.Errorf("failed to copy content from remote '%s' to local '%s': %w", remotePath, localPath, err)
			}

			if err := os.Chmod(localPath, info.Mode().Perm()); err != nil {
				log.Warnf("Failed to set permissions on local file '%s': %v", localPath, err)
			}

			log.Debugf("Successfully downloaded %d bytes to %s", bytesCopied, localPath)
		}
	}

	log.Infof("Finished recursive download from '%s' to '%s'", remoteDir, localDir)
	return nil
}

func (s *SSHConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
	srcStat, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source path %s not found or not accessible: %w", srcPath, err)
	}

	if !srcStat.IsDir() {
		srcFile, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
		}
		defer srcFile.Close()
		return s.writeFileFromReader(ctx, srcFile, dstPath, options)
	}

	return s.copyDirViaTar(ctx, srcPath, dstPath, options)
}

func (s *SSHConnector) copyDirViaTar(ctx context.Context, srcDir, dstDir string, options *FileTransferOptions) error {
	log := logger.Get()
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	var tarball bytes.Buffer
	gzw := gzip.NewWriter(&tarball)
	tw := tar.NewWriter(gzw)

	walkErr := filepath.Walk(srcDir, func(path string, info fs.FileInfo, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}
		header, errHeader := tar.FileInfoHeader(info, info.Name())
		if errHeader != nil {
			return errHeader
		}
		var relPathErr error
		header.Name, relPathErr = filepath.Rel(srcDir, path)
		if relPathErr != nil {
			return fmt.Errorf("failed to make path relative for tar header %s: %w", path, relPathErr)
		}
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", path, err)
		}
		if info.Mode().IsRegular() {
			file, errOpen := os.Open(path)
			if errOpen != nil {
				return fmt.Errorf("failed to open file %s for tar: %w", path, errOpen)
			}
			defer file.Close()
			if _, errCopy := io.Copy(tw, file); errCopy != nil {
				return fmt.Errorf("failed to copy file %s content to tar: %w", path, errCopy)
			}
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("failed during tarball creation for %s: %w", srcDir, walkErr)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	tmpPath := filepath.Join("/tmp", fmt.Sprintf("connector-archive-%d-%s.tar.gz", time.Now().UnixNano(), filepath.Base(srcDir)))
	uploadErr := s.writeFileFromReader(ctx, &tarball, tmpPath, &FileTransferOptions{Sudo: false})
	if uploadErr != nil {
		return fmt.Errorf("failed to upload temporary archive to %s: %w", tmpPath, uploadErr)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _, rmErr := s.Exec(cleanupCtx, fmt.Sprintf("rm -f %s", shellEscape(tmpPath)), &ExecOptions{Sudo: false})
		if rmErr != nil {
			log.Errorf("%v Warning: failed to remove temporary archive %s on host %s: %v\n", os.Stderr, tmpPath, s.connCfg.Host, rmErr)
		}
	}()

	execOptsSudo := &ExecOptions{Sudo: opts.Sudo}

	destParentDir := filepath.Dir(dstDir)
	if destParentDir != "." && destParentDir != "/" && destParentDir != "" {
		_, stderr, mkdirErr := s.Exec(ctx, fmt.Sprintf("mkdir -p %s", shellEscape(destParentDir)), execOptsSudo)
		if mkdirErr != nil {
			return fmt.Errorf("failed to create remote parent directory %s (sudo: %t): %s (underlying error: %w)", destParentDir, opts.Sudo, string(stderr), mkdirErr)
		}
	}

	_, _, _ = s.Exec(ctx, fmt.Sprintf("rm -rf %s", shellEscape(dstDir)), execOptsSudo)

	_, stderr, mkdirErr := s.Exec(ctx, fmt.Sprintf("mkdir -p %s", shellEscape(dstDir)), execOptsSudo)
	if mkdirErr != nil {
		return fmt.Errorf("failed to create remote destination directory %s (sudo: %t): %s (underlying error: %w)", dstDir, opts.Sudo, string(stderr), mkdirErr)
	}

	extractCmd := fmt.Sprintf("tar -xzf %s -C %s", shellEscape(tmpPath), shellEscape(dstDir))
	_, stderr, execErr := s.Exec(ctx, extractCmd, execOptsSudo)
	if execErr != nil {
		return fmt.Errorf("failed to extract remote archive %s to %s (sudo: %t): %s (underlying error: %w)", tmpPath, dstDir, opts.Sudo, string(stderr), execErr)
	}

	if opts.Permissions != "" {
		if _, errP := strconv.ParseUint(opts.Permissions, 8, 32); errP != nil {
			return fmt.Errorf("invalid permissions format '%s' for %s: %w", opts.Permissions, dstDir, errP)
		}
		chmodCmd := fmt.Sprintf("chmod -R %s %s", shellEscape(opts.Permissions), shellEscape(dstDir))
		_, stderr, errChmod := s.Exec(ctx, chmodCmd, execOptsSudo)
		if errChmod != nil {
			return fmt.Errorf("failed to set permissions on %s to %s (sudo: %t): %s (underlying error: %w)", dstDir, opts.Permissions, opts.Sudo, string(stderr), errChmod)
		}
	}
	if opts.Owner != "" {
		ownerAndGroup := opts.Owner
		if opts.Group != "" {
			ownerAndGroup = fmt.Sprintf("%s:%s", opts.Owner, opts.Group)
		}
		chownCmd := fmt.Sprintf("chown -R %s %s", shellEscape(ownerAndGroup), shellEscape(dstDir))
		_, stderr, errChown := s.Exec(ctx, chownCmd, execOptsSudo)
		if errChown != nil {
			return fmt.Errorf("failed to set ownership on %s to %s (sudo: %t): %s (underlying error: %w)", dstDir, ownerAndGroup, opts.Sudo, string(stderr), errChown)
		}
	}

	return nil
}

func (s *SSHConnector) Fetch(ctx context.Context, remotePath, localPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}
	if !opts.Sudo {
		return s.Download(ctx, remotePath, localPath, &opts)
	}

	log := logger.Get()

	remoteTempPath := filepath.Join("/tmp", fmt.Sprintf("kubexm-fetch-%d-%s", time.Now().UnixNano(), filepath.Base(remotePath)))
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _, rmErr := s.Exec(cleanupCtx, fmt.Sprintf("rm -f %s", shellEscape(remoteTempPath)), &ExecOptions{Sudo: true})
		if rmErr != nil {
			log.Errorf("%v Warning: failed to remove temporary fetch file %s on host %s: %v\n", os.Stderr, remoteTempPath, s.connCfg.Host, rmErr)
		}
	}()

	stat, err := s.Stat(ctx, remotePath)
	if err != nil || !stat.IsExist {
		return fmt.Errorf("remote source path %s does not exist or is not accessible", remotePath)
	}

	cpFlags := "-p"
	if stat.IsDir {
		cpFlags += "r"
	}

	cpCmd := fmt.Sprintf("cp %s %s %s", cpFlags, shellEscape(remotePath), shellEscape(remoteTempPath))
	if _, stderr, err := s.Exec(ctx, cpCmd, &ExecOptions{Sudo: true}); err != nil {
		return fmt.Errorf("failed to copy remote file to temporary path with sudo: %s (underlying error: %w)", string(stderr), err)
	}

	chownCmd := fmt.Sprintf("chown %s %s", shellEscape(s.connCfg.User), shellEscape(remoteTempPath))
	if _, stderr, err := s.Exec(ctx, chownCmd, &ExecOptions{Sudo: true}); err != nil {
		log.Warnf("Failed to chown temporary file %s, SFTP download might fail: %s", remoteTempPath, string(stderr))
	}

	downloadOpts := &FileTransferOptions{
		Permissions: opts.Permissions,
		Timeout:     opts.Timeout,
		Sudo:        false,
	}

	if stat.IsDir {
		if err := s.DownloadDir(ctx, remoteTempPath, localPath, downloadOpts); err != nil {
			return fmt.Errorf("failed to download directory from temporary path %s: %w", remoteTempPath, err)
		}
	}

	if err := s.Download(ctx, remoteTempPath, localPath, downloadOpts); err != nil {
		return fmt.Errorf("failed to download from temporary path %s: %w", remoteTempPath, err)
	}

	return nil
}

func (s *SSHConnector) GetConnectionConfig() ConnectionCfg {
	return s.connCfg
}
