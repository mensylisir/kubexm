package images

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/logger"
)

type PushOptions struct {
	ClusterName   string
	ListFile      string
	Registry      string
	DryRun        bool
	Concurrency   int
	AuthFile      string
	SkipTLSVerify bool
}

var pushOptions = &PushOptions{}

func init() {
	ImagesCmd.AddCommand(pushCmd)
	pushCmd.Flags().StringVarP(&pushOptions.ClusterName, "cluster", "c", "", "Cluster name")
	pushCmd.Flags().StringVarP(&pushOptions.ListFile, "list", "l", "", "Path to file containing list of images (one per line)")
	pushCmd.Flags().StringVarP(&pushOptions.Registry, "registry", "r", "", "Target registry URL (e.g., registry.example.com:5000)")
	pushCmd.Flags().BoolVar(&pushOptions.DryRun, "dry-run", false, "Show what would be pushed without pushing")
	pushCmd.Flags().IntVar(&pushOptions.Concurrency, "concurrency", 5, "Number of images to push in parallel")
	pushCmd.Flags().StringVar(&pushOptions.AuthFile, "auth-file", "", "Path to authentication file for registry (docker config.json)")
	pushCmd.Flags().BoolVar(&pushOptions.SkipTLSVerify, "skip-tls-verify", false, "Skip TLS certificate verification")
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push images to a registry",
	Long: `Push images from a list file to a private registry using skopeo.

Example image list file (images.txt):
  docker.io/library/nginx:1.21.6
  docker.io/library/redis:7.0.5
  registry.k8s.io/kube-apiserver:v1.28.0`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		if pushOptions.ListFile == "" {
			return fmt.Errorf("image list file must be provided via --list or -l flag")
		}

		if pushOptions.Registry == "" {
			return fmt.Errorf("target registry must be provided via --registry or -r flag")
		}

		// Check if skopeo is available
		if _, err := exec.LookPath("skopeo"); err != nil {
			return fmt.Errorf("skopeo is required but not found in PATH. Please install skopeo first")
		}

		// Read image list
		data, err := os.ReadFile(pushOptions.ListFile)
		if err != nil {
			return fmt.Errorf("failed to read image list file: %w", err)
		}

		images := parseImageList(string(data))
		if len(images) == 0 {
			log.Info("No images found in the list file.")
			return nil
		}

		log.Infof("Found %d images to push to %s", len(images), pushOptions.Registry)

		if pushOptions.DryRun {
			log.Info("=== DRY RUN MODE ===")
			for _, img := range images {
				newImg := transformImage(img, pushOptions.Registry)
				log.Infof("Would push: %s -> %s", img, newImg)
			}
			return nil
		}

		return pushImages(images, pushOptions.Registry)
	},
}

func pushImages(images []string, registry string) error {
	log := logger.Get()
	jobs := make(chan string, len(images))
	errChan := make(chan error, len(images))
	var wg sync.WaitGroup

	started := time.Now()
	pushed := 0
	failed := 0

	for i := 0; i < pushOptions.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for img := range jobs {
				newImg := transformImage(img, registry)
				if err := pushImage(img, registry); err != nil {
					errChan <- fmt.Errorf("failed to push %s: %w", img, err)
					log.Warnf("Failed to push image: %s -> %s: %v", img, newImg, err)
					failed++
					continue
				}
				pushed++
				log.Debugf("Pushed: %s", newImg)
			}
		}(i)
	}

	for _, img := range images {
		jobs <- img
	}
	close(jobs)
	wg.Wait()
	close(errChan)

	elapsed := time.Since(started)
	var allErrors []string
	for err := range errChan {
		allErrors = append(allErrors, err.Error())
	}

	if len(allErrors) > 0 {
		log.Errorf("Failed to push %d/%d images to %s after %v:\n- %s",
			failed, len(images), registry, elapsed.Round(time.Second), strings.Join(allErrors, "\n- "))
		return fmt.Errorf("failed to push some images:\n- %s", strings.Join(allErrors, "\n- "))
	}

	log.Infof("Successfully pushed %d/%d images to %s in %v", pushed, len(images), registry, elapsed.Round(time.Second))
	return nil
}

func pushImage(image, registry string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	srcName := "docker://" + image
	destName := "docker://" + transformImage(image, registry)

	args := []string{"copy", "--all"}

	// Add auth file if specified
	if pushOptions.AuthFile != "" {
		args = append(args, "--src-auth-file", pushOptions.AuthFile)
		args = append(args, "--dest-auth-file", pushOptions.AuthFile)
	}

	// Handle TLS verification
	if pushOptions.SkipTLSVerify {
		args = append(args, "--dest-tls-verify=false")
	}

	args = append(args, srcName, destName)

	cmd := exec.CommandContext(ctx, "skopeo", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("skopeo failed: %w, output: %s", err, string(output))
	}

	return nil
}

func parseImageList(content string) []string {
	var images []string
	lines := splitLines(content)
	for _, line := range lines {
		line = trimSpace(line)
		if line != "" && !hasPrefix(line, "#") {
			images = append(images, line)
		}
	}
	return images
}

func transformImage(image, registry string) string {
	// Remove existing registry prefix if present
	image = removeRegistryPrefix(image)

	// Parse registry to get host:port
	registryHost := registry
	if u, err := url.Parse("scheme://" + registry); err == nil {
		registryHost = u.Host
		if u.Port() != "" {
			registryHost = fmt.Sprintf("%s:%s", u.Hostname(), u.Port())
		}
	}

	return registryHost + "/" + image
}

func removeRegistryPrefix(image string) string {
	prefixes := []string{
		"docker.io/",
		"registry.k8s.io/",
		"gcr.io/",
		"quay.io/",
		"k8s.gcr.io/",
	}
	for _, prefix := range prefixes {
		if hasPrefix(image, prefix) {
			return image[len(prefix):]
		}
	}
	return image
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
