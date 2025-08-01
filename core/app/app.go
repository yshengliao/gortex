// Package app provides the core application framework for building HTTP and WebSocket servers
package app

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yshengliao/gortex/pkg/config"
	appcontext "github.com/yshengliao/gortex/core/context"
	"github.com/yshengliao/gortex/core/app/doc"
	"github.com/yshengliao/gortex/middleware"
	"github.com/yshengliao/gortex/observability/tracing"
	httpctx "github.com/yshengliao/gortex/transport/http"
	"go.uber.org/zap"
)

// ShutdownHook is a function that gets called during shutdown
type ShutdownHook func(ctx context.Context) error

// RuntimeMode represents the framework runtime mode
type RuntimeMode int

// Runtime mode constants
const (
	ModeGortex RuntimeMode = iota // Use Gortex (default)
)

// startTime tracks when the application started
var startTime = time.Now()

// getRuntimeModeName returns the string name of runtime mode
func getRuntimeModeName(mode RuntimeMode) string {
	switch mode {
	case ModeGortex:
		return "Gortex"
	default:
		return "Unknown"
	}
}

// App represents the main application instance
type App struct {
	router          httpctx.GortexRouter
	server          *http.Server
	config          *Config
	logger          *zap.Logger
	ctx             *appcontext.Context
	shutdownHooks   []ShutdownHook
	shutdownTimeout time.Duration
	mu              sync.RWMutex
	runtimeMode     RuntimeMode
	routeInfos      []RouteLogInfo
	enableRoutesLog bool
	developmentMode bool
	tracer          tracing.Tracer
	docProvider     doc.DocProvider
	docRouteInfos   []doc.RouteInfo  // Stores route info for documentation
}

// RouteLogInfo stores information about a registered route for logging
type RouteLogInfo struct {
	Method      string
	Path        string
	Handler     string
	Middlewares []string
}

// Config is re-exported from the config package for convenience
type Config = config.Config

// Option defines a functional option for App
type Option func(*App) error

// NewApp creates a new application instance with the given options
func NewApp(opts ...Option) (*App, error) {
	app := &App{
		router:          httpctx.NewGortexRouter(),
		ctx:             appcontext.NewContext(),
		shutdownHooks:   make([]ShutdownHook, 0),
		shutdownTimeout: 30 * time.Second, // Default 30 seconds
		runtimeMode:     ModeGortex,       // Default to Gortex
	}

	// Apply all options
	for _, opt := range opts {
		if err := opt(app); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Configure router and middleware
	app.setupRouter()

	// Register development routes if in development mode
	if app.IsDevelopment() {
		app.registerDevelopmentRoutes()
	}
	
	// Register documentation endpoints if doc provider is set
	if app.docProvider != nil {
		app.registerDocumentationRoutes()
	}

	return app, nil
}

// WithConfig sets the application configuration
func WithConfig(cfg *Config) Option {
	return func(app *App) error {
		if cfg == nil {
			return fmt.Errorf("config cannot be nil")
		}
		app.config = cfg
		return nil
	}
}

// WithLogger sets the application logger
func WithLogger(logger *zap.Logger) Option {
	return func(app *App) error {
		if logger == nil {
			return fmt.Errorf("logger cannot be nil")
		}
		app.logger = logger
		return nil
	}
}

// WithHandlers registers handlers using reflection
func WithHandlers(manager any) Option {
	return func(app *App) error {
		// Use Gortex registration
		err := RegisterRoutes(app, manager)
		if err != nil {
			return err
		}

		// Log routes if enabled
		if app.enableRoutesLog && app.logger != nil {
			app.logRoutes()
		}

		return nil
	}
}

// WithDevelopmentMode enables development mode features
func WithDevelopmentMode() Option {
	return func(app *App) error {
		app.developmentMode = true
		// Automatically set debug logging level for development mode
		if app.config != nil {
			app.config.Logger.Level = "debug"
		}
		return nil
	}
}

// WithShutdownTimeout sets the shutdown timeout duration
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(app *App) error {
		if timeout <= 0 {
			return fmt.Errorf("shutdown timeout must be positive")
		}
		app.shutdownTimeout = timeout
		return nil
	}
}

// WithRuntimeMode sets the framework runtime mode (kept for compatibility)
func WithRuntimeMode(mode RuntimeMode) Option {
	return func(app *App) error {
		// Only Gortex mode is supported now
		app.runtimeMode = ModeGortex
		return nil
	}
}

