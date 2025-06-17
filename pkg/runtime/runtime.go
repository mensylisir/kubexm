package runtime

import (
	"context"
	"sync"
	"time"

	// Assuming these packages will exist at these paths based on previous work
	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/config"
)

// Allows runner.NewRunner to be replaced for testing.
var runnerNewRunner = runner.NewRunner

// Host represents a single node in the cluster inventory. It holds all
// necessary information to connect to and operate on the host, including
// its specific Connector and Runner instances.
type Host struct {
	Name            string            // Unique host identifier, e.g., "master1", "worker-node-01"
	Address         string            // Connection address (IP or FQDN) used by the Connector
	InternalAddress string            // Address used for communication within the cluster (e.g., private IP)
	Port            int               // Port for SSH or other connection methods
	User            string            // Username for SSH/connection

	// Password and PrivateKey fields are used for establishing the connection.
	// They are passed to the Connector during its initialization.
	// For security, they should ideally not be stored in the Host struct long-term
	// after the connection is established, unless re-connection logic requires them.
	// The Connector instance itself will manage the secure connection details.
	// However, including them here aligns with config.HostSpec for easier mapping.
	Password       string
	PrivateKey     []byte // Raw private key content
	PrivateKeyPath string // Filesystem path to the private key

	Roles           map[string]bool   // Roles assigned to this host, e.g., {"etcd": true, "master": true}
	Labels          map[string]string // Custom labels for flexible grouping and selection

	Connector connector.Connector // The underlying connector instance for this host (e.g., SSHConnector, LocalConnector)
	Runner    *runner.Runner    // The runner instance providing operational functions for this host

	// WorkDir specifies a default working directory on this host for tasks.
	// This can be overridden by specific task configurations.
	WorkDir string

	// TODO: Consider adding a field for host-specific connection status or last error,
	// if needed for runtime diagnostics outside of what Connector.IsConnected() provides.
	// For now, relies on Connector.IsConnected().
}

// String returns the name of the host.
func (h *Host) String() string {
	return h.Name
}

// HasRole checks if the host has a specific role.
func (h *Host) HasRole(roleName string) bool {
	if h.Roles == nil {
		return false
	}
	_, exists := h.Roles[roleName]
	return exists
}

// GetLabel returns the value of a label and whether it exists.
func (h *Host) GetLabel(labelName string) (string, bool) {
	if h.Labels == nil {
		return "", false
	}
	val, exists := h.Labels[labelName]
	return val, exists
}

// ClusterRuntime holds all global, read-only information and resources
// required for a KubeXMS operation (e.g., cluster creation, scaling).
// It includes the parsed cluster configuration, the inventory of all hosts
// (with their initialized connectors and runners), and shared utilities like the logger.
type ClusterRuntime struct {
	// ClusterConfig is the parsed and validated user-provided configuration for the cluster.
	// The exact type `*config.Cluster` is assumed from the design documents.
	ClusterConfig *config.Cluster

	// Hosts is an ordered slice of all host objects in the runtime.
	// The order is typically derived from the user's configuration file.
	Hosts []*Host
	// Inventory provides quick lookup of a host by its unique name.
	Inventory map[string]*Host
	// RoleInventory maps role names to a slice of hosts that have that role.
	// This allows for easy selection of hosts based on their roles (e.g., all "etcd" nodes).
	RoleInventory map[string][]*Host

	// Logger is the shared logger instance for all runtime operations.
	// It can be pre-configured with global context fields.
	Logger *logger.Logger

	// GlobalTimeout specifies a default timeout for operations like initial host connections
	// during runtime setup. This can be overridden by more specific timeouts at lower levels.
	GlobalTimeout time.Duration

	// WorkDir is a global default working directory that can be used by hosts
	// if they don't have a specific WorkDir set. Useful for temporary files or scripts.
	WorkDir string

	// Verbose indicates whether verbose output is enabled for operations.
	// This can influence logging levels or the amount of detail shown by runners/steps.
	Verbose bool

	// IgnoreErr, if true, suggests that some non-critical errors during operations
	// (particularly in steps or tasks) might be ignored, allowing a pipeline to continue.
	// The actual error ignoring logic resides in the higher-level execution engine (e.g., Task, Module).
	IgnoreErr bool

	// TODO: Consider adding a field for overall cluster state if the runtime needs to track it.
	// For now, it's primarily a container for config and initialized host objects.
}

