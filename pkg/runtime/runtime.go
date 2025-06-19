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

	WorkDir string
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

// GetHost retrieves a host by its name from the inventory.
func (cr *ClusterRuntime) GetHost(name string) *Host { if cr.Inventory == nil { return nil }; return cr.Inventory[name] }

// GetHostsByRole retrieves all hosts that have the specified role.
func (cr *ClusterRuntime) GetHostsByRole(roleName string) []*Host { if cr.RoleInventory == nil { return []*Host{} }; hosts, found := cr.RoleInventory[roleName]; if !found { return []*Host{} }; return hosts }

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
	if cfg == nil { return nil, fmt.Errorf("cluster configuration cannot be nil") }
	if baseLogger == nil {
		baseLogger = logger.Get() // Fallback to global default logger
		baseLogger.Warnf("NewRuntime called with nil baseLogger, using global default logger instance.")
	}

	runtimeLoggerFields := []interface{}{"component", "runtime"}
	if cfg.Metadata.Name != "" {
		runtimeLoggerFields = append(runtimeLoggerFields, "cluster_name", cfg.Metadata.Name)
	}
	rtWrapperLogger := &logger.Logger{SugaredLogger: baseLogger.SugaredLogger.With(runtimeLoggerFields...)}

	cr := &ClusterRuntime{
		ClusterConfig: cfg,
		Hosts:         make([]*Host, 0, len(cfg.Spec.Hosts)),
		Inventory:     make(map[string]*Host, len(cfg.Spec.Hosts)),
		RoleInventory: make(map[string][]*Host),
		Logger:        baseLogger,
		GlobalTimeout: cfg.Spec.Global.ConnectionTimeout,
		WorkDir:       cfg.Spec.Global.WorkDir,
		Verbose:       cfg.Spec.Global.Verbose,
		IgnoreErr:     cfg.Spec.Global.IgnoreErr,
	}

	if cr.GlobalTimeout <= 0 {
		cr.GlobalTimeout = 30 * time.Second
		rtWrapperLogger.Debugf("Global connection timeout not specified or invalid, defaulting to %s", cr.GlobalTimeout)
	}

	g, gCtx := errgroup.WithContext(context.Background())
	initErrs := &InitializationError{}
	initializedHosts := make([]*Host, len(cfg.Spec.Hosts))

	for i, hostCfg := range cfg.Spec.Hosts {
		currentIndex := i
		currentHostCfg := hostCfg

		g.Go(func() error {
			hostInitLogger := rtWrapperLogger.SugaredLogger.With("host_name_init", currentHostCfg.Name, "host_address_init", currentHostCfg.Address).Sugar()
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
			if currentHostCfg.PrivateKey != "" {
				decodedKey, err := base64.StdEncoding.DecodeString(currentHostCfg.PrivateKey)
				if err != nil {
					err = fmt.Errorf("host %s: failed to decode base64 private key: %w", currentHostCfg.Name, err)
					initErrs.Add(err); hostInitLogger.Errorf("Init failed: %v", err); return err
				}
				hostPrivateKeyBytes = decodedKey
				connCfg.PrivateKey = hostPrivateKeyBytes
				connCfg.PrivateKeyPath = ""
				hostInitLogger.Debugf("Using provided base64 private key content.")
			} else if currentHostCfg.PrivateKeyPath != "" {
				keyFileBytes, err := osReadFile(currentHostCfg.PrivateKeyPath) // Use osReadFile variable
				if err != nil {
					err = fmt.Errorf("host %s: failed to read private key file '%s': %w", currentHostCfg.Name, currentHostCfg.PrivateKeyPath, err)
					initErrs.Add(err); hostInitLogger.Errorf("Init failed: %v", err); return err
				}
				hostPrivateKeyBytes = keyFileBytes
				connCfg.PrivateKey = hostPrivateKeyBytes
				hostInitLogger.Debugf("Loaded private key from path: %s", currentHostCfg.PrivateKeyPath)
			}

			var conn connector.Connector
			hostType := strings.ToLower(strings.TrimSpace(currentHostCfg.Type))
			if hostType == "local" { conn = &connector.LocalConnector{}
			} else { conn = &connector.SSHConnector{} }

			connectCtx, connectCancel := context.WithTimeout(gCtx, connCfg.Timeout)
			defer connectCancel()
			if err := conn.Connect(connectCtx, connCfg); err != nil {
				err = fmt.Errorf("host %s: connection failed: %w", currentHostCfg.Name, err)
				initErrs.Add(err); hostInitLogger.Errorf("Init failed: %v", err); return err
			}
			hostInitLogger.Debugf("Connection established.")

			runnerCtx, runnerCancel := context.WithTimeout(gCtx, cr.GlobalTimeout)
			defer runnerCancel()

			newRunner, err := runnerNewRunner(runnerCtx, conn)
			if err != nil {
				conn.Close(); err = fmt.Errorf("host %s: runner init failed: %w", currentHostCfg.Name, err)
				initErrs.Add(err); hostInitLogger.Errorf("Init failed: %v", err); return err
			}
			hostInitLogger.Debugf("Runner initialized. OS: %s, Hostname: %s", newRunner.Facts.OS.ID, newRunner.Facts.Hostname)

			hostWorkDir := currentHostCfg.WorkDir
			if hostWorkDir == "" { hostWorkDir = fmt.Sprintf("/tmp/kubexms_work_%s", currentHostCfg.Name) }

			newHost := &Host{
				Name:            currentHostCfg.Name, Address: currentHostCfg.Address, InternalAddress: currentHostCfg.InternalAddress,
				Port:            currentHostCfg.Port, User: currentHostCfg.User,
				Password:        currentHostCfg.Password,
				PrivateKeyPath:  currentHostCfg.PrivateKeyPath,
				PrivateKey:      hostPrivateKeyBytes,
				Roles:           make(map[string]bool), Labels: currentHostCfg.Labels,
				Connector:       conn, Runner: newRunner, WorkDir: hostWorkDir,
			}
			if newHost.Labels == nil { newHost.Labels = make(map[string]string) }
			for _, role := range currentHostCfg.Roles { if strings.TrimSpace(role) != "" { newHost.Roles[strings.TrimSpace(role)] = true } }

			initializedHosts[currentIndex] = newHost
			hostInitLogger.Infof("Successfully initialized.")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		rtWrapperLogger.Errorf("NewRuntime completed with errors: %v", initErrs.Error())
		return nil, initErrs
	}

	for _, h := range initializedHosts {
		if h != nil {
			cr.Hosts = append(cr.Hosts, h)
			cr.Inventory[h.Name] = h
			for role := range h.Roles { cr.RoleInventory[role] = append(cr.RoleInventory[role], h) }
		}
	}

	if !initErrs.IsEmpty() && g.Wait() == nil {
	    rtWrapperLogger.Warnf("Runtime initialization had non-fatal errors recorded that were not returned by errgroup: %v", initErrs.Error())
	}

	rtWrapperLogger.Successf("ClusterRuntime initialized successfully with %d hosts.", len(cr.Hosts))
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

	var hostSpecificLogger *logger.Logger
	baseLoggerForContext := logger.Get() // Default to global logger

	if cluster != nil && cluster.Logger != nil {
		baseLoggerForContext = cluster.Logger
	} else {
		baseLoggerForContext.Warnf("NewHostContext: ClusterRuntime or ClusterRuntime.Logger was nil, using global logger as base.")
	}

	if host != nil && host.Name != "" {
		hostSpecificLogger = &logger.Logger{SugaredLogger: baseLoggerForContext.SugaredLogger.With("host_name", host.Name, "host_address", host.Address)}
	} else {
		hostSpecificLogger = baseLoggerForContext
		if host == nil {
			hostSpecificLogger.Warnf("NewHostContext called with nil host.")
		} else {
			hostSpecificLogger.Warnf("NewHostContext called with host missing a name.")
		}
	}

	sharedData := &sync.Map{}

	return &Context{
		GoContext:     goCtx,
		Host:          host,
		Cluster:       cluster,
		Logger:        hostSpecificLogger,
		SharedData:    sharedData,
		pipelineCache: pCache,
		moduleCache:   mCache,
		taskCache:     tCache,
		stepCache:     sCache,
	}
}
