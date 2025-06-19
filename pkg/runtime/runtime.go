package runtime

import (
	"context"
	"encoding/base64" // For decoding private key if it's base64 encoded
	"fmt"
	"context"
	"encoding/base64" // For decoding private key if it's base64 encoded
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kubexms/kubexms/pkg/cache"
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/config"
)

// osReadFile is a variable that defaults to os.ReadFile, allowing it to be mocked for tests.
var osReadFile = os.ReadFile

// Host represents a single node in the cluster inventory. It holds all
// necessary information to connect to and operate on the host, including
// its specific Connector and Runner instances.
type Host struct {
	Name            string            // Unique host identifier, e.g., "master1", "worker-node-01"
	Address         string            // Connection address (IP or FQDN) used by the Connector
	InternalAddress string            // Address used for communication within the cluster (e.g., private IP)
	Port            int               // Port for SSH or other connection methods
	User            string            // Username for SSH/connection

	Password       string
	PrivateKey     []byte // This field in runtime.Host will store the actual key bytes
	PrivateKeyPath string
	Roles           map[string]bool   // Roles assigned to this host, e.g., {"etcd": true, "master": true}
	Labels          map[string]string // Custom labels for flexible grouping and selection

	Connector connector.Connector
	Runner    *runner.Runner
}

// String returns the name of the host.
func (h *Host) String() string { return h.Name }

// HasRole checks if the host has a specific role.
func (h *Host) HasRole(roleName string) bool { if h.Roles == nil { return false }; _, exists := h.Roles[roleName]; return exists }

// GetLabel returns the value of a label and whether it exists.
func (h *Host) GetLabel(labelName string) (string, bool) { if h.Labels == nil { return "", false }; val, exists := h.Labels[labelName]; return val, exists }

// ClusterRuntime holds all global, read-only information and resources
// required for a KubeXMS operation (e.g., cluster creation, scaling).
type ClusterRuntime struct {
	ClusterConfig *config.Cluster
	Hosts         []*Host
	Inventory     map[string]*Host
	RoleInventory map[string][]*Host
	Logger        *logger.Logger // Base logger for the runtime, can be enriched further
	GlobalTimeout time.Duration
	WorkDir       string // Global default WorkDir from config.GlobalSpec
	Verbose       bool
	IgnoreErr     bool
}

// required for a KubeXMS operation (e.g., cluster creation, scaling).
type ClusterRuntime struct {
	BaseRuntime   *BaseRuntime // Embedded BaseRuntime
	ClusterConfig *config.Cluster
	GlobalTimeout time.Duration
}

// Delegating methods to BaseRuntime
func (cr *ClusterRuntime) GetHost(name string) *Host { return cr.BaseRuntime.GetHost(name) }
func (cr *ClusterRuntime) GetAllHosts() []*Host { return cr.BaseRuntime.GetAllHosts() }
func (cr *ClusterRuntime) GetHostsByRole(roleName string) []*Host { return cr.BaseRuntime.GetHostsByRole(roleName) }
func (cr *ClusterRuntime) Logger() *logger.Logger { return cr.BaseRuntime.Logger() }
func (cr *ClusterRuntime) GetWorkDir() string { return cr.BaseRuntime.GetWorkDir() }
func (cr *ClusterRuntime) GetHostWorkDir(hostName string) string { return cr.BaseRuntime.GetHostWorkDir(hostName) }
func (cr *ClusterRuntime) IsVerbose() bool { return cr.BaseRuntime.IsVerbose() }
func (cr *ClusterRuntime) ShouldIgnoreErr() bool { return cr.BaseRuntime.ShouldIgnoreErr() }
func (cr *ClusterRuntime) AddHost(host *Host) error { return cr.BaseRuntime.AddHost(host) }
func (cr *ClusterRuntime) RemoveHost(hostName string) error { return cr.BaseRuntime.RemoveHost(hostName) }
func (cr *ClusterRuntime) ObjName() string { return cr.BaseRuntime.ObjName() }

