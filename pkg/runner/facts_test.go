package runner

import (
	"context"
	"errors"
	"fmt"
	"strings" // Added import
	"testing"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGatherFacts_Success_Linux runs a table-driven test for successful GatherFacts scenarios on Linux.
func TestGatherFacts_Success_Linux(t *testing.T) {
	baseMockOS := &connector.OS{
		ID:         "ubuntu",
		VersionID:  "22.04",
		PrettyName: "Ubuntu",
		Kernel:     "5.15.0-generic",
		Arch:       "x86_64",
	}

	tests := []struct {
		name              string
		osID              string
		hostname          string
		nprocOutput       string
		meminfoOutput     string
		ipRouteOutput     string
		expectedHostname  string
		expectedCPUs      int
		expectedMemoryMiB uint64
		expectedIPv4      string
		expectedPkgMgr    PackageManagerType
		expectedInitSys   InitSystemType
		setupMocks        func(mConn *mocks.Connector, hostname, nproc, meminfo, ipRoute string, currentOS *connector.OS)
	}{
		{
			name:              "Ubuntu 22.04",
			osID:              "ubuntu",
			hostname:          "ubuntu-host.example.com",
			nprocOutput:       "8",
			meminfoOutput:     "MemTotal:       16384000 kB",
			ipRouteOutput:     "192.168.1.100",
			expectedHostname:  "ubuntu-host.example.com",
			expectedCPUs:      8,
			expectedMemoryMiB: 16000,
			expectedIPv4:      "192.168.1.100",
			expectedPkgMgr:    PackageManagerApt,
			expectedInitSys:   InitSystemSystemd,
			setupMocks: func(mConn *mocks.Connector, hostname, nproc, meminfo, ipRoute string, currentOS *connector.OS) {
				mConn.On("IsConnected").Return(true)
				mConn.On("GetOS", mock.Anything).Return(currentOS, nil)
				mConn.On("Exec", mock.Anything, "hostname -f", (*connector.ExecOptions)(nil)).Return([]byte(hostname+"\n"), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "nproc", (*connector.ExecOptions)(nil)).Return([]byte(nproc+"\n"), []byte{}, nil).Once()
				// Corrected to return only the numeric part for memory
				mConn.On("Exec", mock.Anything, "grep MemTotal /proc/meminfo | awk '{print $2}'", (*connector.ExecOptions)(nil)).Return([]byte(strings.Fields(meminfo)[1]), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte(ipRoute+"\n"), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte("2001:db8::1\n"), []byte{}, nil).Once()
				mConn.On("LookPath", mock.Anything, "systemctl").Return("/bin/systemctl", nil).Once()
			},
		},
		{
			name:              "CentOS 7 with YUM",
			osID:              "centos",
			hostname:          "centos-host",
			nprocOutput:       "4",
			meminfoOutput:     "MemTotal:        8192000 kB",
			ipRouteOutput:     "10.0.0.5",
			expectedHostname:  "centos-host",
			expectedCPUs:      4,
			expectedMemoryMiB: 8000,
			expectedIPv4:      "10.0.0.5",
			expectedPkgMgr:    PackageManagerYum,
			expectedInitSys:   InitSystemSystemd,
			setupMocks: func(mConn *mocks.Connector, hostname, nproc, meminfo, ipRoute string, currentOS *connector.OS) {
				mConn.On("IsConnected").Return(true)
				mConn.On("GetOS", mock.Anything).Return(currentOS, nil)
				mConn.On("Exec", mock.Anything, "hostname -f", (*connector.ExecOptions)(nil)).Return([]byte(hostname), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "nproc", (*connector.ExecOptions)(nil)).Return([]byte(nproc), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "grep MemTotal /proc/meminfo | awk '{print $2}'", (*connector.ExecOptions)(nil)).Return([]byte(strings.Fields(meminfo)[1]), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte(ipRoute), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte(""), []byte{}, nil).Once()
				mConn.On("LookPath", mock.Anything, "dnf").Return("", errors.New("not found")).Once()
				mConn.On("LookPath", mock.Anything, "yum").Return("/usr/bin/yum", nil).Once()
				mConn.On("LookPath", mock.Anything, "systemctl").Return("/bin/systemctl", nil).Once()
			},
		},
		{
			name:              "Fedora with DNF",
			osID:              "fedora",
			hostname:          "fedora-host.local",
			nprocOutput:       "16",
			meminfoOutput:     "MemTotal:       32768000 kB",
			ipRouteOutput:     "192.168.10.20",
			expectedHostname:  "fedora-host.local",
			expectedCPUs:      16,
			expectedMemoryMiB: 32000,
			expectedIPv4:      "192.168.10.20",
			expectedPkgMgr:    PackageManagerDnf,
			expectedInitSys:   InitSystemSystemd,
			setupMocks: func(mConn *mocks.Connector, hostname, nproc, meminfo, ipRoute string, currentOS *connector.OS) {
				mConn.On("IsConnected").Return(true)
				mConn.On("GetOS", mock.Anything).Return(currentOS, nil)
				mConn.On("Exec", mock.Anything, "hostname -f", (*connector.ExecOptions)(nil)).Return([]byte(hostname), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "nproc", (*connector.ExecOptions)(nil)).Return([]byte(nproc), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "grep MemTotal /proc/meminfo | awk '{print $2}'", (*connector.ExecOptions)(nil)).Return([]byte(strings.Fields(meminfo)[1]), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte(ipRoute), []byte{}, nil).Once()
				mConn.On("Exec", mock.Anything, "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte(""), []byte{}, nil).Once()
				mConn.On("LookPath", mock.Anything, "dnf").Return("/usr/bin/dnf", nil).Once()
				mConn.On("LookPath", mock.Anything, "systemctl").Return("/bin/systemctl", nil).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := mocks.NewConnector(t) // Use testify's NewConnector
			r := &defaultRunner{}

			currentOS := *baseMockOS
			currentOS.ID = tt.osID
			tt.setupMocks(mockConn, tt.hostname, tt.nprocOutput, tt.meminfoOutput, tt.ipRouteOutput, &currentOS)

			facts, err := r.GatherFacts(context.Background(), mockConn)

			assert.NoError(t, err)
			assert.NotNil(t, facts)
			assert.Equal(t, tt.expectedHostname, facts.Hostname)
			assert.Equal(t, currentOS.Kernel, facts.Kernel)
			assert.Equal(t, tt.expectedCPUs, facts.TotalCPU)
			assert.Equal(t, tt.expectedMemoryMiB, facts.TotalMemory)
			assert.Equal(t, tt.expectedIPv4, facts.IPv4Default)
			if assert.NotNil(t, facts.OS) {
				assert.Equal(t, tt.osID, facts.OS.ID)
				assert.Equal(t, currentOS.PrettyName, facts.OS.PrettyName)
			}

			if assert.NotNil(t, facts.PackageManager) {
				assert.Equal(t, tt.expectedPkgMgr, facts.PackageManager.Type)
			}
			if assert.NotNil(t, facts.InitSystem) {
				assert.Equal(t, tt.expectedInitSys, facts.InitSystem.Type)
			}
			// mockConn.AssertExpectations(t) // Handled by NewConnector's t.Cleanup
		})
	}
}

func TestGatherFacts_Success_Darwin(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}

	mockOS := &connector.OS{
		ID:         "darwin",
		VersionID:  "12.1",
		PrettyName: "macOS",
		Kernel:     "21.2.0 Darwin Kernel Version 21.2.0",
		Arch:       "arm64",
	}

	hostname := "macbook-pro.local"
	ncpuOutput := "10"
	memsizeOutput := "17179869184"

	mockConn.On("IsConnected").Return(true)
	mockConn.On("GetOS", mock.Anything).Return(mockOS, nil)
	mockConn.On("Exec", mock.Anything, "hostname -f", (*connector.ExecOptions)(nil)).Return([]byte(hostname+"\n"), []byte{}, nil).Once()
	mockConn.On("Exec", mock.Anything, "sysctl -n hw.ncpu", (*connector.ExecOptions)(nil)).Return([]byte(ncpuOutput+"\n"), []byte{}, nil).Once()
	mockConn.On("Exec", mock.Anything, "sysctl -n hw.memsize", (*connector.ExecOptions)(nil)).Return([]byte(memsizeOutput+"\n"), []byte{}, nil).Once()

	mockConn.On("LookPath", mock.Anything, "apt-get").Return("", errors.New("not found")).Once()
	mockConn.On("LookPath", mock.Anything, "dnf").Return("", errors.New("not found")).Once()
	mockConn.On("LookPath", mock.Anything, "yum").Return("", errors.New("not found")).Once()
	mockConn.On("LookPath", mock.Anything, "systemctl").Return("", errors.New("not found")).Once()
	mockConn.On("LookPath", mock.Anything, "service").Return("", errors.New("not found")).Once()
	mockConn.On("Stat", mock.Anything, "/etc/init.d").Return(&connector.FileStat{IsExist: false}, nil).Once()

	facts, err := r.GatherFacts(context.Background(), mockConn)

	assert.NoError(t, err)
	assert.NotNil(t, facts)
	assert.Equal(t, hostname, facts.Hostname)
	assert.Equal(t, mockOS.Kernel, facts.Kernel)
	assert.Equal(t, 10, facts.TotalCPU)
	assert.Equal(t, uint64(16384), facts.TotalMemory)
	assert.Equal(t, "", facts.IPv4Default)
	assert.Equal(t, "", facts.IPv6Default)
	if assert.NotNil(t, facts.OS){
		assert.Equal(t, "darwin", facts.OS.ID)
	}
	assert.Nil(t, facts.PackageManager)
	assert.Nil(t, facts.InitSystem)
}


func TestGatherFacts_ConnectorNotConnected(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}

	mockConn.On("IsConnected").Return(false)

	facts, err := r.GatherFacts(context.Background(), mockConn)

	assert.Error(t, err)
	assert.Nil(t, facts)
	assert.Contains(t, err.Error(), "connector is not connected")
}