// WithRoutesLogger enables automatic route logging
func WithRoutesLogger() Option {
	return func(app *App) error {
		app.enableRoutesLog = true
		return nil
	}
}

// WithTracer sets the tracer for distributed tracing
func WithTracer(tracer tracing.Tracer) Option {
	return func(app *App) error {
		if tracer == nil {
			return fmt.Errorf("tracer cannot be nil")
		}
		app.tracer = tracer
		// Apply tracing middleware immediately if router is initialized
		if app.router != nil {
			app.router.Use(tracing.TracingMiddleware(tracer))
		}
		return nil
	}
}

// WithDocProvider sets the documentation provider for API documentation generation
func WithDocProvider(provider doc.DocProvider) Option {
	return func(app *App) error {
		if provider == nil {
			return fmt.Errorf("doc provider cannot be nil")
		}
		app.docProvider = provider
		return nil
	}
}

// setupRouter configures the Gortex router with middleware
func (app *App) setupRouter() {
	// Apply middleware based on configuration
	if app.config == nil || app.config.Server.Recovery {
		// TODO: Add recovery middleware for Gortex
	}

	// TODO: Add compression middleware support for Gortex
	// TODO: Add CORS middleware support for Gortex

	// Request ID middleware
	app.router.Use(middleware.RequestID())

	// Development mode enhancements
	if app.IsDevelopment() {
		// Add development error page middleware
		app.router.Use(middleware.RecoverWithErrorPage())
		// TODO: Add development logger middleware
	}

	// TODO: Add error handler middleware
}

// Router returns the underlying Gortex router
func (app *App) Router() httpctx.GortexRouter {
	return app.router
}

// Context returns the application context
func (app *App) Context() *appcontext.Context {
	return app.ctx
}

// IsDevelopment returns true if the application is running in development mode
func (app *App) IsDevelopment() bool {
	return app.developmentMode || (app.config != nil && app.config.Logger.Level == "debug")
}

// Run starts the HTTP server
func (app *App) Run() error {
	address := ":8080"
	if app.config != nil && app.config.Server.Address != "" {
		address = app.config.Server.Address
	}

	if app.logger != nil {
		// Development mode startup messages
		if app.IsDevelopment() {
			app.logger.Info("🔥 Development mode enabled!")
			app.logger.Info("📝 Available debug endpoints:")
			app.logger.Info("   • /_routes   - View all routes")
			app.logger.Info("   • /_config   - View configuration")
			app.logger.Info("   • /_monitor  - System metrics")
			app.logger.Info("   • /_error    - Test error pages")
			app.logger.Info("💡 Tip: Install air for hot reload: go install github.com/cosmtrek/air@latest")
			app.logger.Info("🚀 Starting Gortex server")
		}

		app.logger.Info("Starting server",
			zap.String("address", address),
			zap.String("runtime_mode", getRuntimeModeName(app.runtimeMode)),
			zap.Bool("development_mode", app.IsDevelopment()))
	}

	// Create HTTP server
	app.server = &http.Server{
		Addr:    address,
		Handler: app.router,
	}

	return app.server.ListenAndServe()
}

// RegisterShutdownHook registers a function to be called during shutdown
func (app *App) RegisterShutdownHook(hook ShutdownHook) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.shutdownHooks = append(app.shutdownHooks, hook)
}

// OnShutdown is a convenience method for registering shutdown hooks
func (app *App) OnShutdown(fn func(context.Context) error) {
	app.RegisterShutdownHook(ShutdownHook(fn))
}

// Shutdown gracefully shuts down the server
func (app *App) Shutdown(ctx context.Context) error {
	if app.logger != nil {
		app.logger.Info("Starting graceful shutdown")
	}

	// Create a timeout context if none provided
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, app.shutdownTimeout)
		defer cancel()
	}

	var shutdownErr error

	// Run pre-shutdown hooks in parallel
	if err := app.runShutdownHooks(ctx); err != nil {
		if app.logger != nil {
			app.logger.Error("Error running shutdown hooks", zap.Error(err))
		}
		shutdownErr = err
	}

	// Shutdown the HTTP server
	if app.logger != nil {
		app.logger.Info("Shutting down HTTP server")
	}

	if app.server != nil {
		if err := app.server.Shutdown(ctx); err != nil {
			if app.logger != nil {
				app.logger.Error("Error shutting down HTTP server", zap.Error(err))
			}
			// Server shutdown error takes precedence
			return err
		}
	}

	if app.logger != nil {
		app.logger.Info("Graceful shutdown completed")
	}

	return shutdownErr
}

