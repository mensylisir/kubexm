package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kubexms/kubexms/pkg/logger"
)

// BaseRuntime manages the collection of Host objects and generic runtime properties.
type BaseRuntime struct {
	objName         string
	hosts           []*Host // Slice for ordered access if needed, and primary store
	inventoryByName map[string]*Host // Map for quick lookup by name
	inventoryByRole map[string][]*Host // Map for quick lookup by role

	workDir   string
	verbose   bool
	ignoreErr bool
	logger    *logger.Logger

	mu sync.RWMutex // Protects hosts, inventoryByName, inventoryByRole
}

// NewBaseRuntime creates a new BaseRuntime.
// name: Name for this runtime instance (e.g., cluster name), used in logging.
// workDir: Global working directory. If empty, a default is created.
// verbose: Enable verbose logging.
// ignoreErr: If true, some non-critical errors might be ignored during operations.
// baseLogger: A base logger instance. If nil, a global default is used.
func NewBaseRuntime(name string, workDir string, verbose bool, ignoreErr bool, baseLogger *logger.Logger) (*BaseRuntime, error) {
	if baseLogger == nil {
		baseLogger = logger.Get() // Fallback to global default logger
		baseLogger.Warnf("NewBaseRuntime for '%s' called with nil baseLogger, using global default logger instance.", name)
	}
	// Contextualize the logger for this BaseRuntime instance
	brLogger := &logger.Logger{SugaredLogger: baseLogger.SugaredLogger.With("runtime_obj", name, "component", "BaseRuntime")}

	if workDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback if home directory cannot be determined, log warning and use temp.
			fallbackDir := filepath.Join(os.TempDir(), "kubexms", "workspaces", name)
			brLogger.Warnf("Failed to get user home directory (for default workDir): %v. Using temporary fallback: %s", err, fallbackDir)
			workDir = fallbackDir
		} else {
			workDir = filepath.Join(homeDir, ".kubexms", "workspaces", name)
		}
		brLogger.Infof("Work directory not specified, defaulting to: %s", workDir)
	}

	if err := os.MkdirAll(workDir, 0o750); err != nil { // Corrected octal literal
		brLogger.Errorf("Failed to create BaseRuntime workDir '%s': %v", workDir, err)
		return nil, fmt.Errorf("failed to create BaseRuntime workDir '%s': %w", workDir, err)
	}
	brLogger.Debugf("BaseRuntime workDir is: %s", workDir)

	return &BaseRuntime{
		objName:         name,
		hosts:           make([]*Host, 0),
		inventoryByName: make(map[string]*Host),
		inventoryByRole: make(map[string][]*Host),
		workDir:         workDir,
		verbose:         verbose,
		ignoreErr:       ignoreErr,
		logger:          brLogger,
	}, nil
}

// AddHost adds a host to the runtime and updates all internal inventories.
// It returns an error if the host is nil, has an empty name, or if a host
// with the same name already exists.
func (br *BaseRuntime) AddHost(host *Host) error {
	br.mu.Lock()
	defer br.mu.Unlock()

	if host == nil {
		return fmt.Errorf("cannot add a nil host")
	}
	if host.Name == "" {
		return fmt.Errorf("cannot add a host with an empty name")
	}

	if _, exists := br.inventoryByName[host.Name]; exists {
		br.logger.Warnf("Attempted to add host '%s' which already exists in BaseRuntime.", host.Name)
		return fmt.Errorf("host with name '%s' already exists in BaseRuntime", host.Name)
	}

	br.hosts = append(br.hosts, host)
	br.inventoryByName[host.Name] = host
	for roleName := range host.Roles {
		br.inventoryByRole[roleName] = append(br.inventoryByRole[roleName], host)
	}
	br.logger.Debugf("Host '%s' added to BaseRuntime. Total hosts: %d. Roles: %v", host.Name, len(br.hosts), host.Roles)
	return nil
}

