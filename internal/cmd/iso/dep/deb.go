package dep

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Package-level compiled regex for performance.
var debFilenameRegex = regexp.MustCompile(`^(.+?)_(.+?)_(.+?)\.deb$`)
var pkgNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.+-]+$`)

// DEBResolver resolves and downloads DEB packages with full dependency trees.
type DEBResolver struct {
	ostype      OSType
	osVersion   string
	arch        string
	workDir     string
	packageDir  string
	aptCacheDir string // Directory for apt cache
}

// NewDEBResolver creates a new DEB resolver.
func NewDEBResolver(ostype OSType, osVersion, arch, workDir string) *DEBResolver {
	// Normalize architecture names for DEB
	debArch := arch
	switch arch {
	case "amd64":
		debArch = "x86_64" // APT uses different names
	case "arm64":
		debArch = "aarch64"
	}

	return &DEBResolver{
		ostype:      ostype,
		osVersion:   osVersion,
		arch:        debArch,
		workDir:     workDir,
		packageDir:  filepath.Join(workDir, "debs", ostype.String(), osVersion, debArch),
		aptCacheDir: filepath.Join(workDir, "apt-cache"),
	}
}

// Resolve resolves package names to full DEB packages with dependencies.
func (r *DEBResolver) Resolve(packages []string) (*DEBResolveResult, error) {
	result := &DEBResolveResult{
		Packages:   NewPackageList(),
		Unresolved: []string{},
		PackageDir: r.packageDir,
	}

	if len(packages) == 0 {
		return result, nil
	}

	if err := os.MkdirAll(r.packageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create package directory: %w", err)
	}
	if err := os.MkdirAll(r.aptCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create apt cache directory: %w", err)
	}

	// Check for required tools
	hasAptGet := exec.CommandContext(context.Background(), "apt-get", "--version").Run() == nil
	hasAptCache := exec.CommandContext(context.Background(), "apt-cache", "--version").Run() == nil

	if !hasAptGet || !hasAptCache {
		return nil, fmt.Errorf("apt-get or apt-cache is not available on this system")
	}

	// Install apt-rdepends and dpkg-dev if not present
	r.installTools()

	// Update apt cache
	if err := r.updateAptCache(); err != nil {
		// Continue anyway - we may still be able to resolve some packages
	}

	// Step 1: For each package, get full dependency tree
	allDeps := make(map[string]bool)
	for _, pkg := range packages {
		deps, err := r.getDependencyTree(pkg)
		if err != nil {
			result.Unresolved = append(result.Unresolved, pkg)
			continue
		}
		for dep := range deps {
			allDeps[dep] = true
		}
	}

	// Step 2: Download all packages
	for dep := range allDeps {
		if err := r.downloadPackage(dep, result); err != nil {
			result.Unresolved = append(result.Unresolved, dep)
		}
	}

	// Step 3: Scan downloaded packages
	r.scanDownloadedPackages(result)

	return result, nil
}

func (r *DEBResolver) installTools() {
	// Install required tools in a single apt-get call
	tools := []string{"apt-rdepends", "dpkg-dev", "gdebi-core"}
	args := append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, tools...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Run() // Best effort
}

func (r *DEBResolver) updateAptCache() error {
	// Clean cache first
	cleanCmd := exec.Command("apt-get", "clean")
	cleanCmd.Run()

	// Update package lists
	updateCmd := exec.Command("apt-get", "update")
	updateCmd.Stdout = new(strings.Builder)
	updateCmd.Stderr = new(strings.Builder)
	return updateCmd.Run()
}

func (r *DEBResolver) getDependencyTree(pkg string) (map[string]bool, error) {
	deps := make(map[string]bool)
	deps[pkg] = true

	visited := make(map[string]bool)
	toVisit := []string{pkg}

	// BFS to traverse dependency tree
	for len(toVisit) > 0 {
		current := toVisit[0]
		toVisit = toVisit[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		// Get dependencies
		rawDeps, err := r.getPackageDeps(current)
		if err != nil {
			continue
		}

		for _, dep := range rawDeps {
			// Skip virtual packages and architecture-specific deps
			if strings.HasPrefix(dep, " ") || dep == "" || strings.Contains(dep, ":any") {
				continue
			}
			// Extract package name (strip version constraints)
			depName := r.extractPackageName(dep)
			if depName != "" && !visited[depName] {
				deps[depName] = true
				toVisit = append(toVisit, depName)
			}
		}
	}

	return deps, nil
}

func (r *DEBResolver) getPackageDeps(pkg string) ([]string, error) {
	// Use apt-cache depends to get package dependencies
	args := []string{"apt-cache", "depends", "--installed", pkg}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = new(strings.Builder)
	output, err := cmd.Output()
	if err != nil {
		// Try without --installed
		args = []string{"apt-cache", "depends", pkg}
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Stderr = new(strings.Builder)
		output, err = cmd.Output()
		if err != nil {
			return nil, err
		}
	}

	var deps []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// apt-cache depends output lines:
		//   package
		//   Depends: dep1
		//   PreDepends: dep2
		//   Recommends: dep3
		//   Suggests: dep4
		if strings.HasPrefix(line, "Depends:") {
			deps = append(deps, strings.TrimPrefix(line, "Depends:"))
		} else if strings.HasPrefix(line, "Pre-Depends:") {
			deps = append(deps, strings.TrimPrefix(line, "Pre-Depends:"))
		} else if strings.HasPrefix(line, "Recommends:") {
			// Include recommends as soft dependencies
			deps = append(deps, strings.TrimPrefix(line, "Recommends:"))
		}
	}

	return deps, nil
}

func (r *DEBResolver) extractPackageName(dep string) string {
	// dep can be like:
	//   package
	//   package:i386
	//   package (= 1.2.3-4)
	//   package:i386 (= 1.2.3-4)
	dep = strings.TrimSpace(dep)
	dep = strings.Split(dep, ":")[0]    // Remove arch suffix
	dep = strings.Split(dep, " ")[0]    // Remove version constraint
	dep = strings.Split(dep, "(")[0]   // Remove version in parens
	dep = strings.TrimSpace(dep)
	if dep == "" {
		return ""
	}
	// Only allow valid package names
	if !pkgNameRegex.MatchString(dep) {
		return ""
	}
	return dep
}

func (r *DEBResolver) downloadPackage(pkg string, result *DEBResolveResult) error {
	// Try downloading via apt-get install --download-only
	args := []string{
		"apt-get",
		"install",
		"--download-only",
		"--reinstall",
		"-y",
		"-o", "Dir::Cache::Archives=" + r.packageDir,
		pkg,
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = new(strings.Builder)
	cmd.Stderr = new(strings.Builder)

	if err := cmd.Run(); err != nil {
		// Try downloading via apt download
		return r.downloadPackageAPT(pkg, result)
	}

	return nil
}

func (r *DEBResolver) downloadPackageAPT(pkg string, result *DEBResolveResult) error {
	args := []string{
		"apt",
		"download",
		pkg,
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = r.packageDir
	cmd.Stderr = new(strings.Builder)

	return cmd.Run()
}

func (r *DEBResolver) scanDownloadedPackages(result *DEBResolveResult) {
	entries, err := os.ReadDir(r.packageDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".deb") {
			continue
		}

		matches := debFilenameRegex.FindStringSubmatch(name)
		if len(matches) >= 4 {
			pkg := &Package{
				Name:         matches[1],
				Version:      matches[2],
				Architecture: matches[3],
				LocalPath:    filepath.Join(r.packageDir, name),
			}

			if fi, err := entry.Info(); err == nil {
				pkg.Size = fi.Size()
			}

			result.Packages.Add(pkg)
		}
	}
}

// DEBResolveResult holds the result of DEB resolution.
type DEBResolveResult struct {
	Packages   PackageList
	Unresolved []string
	PackageDir string
	TotalSize  int64
}

// GetPackages returns the resolved package list.
func (r *DEBResolveResult) GetPackages() PackageList { return r.Packages }

// GetUnresolved returns the list of unresolved packages.
func (r *DEBResolveResult) GetUnresolved() []string { return r.Unresolved }

// PackageListSize calculates total size.
func (r *DEBResolveResult) PackageListSize() int64 {
	var total int64
	for _, pkg := range r.Packages.pkgs {
		if pkg.Size > 0 {
			total += pkg.Size
		}
	}
	r.TotalSize = total
	return total
}

// BuildRepo builds a local DEB repository from downloaded packages.
func (r *DEBResolveResult) BuildRepo(outputDir string, log func(string, ...interface{})) (string, error) {
	repoDir := filepath.Join(outputDir, "repos", "offline", "dists", "offline")

	if err := os.MkdirAll(filepath.Join(repoDir, "main", "binary-amd64"), 0755); err != nil {
		return "", fmt.Errorf("failed to create repo directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "main", "binary-arm64"), 0755); err != nil {
		return "", fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Copy all DEBs to pool directory
	poolDir := filepath.Join(outputDir, "repos", "offline", "pool", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create pool directory: %w", err)
	}

	for _, pkg := range r.Packages.pkgs {
		if pkg.LocalPath != "" {
			src := pkg.LocalPath
			dest := filepath.Join(poolDir, filepath.Base(src))

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

	// Generate Packages files
	archDirs := []string{"binary-amd64", "binary-arm64"}
	for _, archDir := range archDirs {
		pkgDir := filepath.Join(repoDir, "main", archDir)
		if err := r.generatePackagesFile(pkgDir, poolDir, archDir); err != nil {
			if log != nil {
				log("Warning: failed to generate Packages for %s: %v", archDir, err)
			}
		}
	}

	// Generate Release file
	if err := r.generateReleaseFile(repoDir); err != nil {
		if log != nil {
			log("Warning: failed to generate Release file: %v", err)
		}
	}

	if log != nil {
		log("DEB repository created at: %s", repoDir)
	}

	return repoDir, nil
}

func (r *DEBResolveResult) generatePackagesFile(pkgDir, poolDir, arch string) error {
	// Use dpkg-scanpackages if available
	for _, tool := range []string{"dpkg-scanpackages", "apt-get"} {
		cmd := exec.Command(tool, "--help")
		if cmd.Run() != nil {
			continue
		}

		// Try dpkg-scanpackages
		repoRoot := filepath.Dir(filepath.Dir(pkgDir))
		scanCmd := exec.Command("dpkg-scanpackages",
			poolDir,
			"/dev/null",
		)
		scanCmd.Dir = repoRoot

		var buf bytes.Buffer
		scanCmd.Stdout = &buf
		scanCmd.Stderr = new(strings.Builder)

		gzipped := bytes.Buffer{}
		gz := gzip.NewWriter(&gzipped)
		scanCmd.Stdout = gz

		if err := scanCmd.Run(); err != nil {
			gz.Close()
			// Fall back to manual generation
			return r.generatePackagesFileManual(pkgDir, poolDir, arch)
		}
		gz.Close()

		pkgFile := filepath.Join(pkgDir, "Packages.gz")
		return os.WriteFile(pkgFile, gzipped.Bytes(), 0644)
	}

	return r.generatePackagesFileManual(pkgDir, poolDir, arch)
}

func (r *DEBResolveResult) generatePackagesFileManual(pkgDir, poolDir, arch string) error {
	var buf bytes.Buffer

	entries, err := os.ReadDir(poolDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".deb") {
			continue
		}

		debPath := filepath.Join(poolDir, entry.Name())

		// Parse DEB control info using dpkg-deb
		cmd := exec.Command("dpkg-deb", "-I", debPath, "control")
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		buf.Write(output)
		buf.WriteString("\n")
	}

	pkgFile := filepath.Join(pkgDir, "Packages")
	if err := os.WriteFile(pkgFile, buf.Bytes(), 0644); err != nil {
		return err
	}

	// Compress to Packages.gz
	gzFile := filepath.Join(pkgDir, "Packages.gz")
	f, err := os.Create(gzFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	_, err = gz.Write(buf.Bytes())
	return err
}

func (r *DEBResolveResult) generateReleaseFile(repoDir string) error {
	// Generate Release file
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Archive: stable\nVersion: %s\nComponent: main\nOrigin: KubeXM\nLabel: KubeXM Offline Repository\n", r.PackageDir))

	releaseFile := filepath.Join(repoDir, "Release")
	return os.WriteFile(releaseFile, buf.Bytes(), 0644)
}

// RepoConfig returns the DEB repository configuration for sources.list.
func (r *DEBResolveResult) RepoConfig() string {
	return `deb [trusted=yes] file:/mnt/kubexm/repos/offline offline main
