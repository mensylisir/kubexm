package dep

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Package-level compiled regex for performance.
var rpmFilenameRegex = regexp.MustCompile(`^(.+)-([0-9][^-]*)-([^-]+)\.([^.]+)\.rpm$`)
var rpmNVRARegex = regexp.MustCompile(`^(.+)-([0-9][^-]*)-([^-]+)\.([^.]+)$`)

// RPMResolver resolves and downloads RPM packages with full dependency trees.
type RPMResolver struct {
	ostype       OSType
	osVersion    string
	arch         string
	workDir      string
	packageDir   string
	useDNF       bool   // Use DNF instead of YUM
	downloadOnly bool   // Only download, don't install
}

// NewRPMResolver creates a new RPM resolver.
func NewRPMResolver(ostype OSType, osVersion, arch, workDir string) *RPMResolver {
	return &RPMResolver{
		ostype:      ostype,
		osVersion:   osVersion,
		arch:        arch,
		workDir:     workDir,
		packageDir:  filepath.Join(workDir, "rpms", ostype.String(), osVersion, arch),
		useDNF:      true,
		downloadOnly: true,
	}
}

// Resolve resolves package names to full RPM packages with dependencies.
func (r *RPMResolver) Resolve(packages []string) (*RPMResolveResult, error) {
	result := &RPMResolveResult{
		Packages:  NewPackageList(),
		Unresolved: []string{},
	}

	if len(packages) == 0 {
		return result, nil
	}

	if err := os.MkdirAll(r.packageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create package directory: %w", err)
	}

	// Check which tool is available
	hasDNF := exec.CommandContext(context.Background(), "dnf", "--version").Run() == nil
	hasYUM := exec.CommandContext(context.Background(), "yum", "--version").Run() == nil
	hasYumdownloader := exec.CommandContext(context.Background(), "yumdownloader", "--version").Run() == nil

	if !hasDNF && !hasYUM {
		return nil, fmt.Errorf("neither dnf nor yum is available on this system")
	}

	// Enable fastest mirror
	env := os.Environ()
	env = append(env, "YUM.fastestmirror=True")

	if r.useDNF && hasDNF {
		return r.resolveWithDNF(packages, result, env)
	}

	// Fall back to yum
	if hasYUM || hasDNF {
		return r.resolveWithYUM(packages, result, env, hasYumdownloader)
	}

	return nil, fmt.Errorf("no suitable package resolver found")
}

func (r *RPMResolver) resolveWithDNF(packages []string, result *RPMResolveResult, env []string) (*RPMResolveResult, error) {
	// Step 1: Install dnf-plugins-core if not present (needed for download)
	installCmd := exec.Command("dnf", "install", "-y", "dnf-plugins-core")
	installCmd.Env = env
	installCmd.Run() // Best effort, may already be installed

	// Step 2: Install yum-utils for yumdownloader (provides download with resolve)
	installCmd2 := exec.Command("dnf", "install", "-y", "yum-utils")
	installCmd2.Env = env
	installCmd2.Run() // Best effort

	// Step 3: Download packages with full dependency resolution
	// Use yumdownloader which supports --resolve
	args := []string{
		"yumdownloader",
		"--resolve",
		"--destdir", r.packageDir,
		"--archlist", r.arch,
	}

	// For CentOS 7, use el7 repo naming; for Rocky/RHEL 8+, use el8
	if r.ostype == OSTypeCentOS && r.osVersion == "7" {
		args = append(args, "--releasever=7")
	}

	args = append(args, packages...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stderr = new(strings.Builder)
	cmd.Stdout = new(strings.Builder)

	if err := cmd.Run(); err != nil {
		// yumdownloader might not be available, try dnf download
		return r.resolveWithDNFNative(packages, result, env)
	}

	// Parse the output to identify downloaded packages
	if out, ok := cmd.Stdout.(*strings.Builder); ok {
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") ||
				strings.HasPrefix(line, "ftp://") || strings.HasSuffix(line, ".rpm") {
				pkg := r.parseDownloadURL(line)
				if pkg != nil {
					result.Packages.Add(pkg)
				}
			}
		}
	}

	// Also scan the package directory
	r.scanDownloadedPackages(result)

	return result, nil
}