// runShutdownHooks executes all registered shutdown hooks
func (app *App) runShutdownHooks(ctx context.Context) error {
	app.mu.RLock()
	hooks := make([]ShutdownHook, len(app.shutdownHooks))
	copy(hooks, app.shutdownHooks)
	app.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	if app.logger != nil {
		app.logger.Info("Running shutdown hooks", zap.Int("count", len(hooks)))
	}

	// Run hooks in parallel with error collection
	var wg sync.WaitGroup
	errChan := make(chan error, len(hooks))

	for i, hook := range hooks {
		wg.Add(1)
		go func(idx int, h ShutdownHook) {
			defer wg.Done()

			if app.logger != nil {
				app.logger.Debug("Running shutdown hook", zap.Int("index", idx))
			}

			if err := h(ctx); err != nil {
				errChan <- fmt.Errorf("shutdown hook %d failed: %w", idx, err)
			}
		}(i, hook)
	}

	// Wait for all hooks to complete or context to expire
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All hooks completed
	case <-ctx.Done():
		// Context expired
		return fmt.Errorf("shutdown hooks timed out: %w", ctx.Err())
	}

	close(errChan)

	// Collect any errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown hooks failed: %v", errs)
	}

	return nil
}

// registerDevelopmentRoutes registers development-only routes
func (app *App) registerDevelopmentRoutes() {
	// Import handlers package
	devHandlers := &devHandler{
		logger: app.logger,
		router: app.router,
		config: app.config,
	}

	// Register development routes
	app.router.GET("/_routes", devHandlers.Routes)
	app.router.GET("/_error", devHandlers.Error)
	app.router.GET("/_config", devHandlers.Config)
	app.router.GET("/_monitor", devHandlers.Monitor)

	if app.logger != nil {
		app.logger.Info("Development routes registered",
			zap.String("routes", "/_routes"),
			zap.String("error", "/_error"),
			zap.String("config", "/_config"),
			zap.String("monitor", "/_monitor"),
		)
	}
}

// registerDocumentationRoutes registers API documentation endpoints
func (app *App) registerDocumentationRoutes() {
	// First, generate documentation from collected route info
	if len(app.docRouteInfos) > 0 {
		// Generate documentation
		_, err := app.docProvider.Generate(app.docRouteInfos)
		if err != nil && app.logger != nil {
			app.logger.Error("Failed to generate documentation", zap.Error(err))
		}
	}
	
	// Get endpoints from the doc provider
	endpoints := app.docProvider.Endpoints()
	
	registeredPaths := []string{}
	for path, handler := range endpoints {
		// Register each endpoint with the router
		// Use GET for documentation endpoints
		app.router.GET(path, func(c httpctx.Context) error {
			handler.ServeHTTP(c.Response(), c.Request())
			return nil
		})
		registeredPaths = append(registeredPaths, path)
	}
	
	if app.logger != nil && len(registeredPaths) > 0 {
		app.logger.Info("Documentation routes registered",
			zap.Strings("paths", registeredPaths),
			zap.Int("documented_routes", len(app.docRouteInfos)),
		)
	}
}

// AddDocumentationRoute adds a route to the documentation collection
func (app *App) AddDocumentationRoute(routeInfo doc.RouteInfo) {
	app.mu.Lock()
	defer app.mu.Unlock()
	
	if app.docProvider != nil {
		app.docRouteInfos = append(app.docRouteInfos, routeInfo)
	}
}

// devHandler provides development endpoints
type devHandler struct {
	logger *zap.Logger
	router httpctx.GortexRouter
	config *Config
}

// Routes returns debug information about all registered routes
func (h *devHandler) Routes(c httpctx.Context) error {
	// Since GortexRouter doesn't expose routes, we'll return a placeholder
	// In a real implementation, we'd need to add a Routes() method to the router

	return c.JSON(200, map[string]any{
		"total_routes": 4, // Dev routes count
		"routes": []map[string]string{
			{"method": "GET", "path": "/_routes"},
			{"method": "GET", "path": "/_error"},
			{"method": "GET", "path": "/_config"},
			{"method": "GET", "path": "/_monitor"},
		},
		"framework": "Gortex",
	})
}

