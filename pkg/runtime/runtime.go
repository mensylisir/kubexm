package runtime

import (
	"context"
	"encoding/base64" // For decoding private key if it's base64 encoded
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kubexms/kubexms/pkg/cache" // Added cache import
	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner"
	// "github.com/kubexms/kubexms/pkg/config" // This was the old config path, no longer needed.
	// "{{MODULE_NAME}}/pkg/config" // No longer needed as parser returns v1alpha1.Cluster directly
	"{{MODULE_NAME}}/pkg/parser" // The new parser
	"github.com/kubexms/kubexms/pkg/apis/kubexms/v1alpha1"
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
	// WorkDir string // Removed
}

// String returns the name of the host.
func (h *Host) String() string { return h.Name }

// HasRole checks if the host has a specific role.
func (h *Host) HasRole(roleName string) bool { if h.Roles == nil { return false }; _, exists := h.Roles[roleName]; return exists }

// GetLabel returns the value of a label and whether it exists.
func (h *Host) GetLabel(labelName string) (string, bool) { if h.Labels == nil { return "", false }; val, exists := h.Labels[labelName]; return val, exists }

// ClusterRuntime holds all global, read-only information and resources
// required for a KubeXMS operation (e.g., cluster creation, scaling).
// This is the canonical definition.
type ClusterRuntime struct {
	BaseRuntime      *BaseRuntime // Embedded BaseRuntime
	ClusterConfig    *v1alpha1.Cluster
	GlobalTimeout    time.Duration
	ConnectionPool *connector.ConnectionPool // Added connection pool
}

// ShutdownConnectionPool gracefully shuts down the connection pool.
func (cr *ClusterRuntime) ShutdownConnectionPool() {
	if cr.ConnectionPool != nil {
		cr.Logger().Infof("Shutting down SSH connection pool...")
		cr.ConnectionPool.Shutdown()
		cr.Logger().Infof("SSH connection pool shutdown complete.")
	}
}

// Delegating methods to BaseRuntime
func (cr *ClusterRuntime) GetHost(name string) *Host {
	if cr.BaseRuntime == nil { return nil }
	return cr.BaseRuntime.GetHost(name)
}
func (cr *ClusterRuntime) GetAllHosts() []*Host {
	if cr.BaseRuntime == nil { return []*Host{} }
	return cr.BaseRuntime.GetAllHosts()
}
func (cr *ClusterRuntime) GetHostsByRole(roleName string) []*Host {
	if cr.BaseRuntime == nil { return []*Host{} }
	return cr.BaseRuntime.GetHostsByRole(roleName)
}
func (cr *ClusterRuntime) Logger() *logger.Logger {
	if cr.BaseRuntime == nil { return logger.Get() /* fallback */ }
	return cr.BaseRuntime.Logger()
}
func (cr *ClusterRuntime) GetWorkDir() string {
	if cr.BaseRuntime == nil { return "" }
	return cr.BaseRuntime.GetWorkDir()
}
func (cr *ClusterRuntime) GetHostWorkDir(hostName string) string {
	if cr.BaseRuntime == nil { return "" }
	return cr.BaseRuntime.GetHostWorkDir(hostName)
}
func (cr *ClusterRuntime) IsVerbose() bool {
	if cr.BaseRuntime == nil { return false }
	return cr.BaseRuntime.IsVerbose()
}
func (cr *ClusterRuntime) ShouldIgnoreErr() bool {
	if cr.BaseRuntime == nil { return false }
	return cr.BaseRuntime.ShouldIgnoreErr()
}
func (cr *ClusterRuntime) AddHost(host *Host) error {
	if cr.BaseRuntime == nil { return fmt.Errorf("BaseRuntime not initialized in ClusterRuntime") }
	return cr.BaseRuntime.AddHost(host)
}
func (cr *ClusterRuntime) RemoveHost(hostName string) error {
	if cr.BaseRuntime == nil { return fmt.Errorf("BaseRuntime not initialized in ClusterRuntime") }
	return cr.BaseRuntime.RemoveHost(hostName)
}
func (cr *ClusterRuntime) ObjName() string {
	if cr.BaseRuntime == nil { return "" }
	return cr.BaseRuntime.ObjName()
}

// Copy creates a new ClusterRuntime instance.
func (cr *ClusterRuntime) Copy() *ClusterRuntime {
    if cr == nil { return nil }
    if cr.BaseRuntime == nil {
        // Log this problematic state if possible, maybe with a package-level logger
        // For now, returning nil to prevent panic.
        return nil
    }
    copiedBaseRuntime := cr.BaseRuntime.Copy()
    newCr := &ClusterRuntime{
        BaseRuntime:   copiedBaseRuntime,
        ClusterConfig: cr.ClusterConfig, // Pointer copy is fine for config
        GlobalTimeout: cr.GlobalTimeout,
    }
    if newCr.Logger() != nil { // Logger comes from copiedBaseRuntime
       newCr.Logger().Debugf("Created a copy of ClusterRuntime for '%s'.", newCr.ObjName())
    }
    return newCr
}

