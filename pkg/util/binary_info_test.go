package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetBinaryInfo(t *testing.T) {
	// Mock os.Getenv for KXZONE
	origKXZONE := os.Getenv("KXZONE")
	defer os.Setenv("KXZONE", origKXZONE)

	// Mock ArchAlias if it were complex, but it's simple enough.
	// If RenderTemplate was complex or had external dependencies, it would also need mocking/testing.

	testWorkDir := "/testwork"
	testClusterName := "mycluster"

	// defaultArch := "amd64" // Removed as unused
	// Logic to get host architecture for tests if needed, or assume defaultArch.
	// For simplicity, we'll use defaultArch when arch is passed as empty.
	// In real GetBinaryInfo, it tries to get control node facts. We can't do that easily in unit test
	// without significant mocking of TaskContext. So, we test with explicit arch.

	tests := []struct {
		name                string
		componentName       string
		version             string
		arch                string // Arch to pass to GetBinaryInfo
		zone                string
		expectedOS          string
		expectedArch        string // Expected resolved arch in BinaryInfo
		expectedFileName    string
		expectedURLContains string // A substring to check in the URL
		expectedFilePath    string
		expectedIsArchive   bool
		expectError         bool
	}{
		{
			name:                "etcd_default_zone",
			componentName:       "etcd",
			version:             "v3.5.9",
			arch:                "amd64",
			zone:                "",
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "etcd-v3.5.9-linux-amd64.tar.gz",
			expectedURLContains: "github.com/coreos/etcd/releases/download/v3.5.9/etcd-v3.5.9-linux-amd64.tar.gz",
			expectedFilePath:    filepath.Join(testWorkDir, ".kubexm", testClusterName, "etcd", "v3.5.9", "amd64", "etcd-v3.5.9-linux-amd64.tar.gz"),
			expectedIsArchive:   true,
		},
		{
			name:                "etcd_cn_zone",
			componentName:       "etcd",
			version:             "v3.5.9",
			arch:                "arm64",
			zone:                "cn",
			expectedOS:          "linux",
			expectedArch:        "arm64",
			expectedFileName:    "etcd-v3.5.9-linux-arm64.tar.gz",
			expectedURLContains: "kubernetes-release.pek3b.qingstor.com/etcd/release/download/v3.5.9/etcd-v3.5.9-linux-arm64.tar.gz",
			expectedFilePath:    filepath.Join(testWorkDir, ".kubexm", testClusterName, "etcd", "v3.5.9", "arm64", "etcd-v3.5.9-linux-arm64.tar.gz"),
			expectedIsArchive:   true,
		},
		{
			name:                "kubeadm_default_zone_version_no_v",
			componentName:       "kubeadm",
			version:             "v1.23.5",
			arch:                "amd64",
			zone:                "",
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "kubeadm",
			expectedURLContains: "dl.k8s.io/release/v1.23.5/bin/linux/amd64/kubeadm",
			expectedFilePath:    filepath.Join(testWorkDir, ".kubexm", testClusterName, "kubernetes", "v1.23.5", "amd64", "kubeadm"),
			expectedIsArchive:   false,
		},
		{
			name:                "containerd_cn_zone",
			componentName:       "containerd",
			version:             "1.7.1", // Version without v
			arch:                "arm64",
			zone:                "cn",
			expectedOS:          "linux",
			expectedArch:        "arm64",
			expectedFileName:    "containerd-1.7.1-linux-arm64.tar.gz",
			expectedURLContains: "kubernetes-release.pek3b.qingstor.com/containerd/containerd/releases/download/v1.7.1/containerd-1.7.1-linux-arm64.tar.gz",
			// Path: workdir/.kubexm/${cluster_name}/container_runtime/${containerd_component_name}/${version}/${arch}/
			expectedFilePath:  filepath.Join(testWorkDir, ".kubexm", testClusterName, "containerd", "containerd", "1.7.1", "arm64", "containerd-1.7.1-linux-arm64.tar.gz"),
			expectedIsArchive: true,
		},
		{
			name:                "docker_default_zone",
			componentName:       "docker",
			version:             "20.10.17",
			arch:                "amd64",
			zone:                "",
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "docker-20.10.17.tgz",
			expectedURLContains: "download.docker.com/linux/static/stable/x86_64/docker-20.10.17.tgz",
			expectedFilePath:    filepath.Join(testWorkDir, ".kubexm", testClusterName, "docker", "docker", "20.10.17", "amd64", "docker-20.10.17.tgz"),
			expectedIsArchive:   true,
		},
		{
			name:                "runc_default_zone",
			componentName:       "runc",
			version:             "v1.1.12",
			arch:                "amd64",
			zone:                "",
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "runc.amd64",
			expectedURLContains: "github.com/opencontainers/runc/releases/download/v1.1.12/runc.amd64",
			expectedFilePath:    filepath.Join(testWorkDir, ".kubexm", testClusterName, "runc", "runc", "v1.1.12", "amd64", "runc.amd64"),
			expectedIsArchive:   false,
		},
		{
			name:          "unknown_component",
			componentName: "unknown-component",
			version:       "v1.0.0",
			arch:          "amd64",
			zone:          "",
			expectError:   true,
		},
		{
			name:          "etcd_no_version",
			componentName: "etcd",
			version:       "", // Empty version
			arch:          "amd64",
			zone:          "",
			expectError:   true, // Assuming version is required
		},
		{
			name:                "k3s_amd64",
			componentName:       "k3s",
			version:             "v1.25.4+k3s1",
			arch:                "amd64",
			zone:                "",
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "k3s",
			expectedURLContains: "github.com/k3s-io/k3s/releases/download/v1.25.4+k3s1/k3s",
			expectedFilePath:    filepath.Join(testWorkDir, ".kubexm", testClusterName, "k3s", "v1.25.4+k3s1", "amd64", "k3s"),
			expectedIsArchive:   false,
		},
		{
			name:                "k3s_arm64",
			componentName:       "k3s",
			version:             "v1.25.4+k3s1",
			arch:                "arm64",
			zone:                "",
			expectedOS:          "linux",
			expectedArch:        "arm64",
			expectedFileName:    "k3s-arm64",
			expectedURLContains: "github.com/k3s-io/k3s/releases/download/v1.25.4+k3s1/k3s-arm64",
			expectedFilePath:    filepath.Join(testWorkDir, ".kubexm", testClusterName, "k3s", "v1.25.4+k3s1", "arm64", "k3s-arm64"),
			expectedIsArchive:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.zone == "cn" {
				os.Setenv("KXZONE", "cn")
			} else {
				os.Setenv("KXZONE", "")
			}

			binInfo, err := GetBinaryInfo(tt.componentName, tt.version, tt.arch, GetZone(), testWorkDir, testClusterName)

			if tt.expectError {
				if err == nil {
					t.Errorf("GetBinaryInfo() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetBinaryInfo() returned error: %v", err)
			}

			if binInfo.OS != tt.expectedOS {
				t.Errorf("Expected OS %s, got %s", tt.expectedOS, binInfo.OS)
			}
			if binInfo.Arch != tt.expectedArch {
				t.Errorf("Expected Arch %s, got %s", tt.expectedArch, binInfo.Arch)
			}
			if binInfo.FileName != tt.expectedFileName {
				t.Errorf("Expected FileName %s, got %s", tt.expectedFileName, binInfo.FileName)
			}
			if !strings.Contains(binInfo.URL, tt.expectedURLContains) {
				t.Errorf("Expected URL to contain %s, got %s", tt.expectedURLContains, binInfo.URL)
			}
			if binInfo.IsArchive != tt.expectedIsArchive {
				t.Errorf("Expected IsArchive %v, got %v", tt.expectedIsArchive, binInfo.IsArchive)
			}
			// Normalize paths for comparison
			expectedFP := filepath.Clean(tt.expectedFilePath)
			actualFP := filepath.Clean(binInfo.FilePath)
			if actualFP != expectedFP {
				t.Errorf("Expected FilePath %s, got %s", expectedFP, actualFP)
			}

			// Verify BaseDir and ComponentDir structure
			// BaseDir: ${WORK_DIR}/.kubexm/${CLUSTER_NAME}/${Type}/
			// ComponentDir: ${BaseDir}/${ComponentNameFromRegistry}/${Version}/${Arch}/ OR ${BaseDir}/${Version}/${Arch}/
			expectedBaseDir := filepath.Join(testWorkDir, ".kubexm", testClusterName, string(binInfo.Type))
			if filepath.Clean(binInfo.BaseDir) != filepath.Clean(expectedBaseDir) {
				t.Errorf("Expected BaseDir %s, got %s", expectedBaseDir, binInfo.BaseDir)
			}

			var expectedComponentDir string
			if binInfo.Type == CONTAINERD || binInfo.Type == DOCKER || binInfo.Type == RUNC || binInfo.Type == CRIDOCKERD {
				// Find componentNameForDir from knownBinaryDetails
				details, _ := knownBinaryDetails[strings.ToLower(tt.componentName)]
				compDirName := details.componentNameForDir
				if compDirName == "" {compDirName = tt.componentName}
				expectedComponentDir = filepath.Join(expectedBaseDir, compDirName, tt.version, tt.expectedArch)
			} else {
				expectedComponentDir = filepath.Join(expectedBaseDir, tt.version, tt.expectedArch)
			}
			if filepath.Clean(binInfo.ComponentDir) != filepath.Clean(expectedComponentDir) {
				t.Errorf("Expected ComponentDir %s, got %s", expectedComponentDir, binInfo.ComponentDir)
			}

		})
	}
}

func TestArchAlias(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"amd64", "x86_64"},
		{"arm64", "aarch64"},
		{"x86_64", "x86_64"}, // Should already be aliased or stay same
		{"aarch64", "aarch64"},
		{"arm", "arm"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ArchAlias(tt.input); got != tt.expected {
				t.Errorf("ArchAlias(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// Test for RenderTemplate (basic)
func TestRenderTemplate(t *testing.T) {
	data := struct {
		Version string
		Arch    string
	}{"v1.0", "amd64"}
	tmplStr := "file-{{.Version}}-{{.Arch}}.zip"
	expected := "file-v1.0-amd64.zip"
	got, err := RenderTemplate(tmplStr, data)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}
	if got != expected {
		t.Errorf("RenderTemplate got %q, want %q", got, expected)
	}
}