// Error returns test error pages for development
func (h *devHandler) Error(c httpctx.Context) error {
	errorType := c.QueryParam("type")

	switch errorType {
	case "panic":
		panic("Development test panic")
	case "internal":
		return fmt.Errorf("internal server error test")
	default:
		return c.JSON(200, map[string]any{
			"message": "Use ?type=<error_type> to test different error responses",
			"available_types": []string{
				"panic",
				"internal",
			},
		})
	}
}

// Config returns masked configuration for development
func (h *devHandler) Config(c httpctx.Context) error {
	// In a real implementation, you would get config from context
	// For now, return a simple response
	return c.JSON(200, map[string]any{
		"message": "Configuration endpoint",
		"mode":    "development",
	})
}

// Monitor returns system monitoring information
func (h *devHandler) Monitor(c httpctx.Context) error {
	// Get memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get system info
	systemInfo := map[string]any{
		"goroutines":     runtime.NumGoroutine(),
		"cpu_count":      runtime.NumCPU(),
		"go_version":     runtime.Version(),
		"max_procs":      runtime.GOMAXPROCS(0),
		"timestamp":      time.Now().Format(time.RFC3339),
		"uptime_seconds": time.Since(startTime).Seconds(),
	}

	// Memory statistics
	memoryInfo := map[string]any{
		"alloc_mb":         float64(m.Alloc) / 1024 / 1024,
		"total_alloc_mb":   float64(m.TotalAlloc) / 1024 / 1024,
		"sys_mb":           float64(m.Sys) / 1024 / 1024,
		"heap_alloc_mb":    float64(m.HeapAlloc) / 1024 / 1024,
		"heap_sys_mb":      float64(m.HeapSys) / 1024 / 1024,
		"heap_idle_mb":     float64(m.HeapIdle) / 1024 / 1024,
		"heap_inuse_mb":    float64(m.HeapInuse) / 1024 / 1024,
		"heap_released_mb": float64(m.HeapReleased) / 1024 / 1024,
		"heap_objects":     m.HeapObjects,
		"stack_inuse_mb":   float64(m.StackInuse) / 1024 / 1024,
		"stack_sys_mb":     float64(m.StackSys) / 1024 / 1024,
		"num_gc":           m.NumGC,
		"gc_cpu_fraction":  m.GCCPUFraction,
	}

	// Get GC stats
	gcStats := make([]map[string]any, 0)
	if m.NumGC > 0 {
		// Get last 5 GC pause times
		numPauses := int(m.NumGC)
		if numPauses > 5 {
			numPauses = 5
		}
		for i := 0; i < numPauses; i++ {
			gcStats = append(gcStats, map[string]any{
				"pause_ms": float64(m.PauseNs[(m.NumGC+255-uint32(i))%256]) / 1e6,
			})
		}
	}

	// TODO: Get Gortex routes count
	routesInfo := map[string]any{
		"total_routes": "TBD",
	}

	// Get compression status
	compressionInfo := map[string]any{
		"enabled":           false,
		"gzip_enabled":      false,
		"brotli_enabled":    false,
		"compression_level": "not configured",
	}

	if h.config != nil {
		// Check new compression config first
		if h.config.Server.Compression.Enabled {
			compressionInfo["enabled"] = true
			compressionInfo["gzip_enabled"] = true
			compressionInfo["brotli_enabled"] = h.config.Server.Compression.EnableBrotli
			compressionInfo["prefer_brotli"] = h.config.Server.Compression.PreferBrotli
			compressionInfo["compression_level"] = h.config.Server.Compression.Level
			compressionInfo["min_size_bytes"] = h.config.Server.Compression.MinSize

			contentTypes := h.config.Server.Compression.ContentTypes
			if len(contentTypes) == 0 {
				// Use defaults if not specified
				contentTypes = []string{
					"text/html",
					"text/css",
					"text/plain",
					"text/javascript",
					"application/javascript",
					"application/json",
					"application/xml",
				}
			}
			compressionInfo["content_types"] = contentTypes
		} else if h.config.Server.GZip {
			// Legacy GZip config
			compressionInfo["enabled"] = true
			compressionInfo["gzip_enabled"] = true
			compressionInfo["compression_level"] = "default (gzip.DefaultCompression)"
			compressionInfo["content_types"] = []string{
				"text/html",
				"text/css",
				"text/plain",
				"text/javascript",
				"application/javascript",
				"application/json",
				"application/xml",
			}
			compressionInfo["min_size_bytes"] = 1024
		}
	}

	// Final response
	return c.JSON(200, map[string]any{
		"status":      "healthy",
		"system":      systemInfo,
		"memory":      memoryInfo,
		"gc_stats":    gcStats,
		"routes":      routesInfo,
		"compression": compressionInfo,
		"server_info": map[string]any{
			"framework":  "Gortex",
			"debug_mode": h.config != nil && h.config.Logger.Level == "debug",
		},
	})
}

