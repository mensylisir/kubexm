package util

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetBinaryInfo(t *testing.T) {
	origKXZONE := os.Getenv("KXZONE")
	defer os.Setenv("KXZONE", origKXZONE)

	baseWorkDir := "/testwork"
	baseClusterName := "mycluster"

	makeExpectedPaths := func(componentType BinaryType, componentNameForDir, version, arch, fileName string) (baseDir, componentDir, filePath string) {
		kubexmRoot := filepath.Join(baseWorkDir, common.KubexmRootDirName) // Updated
		clusterBase := filepath.Join(kubexmRoot, baseClusterName)
		typeDirName := ""
		pathParts := []string{}

		switch componentType {
		case ETCD:
			typeDirName = common.DefaultEtcdDir // Updated
			pathParts = append(pathParts, version, arch)
		case KUBE, K3S, K8E:
			typeDirName = common.DefaultKubernetesDir // Updated
			pathParts = append(pathParts, version, arch)
		case CNI, CALICOCTL:
			typeDirName = filepath.Join(common.DefaultKubernetesDir, "cni") // Updated
			pathParts = append(pathParts, componentNameForDir, version, arch)
		case CONTAINERD, DOCKER, RUNC, CRIDOCKERD, CRICTL:
			typeDirName = common.DefaultContainerRuntimeDir // Updated
			pathParts = append(pathParts, componentNameForDir, version, arch)
		case HELM, BUILD, REGISTRY:
			typeDirName = string(componentType)
			pathParts = append(pathParts, componentNameForDir, version, arch)
		default:
			typeDirName = "unknown_type"
			pathParts = append(pathParts, componentNameForDir, version, arch)
		}
		baseDir = filepath.Join(clusterBase, typeDirName)
		allComponentPathElements := append([]string{baseDir}, pathParts...)
		componentDir = filepath.Join(allComponentPathElements...)
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
		expectedArch          string
		expectedFileName      string
		expectedURLContains   string
		expectedIsArchive     bool
		expectedBaseDir       string
		expectedComponentDir  string
		expectedFilePath      string
		expectedChecksumVal   string
		expectedChecksumType  string
		expectError           bool
		expectedErrorContains string
	}{
		{
			name:                 "etcd_default_zone_amd64",
			componentName:        ComponentEtcd,
			version:              "v3.5.9",
			arch:                 "amd64",
			zone:                 "",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         ETCD,
			expectedOS:           "linux",
			expectedArch:         "amd64",
			expectedFileName:     "etcd-v3.5.9-linux-amd64.tar.gz",
			expectedURLContains:  "github.com/coreos/etcd/releases/download/v3.5.9/etcd-v3.5.9-linux-amd64.tar.gz",
			expectedIsArchive:    true,
			expectedChecksumVal:  "dummy-etcd-checksum-val",
			expectedChecksumType: "sha256",
		},
		{
			name:                 "etcd_cn_zone_arm64",
			componentName:        ComponentEtcd,
			version:              "v3.5.9",
			arch:                 "arm64",
			zone:                 "cn",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         ETCD,
			expectedOS:           "linux",
			expectedArch:         "arm64",
			expectedFileName:     "etcd-v3.5.9-linux-arm64.tar.gz",
			expectedURLContains:  "kubernetes-release.pek3b.qingstor.com/etcd/release/download/v3.5.9/etcd-v3.5.9-linux-arm64.tar.gz",
			expectedIsArchive:    true,
			expectedChecksumVal:  "dummy-etcd-checksum-val",
			expectedChecksumType: "sha256",
		},
		{
			name:                 "kubeadm_default_zone_amd64",
			componentName:        ComponentKubeadm,
			version:              "v1.23.5",
			arch:                 "amd64",
			zone:                 "",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         KUBE,
			expectedOS:           "linux",
			expectedArch:         "amd64",
			expectedFileName:     "kubeadm",
			expectedURLContains:  "dl.k8s.io/release/v1.23.5/bin/linux/amd64/kubeadm",
			expectedIsArchive:    false,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
		{
			name:                 "containerd_cn_zone_arm64",
			componentName:        ComponentContainerd,
			version:              "1.7.1",
			arch:                 "arm64",
			zone:                 "cn",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         CONTAINERD,
			expectedOS:           "linux",
			expectedArch:         "arm64",
			expectedFileName:     "containerd-1.7.1-linux-arm64.tar.gz",
			expectedURLContains:  "kubernetes-release.pek3b.qingstor.com/containerd/containerd/releases/download/v1.7.1/containerd-1.7.1-linux-arm64.tar.gz",
			expectedIsArchive:    true,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
		{
			name:                 "docker_default_zone_amd64",
			componentName:        ComponentDocker,
			version:              "20.10.17",
			arch:                 "amd64",
			zone:                 "",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         DOCKER,
			expectedOS:           "linux",
			expectedArch:         "amd64",
			expectedFileName:     "docker-20.10.17.tgz",
			expectedURLContains:  "download.docker.com/linux/static/stable/x86_64/docker-20.10.17.tgz",
			expectedIsArchive:    true,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
		{
			name:                 "runc_default_zone_amd64",
			componentName:        ComponentRunc,
			version:              "v1.1.12",
			arch:                 "amd64",
			zone:                 "",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         RUNC,
			expectedOS:           "linux",
			expectedArch:         "amd64",
			expectedFileName:     "runc.amd64",
			expectedURLContains:  "github.com/opencontainers/runc/releases/download/v1.1.12/runc.amd64",
			expectedIsArchive:    false,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
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
			version:               "",
			arch:                  "amd64",
			zone:                  "",
			workDir:               baseWorkDir,
			clusterName:           baseClusterName,
			expectError:           true,
			expectedErrorContains: "version cannot be empty for component etcd",
		},
		{
			name:                  "etcd_no_workdir",
			componentName:         ComponentEtcd,
			version:               "v3.5.9",
			arch:                  "amd64",
			zone:                  "",
			workDir:               "",
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
			clusterName:           "",
			expectError:           true,
			expectedErrorContains: "clusterName cannot be empty",
		},
		{
			name:                 "k3s_amd64",
			componentName:        ComponentK3s,
			version:              "v1.25.4+k3s1",
			arch:                 "amd64",
			zone:                 "",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         K3S,
			expectedOS:           "linux",
			expectedArch:         "amd64",
			expectedFileName:     "k3s",
			expectedURLContains:  "github.com/k3s-io/k3s/releases/download/v1.25.4+k3s1/k3s",
			expectedIsArchive:    false,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
		{
			name:                 "k3s_arm64",
			componentName:        ComponentK3s,
			version:              "v1.25.4+k3s1",
			arch:                 "arm64",
			zone:                 "",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         K3S,
			expectedOS:           "linux",
			expectedArch:         "arm64",
			expectedFileName:     "k3s-arm64",
			expectedURLContains:  "github.com/k3s-io/k3s/releases/download/v1.25.4+k3s1/k3s-arm64",
			expectedIsArchive:    false,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
		{
			name:                 "kubectl_cn_zone_amd64",
			componentName:        ComponentKubectl,
			version:              "v1.23.5",
			arch:                 "amd64",
			zone:                 "cn",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         KUBE,
			expectedOS:           "linux",
			expectedArch:         "amd64",
			expectedFileName:     "kubectl",
			expectedURLContains:  "kubernetes-release.pek3b.qingstor.com/release/v1.23.5/bin/linux/amd64/kubectl",
			expectedIsArchive:    false,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
		{
			name:                 "cni_default_zone_amd64",
			componentName:        ComponentKubeCNI,
			version:              "v1.2.0",
			arch:                 "amd64",
			zone:                 "",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         CNI,
			expectedOS:           "linux",
			expectedArch:         "amd64",
			expectedFileName:     "cni-plugins-linux-amd64-v1.2.0.tgz",
			expectedURLContains:  "github.com/containernetworking/plugins/releases/download/v1.2.0/cni-plugins-linux-amd64-v1.2.0.tgz",
			expectedIsArchive:    true,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
		{
			name:                 "helm_cn_zone_arm64",
			componentName:        ComponentHelm,
			version:              "v3.9.0",
			arch:                 "arm64",
			zone:                 "cn",
			workDir:              baseWorkDir,
			clusterName:          baseClusterName,
			expectedType:         HELM,
			expectedOS:           "linux",
			expectedArch:         "arm64",
			expectedFileName:     "helm-v3.9.0-linux-arm64.tar.gz",
			expectedURLContains:  "kubernetes-helm.pek3b.qingstor.com/linux-arm64/v3.9.0/helm-v3.9.0-linux-arm64.tar.gz",
			expectedIsArchive:    true,
			expectedChecksumVal:  "",
			expectedChecksumType: "",
		},
	}

	for i := range tests {
		tt := &tests[i]
		if !tt.expectError {
			details, ok := defaultKnownBinaryDetails[strings.ToLower(tt.componentName)]
			if !ok {
				t.Fatalf("Test case %s uses an unknown component %s for path generation", tt.name, tt.componentName)
			}
			compNameForDir := details.ComponentNameForDir
			if compNameForDir == "" {
				compNameForDir = tt.componentName
			}
			if details.BinaryType == CNI || details.BinaryType == CALICOCTL {
				compNameForDir = tt.componentName
			}
			expectedBase, expectedComp, expectedFP := makeExpectedPaths(details.BinaryType, compNameForDir, tt.version, tt.expectedArch, tt.expectedFileName)
			tt.expectedBaseDir = expectedBase
			tt.expectedComponentDir = expectedComp
			tt.expectedFilePath = expectedFP
		}
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			if tc.zone == "cn" {
				os.Setenv("KXZONE", "cn")
			} else {
				os.Setenv("KXZONE", "")
			}

			currentWorkDir := tc.workDir
			currentClusterName := tc.clusterName
			if tc.name == "etcd_no_workdir" { currentWorkDir = "" }
			if tc.name == "etcd_no_clustername" { currentClusterName = "" }

			provider := NewBinaryProvider()
			binInfo, err := provider.GetBinaryInfo(tc.componentName, tc.version, tc.arch, GetZone(), currentWorkDir, currentClusterName)

			if tc.expectError {
				if err == nil {
					t.Errorf("GetBinaryInfo() expected error, got nil")
				} else if tc.expectedErrorContains != "" && !strings.Contains(err.Error(), tc.expectedErrorContains) {
					t.Errorf("GetBinaryInfo() expected error containing %q, got %q", tc.expectedErrorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("GetBinaryInfo() returned error: %v", err)
			}
			if binInfo.Type != tc.expectedType {
				t.Errorf("Expected Type %s, got %s", tc.expectedType, binInfo.Type)
			}
			if binInfo.OS != tc.expectedOS {
				t.Errorf("Expected OS %s, got %s", tc.expectedOS, binInfo.OS)
			}
			if binInfo.Arch != tc.expectedArch {
				t.Errorf("Expected Arch %s, got %s", tc.expectedArch, binInfo.Arch)
			}
			if binInfo.FileName != tc.expectedFileName {
				t.Errorf("Expected FileName %s, got %s", tc.expectedFileName, binInfo.FileName)
			}
			if !strings.Contains(binInfo.URL, tc.expectedURLContains) {
				t.Errorf("Expected URL to contain %s, got %s", tc.expectedURLContains, binInfo.URL)
			}
			if binInfo.IsArchive != tc.expectedIsArchive {
				t.Errorf("Expected IsArchive %v, got %v", tc.expectedIsArchive, binInfo.IsArchive)
			}
			if filepath.Clean(binInfo.BaseDir) != filepath.Clean(tc.expectedBaseDir) {
				t.Errorf("Expected BaseDir %s, got %s", tc.expectedBaseDir, binInfo.BaseDir)
			}
			if filepath.Clean(binInfo.ComponentDir) != filepath.Clean(tc.expectedComponentDir) {
				t.Errorf("Expected ComponentDir %s, got %s", tc.expectedComponentDir, binInfo.ComponentDir)
			}
			if filepath.Clean(binInfo.FilePath) != filepath.Clean(tc.expectedFilePath) {
				t.Errorf("Expected FilePath %s, got %s", tc.expectedFilePath, binInfo.FilePath)
			}
			if binInfo.ExpectedChecksum != tc.expectedChecksumVal {
				t.Errorf("Expected ChecksumValue %s, got %s", tc.expectedChecksumVal, binInfo.ExpectedChecksum)
			}
			if binInfo.ExpectedChecksumType != tc.expectedChecksumType {
				t.Errorf("Expected ChecksumType %s, got %s", tc.expectedChecksumType, binInfo.ExpectedChecksumType)
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
		{"x86_64", "x86_64"},
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
		{"not_set", " KXZONE_NOT_SET ", ""},
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