// Copy creates a new ClusterRuntime instance that is a shallow copy of the original,
// but with a deep copy of the BaseRuntime's host collections.
func (cr *ClusterRuntime) Copy() *ClusterRuntime {
	if cr == nil {
		return nil
	}
	if cr.BaseRuntime == nil {
		// This indicates an inconsistent state, NewRuntime should prevent this.
		// Log using a package-level logger or print to stderr if no context logger is available.
		// For example: logger.Get().Errorf("Attempted to copy ClusterRuntime with nil BaseRuntime for object: %s", cr.ClusterConfig.Metadata.Name)
		// Depending on policy, could panic or return nil/error. Returning nil for now.
		return nil
	}
	copiedBaseRuntime := cr.BaseRuntime.Copy()
	newCr := &ClusterRuntime{
		BaseRuntime:   copiedBaseRuntime,
		ClusterConfig: cr.ClusterConfig, // Shallow copy of config pointer
		GlobalTimeout: cr.GlobalTimeout,
	}
	newCr.Logger().Debugf("Created a copy of ClusterRuntime for '%s'.", newCr.ObjName())
	return newCr
}

// Context is passed to each execution unit (e.g., a Step in a Task).
type Context struct {
	GoContext     context.Context
	Host          *Host
	Cluster       *ClusterRuntime
	Logger        *logger.Logger // Logger contextualized for Host, Task, Step etc.
	// SharedData is deprecated, will be removed after all steps migrate to scoped caches.
	SharedData    *sync.Map

	pipelineCache cache.PipelineCache
	moduleCache   cache.ModuleCache
	taskCache     cache.TaskCache
	stepCache     cache.StepCache
}

// Accessor methods for caches
func (c *Context) Pipeline() cache.PipelineCache { return c.pipelineCache }
func (c *Context) Module() cache.ModuleCache    { return c.moduleCache }
func (c *Context) Task() cache.TaskCache       { return c.taskCache }
func (c *Context) Step() cache.StepCache          { return c.stepCache }

// runnerNewRunner allows runner.NewRunner to be replaced for testing.
var runnerNewRunner = runner.NewRunner

