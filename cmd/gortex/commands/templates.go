package commands

const goModTemplate = `module {{.ModuleName}}

go 1.24

require (
	github.com/yshengliao/gortex {{.GORTEXVersion}}
	github.com/labstack/echo/v4 v4.13.4
	go.uber.org/zap v1.27.0
)
`

const mainTemplate = `package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"{{.ModuleName}}/handlers"
	"{{.ModuleName}}/services"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/config"
	"github.com/yshengliao/gortex/hub"
)

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	// Load configuration
	cfg := config.DefaultConfig()
	loader := config.NewSimpleLoader().
		WithYAMLFile("config/config.yaml").
		WithEnvPrefix("{{.ProjectName | upper}}_")
	
	if err := loader.Load(cfg); err != nil {
		logger.Warn("Failed to load config, using defaults", zap.Error(err))
	}

	// Initialize services
	userService := services.NewUserService(logger)

	// Initialize WebSocket hub
	wsHub := hub.NewHub(logger)
	go wsHub.Run()

	// Initialize handlers
	handlersManager := &handlers.HandlersManager{
		Health: &handlers.HealthHandler{
			Logger: logger,
		},
		// Add more handlers here
	}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlersManager),
	)
	if err != nil {
		logger.Fatal("Failed to create application", zap.Error(err))
	}

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting server",
			zap.String("address", cfg.Server.Address),
		)
		if err := application.Run(); err != nil {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down server...")

	// Shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}
`

const configTemplate = `server:
  address: ":8080"
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 10s
  gzip: true
  cors: true

logger:
  level: "info"
  encoding: "json"

jwt:
  secret_key: "your-secret-key-change-in-production"
  access_token_ttl: 1h
  refresh_token_ttl: 168h
  issuer: "gortex-app"

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  database: "gortex_db"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 1h
`

const handlersManagerTemplate = `package handlers

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"{{.ModuleName}}/services"
	"github.com/yshengliao/gortex/hub"
)

// HandlersManager contains all handlers
type HandlersManager struct {
	Health    *HealthHandler    ` + "`url:\"/health\"`" + `
	// Add more handlers here
	// Example:
	// Users     *UserHandler      ` + "`url:\"/api/users\"`" + `
	// Auth      *AuthHandler      ` + "`url:\"/api/auth\"`" + `
	// WebSocket *WebSocketHandler ` + "`url:\"/ws\" hijack:\"ws\"`" + `
}
`

const healthHandlerTemplate = `package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type HealthHandler struct {
	Logger *zap.Logger
}

func (h *HealthHandler) GET(c echo.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now().Unix(),
		"runtime": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"memory_mb": m.Alloc / 1024 / 1024,
		},
	})
}
`

const servicesInterfacesTemplate = `package services

import (
	"context"
	"go.uber.org/zap"
)

// Add your service interfaces here
`

const projectReadmeTemplate = `# {{.ProjectName}}

A web application built with GORTEX framework.

## Getting Started

### Prerequisites

- Go 1.24 or higher
- PostgreSQL (optional, for database features)

### Installation

` + "```bash" + `
go mod tidy
` + "```" + `

### Running the Application

` + "```bash" + `
# Development mode with hot reload
gortex server

# Or run directly
go run cmd/server/main.go
` + "```" + `

### Configuration

Configuration is loaded from ` + "`config/config.yaml`" + ` and can be overridden with environment variables.

Example:
` + "```bash" + `
export {{.ProjectName | upper}}_SERVER_ADDRESS=:3000
export {{.ProjectName | upper}}_JWT_SECRET_KEY=your-secret-key
` + "```" + `

## Project Structure

` + "```" + `
.
├── cmd/server/        # Application entry point
├── handlers/          # HTTP handlers
├── services/          # Business logic
├── models/           # Data models
└── config/           # Configuration files
` + "```" + `

## Development

### Generate a new handler

` + "```bash" + `
gortex generate handler user --methods GET,POST,PUT,DELETE
` + "```" + `

### Generate a new service

` + "```bash" + `
gortex generate service user
` + "```" + `

### Generate a new model

` + "```bash" + `
gortex generate model user --fields username:string,email:string,password:string
` + "```" + `

## License

MIT
`

const gitignoreTemplate = `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
.gortex-dev-server

# Test binary, built with go test -c
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# Go workspace file
go.work

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Environment
.env
.env.local

# Logs
*.log

# Temporary files
tmp/
temp/
`