// Context is passed to each execution unit (e.g., a Step in a Task).
type Context struct {
	GoContext     context.Context
	Host          *Host
	Cluster       *ClusterRuntime
	Logger        *logger.Logger // Logger contextualized for Host, Task, Step etc.
	// SharedData field removed.

	pipelineCache cache.PipelineCache
	moduleCache   cache.ModuleCache
	taskCache     cache.TaskCache
	stepCache     cache.StepCache
}

// NewRuntimeFromYAML is a new constructor that takes YAML data directly,
// parses it, (notionally) converts it, and then calls NewRuntime.
func NewRuntimeFromYAML(yamlData []byte, baseLogger *logger.Logger) (*ClusterRuntime, error) {
	if baseLogger == nil {
		baseLogger = logger.Get() // Fallback to global default logger
	}
	initPhaseLogger := &logger.Logger{SugaredLogger: baseLogger.SugaredLogger.With("phase", "runtime_init_from_yaml")}

	parsedConfig, err := parser.ParseClusterYAML(yamlData)
	if err != nil {
		initPhaseLogger.Errorf("Failed to parse cluster YAML: %v", err)
		return nil, fmt.Errorf("failed to parse cluster YAML: %w", err)
	}

	// parsedConfig is now directly *v1alpha1.Cluster
	if parsedConfig == nil {
		// This case should ideally be caught by the parser if YAML is truly empty or invalid.
		initPhaseLogger.Errorf("Parsed configuration is nil after parsing YAML.")
		return nil, fmt.Errorf("parsed configuration is nil after parsing YAML")
	}

	// Log using ObjectMeta.Name
	initPhaseLogger.Infof("Successfully parsed cluster YAML for cluster: %s", parsedConfig.ObjectMeta.Name)

	// Apply defaults from the v1alpha1 package
	v1alpha1.SetDefaults_Cluster(parsedConfig)
	initPhaseLogger.Infof("Applied v1alpha1 defaults to the cluster configuration for: %s", parsedConfig.ObjectMeta.Name)

	// No explicit conversion/cast function call needed anymore.
	// parsedConfig is already the *v1alpha1.Cluster type needed by NewRuntime.

	// Call the existing NewRuntime function with the parsed v1alpha1.Cluster config
	return NewRuntime(parsedConfig, baseLogger)
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
func NewRuntime(cfg *v1alpha1.Cluster, baseLogger *logger.Logger) (*ClusterRuntime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cluster configuration cannot be nil")
	}
	initErrs := &InitializationError{}

	if baseLogger == nil {
		baseLogger = logger.Get() // Fallback to global default logger
		baseLogger.Warnf("NewRuntime called with nil baseLogger, using global default logger instance.")
	}

	initPhaseLogger := &logger.Logger{SugaredLogger: baseLogger.SugaredLogger.With("cluster_name", cfg.ObjectMeta.Name, "phase", "runtime_init")}

	var globalWorkDir string
	var globalVerbose bool
	var globalIgnoreErr bool
	var globalConnectionTimeout time.Duration

	if cfg.Spec.Global != nil {
		globalWorkDir = cfg.Spec.Global.WorkDir
		globalVerbose = cfg.Spec.Global.Verbose
		globalIgnoreErr = cfg.Spec.Global.IgnoreErr
		globalConnectionTimeout = cfg.Spec.Global.ConnectionTimeout
	} else {
		initPhaseLogger.Warnf("cfg.Spec.Global is nil, using default values for BaseRuntime parameters and GlobalTimeout.")
		globalConnectionTimeout = 30 * time.Second // Default timeout if GlobalSpec is missing
		// NewBaseRuntime will handle default for globalWorkDir if empty
	}

	baseRuntime, err := NewBaseRuntime(cfg.ObjectMeta.Name, globalWorkDir, globalVerbose, globalIgnoreErr, baseLogger)
	if err != nil {
		initPhaseLogger.Errorf("Failed to create BaseRuntime: %v", err)
		return nil, err
	}

	// Initialize Connection Pool
	poolCfg := connector.DefaultPoolConfig()
	// TODO: Customize poolCfg from cfg (v1alpha1.Cluster) if needed in the future
	pool := connector.NewConnectionPool(poolCfg)

	cr := &ClusterRuntime{
		BaseRuntime:      baseRuntime,
		ClusterConfig:    cfg,
		GlobalTimeout:    globalConnectionTimeout,
		ConnectionPool: pool, // Assign the initialized pool
	}

	if cr.GlobalTimeout <= 0 {
		cr.GlobalTimeout = 30 * time.Second
		// Ensure logger is available before using it
		if cr.Logger() != nil {
			cr.Logger().Debugf("Global connection timeout defaulted to 30s")
		}
	}

	g, gCtx := errgroup.WithContext(context.Background())

	for _, hostCfg := range cfg.Spec.Hosts { // hostCfg is v1alpha1.HostSpec
		currentHostCfg := hostCfg // Capture range variable

		g.Go(func() error {
			hostInitLogger := cr.Logger().SugaredLogger.With("host_name_init", currentHostCfg.Name, "host_address_init", currentHostCfg.Address).Sugar()
			hostInitLogger.Debugf("Initializing...")

			connCfg := connector.ConnectionCfg{
				Host:           currentHostCfg.Address,
				Port:           currentHostCfg.Port,
				User:           currentHostCfg.User,
				Password:       currentHostCfg.Password,
				PrivateKeyPath: currentHostCfg.PrivateKeyPath,
				Timeout:        cr.GlobalTimeout,
			}

			var hostPrivateKeyBytes []byte
			if currentHostCfg.PrivateKey != "" { // This is base64 encoded string from v1alpha1.HostSpec
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
				connCfg.PrivateKey = hostPrivateKeyBytes
				hostInitLogger.Debugf("Loaded private key from path: %s", currentHostCfg.PrivateKeyPath)
			}

			var conn connector.Connector
			hostType := strings.ToLower(strings.TrimSpace(currentHostCfg.Type))
			if hostType == "local" {
				conn = &connector.LocalConnector{}
			} else {
				// Use the pool-aware SSHConnector constructor
				conn = connector.NewSSHConnector(cr.ConnectionPool)
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

			runnerCtx, runnerCancel := context.WithTimeout(gCtx, cr.GlobalTimeout)
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

			newHostRoles := make(map[string]bool)
			for _, roleName := range currentHostCfg.Roles { // hostCfg.Roles is []string
				if strings.TrimSpace(roleName) != "" {
					newHostRoles[strings.TrimSpace(roleName)] = true
				}
			}

			newHost := &Host{
				Name:            currentHostCfg.Name,
				Address:         currentHostCfg.Address,
				InternalAddress: currentHostCfg.InternalAddress,
				Port:            currentHostCfg.Port,
				User:            currentHostCfg.User,
				Password:        currentHostCfg.Password,
				PrivateKeyPath:  currentHostCfg.PrivateKeyPath,
				PrivateKey:      hostPrivateKeyBytes,
				Roles:           newHostRoles, // Use converted map
				Labels:          currentHostCfg.Labels, // Already map[string]string
				Connector:       conn,
				Runner:          newRunner,
				// WorkDir field is removed from Host struct
			}

			if err := cr.AddHost(newHost); err != nil {
				err = fmt.Errorf("host %s: failed to add to runtime: %w", currentHostCfg.Name, err)
				initErrs.Add(err)
				hostInitLogger.Errorf("Failed to add to runtime: %v", err)
				conn.Close() // Close connection as we failed to add the host post-connect
				return err
			}

			hostInitLogger.Infof("Successfully initialized and added to runtime.")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		if cr.Logger() != nil {
			cr.Logger().Warnf("Host initialization process encountered errors. First error: %v", err)
		}
	}

	if !initErrs.IsEmpty() {
		if cr.Logger() != nil {
			cr.Logger().Errorf("ClusterRuntime initialization failed: %v", initErrs)
		}
		return nil, initErrs
	}

	if cr.Logger() != nil {
		cr.Logger().Successf("ClusterRuntime initialized successfully with %d hosts.", len(cr.GetAllHosts()))
	}
	return cr, nil
}

// NewHostContext creates a new Context specific to a given host and operation.
// It now initializes its own cache instances.
func NewHostContext(
	goCtx context.Context,
	host *Host,
	cluster *ClusterRuntime,
) *Context {
	if goCtx == nil {
		goCtx = context.Background()
	}

	var baseLoggerForContext *logger.Logger
	if cluster != nil && cluster.BaseRuntime != nil && cluster.Logger() != nil {
		baseLoggerForContext = cluster.Logger()
	} else {
		baseLoggerForContext = logger.Get() // Fallback to global default logger
		if cluster == nil {
			baseLoggerForContext.Warnf("NewHostContext called with nil ClusterRuntime, using global logger.")
		} else { // cluster != nil but BaseRuntime or its logger is nil
			baseLoggerForContext.Warnf("NewHostContext called with ClusterRuntime with nil or uninitialized BaseRuntime, using global logger.")
		}
	}

	hostSpecificLogger := baseLoggerForContext // Default to base
	if host != nil && host.Name != "" {
		hostSpecificLogger = &logger.Logger{SugaredLogger: baseLoggerForContext.SugaredLogger.With("host_name", host.Name, "host_address", host.Address)}
	} else {
		if host == nil {
			hostSpecificLogger.Warnf("NewHostContext called with nil host.")
		} else { // host != nil but host.Name is empty
			hostSpecificLogger.Warnf("NewHostContext called with host missing a name.")
		}
	}

	// sharedData := &sync.Map{} // Removed

	return &Context{
		GoContext:     goCtx,
		Host:          host,
		Cluster:       cluster,
		Logger:        hostSpecificLogger,
		// SharedData:    sharedData, // Removed
	pipelineCache: cache.NewPipelineCache(),
	moduleCache:   cache.NewModuleCache(),
	taskCache:     cache.NewTaskCache(),
	stepCache:     cache.NewStepCache(),
	}
}