// NewRuntime is the factory function for ClusterRuntime.
// It takes the parsed and defaulted cluster configuration and a base logger,
// then initializes all hosts, including setting up their connectors and runners.
// Host initializations are performed concurrently.
func NewRuntime(cfg *config.Cluster, baseLogger *logger.Logger) (*ClusterRuntime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cluster configuration cannot be nil")
	}
	if baseLogger == nil {
		baseLogger = logger.Get() // Fallback to global default logger
		baseLogger.Warnf("NewRuntime called with nil baseLogger, using global default logger instance.")
	}

	initPhaseLogger := &logger.Logger{SugaredLogger: baseLogger.SugaredLogger.With("cluster_name", cfg.Metadata.Name, "phase", "runtime_init")}

	baseRuntime, err := NewBaseRuntime(cfg.Metadata.Name, cfg.Spec.Global.WorkDir, cfg.Spec.Global.Verbose, cfg.Spec.Global.IgnoreErr, baseLogger)
	if err != nil {
		initPhaseLogger.Errorf("Failed to create BaseRuntime: %v", err)
		return nil, err // Propagate the error from NewBaseRuntime
	}

	cr := &ClusterRuntime{
		BaseRuntime:   baseRuntime,
		ClusterConfig: cfg,
		GlobalTimeout: cfg.Spec.Global.ConnectionTimeout,
	}

	if cr.GlobalTimeout <= 0 {
		cr.GlobalTimeout = 30 * time.Second
		cr.Logger().Debugf("Global connection timeout not specified or invalid, defaulting to %s", cr.GlobalTimeout)
	}

	g, gCtx := errgroup.WithContext(context.Background())
	initErrs := &InitializationError{} // Assuming InitializationError is defined with an Add method and an errors field/method

	for _, hostCfg := range cfg.Spec.Hosts {
		currentHostCfg := hostCfg // Capture range variable

		g.Go(func() error {
			// Use the logger from the ClusterRuntime (which is from BaseRuntime)
			hostInitLogger := cr.Logger().SugaredLogger.With("host_name_init", currentHostCfg.Name, "host_address_init", currentHostCfg.Address).Sugar()
			hostInitLogger.Debugf("Initializing...")

			connCfg := connector.ConnectionCfg{
				Host:           currentHostCfg.Address,
				Port:           currentHostCfg.Port,
				User:           currentHostCfg.User,
				Password:       currentHostCfg.Password,
				PrivateKeyPath: currentHostCfg.PrivateKeyPath,
				Timeout:        cr.GlobalTimeout, // Use GlobalTimeout from ClusterRuntime
			}

			var hostPrivateKeyBytes []byte
			if currentHostCfg.PrivateKey != "" {
				decodedKey, err := base64.StdEncoding.DecodeString(currentHostCfg.PrivateKey)
				if err != nil {
					err = fmt.Errorf("host %s: failed to decode base64 private key: %w", currentHostCfg.Name, err)
					initErrs.Add(err)
					hostInitLogger.Errorf("Init failed: %v", err)
					return err
				}
				hostPrivateKeyBytes = decodedKey
				connCfg.PrivateKey = hostPrivateKeyBytes
				connCfg.PrivateKeyPath = "" // Clear path if key content is used
				hostInitLogger.Debugf("Using provided base64 private key content.")
			} else if currentHostCfg.PrivateKeyPath != "" {
				keyFileBytes, err := osReadFile(currentHostCfg.PrivateKeyPath)
				if err != nil {
					err = fmt.Errorf("host %s: failed to read private key file '%s': %w", currentHostCfg.Name, currentHostCfg.PrivateKeyPath, err)
					initErrs.Add(err)
					hostInitLogger.Errorf("Init failed: %v", err)
					return err
				}
				hostPrivateKeyBytes = keyFileBytes
				connCfg.PrivateKey = hostPrivateKeyBytes // SSHConnector will use this
				hostInitLogger.Debugf("Loaded private key from path: %s", currentHostCfg.PrivateKeyPath)
			}

			var conn connector.Connector
			hostType := strings.ToLower(strings.TrimSpace(currentHostCfg.Type))
			if hostType == "local" {
				conn = &connector.LocalConnector{}
			} else {
				conn = &connector.SSHConnector{}
			}

			connectCtx, connectCancel := context.WithTimeout(gCtx, connCfg.Timeout)
			defer connectCancel()
			if err := conn.Connect(connectCtx, connCfg); err != nil {
				err = fmt.Errorf("host %s: connection failed: %w", currentHostCfg.Name, err)
				initErrs.Add(err)
				hostInitLogger.Errorf("Init failed: %v", err)
				return err
			}
			hostInitLogger.Debugf("Connection established.")

			runnerCtx, runnerCancel := context.WithTimeout(gCtx, cr.GlobalTimeout) // Use GlobalTimeout from ClusterRuntime
			defer runnerCancel()

			newRunner, err := runnerNewRunner(runnerCtx, conn)
			if err != nil {
				conn.Close() // Close connection if runner init fails
				err = fmt.Errorf("host %s: runner init failed: %w", currentHostCfg.Name, err)
				initErrs.Add(err)
				hostInitLogger.Errorf("Init failed: %v", err)
				return err
			}
			hostInitLogger.Debugf("Runner initialized. OS: %s, Hostname: %s", newRunner.Facts.OS.ID, newRunner.Facts.Hostname)

			newHost := &Host{
				Name:            currentHostCfg.Name,
				Address:         currentHostCfg.Address,
				InternalAddress: currentHostCfg.InternalAddress,
				Port:            currentHostCfg.Port,
				User:            currentHostCfg.User,
				Password:        currentHostCfg.Password, // Store for potential use, though connector handles auth
				PrivateKeyPath:  currentHostCfg.PrivateKeyPath,
				PrivateKey:      hostPrivateKeyBytes, // Store actual key bytes
				Roles:           make(map[string]bool),
				Labels:          currentHostCfg.Labels,
				Connector:       conn,
				Runner:          newRunner,
				// WorkDir is no longer a field on Host
			}
			if newHost.Labels == nil {
				newHost.Labels = make(map[string]string)
			}
			for _, role := range currentHostCfg.Roles {
				if strings.TrimSpace(role) != "" {
					newHost.Roles[strings.TrimSpace(role)] = true
				}
			}

			// Add host to BaseRuntime's inventory
			if err := cr.AddHost(newHost); err != nil {
				err = fmt.Errorf("host %s: failed to add to runtime: %w", currentHostCfg.Name, err)
				initErrs.Add(err) // Collect error
				hostInitLogger.Errorf("Failed to add to runtime: %v", err)
				conn.Close() // Close connection as we failed to add the host post-connect
				return err   // Return error for this goroutine
			}

			hostInitLogger.Infof("Successfully initialized and added to runtime.")
			return nil
		})
	}

	// Wait for all host initializations to complete or fail
	if err := g.Wait(); err != nil {
		// This error is the first non-nil error returned by a goroutine
		cr.Logger().Warnf("Host initialization process encountered errors (first error: %v). See collected errors below.", err)
	}

	// Check collected errors after all goroutines have finished
	if !initErrs.IsEmpty() {
		numErrors := 0
		if ie, ok := initErrs.(*InitializationError); ok { // Type assertion
			numErrors = len(ie.errors)
		}
		cr.Logger().Errorf("ClusterRuntime initialization failed with %d error(s): %v", numErrors, initErrs.Error())
		return nil, initErrs // Return combined errors
	}

	cr.Logger().Successf("ClusterRuntime initialized successfully with %d hosts.", len(cr.GetAllHosts()))
	return cr, nil
}