func (r *RPMResolver) resolveWithDNFNative(packages []string, result *RPMResolveResult, env []string) (*RPMResolveResult, error) {
	// Use dnf repoquery to get all dependencies
	for _, pkg := range packages {
		args := []string{
			"dnf", "repoquery",
			"--qf", "%{name}-%{version}-%{release}.%{arch}",
			"--resolve",
			"--recursive",
			pkg,
		}

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = env
		output, err := cmd.Output()
		if err != nil {
			result.Unresolved = append(result.Unresolved, pkg)
			continue
		}

		depPackages := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, depPkg := range depPackages {
			depPkg = strings.TrimSpace(depPkg)
			if depPkg == "" {
				continue
			}
			pkgObj := r.parseRepoqueryOutput(depPkg)
			result.Packages.Add(pkgObj)
		}
	}

	// Now download all the packages
	if result.Packages.Len() > 0 {
		pkgNames := result.Packages.Names()
		downloadArgs := []string{
			"dnf", "download",
			"--destdir", r.packageDir,
			"--resolve",
		}
		downloadArgs = append(downloadArgs, pkgNames...)

		cmd := exec.Command(downloadArgs[0], downloadArgs[1:]...)
		cmd.Env = env
		cmd.Stderr = new(strings.Builder)
		cmd.Run() // Best effort
	}

	r.scanDownloadedPackages(result)
	return result, nil
}

func (r *RPMResolver) resolveWithYUM(packages []string, result *RPMResolveResult, env []string, hasYumdownloader bool) (*RPMResolveResult, error) {
	if hasYumdownloader {
		args := []string{
			"yumdownloader",
			"--resolve",
			"--destdir", r.packageDir,
		}
		args = append(args, packages...)

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = env
		cmd.Stderr = new(strings.Builder)

		if err := cmd.Run(); err != nil {
			result.Unresolved = append(result.Unresolved, packages...)
		}
	} else {
		// Fall back to yum install --downloadonly
		for _, pkg := range packages {
			args := []string{
				"yum", "install",
				"--downloadonly",
				"--downloaddir", r.packageDir,
				"-y", pkg,
			}

			cmd := exec.Command(args[0], args[1:]...)
			cmd.Env = env
			cmd.Stderr = new(strings.Builder)
			cmd.Run() // Best effort
		}
	}

	r.scanDownloadedPackages(result)
	return result, nil
}

func (r *RPMResolver) scanDownloadedPackages(result *RPMResolveResult) {
	entries, err := os.ReadDir(r.packageDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".rpm") {
			continue
		}

		matches := rpmFilenameRegex.FindStringSubmatch(name)
		if len(matches) >= 5 {
			pkg := &Package{
				Name:         matches[1],
				Version:      matches[2] + "-" + matches[3],
				Architecture: matches[4],
				LocalPath:    filepath.Join(r.packageDir, name),
			}

			if fi, err := entry.Info(); err == nil {
				pkg.Size = fi.Size()
			}

			result.Packages.Add(pkg)
		}
	}
}

func (r *RPMResolver) parseDownloadURL(line string) *Package {
	// Line format: http://mirror.centos.org/.../package-name-version-release.arch.rpm
	line = strings.TrimSpace(line)
	if !strings.HasSuffix(line, ".rpm") {
		return nil
	}

	parts := strings.Split(filepath.Base(line), "-")
	if len(parts) < 2 {
		return nil
	}

	// Try to extract version.arch from filename
	name := strings.TrimSuffix(filepath.Base(line), ".rpm")
	matches := rpmNVRARegex.FindStringSubmatch(name)
	if len(matches) >= 5 {
		return &Package{
			Name:         matches[1],
			Version:      matches[2] + "-" + matches[3],
			Architecture: matches[4],
			DownloadURL:  line,
			LocalPath:    filepath.Join(r.packageDir, filepath.Base(line)),
		}
	}

	return &Package{
		Name:        name,
		DownloadURL: line,
		LocalPath:   filepath.Join(r.packageDir, filepath.Base(line)),
	}
}