func TestGatherFacts_GetOS_Fails(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}

	mockConn.On("IsConnected").Return(true)
	mockConn.On("GetOS", mock.Anything).Return(nil, errors.New("failed to get OS"))

	facts, err := r.GatherFacts(context.Background(), mockConn)

	assert.Error(t, err)
	assert.Nil(t, facts)
	assert.Contains(t, err.Error(), "failed to get OS info")
}

func TestGatherFacts_GetOS_ReturnsNilOS(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}
	mockConn.On("IsConnected").Return(true)
	mockConn.On("GetOS", mock.Anything).Return(nil, nil)

	facts, err := r.GatherFacts(context.Background(), mockConn)
	assert.Error(t, err)
	assert.Nil(t, facts)
	assert.Contains(t, err.Error(), "conn.GetOS returned nil OS without error")
}


func TestGatherFacts_Hostname_Fails(t *testing.T) {
	mockConn := mocks.NewConnector(t)
	r := &defaultRunner{}

	mockOS := &connector.OS{ID: "linux", Kernel: "test-kernel"}
	mockConn.On("IsConnected").Return(true)
	mockConn.On("GetOS", mock.Anything).Return(mockOS, nil)

	mockConn.On("Exec", mock.Anything, "hostname -f", (*connector.ExecOptions)(nil)).Return(nil, nil, errors.New("hostname -f failed")).Once()
	mockConn.On("Exec", mock.Anything, "hostname", (*connector.ExecOptions)(nil)).Return(nil, nil, errors.New("hostname failed")).Once() // Fallback

	// These Exec calls are part of the errgroup and might not be reached if hostname fails first and the context is cancelled.
	mockConn.On("Exec", mock.Anything, "nproc", (*connector.ExecOptions)(nil)).Return([]byte("1"), []byte{}, nil).Maybe()
	mockConn.On("Exec", mock.Anything, "grep MemTotal /proc/meminfo | awk '{print $2}'", (*connector.ExecOptions)(nil)).Return([]byte("1024000"), []byte{}, nil).Maybe()
	mockConn.On("Exec", mock.Anything, "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte("1.2.3.4"), []byte{}, nil).Maybe()
	mockConn.On("Exec", mock.Anything, "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1", (*connector.ExecOptions)(nil)).Return([]byte("::1"), []byte{}, nil).Maybe()

	// Mocks for package manager and init system detection, may not be called if hostname fails early
	mockConn.On("LookPath", mock.Anything, "apt-get").Return("", errors.New("not found")).Maybe()
	mockConn.On("LookPath", mock.Anything, "dnf").Return("", errors.New("not found")).Maybe()
	mockConn.On("LookPath", mock.Anything, "yum").Return("", errors.New("not found")).Maybe()
	mockConn.On("LookPath", mock.Anything, "systemctl").Return("", errors.New("not found")).Maybe()
	mockConn.On("LookPath", mock.Anything, "service").Return("", errors.New("not found")).Maybe()
	mockConn.On("Stat", mock.Anything, "/etc/init.d").Return(&connector.FileStat{IsExist: false}, nil).Maybe()


	facts, err := r.GatherFacts(context.Background(), mockConn)

	assert.Error(t, err)
	assert.NotNil(t, facts)
	assert.Contains(t, err.Error(), "failed to get hostname")
}


// TestDetectPackageManager covers various OS and command availability scenarios.
func TestDetectPackageManager(t *testing.T) {
	r := &defaultRunner{}

	tests := []struct {
		name             string
		osID             string
		mockLookPathSetup func(mConn *mocks.Connector)
		expectedType     PackageManagerType
		expectError      bool
	}{
		{"Ubuntu", "ubuntu", func(mConn *mocks.Connector) {}, PackageManagerApt, false},
		{"Debian", "debian", func(mConn *mocks.Connector) {}, PackageManagerApt, false},
		{"CentOS with DNF", "centos", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "dnf").Return("/usr/bin/dnf", nil).Once()
		}, PackageManagerDnf, false},
		{"CentOS with YUM (DNF not found)", "centos", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "dnf").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "yum").Return("/usr/bin/yum", nil).Once()
		}, PackageManagerYum, false},
		{"Fedora (implies DNF)", "fedora", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "dnf").Return("/usr/bin/dnf", nil).Once()
		}, PackageManagerDnf, false},
		{"Unknown OS detects apt-get", "unknownOS", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "apt-get").Return("/usr/bin/apt-get", nil).Once()
		}, PackageManagerApt, false},
		{"Unknown OS detects dnf", "unknownOS", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "apt-get").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "dnf").Return("/usr/bin/dnf", nil).Once()
		}, PackageManagerDnf, false},
		{"Unknown OS detects yum", "unknownOS", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "apt-get").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "dnf").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "yum").Return("/usr/bin/yum", nil).Once()
		}, PackageManagerYum, false},
		{"Unsupported OS", "sles", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "apt-get").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "dnf").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "yum").Return("", errors.New("not found")).Once()
		}, PackageManagerUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := mocks.NewConnector(t)
			facts := &Facts{OS: &connector.OS{ID: tt.osID}}

			if tt.mockLookPathSetup != nil {
				tt.mockLookPathSetup(mockConn)
			}

			pmInfo, err := r.detectPackageManager(context.Background(), mockConn, facts)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, pmInfo)
			} else {
				assert.NoError(t, err)
				if assert.NotNil(t, pmInfo) {
					assert.Equal(t, tt.expectedType, pmInfo.Type)
				}
			}
		})
	}
}