// RemoveHost removes a host from the runtime and updates all internal inventories.
// Returns an error if the hostName is empty or if the host is not found.
func (br *BaseRuntime) RemoveHost(hostName string) error {
	br.mu.Lock()
	defer br.mu.Unlock()

	if hostName == "" {
		return fmt.Errorf("cannot remove a host with an empty name")
	}

	hostToRemove, exists := br.inventoryByName[hostName]
	if !exists {
		br.logger.Warnf("Attempted to remove non-existent host '%s'.", hostName)
		return fmt.Errorf("host with name '%s' not found in BaseRuntime for removal", hostName)
	}

	// 1. Remove from hosts slice
	newHostsSlice := make([]*Host, 0, len(br.hosts)-1)
	for _, h := range br.hosts {
		if h.Name != hostName {
			newHostsSlice = append(newHostsSlice, h)
		}
	}
	br.hosts = newHostsSlice

	// 2. Remove from inventoryByName map
	delete(br.inventoryByName, hostName)

	// 3. Remove from inventoryByRole map
	for roleName := range hostToRemove.Roles { // Iterate only over the roles of the host being removed
		if hostsInSpecificRole, ok := br.inventoryByRole[roleName]; ok {
			updatedHostsInRole := make([]*Host, 0, len(hostsInSpecificRole)-1)
			for _, h := range hostsInSpecificRole {
				if h.Name != hostName {
					updatedHostsInRole = append(updatedHostsInRole, h)
				}
			}
			if len(updatedHostsInRole) == 0 {
				delete(br.inventoryByRole, roleName)
			} else {
				br.inventoryByRole[roleName] = updatedHostsInRole
			}
		}
	}
	br.logger.Debugf("Host '%s' removed from BaseRuntime. Total hosts: %d", hostName, len(br.hosts))
	return nil
}

// GetHost retrieves a host by its name. Returns nil if not found.
func (br *BaseRuntime) GetHost(name string) *Host {
	br.mu.RLock()
	defer br.mu.RUnlock()
	return br.inventoryByName[name]
}

// GetAllHosts returns a new slice containing all hosts.
// Modifications to the returned slice will not affect the internal state.
func (br *BaseRuntime) GetAllHosts() []*Host {
	br.mu.RLock()
	defer br.mu.RUnlock()
	hostsCopy := make([]*Host, len(br.hosts))
	copy(hostsCopy, br.hosts)
	return hostsCopy
}

// GetHostsByRole retrieves hosts that have the specified role.
// Returns an empty slice if the role is not found or has no hosts.
// Modifications to the returned slice will not affect the internal state.
func (br *BaseRuntime) GetHostsByRole(roleName string) []*Host {
	br.mu.RLock()
	defer br.mu.RUnlock()
	hosts, found := br.inventoryByRole[roleName]
	if !found {
		return []*Host{}
	}
	hostsCopy := make([]*Host, len(hosts))
	copy(hostsCopy, hosts)
	return hostsCopy
}

// ObjName returns the name of this runtime instance (e.g., cluster name).
func (br *BaseRuntime) ObjName() string {
	return br.objName
}

// GetWorkDir returns the global work directory for the runtime.
func (br *BaseRuntime) GetWorkDir() string {
	return br.workDir
}

// GetHostWorkDir returns the path to a specific host's work directory.
// This is typically a subdirectory under the global workDir (e.g., <globalWorkDir>/hosts/<hostName>).
// This method does not create the directory; it only constructs the path.
func (br *BaseRuntime) GetHostWorkDir(hostName string) string {
	if hostName == "" {
		br.logger.Warnf("GetHostWorkDir called with empty hostName.")
		return filepath.Join(br.workDir, "hosts", "_unknown_host_")
	}
	return filepath.Join(br.workDir, "hosts", hostName)
}

// IsVerbose returns true if verbose logging is enabled.
func (br *BaseRuntime) IsVerbose() bool {
	return br.verbose
}

// ShouldIgnoreErr returns true if errors should be ignored during some operations.
func (br *BaseRuntime) ShouldIgnoreErr() bool {
	return br.ignoreErr
}

// Logger returns the contextualized logger associated with this BaseRuntime.
func (br *BaseRuntime) Logger() *logger.Logger {
	return br.logger
}

// Copy creates a new BaseRuntime instance that is a shallow copy of the original.
// The host objects themselves are pointers to the same underlying Host structs,
// but the collections (slice and maps) holding these hosts are new.
// The logger is shared. WorkDir, verbose, ignoreErr, and objName are copied by value.
func (br *BaseRuntime) Copy() *BaseRuntime {
	br.mu.RLock()
	defer br.mu.RUnlock()

	newBr := &BaseRuntime{
		objName:         br.objName,
		workDir:         br.workDir,
		verbose:         br.verbose,
		ignoreErr:       br.ignoreErr,
		logger:          br.logger,
		hosts:           make([]*Host, len(br.hosts)),
		inventoryByName: make(map[string]*Host, len(br.inventoryByName)),
		inventoryByRole: make(map[string][]*Host, len(br.inventoryByRole)),
	}

	copy(newBr.hosts, br.hosts)

	for name, host := range br.inventoryByName {
		newBr.inventoryByName[name] = host
	}

	for role, hostsInRole := range br.inventoryByRole {
		newHostsInRole := make([]*Host, len(hostsInRole))
		copy(newHostsInRole, hostsInRole)
		newBr.inventoryByRole[role] = newHostsInRole
	}

	br.logger.Debugf("Created a copy of BaseRuntime for '%s'.", br.objName)
	return newBr
}
