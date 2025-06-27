package util

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetBinaryInfo(t *testing.T) {
	// Mock os.Getenv for KXZONE
	origKXZONE := os.Getenv("KXZONE")
	defer os.Setenv("KXZONE", origKXZONE)

	// Mock os.Getenv for KXZONE
	origKXZONE := os.Getenv("KXZONE")
	defer os.Setenv("KXZONE", origKXZONE)

	baseWorkDir := "/testwork"
	baseClusterName := "mycluster"

	// Helper to construct expected paths for readability in test cases
	// Example: makePath(baseWorkDir, baseClusterName, "etcd", "v3.5.9", "amd64", "etcd-v3.5.9-linux-amd64.tar.gz")
	// For container runtimes: makePath(baseWorkDir, baseClusterName, common.DefaultContainerRuntimeDir, "docker", "20.10.17", "amd64", "docker-20.10.17.tgz")
	// For tools: makePath(baseWorkDir, baseClusterName, "helm", "helm", "v3.9.0", "amd64", "helm-v3.9.0-linux-amd64.tar.gz")
	makeExpectedPaths := func(componentType BinaryType, componentNameForDir, version, arch, fileName string) (baseDir, componentDir, filePath string) {
		kubexmRoot := filepath.Join(baseWorkDir, common.KUBEXM)
		clusterBase := filepath.Join(kubexmRoot, baseClusterName)

		typeDirName := ""
		pathParts := []string{} // parts for componentDir relative to typeDirName

		switch componentType {
		case ETCD:
			typeDirName = DirNameEtcd
			pathParts = append(pathParts, version, arch)
		case KUBE, K3S, K8E:
			typeDirName = DirNameKubernetes
			pathParts = append(pathParts, version, arch)
		case CNI, CALICOCTL:
			typeDirName = filepath.Join(DirNameKubernetes, "cni")
			pathParts = append(pathParts, componentNameForDir, version, arch) // componentNameForDir here is actual component name
		case CONTAINERD, DOCKER, RUNC, CRIDOCKERD, CRICTL:
			typeDirName = DirNameContainerRuntime
			pathParts = append(pathParts, componentNameForDir, version, arch) // componentNameForDir is like "docker", "containerd"
		case HELM, BUILD, REGISTRY: // TOOLS
			typeDirName = string(componentType) // e.g., "helm", "registry"
			pathParts = append(pathParts, componentNameForDir, version, arch) // componentNameForDir is actual component name
		default:
			// This case should ideally not be hit if all types are handled
			// Or, could use componentName directly under clusterBase for unknown/new types
			typeDirName = "unknown_type"
			pathParts = append(pathParts, componentNameForDir, version, arch)
		}

		baseDir = filepath.Join(clusterBase, typeDirName)
		componentDir = filepath.Join(baseDir, pathParts...)
		filePath = filepath.Join(componentDir, fileName)
		return
	}

	tests := []struct {
		name                  string
		componentName         string
		version               string
		arch                  string
		zone                  string
		workDir               string
		clusterName           string
		expectedType          BinaryType
		expectedOS            string
		expectedArch          string // Expected resolved arch in BinaryInfo
		expectedFileName      string
		expectedURLContains   string
		expectedIsArchive     bool
		expectedBaseDir       string
		expectedComponentDir  string
		expectedFilePath      string
		expectError           bool
		expectedErrorContains string
	}{
		{
			name:                "etcd_default_zone_amd64",
			componentName:       ComponentEtcd,
			version:             "v3.5.9",
			arch:                "amd64",
			zone:                "",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        ETCD,
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "etcd-v3.5.9-linux-amd64.tar.gz",
			expectedURLContains: "github.com/coreos/etcd/releases/download/v3.5.9/etcd-v3.5.9-linux-amd64.tar.gz",
			expectedIsArchive:   true,
			expectedBaseDir:     filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameEtcd),
			expectedComponentDir: filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameEtcd, "v3.5.9", "amd64"),
			expectedFilePath:    filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameEtcd, "v3.5.9", "amd64", "etcd-v3.5.9-linux-amd64.tar.gz"),
		},
		{
			name:                "etcd_cn_zone_arm64",
			componentName:       ComponentEtcd,
			version:             "v3.5.9",
			arch:                "arm64",
			zone:                "cn",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        ETCD,
			expectedOS:          "linux",
			expectedArch:        "arm64",
			expectedFileName:    "etcd-v3.5.9-linux-arm64.tar.gz",
			expectedURLContains: "kubernetes-release.pek3b.qingstor.com/etcd/release/download/v3.5.9/etcd-v3.5.9-linux-arm64.tar.gz",
			expectedIsArchive:   true,
			expectedBaseDir:     filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameEtcd),
			expectedComponentDir: filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameEtcd, "v3.5.9", "arm64"),
			expectedFilePath:    filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameEtcd, "v3.5.9", "arm64", "etcd-v3.5.9-linux-arm64.tar.gz"),
		},
		{
			name:                "kubeadm_default_zone_amd64",
			componentName:       ComponentKubeadm,
			version:             "v1.23.5",
			arch:                "amd64",
			zone:                "",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        KUBE,
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "kubeadm",
			expectedURLContains: "dl.k8s.io/release/v1.23.5/bin/linux/amd64/kubeadm",
			expectedIsArchive:   false,
			expectedBaseDir:     filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameKubernetes),
			expectedComponentDir: filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameKubernetes, "v1.23.5", "amd64"),
			expectedFilePath:    filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameKubernetes, "v1.23.5", "amd64", "kubeadm"),
		},
		{
			name:                "containerd_cn_zone_arm64",
			componentName:       ComponentContainerd,
			version:             "1.7.1", // Version without v for containerd filename/url
			arch:                "arm64",
			zone:                "cn",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        CONTAINERD,
			expectedOS:          "linux",
			expectedArch:        "arm64",
			expectedFileName:    "containerd-1.7.1-linux-arm64.tar.gz",
			expectedURLContains: "kubernetes-release.pek3b.qingstor.com/containerd/containerd/releases/download/v1.7.1/containerd-1.7.1-linux-arm64.tar.gz",
			expectedIsArchive:   true,
			expectedBaseDir:     filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime),
			expectedComponentDir: filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime, "containerd", "1.7.1", "arm64"),
			expectedFilePath:    filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime, "containerd", "1.7.1", "arm64", "containerd-1.7.1-linux-arm64.tar.gz"),
		},
		{
			name:                "docker_default_zone_amd64",
			componentName:       ComponentDocker,
			version:             "20.10.17",
			arch:                "amd64",
			zone:                "",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        DOCKER,
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "docker-20.10.17.tgz",
			expectedURLContains: "download.docker.com/linux/static/stable/x86_64/docker-20.10.17.tgz",
			expectedIsArchive:   true,
			expectedBaseDir:     filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime),
			expectedComponentDir: filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime, "docker", "20.10.17", "amd64"),
			expectedFilePath:    filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime, "docker", "20.10.17", "amd64", "docker-20.10.17.tgz"),
		},
		{
			name:                "runc_default_zone_amd64",
			componentName:       ComponentRunc,
			version:             "v1.1.12",
			arch:                "amd64",
			zone:                "",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        RUNC,
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "runc.amd64",
			expectedURLContains: "github.com/opencontainers/runc/releases/download/v1.1.12/runc.amd64",
			expectedIsArchive:   false,
			expectedBaseDir:     filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime),
			expectedComponentDir: filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime, "runc", "v1.1.12", "amd64"),
			expectedFilePath:    filepath.Join(baseWorkDir, common.KUBEXM, baseClusterName, DirNameContainerRuntime, "runc", "v1.1.12", "amd64", "runc.amd64"),
		},
		{
			name:                  "unknown_component",
			componentName:         "unknown-component",
			version:               "v1.0.0",
			arch:                  "amd64",
			zone:                  "",
			workDir:               baseWorkDir,
			clusterName:           baseClusterName,
			expectError:           true,
			expectedErrorContains: "unknown binary component: unknown-component",
		},
		{
			name:                  "etcd_no_version",
			componentName:         ComponentEtcd,
			version:               "", // Empty version
			arch:                  "amd64",
			zone:                  "",
			workDir:               baseWorkDir,
			clusterName:           baseClusterName,
			expectError:           true,
			expectedErrorContains: "failed to render filename for etcd", // Error from template rendering due to empty version
		},
		{
			name:                  "etcd_no_workdir",
			componentName:         ComponentEtcd,
			version:               "v3.5.9",
			arch:                  "amd64",
			zone:                  "",
			workDir:               "", // Empty workDir
			clusterName:           baseClusterName,
			expectError:           true,
			expectedErrorContains: "workDir cannot be empty",
		},
		{
			name:                  "etcd_no_clustername",
			componentName:         ComponentEtcd,
			version:               "v3.5.9",
			arch:                  "amd64",
			zone:                  "",
			workDir:               baseWorkDir,
			clusterName:           "", // Empty clusterName
			expectError:           true,
			expectedErrorContains: "clusterName cannot be empty",
		},
		{
			name:                "k3s_amd64",
			componentName:       ComponentK3s,
			version:             "v1.25.4+k3s1",
			arch:                "amd64",
			zone:                "",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        K3S,
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "k3s",
			expectedURLContains: "github.com/k3s-io/k3s/releases/download/v1.25.4+k3s1/k3s",
			expectedIsArchive:   false,
		},
		{
			name:                "k3s_arm64",
			componentName:       ComponentK3s,
			version:             "v1.25.4+k3s1",
			arch:                "arm64",
			zone:                "",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        K3S,
			expectedOS:          "linux",
			expectedArch:        "arm64",
			expectedFileName:    "k3s-arm64",
			expectedURLContains: "github.com/k3s-io/k3s/releases/download/v1.25.4+k3s1/k3s-arm64",
			expectedIsArchive:   false,
		},
		// Kubectl example (KUBE type)
		{
			name:                "kubectl_cn_zone_amd64",
			componentName:       ComponentKubectl,
			version:             "v1.23.5",
			arch:                "amd64",
			zone:                "cn",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        KUBE,
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "kubectl",
			expectedURLContains: "kubernetes-release.pek3b.qingstor.com/release/v1.23.5/bin/linux/amd64/kubectl",
			expectedIsArchive:   false,
		},
		// CNI example
		{
			name:                "cni_default_zone_amd64",
			componentName:       ComponentKubeCNI,
			version:             "v1.2.0",
			arch:                "amd64",
			zone:                "",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        CNI,
			expectedOS:          "linux",
			expectedArch:        "amd64",
			expectedFileName:    "cni-plugins-linux-amd64-v1.2.0.tgz",
			expectedURLContains: "github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz",
			expectedIsArchive:   true,
		},
		// Helm example
		{
			name:                "helm_cn_zone_arm64",
			componentName:       ComponentHelm,
			version:             "v3.9.0",
			arch:                "arm64",
			zone:                "cn",
			workDir:             baseWorkDir,
			clusterName:         baseClusterName,
			expectedType:        HELM,
			expectedOS:          "linux",
			expectedArch:        "arm64",
			expectedFileName:    "helm-v3.9.0-linux-arm64.tar.gz",
			expectedURLContains: "kubernetes-helm.pek3b.qingstor.com/linux-arm64/v3.9.0/helm-v3.9.0-linux-arm64.tar.gz",
			expectedIsArchive:   true,
		},
	}

	// Dynamically populate expected paths for test cases that don't expect errors
	for i, tt := range tests {
		if !tt.expectError {
			// Get the details for the component to determine its type and componentNameForDir
			details, ok := knownBinaryDetails[strings.ToLower(tt.componentName)]
			if !ok {
				// This should not happen for valid test cases, but as a safeguard
				t.Fatalf("Test case %s uses an unknown component %s for path generation", tt.name, tt.componentName)
			}
			compNameForDir := details.componentNameForDir
			if compNameForDir == "" {
				compNameForDir = tt.componentName
			}
			// For CNI and CALICOCTL, the componentNameForDir in makeExpectedPaths should be the actual component name
			if details.binaryType == CNI || details.binaryType == CALICOCTL {
				compNameForDir = tt.componentName
			}


			expectedBase, expectedComp, expectedFP := makeExpectedPaths(details.binaryType, compNameForDir, tt.version, tt.expectedArch, tt.expectedFileName)
			tests[i].expectedBaseDir = expectedBase
			tests[i].expectedComponentDir = expectedComp
			tests[i].expectedFilePath = expectedFP
		}
	}


	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.zone == "cn" {
				os.Setenv("KXZONE", "cn")
			} else {
				os.Setenv("KXZONE", "") // Explicitly set to empty for default zone tests
			}

			// Use tt.workDir and tt.clusterName if provided, else use base defaults
			currentWorkDir := tt.workDir
			if currentWorkDir == "" { // Should only be for specific error tests
				currentWorkDir = baseWorkDir // Fallback, though error tests might expect this to be specifically empty
			}
			currentClusterName := tt.clusterName
			if currentClusterName == "" {
				currentClusterName = baseClusterName
			}

			// For error test cases where workDir or clusterName is intentionally empty
			if tt.name == "etcd_no_workdir" {
				currentWorkDir = ""
			}
			if tt.name == "etcd_no_clustername" {
				currentClusterName = ""
			}


			binInfo, err := GetBinaryInfo(tt.componentName, tt.version, tt.arch, GetZone(), currentWorkDir, currentClusterName)

			if tt.expectError {
				if err == nil {
					t.Errorf("GetBinaryInfo() expected error, got nil")
				} else if tt.expectedErrorContains != "" && !strings.Contains(err.Error(), tt.expectedErrorContains) {
					t.Errorf("GetBinaryInfo() expected error containing %q, got %q", tt.expectedErrorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("GetBinaryInfo() returned error: %v", err)
			}
			if binInfo.Type != tt.expectedType {
				t.Errorf("Expected Type %s, got %s", tt.expectedType, binInfo.Type)
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

			// Normalize paths for comparison, though makeExpectedPaths should already do this.
			if filepath.Clean(binInfo.BaseDir) != filepath.Clean(tt.expectedBaseDir) {
				t.Errorf("Expected BaseDir %s, got %s", tt.expectedBaseDir, binInfo.BaseDir)
			}
			if filepath.Clean(binInfo.ComponentDir) != filepath.Clean(tt.expectedComponentDir) {
				t.Errorf("Expected ComponentDir %s, got %s", tt.expectedComponentDir, binInfo.ComponentDir)
			}
			if filepath.Clean(binInfo.FilePath) != filepath.Clean(tt.expectedFilePath) {
				t.Errorf("Expected FilePath %s, got %s", tt.expectedFilePath, binInfo.FilePath)
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

func TestGetZone(t *testing.T) {
	origKXZONE := os.Getenv("KXZONE")
	defer os.Setenv("KXZONE", origKXZONE)

	tests := []struct {
		name      string
		kxZoneEnv string
		expected  string
	}{
		{"cn_lowercase", "cn", "cn"},
		{"cn_uppercase", "CN", "cn"},
		{"empty", "", ""},
		{"other_value", "us", ""},
		{"not_set", " KXZONE_NOT_SET ", ""}, // Special value to signal unsetting
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.kxZoneEnv == " KXZONE_NOT_SET " {
				os.Unsetenv("KXZONE")
			} else {
				os.Setenv("KXZONE", tt.kxZoneEnv)
			}
			if got := GetZone(); got != tt.expected {
				t.Errorf("GetZone() with KXZONE=%s got %q, want %q", tt.kxZoneEnv, got, tt.expected)
			}
		})
	}
}

func TestGetImageNames(t *testing.T) {
	imageNames := GetImageNames()
	if len(imageNames) == 0 {
		t.Error("GetImageNames() returned an empty list, expected some predefined image names")
	}

	// Spot check a few key images
	expectedImages := []string{"pause", "kube-apiserver", "coredns"}
	for _, expectedImage := range expectedImages {
		found := false
		for _, actualImage := range imageNames {
			if actualImage == expectedImage {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetImageNames() missing expected image: %s", expectedImage)
		}
	}
}