// Example templates
const userHandlerTemplate = `package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"{{.ModuleName}}/models"
	"{{.ModuleName}}/services"
	"github.com/yshengliao/gortex/response"
	"github.com/yshengliao/gortex/validation"
)

type UserHandler struct {
	UserService services.UserService
	Logger      *zap.Logger
}

type CreateUserRequest struct {
	Username string ` + "`json:\"username\" validate:\"required,min=3,max=30\"`" + `
	Email    string ` + "`json:\"email\" validate:\"required,email\"`" + `
	Password string ` + "`json:\"password\" validate:\"required,min=8\"`" + `
}

func (h *UserHandler) GET(c echo.Context) error {
	users, err := h.UserService.ListUsers(c.Request().Context())
	if err != nil {
		h.Logger.Error("Failed to list users", zap.Error(err))
		return response.InternalServerError(c, "Failed to fetch users")
	}

	return response.Success(c, http.StatusOK, users)
}

func (h *UserHandler) POST(c echo.Context) error {
	var req CreateUserRequest
	if err := validation.BindAndValidate(c, &req); err != nil {
		return err
	}

	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
	}

	created, err := h.UserService.CreateUser(c.Request().Context(), user, req.Password)
	if err != nil {
		h.Logger.Error("Failed to create user", zap.Error(err))
		return response.InternalServerError(c, "Failed to create user")
	}

	return response.Success(c, http.StatusCreated, created)
}
`

const authHandlerTemplate = `package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"{{.ModuleName}}/services"
	"github.com/yshengliao/gortex/auth"
	"github.com/yshengliao/gortex/response"
	"github.com/yshengliao/gortex/validation"
)

type AuthHandler struct {
	UserService services.UserService
	JWTService  *auth.JWTService
	Logger      *zap.Logger
}

type LoginRequest struct {
	Username string ` + "`json:\"username\" validate:\"required\"`" + `
	Password string ` + "`json:\"password\" validate:\"required\"`" + `
}

type LoginResponse struct {
	AccessToken  string ` + "`json:\"access_token\"`" + `
	RefreshToken string ` + "`json:\"refresh_token\"`" + `
	ExpiresIn    int    ` + "`json:\"expires_in\"`" + `
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := validation.BindAndValidate(c, &req); err != nil {
		return err
	}

	user, err := h.UserService.Authenticate(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		return response.Unauthorized(c, "Invalid credentials")
	}

	accessToken, err := h.JWTService.GenerateAccessToken(user.ID, user.Username, user.Email, "user")
	if err != nil {
		h.Logger.Error("Failed to generate access token", zap.Error(err))
		return response.InternalServerError(c, "Failed to generate token")
	}

	refreshToken, err := h.JWTService.GenerateRefreshToken(user.ID)
	if err != nil {
		h.Logger.Error("Failed to generate refresh token", zap.Error(err))
		return response.InternalServerError(c, "Failed to generate token")
	}

	return response.Success(c, http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(time.Hour.Seconds()),
	})
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	// TODO: Implement refresh token logic
	return response.Success(c, http.StatusOK, map[string]string{
		"message": "Token refreshed",
	})
}
`

const websocketHandlerTemplate = `package handlers

import (
	"net/http"
	
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/auth"
	"github.com/yshengliao/gortex/hub"
)

type WebSocketHandler struct {
	Hub    *hub.Hub
	Logger *zap.Logger
}

func (h *WebSocketHandler) HandleConnection(c echo.Context) error {
	// Get user ID from JWT
	userID := auth.GetUserID(c)
	if userID == "" {
		return echo.NewHTTPError(401, "Unauthorized")
	}

	// Create WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Configure appropriately for production
		},
	}
	
	// Upgrade connection
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.Logger.Error("Failed to upgrade connection", zap.Error(err))
		return err
	}

	// Create client
	client := hub.NewClient(h.Hub, conn, userID, h.Logger)
	h.Hub.RegisterClient(client)

	// Start client pumps
	go client.WritePump()
	go client.ReadPump()

	h.Logger.Info("WebSocket client connected", 
		zap.String("user_id", userID),
		zap.String("remote_addr", c.Request().RemoteAddr),
	)

	return nil
}
`