// GetHost retrieves a host by its name from the inventory.
// Returns nil if the host is not found.
func (cr *ClusterRuntime) GetHost(name string) *Host {
	if cr.Inventory == nil {
		return nil
	}
	return cr.Inventory[name]
}

// GetHostsByRole retrieves all hosts that have the specified role.
// Returns an empty slice if the role is not found or no hosts have that role.
func (cr *ClusterRuntime) GetHostsByRole(roleName string) []*Host {
	if cr.RoleInventory == nil {
		return []*Host{}
	}
	hosts, found := cr.RoleInventory[roleName]
	if !found {
		return []*Host{}
	}
	return hosts
}


// Context is passed to each execution unit (e.g., a Step in a Task).
// It carries all necessary information and tools for that unit to perform its operation.
type Context struct {
	// GoContext is the standard Go context, used for managing deadlines,
	// cancellation signals, and other request-scoped values across API boundaries
	// and between processes.
	GoContext context.Context

	// Host is the specific host on which the current operation is being performed.
	Host *Host

	// Cluster provides read-only access to the global ClusterRuntime,
	// allowing steps to query information about other hosts or global configurations
	// if necessary (though direct inter-host operations within a single step are discouraged;
	// such logic usually belongs in higher-level orchestrators like Modules or Pipelines).
	Cluster *ClusterRuntime

	// Logger is a logger instance that can be pre-configured with contextual information,
	// such as the current host name and the name of the step being executed.
	// This allows for structured and easily traceable logging.
	Logger *logger.Logger // In 3.md, this was SugaredLogger. Our logger.Logger wraps SugaredLogger.

	// SharedData is a concurrency-safe map that can be used by different Steps
	// operating on the *same host* within the *same task execution* to share dynamic data.
	// For example, one step might generate a certificate path and store it here,
	// and a subsequent step on the same host can retrieve it.
	// Data shared across different hosts or different task executions should be managed
	// through more persistent or explicit means.
	SharedData *sync.Map
}