`
}

// SourceConfig returns the source.list entry.
func (r *DEBResolveResult) SourceConfig() string {
	return `deb-src [trusted=yes] file:/mnt/kubexm/repos/offline offline main
`
}

// VerifyRepo verifies the repository is valid.
func (r *DEBResolveResult) VerifyRepo(repoDir string) error {
	// Check Packages files exist
	for _, arch := range []string{"binary-amd64", "binary-arm64"} {
		pkgFile := filepath.Join(repoDir, "main", arch, "Packages.gz")
		if _, err := os.Stat(pkgFile); os.IsNotExist(err) {
			pkgFile = filepath.Join(repoDir, "main", arch, "Packages")
			if _, err := os.Stat(pkgFile); os.IsNotExist(err) {
				return fmt.Errorf("Packages file not found for %s", arch)
			}
		}
	}
	return nil
}

// DumpAptSources dumps current APT sources for offline use.
func DumpAptSources() ([]byte, error) {
	var buf bytes.Buffer

	sourcesDirs := []string{"/etc/apt/sources.list", "/etc/apt/sources.list.d/"}

	data, err := os.ReadFile(sourcesDirs[0])
	if err == nil {
		buf.Write(data)
	}

	entries, err := os.ReadDir(sourcesDirs[1])
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".list") {
				continue
			}
			fullPath := filepath.Join(sourcesDirs[1], entry.Name())
			data, err := os.ReadFile(fullPath)
			if err == nil {
				buf.WriteString("# ")
				buf.WriteString(entry.Name())
				buf.WriteString("\n")
				buf.Write(data)
				buf.WriteString("\n")
			}
		}
	}

	return buf.Bytes(), nil
}

// DownloadPackageFromURL downloads a DEB package directly from a URL.
func DownloadPackageFromURL(url, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	filename := filepath.Base(url)
	destPath := filepath.Join(destDir, filename)

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// DetectHostArch returns the host architecture in DEB format.
func DetectHostArch() string {
	a := runtime.GOARCH
	switch a {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "arm":
		return "armhf"
	case "386":
		return "i386"
	case "ppc64le":
		return "ppc64el"
	case "s390x":
		return "s390x"
	default:
		return a
	}
}

// DetectHostArchRPM returns the host architecture in RPM format.
func DetectHostArchRPM() string {
	a := runtime.GOARCH
	switch a {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "arm":
		return "armhfp"
	case "386":
		return "i686"
	case "ppc64le":
		return "ppc64le"
	case "s390x":
		return "s390x"
	default:
		return a
	}
}

// extractTarGzFromDeb extracts files from a .deb archive.
func extractTarGzFromDeb(debPath, destDir string) error {
	// .deb is an ar archive. We need to extract the data.tar.gz or data.tar.xz
	cmd := exec.Command("dpkg-deb", "-x", debPath, destDir)
	return cmd.Run()
}

// ReadDebControl reads the control information from a .deb package.
func ReadDebControl(debPath string) (map[string]string, error) {
	cmd := exec.Command("dpkg-deb", "-I", debPath, "control")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	control := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var currentKey string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// Continuation of previous field
			if currentKey != "" {
				control[currentKey] += " " + strings.TrimSpace(line)
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			currentKey = strings.TrimSpace(parts[0])
			control[currentKey] = strings.TrimSpace(parts[1])
		}
	}

	return control, nil
}