func (r *RPMResolver) parseRepoqueryOutput(line string) *Package {
	// Output format: name-version-release.arch
	matches := rpmNVRARegex.FindStringSubmatch(line)
	if len(matches) >= 5 {
		return &Package{
			Name:         matches[1],
			Version:      matches[2] + "-" + matches[3],
			Architecture: matches[4],
		}
	}
	return &Package{Name: line}
}

// RPMResolveResult holds the result of RPM resolution.
type RPMResolveResult struct {
	Packages    PackageList
	Unresolved  []string
	PackageDir  string
	TotalSize   int64
}

// GetPackages returns the resolved package list.
func (r *RPMResolveResult) GetPackages() PackageList { return r.Packages }

// GetUnresolved returns the list of unresolved packages.
func (r *RPMResolveResult) GetUnresolved() []string { return r.Unresolved }

// BuildRepo builds a local RPM repository from downloaded packages.
func (r *RPMResolveResult) BuildRepo(outputDir string, log func(string, ...interface{})) (string, error) {
	repoDir := filepath.Join(outputDir, "repos", "offline")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Copy all RPMs to repo directory
	for _, pkg := range r.Packages.pkgs {
		if pkg.LocalPath != "" && !strings.HasPrefix(pkg.LocalPath, repoDir) {
			src := pkg.LocalPath
			dest := filepath.Join(repoDir, filepath.Base(src))

			srcFile, err := os.Open(src)
			if err != nil {
				continue
			}
			destFile, err := os.Create(dest)
			if err != nil {
				srcFile.Close()
				continue
			}
			io.Copy(destFile, srcFile)
			srcFile.Close()
			destFile.Close()
		}
	}

	// Check for createrepo_c or createrepo
	var cmd *exec.Cmd
	for _, tool := range []string{"createrepo_c", "createrepo"} {
		if exec.Command(tool, "--version").Run() == nil {
			cmd = exec.Command(tool, repoDir)
			break
		}
	}

	if cmd == nil {
		return "", fmt.Errorf("neither createrepo_c nor createrepo is available. Install with: dnf install createrepo_c")
	}

	cmd.Stdout = new(strings.Builder)
	cmd.Stderr = new(strings.Builder)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("createrepo failed: %w", err)
	}

	if log != nil {
		log("RPM repository created at: %s", repoDir)
	}

	return repoDir, nil
}

// RepoConfig returns the repository configuration file content for this RPM repo.
func (r *RPMResolveResult) RepoConfig() string {
	return `[kubexm-offline]
name=KubeXM Offline Repository
baseurl=file:///mnt/kubexm/repos/offline
enabled=1
gpgcheck=0
priority=1
module_hotfixes=1
`
}

// VerifyRepo verifies the repository is valid.
func (r *RPMResolveResult) VerifyRepo(repoDir string) error {
	// Check that repodata exists
	repodataDir := filepath.Join(repoDir, "repodata")
	if _, err := os.Stat(repodataDir); os.IsNotExist(err) {
		return fmt.Errorf("repository metadata not found at %s", repodataDir)
	}

	// Check that primary.xml exists
	primaryXML := filepath.Join(repodataDir, "primary.xml")
	if _, err := os.Stat(primaryXML); os.IsNotExist(err) {
		return fmt.Errorf("primary metadata not found")
	}

	return nil
}

// PackageListSize calculates total size of all packages.
func (r *RPMResolveResult) PackageListSize() int64 {
	var total int64
	for _, pkg := range r.Packages.pkgs {
		if pkg.Size > 0 {
			total += pkg.Size
		}
	}
	r.TotalSize = total
	return total
}

// dumpYUMRepo dumps the current YUM/DNF repository configuration for offline use.
func DumpYUMRepos() ([]byte, error) {
	var buf bytes.Buffer

	// Get all repo files from /etc/yum.repos.d/
	repoFiles, _ := filepath.Glob("/etc/yum.repos.d/*.repo")
	for _, f := range repoFiles {
		data, err := os.ReadFile(f)
		if err == nil {
			buf.Write(data)
			buf.WriteString("\n")
		}
	}

	return buf.Bytes(), nil
}