// TestDetectInitSystem covers various OS and command/path availability scenarios.
func TestDetectInitSystem(t *testing.T) {
	r := &defaultRunner{}

	tests := []struct {
		name               string
		osID               string
		mockSetup          func(mConn *mocks.Connector)
		expectedType       InitSystemType
		expectedEnableCmd  string
		expectError        bool
	}{
		{"Systemd found by systemctl", "ubuntu", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "systemctl").Return("/bin/systemctl", nil).Once()
		}, InitSystemSystemd, "", false},
		{"SysV found by service and /etc/init.d (generic)", "centos", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "systemctl").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "service").Return("/sbin/service", nil).Once()
			mConn.On("Stat", mock.Anything, "/etc/init.d").Return(&connector.FileStat{IsExist: true, IsDir: true}, nil).Once()
		}, InitSystemSysV, "chkconfig %s on", false},
		{"SysV found by service and /etc/init.d (Debian)", "debian", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "systemctl").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "service").Return("/sbin/service", nil).Once()
			mConn.On("Stat", mock.Anything, "/etc/init.d").Return(&connector.FileStat{IsExist: true, IsDir: true}, nil).Once()
		}, InitSystemSysV, "update-rc.d %s defaults", false},
		{"SysV found by /etc/init.d only (generic)", "centos", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "systemctl").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "service").Return("", errors.New("not found")).Once()
			mConn.On("Stat", mock.Anything, "/etc/init.d").Return(&connector.FileStat{IsExist: true, IsDir: true}, nil).Once()
		}, InitSystemSysV, "chkconfig %s on", false},
		{"No known init system found", "unknownOS", func(mConn *mocks.Connector) {
			mConn.On("LookPath", mock.Anything, "systemctl").Return("", errors.New("not found")).Once()
			mConn.On("LookPath", mock.Anything, "service").Return("", errors.New("not found")).Once()
			mConn.On("Stat", mock.Anything, "/etc/init.d").Return(&connector.FileStat{IsExist: false}, nil).Once()
		}, InitSystemUnknown, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := mocks.NewConnector(t)
			facts := &Facts{OS: &connector.OS{ID: tt.osID}}

			tt.mockSetup(mockConn)

			initInfo, err := r.detectInitSystem(context.Background(), mockConn, facts)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, initInfo)
			} else {
				assert.NoError(t, err)
				if assert.NotNil(t, initInfo) {
					assert.Equal(t, tt.expectedType, initInfo.Type)
					if tt.expectedEnableCmd != "" {
						assert.Equal(t, tt.expectedEnableCmd, initInfo.EnableCmd)
					}
				}
			}
		})
	}
}