// NewRuntime is the factory function for ClusterRuntime.
// It takes the parsed cluster configuration and a logger, then initializes
// all hosts, including setting up their connectors and runners.
// Host initializations are performed concurrently.
func NewRuntime(cfg *config.Cluster, baseLogger *logger.Logger) (*ClusterRuntime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cluster configuration cannot be nil")
	}
	if baseLogger == nil {
		// Fallback to a default logger if none is provided, though it's better if one is always passed in.
		baseLogger = logger.Get() // Gets the global logger, initializes with defaults if needed.
		baseLogger.Warnf("NewRuntime called with nil baseLogger, using global default logger.")
	}

	rtLogger := baseLogger.SugaredLogger.With("component", "runtime").Sugar() // Create a child logger for runtime

	// Initialize ClusterRuntime fields from global config
	cr := &ClusterRuntime{
		ClusterConfig: cfg,
		Hosts:         make([]*Host, 0, len(cfg.Spec.Hosts)),
		Inventory:     make(map[string]*Host, len(cfg.Spec.Hosts)),
		RoleInventory: make(map[string][]*Host),
		Logger:        baseLogger, // Store the original baseLogger
		GlobalTimeout: cfg.Spec.Global.ConnectionTimeout,
		WorkDir:       cfg.Spec.Global.WorkDir,
		Verbose:       cfg.Spec.Global.Verbose,
		IgnoreErr:     cfg.Spec.Global.IgnoreErr,
	}

	if cr.GlobalTimeout <= 0 {
		cr.GlobalTimeout = 30 * time.Second // Default connection timeout
		rtLogger.Debugf("Global connection timeout not specified, defaulting to %s", cr.GlobalTimeout)
	}

	// Use an errgroup for concurrent initialization of hosts
	g, gCtx := errgroup.WithContext(context.Background())
	// Could use context.WithTimeout(context.Background(), cr.GlobalTimeout) for overall init timeout for all hosts
	// but individual connector configs also have timeouts. Let's use a per-host timeout based on GlobalTimeout.

	// Mutex to protect shared slices/maps during concurrent appends
	// var mu sync.Mutex // Not strictly needed if writing to fixed indices or using thread-safe appends for RoleInventory
	initErrs := &InitializationError{}

	// Temporary slice to hold successfully initialized hosts to maintain order
	// The number of hosts is known. We can use an array and fill it, then convert to slice.
	initializedHosts := make([]*Host, len(cfg.Spec.Hosts))


	for i, hostCfg := range cfg.Spec.Hosts {
		// Capture loop variables for the goroutine
		currentIndex := i
		currentHostCfg := hostCfg

		g.Go(func() error {
			// Create a logger instance specific to this host initialization goroutine
			// This uses the original baseLogger, so it doesn't inherit the "component: runtime" field.
			// If host-specific logging from the start is desired, NewLogger could be used,
			// or pass rtLogger and add host field. For now, using baseLogger for simplicity.
			hostLogger := baseLogger.SugaredLogger.With("host", currentHostCfg.Name).Sugar()
			hostLogger.Debugf("Initializing host...")

			connCfg := connector.ConnectionCfg{
				Host:           currentHostCfg.Address,
				Port:           currentHostCfg.Port,
				User:           currentHostCfg.User,
				Password:       currentHostCfg.Password,
				PrivateKeyPath: currentHostCfg.PrivateKeyPath,
				Timeout:        cr.GlobalTimeout, // Use global timeout for individual connection attempt
				// Bastion support would be configured here if HostSpec had Bastion details
			}

			// Load private key content if path is given and content not already set
			if len(currentHostCfg.PrivateKey) > 0 {
			    connCfg.PrivateKey = currentHostCfg.PrivateKey
			} else if currentHostCfg.PrivateKeyPath != "" {
				keyBytes, err := os.ReadFile(currentHostCfg.PrivateKeyPath)
				if err != nil {
					err = fmt.Errorf("host %s: failed to read private key %s: %w", currentHostCfg.Name, currentHostCfg.PrivateKeyPath, err)
					initErrs.Add(err) // Thread-safe Add
					hostLogger.Errorf("Initialization failed: %v", err)
					return err // errgroup will handle this error
				}
				connCfg.PrivateKey = keyBytes
			}


			var conn connector.Connector
			hostType := strings.ToLower(strings.TrimSpace(currentHostCfg.Type))
			if hostType == "local" {
				conn = &connector.LocalConnector{}
			} else { // Default to SSH if type is empty or "ssh"
				conn = &connector.SSHConnector{}
			}

			// Connect with a context that respects the host's connection timeout
			connectCtx, connectCancel := context.WithTimeout(gCtx, connCfg.Timeout)
			defer connectCancel()

			if err := conn.Connect(connectCtx, connCfg); err != nil {
				err = fmt.Errorf("host %s: connection failed: %w", currentHostCfg.Name, err)
				initErrs.Add(err)
				hostLogger.Errorf("Initialization failed: %v", err)
				return err
			}
			hostLogger.Debugf("Connection established.")

			// Create Runner for the host
			// NewRunner also performs initial fact gathering, which might take time.
			// Use a context for NewRunner as well.
			runnerCtx, runnerCancel := context.WithTimeout(gCtx, cr.GlobalTimeout) // Reuse global timeout for runner init
			defer runnerCancel()

			newRunner, err := runnerNewRunner(runnerCtx, conn) // Use the package variable
			if err != nil {
				conn.Close() // Attempt to clean up connection
				err = fmt.Errorf("host %s: runner initialization failed: %w", currentHostCfg.Name, err)
				initErrs.Add(err)
				hostLogger.Errorf("Initialization failed: %v", err)
				return err
			}
			hostLogger.Debugf("Runner initialized. OS: %s, Hostname: %s", newRunner.Facts.OS.ID, newRunner.Facts.Hostname)

			hostWorkDir := currentHostCfg.WorkDir
			if hostWorkDir == "" {
				hostWorkDir = cr.WorkDir // Use global workdir if host-specific one is not set
			}


			newHost := &Host{
				Name:            currentHostCfg.Name,
				Address:         currentHostCfg.Address,
				InternalAddress: currentHostCfg.InternalAddress,
				Port:            currentHostCfg.Port,
				User:            currentHostCfg.User,
				Password:        currentHostCfg.Password,
				PrivateKeyPath:  currentHostCfg.PrivateKeyPath,
				PrivateKey:      currentHostCfg.PrivateKey,
				Roles:           make(map[string]bool),
				Labels:          currentHostCfg.Labels,
				Connector:       conn,
				Runner:          newRunner,
				WorkDir:         hostWorkDir,
			}
			for _, role := range currentHostCfg.Roles {
				if strings.TrimSpace(role) != "" {
					newHost.Roles[strings.TrimSpace(role)] = true
				}
			}

			initializedHosts[currentIndex] = newHost
			hostLogger.Infof("Successfully initialized.")
			return nil
		})
	}

	// Wait for all host initializations to complete
	if err := g.Wait(); err != nil {
		// err here is the first non-nil error returned by a goroutine.
		// initErrs will contain all errors.
		rtLogger.Errorf("NewRuntime completed with errors: %v", initErrs.Error())
		return nil, initErrs
	}

	// Populate Hosts, Inventory, and RoleInventory from the ordered initializedHosts
	// This ensures cr.Hosts maintains the original order from the config.
	// This part needs to be thread-safe if goroutines were directly appending to cr.Hosts/Inventory/RoleInventory.
	// Since we are iterating over initializedHosts *after* g.Wait(), this part is sequential.
	for _, h := range initializedHosts {
		if h != nil { // Goroutines that errored out might leave nil entries if not handled by errgroup correctly
			cr.Hosts = append(cr.Hosts, h)
			cr.Inventory[h.Name] = h
			for role := range h.Roles {
				cr.RoleInventory[role] = append(cr.RoleInventory[role], h)
			}
		}
	}

	if !initErrs.IsEmpty() && g.Wait() == nil {
	    rtLogger.Warnf("Runtime initialization had non-fatal errors recorded but errgroup reported success: %v", initErrs.Error())
	}


	rtLogger.Infof("ClusterRuntime initialized successfully with %d hosts.", len(cr.Hosts))
	return cr, nil
}