const userServiceTemplate = `package services

import (
	"context"
	"errors"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"
	"{{.ModuleName}}/models"
)

type UserService interface {
	CreateUser(ctx context.Context, user *models.User, password string) (*models.User, error)
	GetUser(ctx context.Context, id string) (*models.User, error)
	ListUsers(ctx context.Context) ([]*models.User, error)
	Authenticate(ctx context.Context, username, password string) (*models.User, error)
}

type userService struct {
	logger *zap.Logger
	users  map[string]*models.User
	mu     sync.RWMutex
}

func NewUserService(logger *zap.Logger) UserService {
	return &userService{
		logger: logger,
		users:  make(map[string]*models.User),
	}
}

func (s *userService) CreateUser(ctx context.Context, user *models.User, password string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Generate ID
	user.ID = uuid.New().String()
	user.Password = string(hashedPassword)

	// Check if username exists
	for _, u := range s.users {
		if u.Username == user.Username {
			return nil, errors.New("username already exists")
		}
	}

	// Store user
	s.users[user.ID] = user

	// Return user without password
	result := *user
	result.Password = ""
	return &result, nil
}

func (s *userService) GetUser(ctx context.Context, id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}

	// Return copy without password
	result := *user
	result.Password = ""
	return &result, nil
}

func (s *userService) ListUsers(ctx context.Context) ([]*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*models.User, 0, len(s.users))
	for _, user := range s.users {
		// Copy without password
		u := *user
		u.Password = ""
		users = append(users, &u)
	}

	return users, nil
}

func (s *userService) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find user by username
	var foundUser *models.User
	for _, user := range s.users {
		if user.Username == username {
			foundUser = user
			break
		}
	}

	if foundUser == nil {
		return nil, errors.New("invalid credentials")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(foundUser.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Return copy without password
	result := *foundUser
	result.Password = ""
	return &result, nil
}
`

const userModelTemplate = `package models

import "time"

type User struct {
	ID        string    ` + "`json:\"id\"`" + `
	Username  string    ` + "`json:\"username\"`" + `
	Email     string    ` + "`json:\"email\"`" + `
	Password  string    ` + "`json:\"-\"`" + ` // Never expose in JSON
	Role      string    ` + "`json:\"role\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\"`" + `
}
`

// Code generation templates
const httpHandlerGenerateTemplate = `package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/response"
)

type {{.HandlerName}}Handler struct {
	Logger *zap.Logger
	// Add your dependencies here
}

{{range .Methods}}
func (h *{{$.HandlerName}}Handler) {{.}}(c echo.Context) error {
	// TODO: Implement {{.}} {{$.HandlerName | lower}}
	return response.Success(c, http.StatusOK, map[string]string{
		"message": "{{.}} {{$.HandlerName | lower}} endpoint",
	})
}
{{end}}
`

const websocketHandlerGenerateTemplate = `package handlers

import (
	"net/http"
	
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"github.com/yshengliao/gortex/hub"
)

type {{.HandlerName}}Handler struct {
	Hub    *hub.Hub
	Logger *zap.Logger
}

func (h *{{.HandlerName}}Handler) HandleConnection(c echo.Context) error {
	// Create WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Configure appropriately for production
		},
	}
	
	// Upgrade connection
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.Logger.Error("Failed to upgrade connection", zap.Error(err))
		return err
	}

	// TODO: Create and register client
	// client := hub.NewClient(h.Hub, conn, clientID, h.Logger)
	// h.Hub.RegisterClient(client)

	return nil
}
`

const serviceInterfaceTemplate = `package services

import (
	"context"
)

type {{.ServiceName}}Service interface {
	// TODO: Define your service methods
	// Example:
	// Create{{.ServiceName}}(ctx context.Context, data interface{}) error
	// Get{{.ServiceName}}(ctx context.Context, id string) (interface{}, error)
	// List{{.ServiceName}}(ctx context.Context) ([]interface{}, error)
	// Update{{.ServiceName}}(ctx context.Context, id string, data interface{}) error
	// Delete{{.ServiceName}}(ctx context.Context, id string) error
}
`

const serviceImplTemplate = `package services

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

type {{.ServiceLower}}Service struct {
	logger *zap.Logger
	mu     sync.RWMutex
	// Add your data storage here
}

func New{{.ServiceName}}Service(logger *zap.Logger) {{.ServiceName}}Service {
	return &{{.ServiceLower}}Service{
		logger: logger,
	}
}

// TODO: Implement your service methods
`

const modelTemplate = `package models

import "time"

type {{.ModelName}} struct {
{{range .Fields}}	{{.Name}} {{.Type}} ` + "`json:\"{{.JSON}}\"`" + `
{{end}}}
`