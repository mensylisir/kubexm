package runner

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path from go.mod
	"golang.org/x/sync/errgroup"
)

// Facts contains read-only information about a host, gathered upon Runner initialization.
// This forms the basis for many of the Runner's "intelligent" decisions.
type Facts struct {
	OS          *connector.OS
	Hostname    string
	Kernel      string
	TotalMemory uint64 // in MiB
	TotalCPU    int    // Number of logical CPU cores
	IPv4Default string // Default outbound IPv4 address
	IPv6Default string // Default outbound IPv6 address
}

// Runner is a feature-rich operations executor.
// Each Runner instance is bound to a specific host.
type Runner struct {
	Conn  connector.Connector
	Facts *Facts // Cached host facts to avoid repeated probing

	// Internal lock to protect potentially concurrent-unsafe operations if any are added later.
	// For now, most operations are on distinct runner instances or are read-only on shared facts.
	mu sync.Mutex
}

// NewRunner is the factory function for Runner.
// Upon creation, it immediately gathers basic host information (Facts)
// concurrently via the Connector. This process itself serves as a deep
// validation of the connection's effectiveness.
func NewRunner(ctx context.Context, conn connector.Connector) (*Runner, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}
	if !conn.IsConnected() {
		// Attempt to connect if a config was previously stored, or return error.
		// This part depends on how ConnectionCfg is managed if not connected.
		// For now, assume Connect was called on the connector instance before NewRunner.
		return nil, fmt.Errorf("connector is not connected")
	}

	facts := &Facts{}
	var getOSError error // To capture errors from GetOS if it's the first one in errgroup

	// Use errgroup for concurrent fact gathering
	g, gCtx := errgroup.WithContext(ctx)

	// Get OS information
	g.Go(func() error {
		var err error
		facts.OS, err = conn.GetOS(gCtx)
		if err != nil {
			getOSError = fmt.Errorf("failed to get OS info: %w", err)
			return getOSError
		}
		return nil
	})

	// Get hostname and kernel version
	g.Go(func() error {
		// Wait for OS info to be available if needed for OS-specific commands, though not strictly for these two.
		// However, if GetOS fails, we might want to stop early. The errgroup context cancellation handles this.
		if gCtx.Err() != nil { // Check if context was cancelled (e.g., by GetOS failure)
			return gCtx.Err()
		}
		hostnameBytes, _, execErr := conn.Exec(gCtx, "hostname", nil)
		if execErr != nil {
			return fmt.Errorf("failed to get hostname: %w", execErr)
		}
		facts.Hostname = strings.TrimSpace(string(hostnameBytes))

		kernelBytes, _, execErr := conn.Exec(gCtx, "uname -r", nil)
		if execErr != nil {
			return fmt.Errorf("failed to get kernel version: %w", execErr)
		}
		facts.Kernel = strings.TrimSpace(string(kernelBytes))
		return nil
	})

	// Get CPU and Memory
	g.Go(func() error {
		if gCtx.Err() != nil {
			return gCtx.Err()
		}
		// Ensure facts.OS is populated before attempting OS-specific commands
		// This requires GetOS to complete first, or careful handling if it could be nil.
		// A simple way is to ensure GetOS has run, or check facts.OS within this goroutine.
		// For now, assuming GetOS might still be running, so check facts.OS directly.

		currentOS := facts.OS // Read once, in case it's being written by another goroutine

		// nproc might not be available on all systems (e.g. macOS, some minimal Linux)
		cpuCmd := "nproc"
		if currentOS != nil && currentOS.ID == "darwin" {
			cpuCmd = "sysctl -n hw.ncpu"
		}

		cpuBytes, _, execErr := conn.Exec(gCtx, cpuCmd, nil)
		if execErr == nil {
			facts.TotalCPU, _ = strconv.Atoi(strings.TrimSpace(string(cpuBytes)))
		} else {
			// Fallback or log error for CPU count
			facts.TotalCPU = 0 // Default or mark as unknown
		}


		memCmd := "grep MemTotal /proc/meminfo | awk '{print $2}'" // KB
		memIsBytes := false
		if currentOS != nil && currentOS.ID == "darwin" {
			memCmd = "sysctl -n hw.memsize" // Bytes
			memIsBytes = true
		}

		memBytes, _, execErr := conn.Exec(gCtx, memCmd, nil)
		if execErr == nil {
			memVal, _ := strconv.ParseUint(strings.TrimSpace(string(memBytes)), 10, 64)
			if memIsBytes {
				facts.TotalMemory = memVal / (1024 * 1024) // Convert bytes to MiB
			} else {
				facts.TotalMemory = memVal / 1024 // Convert KB to MiB
			}
		} else {
			facts.TotalMemory = 0 // Default or mark as unknown
		}
		return nil
	})

	// Get default IP addresses
	g.Go(func() error {
		if gCtx.Err() != nil {
			return gCtx.Err()
		}
		currentOS := facts.OS // Read once

		// These commands are Linux-specific. Need alternatives for other OSes (e.g. route on macOS)
		ip4Cmd := "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1"
		ip6Cmd := "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1"

		if currentOS != nil && currentOS.ID == "darwin" {
			// Simplified example for macOS, might need more robust solution
			// For IPv4: ipconfig getifaddr $(route -n get default | grep 'interface:' | awk '{print $2}')
			// For IPv6: ipconfig getifaddr $(route -n get default -inet6 | grep 'interface:' | awk '{print $2}')
			// These are complex to run directly, often requiring multiple commands or more specific parsing.
			// For this example, we'll placeholder or use a command that might fail gracefully.
			ip4Cmd = "ifconfig $(route -n get default | grep 'interface:' | awk '{print $2}') | grep 'inet ' | awk '{print $2}' | head -n1"
			ip6Cmd = "ifconfig $(route -n get default -inet6 | grep 'interface:' | awk '{print $2}') | grep 'inet6 ' | awk '{print $2}' | head -n1 | cut -d'%' -f1"
		}

		ip4Bytes, _, _ := conn.Exec(gCtx, ip4Cmd, nil) // Errors are ignored for IP as they might not exist
		facts.IPv4Default = strings.TrimSpace(string(ip4Bytes))

		ip6Bytes, _, _ := conn.Exec(gCtx, ip6Cmd, nil) // Errors ignored
		facts.IPv6Default = strings.TrimSpace(string(ip6Bytes))
		return nil
	})

	if err := g.Wait(); err != nil {
		// If getOSError is set, it means GetOS failed, which is critical.
		if getOSError != nil {
			return nil, getOSError // Return the specific GetOS error
		}
		// Otherwise, it's an error from another fact-gathering step.
		return nil, fmt.Errorf("failed to gather some host facts: %w", err)
	}

	// After g.Wait(), if getOSError was not nil, we would have returned already.
	// If facts.OS is still nil here, it means GetOS completed without error but returned nil OS.
	// This should ideally be caught by an error return from GetOS itself if it's an invalid state.
	if facts.OS == nil {
	    return nil, fmt.Errorf("critical failure: OS information could not be retrieved (GetOS returned nil OS without error)")
	}

	return &Runner{
		Conn:  conn,
		Facts: facts,
	}, nil
}