// logRoutes logs all registered routes in a formatted table
func (app *App) logRoutes() {
	if len(app.routeInfos) == 0 {
		app.logger.Info("No routes registered")
		return
	}

	// Sort routes by path and method for better readability
	sortedRoutes := make([]RouteLogInfo, len(app.routeInfos))
	copy(sortedRoutes, app.routeInfos)

	// Simple sort by path then method
	for i := 0; i < len(sortedRoutes); i++ {
		for j := i + 1; j < len(sortedRoutes); j++ {
			if sortedRoutes[i].Path > sortedRoutes[j].Path ||
				(sortedRoutes[i].Path == sortedRoutes[j].Path && sortedRoutes[i].Method > sortedRoutes[j].Method) {
				sortedRoutes[i], sortedRoutes[j] = sortedRoutes[j], sortedRoutes[i]
			}
		}
	}

	// Calculate column widths
	methodWidth := 6      // "Method"
	pathWidth := 4        // "Path"
	handlerWidth := 7     // "Handler"
	middlewareWidth := 11 // "Middlewares"

	for _, route := range sortedRoutes {
		if len(route.Method) > methodWidth {
			methodWidth = len(route.Method)
		}
		if len(route.Path) > pathWidth {
			pathWidth = len(route.Path)
		}
		if len(route.Handler) > handlerWidth {
			handlerWidth = len(route.Handler)
		}
		middlewareStr := strings.Join(route.Middlewares, ", ")
		if len(middlewareStr) > middlewareWidth {
			middlewareWidth = len(middlewareStr)
		}
	}

	// Add padding
	methodWidth += 2
	pathWidth += 2
	handlerWidth += 2
	middlewareWidth += 2

	// Build the table
	var output strings.Builder

	// Header
	output.WriteString("\n")
	output.WriteString("┌")
	output.WriteString(strings.Repeat("─", methodWidth))
	output.WriteString("┬")
	output.WriteString(strings.Repeat("─", pathWidth))
	output.WriteString("┬")
	output.WriteString(strings.Repeat("─", handlerWidth))
	output.WriteString("┬")
	output.WriteString(strings.Repeat("─", middlewareWidth))
	output.WriteString("┐\n")

	// Header row
	output.WriteString("│")
	output.WriteString(padRight(" Method", methodWidth))
	output.WriteString("│")
	output.WriteString(padRight(" Path", pathWidth))
	output.WriteString("│")
	output.WriteString(padRight(" Handler", handlerWidth))
	output.WriteString("│")
	output.WriteString(padRight(" Middlewares", middlewareWidth))
	output.WriteString("│\n")

	// Separator
	output.WriteString("├")
	output.WriteString(strings.Repeat("─", methodWidth))
	output.WriteString("┼")
	output.WriteString(strings.Repeat("─", pathWidth))
	output.WriteString("┼")
	output.WriteString(strings.Repeat("─", handlerWidth))
	output.WriteString("┼")
	output.WriteString(strings.Repeat("─", middlewareWidth))
	output.WriteString("┤\n")

	// Data rows
	for _, route := range sortedRoutes {
		output.WriteString("│")
		output.WriteString(padRight(" "+route.Method, methodWidth))
		output.WriteString("│")
		output.WriteString(padRight(" "+route.Path, pathWidth))
		output.WriteString("│")
		output.WriteString(padRight(" "+route.Handler, handlerWidth))
		output.WriteString("│")
		middlewareStr := strings.Join(route.Middlewares, ", ")
		if middlewareStr == "" {
			middlewareStr = "none"
		}
		output.WriteString(padRight(" "+middlewareStr, middlewareWidth))
		output.WriteString("│\n")
	}

	// Footer
	output.WriteString("└")
	output.WriteString(strings.Repeat("─", methodWidth))
	output.WriteString("┴")
	output.WriteString(strings.Repeat("─", pathWidth))
	output.WriteString("┴")
	output.WriteString(strings.Repeat("─", handlerWidth))
	output.WriteString("┴")
	output.WriteString(strings.Repeat("─", middlewareWidth))
	output.WriteString("┘\n")

	app.logger.Info("Registered routes", zap.String("table", output.String()))
}

// padRight pads a string to the right with spaces
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