// NewHostContext creates a new Context specific to a given host and operation.
// It now accepts cache instances to be associated with this context.
func NewHostContext(
	goCtx context.Context,
	host *Host,
	cluster *ClusterRuntime,
	pCache cache.PipelineCache,
	mCache cache.ModuleCache,
	tCache cache.TaskCache,
	sCache cache.StepCache,
) *Context {
	if goCtx == nil {
		goCtx = context.Background()
	}

	var baseLoggerForContext *logger.Logger
	if cluster != nil && cluster.BaseRuntime != nil { // Check BaseRuntime
		baseLoggerForContext = cluster.Logger() // Use the delegating Logger() method
	} else {
		baseLoggerForContext = logger.Get() // Fallback to global default logger
		if cluster == nil {
			baseLoggerForContext.Warnf("NewHostContext called with nil ClusterRuntime, using global logger.")
		} else { // cluster != nil but BaseRuntime is nil (should ideally not happen if NewRuntime is used)
			baseLoggerForContext.Warnf("NewHostContext called with ClusterRuntime with nil BaseRuntime, using global logger.")
		}
	}

	hostSpecificLogger := baseLoggerForContext // Default to base
	if host != nil && host.Name != "" {
		// Contextualize from the determined baseLoggerForContext
		hostSpecificLogger = &logger.Logger{SugaredLogger: baseLoggerForContext.SugaredLogger.With("host_name", host.Name, "host_address", host.Address)}
	} else {
		// Log warning using the already determined baseLoggerForContext or hostSpecificLogger (which is baseLoggerForContext here)
		if host == nil {
			hostSpecificLogger.Warnf("NewHostContext called with nil host.")
		} else { // host != nil but host.Name is empty
			hostSpecificLogger.Warnf("NewHostContext called with host missing a name.")
		}
	}

	sharedData := &sync.Map{} // Still present as per instructions

	return &Context{
		GoContext:     goCtx,
		Host:          host,
		Cluster:       cluster,
		Logger:        hostSpecificLogger,
		SharedData:    sharedData, // Retained as per instructions
		pipelineCache: pCache,
		moduleCache:   mCache,
		taskCache:     tCache,
		stepCache:     sCache,
	}
}