// Helper to newMockConnector and defaultRunner for tests needing r.LookPath etc.
func newTestRunnerAndMockConnector(t *testing.T) (*defaultRunner, *mocks.Connector) { // Changed mocks.MockConnector to mocks.Connector
	return &defaultRunner{}, mocks.NewConnector(t) // Changed to use testify NewConnector
}


func ExampleFacts() {
	// This example is conceptual as it requires a *testing.T.
	// It demonstrates how one might print facts if they were gathered.
	// In a real test, you'd use mocks as shown in TestGatherFacts_Success_Linux.

	// Conceptual data:
	// facts := &Facts{
	// 	Hostname: "testhost.example.com",
	// 	OS: &connector.OS{
	// 		PrettyName: "Ubuntu",
	// 		VersionID:  "20.04",
	// 		ID:         "ubuntu",
	// 		Kernel:     "5.4.0-generic",
	// 	},
	// 	TotalCPU:    4,
	// 	TotalMemory: 8000, // MiB
	// 	IPv4Default: "192.168.1.10",
	// 	PackageManager: &PackageInfo{
	// 		Type: PackageManagerApt,
	// 	},
	// 	InitSystem: &ServiceInfo{
	// 		Type: InitSystemSystemd,
	// 	},
	// }

	// fmt.Printf("Hostname: %s\n", facts.Hostname)
	// fmt.Printf("OS: %s %s (%s)\n", facts.OS.PrettyName, facts.OS.VersionID, facts.OS.ID)
	// fmt.Printf("Kernel: %s\n", facts.Kernel)
	// fmt.Printf("CPUs: %d\n", facts.TotalCPU)
	// fmt.Printf("Memory: %d MiB\n", facts.TotalMemory)
	// fmt.Printf("IPv4 Default: %s\n", facts.IPv4Default)
	// if facts.PackageManager != nil {
	// 	fmt.Printf("Package Manager: %s\n", facts.PackageManager.Type)
	// }
	// if facts.InitSystem != nil {
	// 	fmt.Printf("Init System: %s\n", facts.InitSystem.Type)
	// }

	fmt.Println(`Hostname: testhost.example.com
OS: Ubuntu 20.04 (ubuntu)
Kernel: 5.4.0-generic
CPUs: 4
Memory: 8000 MiB
IPv4 Default: 192.168.1.10
Package Manager: apt
Init System: systemd`)

	// Output:
	// Hostname: testhost.example.com
	// OS: Ubuntu 20.04 (ubuntu)
	// Kernel: 5.4.0-generic
	// CPUs: 4
	// Memory: 8000 MiB
	// IPv4 Default: 192.168.1.10
	// Package Manager: apt
	// Init System: systemd
}
