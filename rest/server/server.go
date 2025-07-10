package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"context"
	"time"
	"net/http"

	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/rest/app"
	// Gin will be imported by router.go if SetupRouter uses it.
)

// APIServer represents the REST API server.
type APIServer struct {
	log    *logger.Logger
	config *Config // Configuration for the server (e.g., port, timeouts)
	// Add other dependencies like database connections, shared services if needed
}

// Config holds configuration for the APIServer.
type Config struct {
	ListenAddress string // e.g., ":8080" or "127.0.0.1:8080"
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	// Add other config like TLS paths if HTTPS is needed
}

// NewDefaultConfig creates a default configuration for the server.
func NewDefaultConfig() *Config {
	return &Config{
		ListenAddress: ":8080", // Default port
		ReadTimeout:   10 * time.Second,
		WriteTimeout:  10 * time.Second,
	}
}

// NewAPIServer creates a new APIServer instance.
func NewAPIServer(cfg *Config, log *logger.Logger) *APIServer {
	if cfg == nil {
		cfg = NewDefaultConfig()
	}
	return &APIServer{
		log:    log,
		config: cfg,
	}
}

// Start initializes services, sets up routing, and starts the HTTP server.
func (s *APIServer) Start() error {
	s.log.Info("Initializing API server...")

	// Initialize application services
	// For a real application, these services might take database connections or other shared resources.
	clusterSvc := app.NewClusterService(s.log) // Pass logger to service

	// Setup router
	router := SetupRouter(s.log, clusterSvc) // Pass logger and service to router setup

	// Configure HTTP server
	httpServer := &http.Server{
		Addr:         s.config.ListenAddress,
		Handler:      router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		// TODO: Add TLSConfig if HTTPS is required
	}

	// Goroutine to start the server
	go func() {
		s.log.Infof("API server starting to listen on %s", s.config.ListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Block until a signal is received

	s.log.Info("API server shutting down...")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		s.log.Errorf("API server graceful shutdown failed: %v", err)
		return fmt.Errorf("API server shutdown failed: %w", err)
	}

	s.log.Info("API server shutdown complete.")
	return nil
}

// Example main function if this were a standalone server binary:
/*
func main() {
	logOpts := logger.DefaultOptions()
	logOpts.ColorConsole = true
	logger.Init(logOpts) // Initialize global logger for the application
	defer logger.SyncGlobal()

	appLogger := logger.Get().With("service", "api-server")

	serverCfg := NewDefaultConfig()
	// TODO: Populate serverCfg from flags or environment variables if needed

	apiSrv := NewAPIServer(serverCfg, appLogger)
	if err := apiSrv.Start(); err != nil {
		appLogger.Fatalf("API Server exited with error: %v", err)
	}
}
*/
```