// NewHostContext creates a new Context specific to a given host and operation.
// It populates the Context with the necessary Go context, host information,
// a reference to the global cluster runtime, a host-specific logger, and
// a new sync.Map for shared data within the scope of this host's operations.
func NewHostContext(goCtx context.Context, host *Host, cluster *ClusterRuntime) *Context {
	if goCtx == nil {
		goCtx = context.Background() // Ensure GoContext is never nil
	}
	if host == nil {
		// This should ideally not happen if called correctly.
		// Log an error or panic, as a nil host in HostContext is problematic.
		if cluster != nil && cluster.Logger != nil {
			cluster.Logger.Errorf("NewHostContext called with nil host for cluster (This is unexpected)")
		} else {
			// If cluster or its logger is also nil, use global logger for this critical warning
			logger.Get().Errorf("NewHostContext called with nil host and nil cluster/logger (This is unexpected)")
		}
		// Depending on strictness, could panic or return a context that indicates this issue.
		// For now, proceed but log it. A nil host will likely cause issues downstream.
	}

	var hostSpecificLogger *logger.Logger
	if cluster != nil && cluster.Logger != nil {
		// Create a child logger from the ClusterRuntime's logger, adding host-specific fields.
		if host != nil {
			// Assuming logger.Logger wraps a zap.SugaredLogger and we can create a child from it.
			// The logger.Logger itself doesn't have a With method that returns *logger.Logger.
			// We access its SugaredLogger, call With, and then need to re-wrap it if we want
			// our custom methods like Successf, Failf on the hostSpecificLogger.
			// For simplicity, if logger.Logger's opts are needed by the derived logger,
			// a proper With method on logger.Logger would be better.
			// Here, we create a new Logger instance, effectively inheriting opts from the original
			// if we had a way to access them, or using new ones.
			// The most straightforward way with current logger.Logger is to use its SugaredLogger.
			sl := cluster.Logger.SugaredLogger
			if host != nil { // Check again in case we decided to proceed with nil host
				sl = sl.With("host.name", host.Name, "host.address", host.Address)
			}
			hostSpecificLogger = &logger.Logger{SugaredLogger: sl}
			// Note: This hostSpecificLogger will use the same underlying zap.Core (and thus options like
			// timestamp format, console/file output settings) as the cluster.Logger.
			// If truly independent options were needed for this contextual logger, NewLogger would be used.
		} else { // Host is nil, use cluster logger directly
			hostSpecificLogger = cluster.Logger
		}
	} else {
		// Fallback if cluster or cluster.Logger is nil
		hostSpecificLogger = logger.Get() // Get global default
		if host != nil { // Add host context if possible even with global logger
			hostSpecificLogger = &logger.Logger{SugaredLogger: hostSpecificLogger.SugaredLogger.With("host.name", host.Name, "host.address", host.Address)}
		}
		hostSpecificLogger.Warnf("NewHostContext: Cluster or Cluster.Logger was nil, using global logger.")
	}


	return &Context{
		GoContext:  goCtx,
		Host:       host,
		Cluster:    cluster,
		Logger:     hostSpecificLogger,
		SharedData: &sync.Map{},
	}
}
