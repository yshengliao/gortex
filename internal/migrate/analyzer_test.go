package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzer(t *testing.T) {
	// Create a temporary test project
	tmpDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.go": `package main

import "github.com/labstack/echo/v4"

func main() {
	e := echo.New()
	e.GET("/", handleHome)
	e.Start(":8080")
}

func handleHome(c echo.Context) error {
	return c.String(200, "Home")
}`,

		"handlers/user_handler.go": `package handlers

import (
	"github.com/labstack/echo/v4"
	"database/sql"
)

type UserHandler struct {
	db *sql.DB
}

// Simple handler
func (h *UserHandler) GetUser(c echo.Context) error {
	id := c.Param("id")
	return c.JSON(200, map[string]string{"id": id})
}

// Medium complexity handler
func (h *UserHandler) CreateUser(c echo.Context) error {
	var user User
	if err := c.Bind(&user); err != nil {
		return err
	}
	
	if user.Name == "" {
		return c.JSON(400, "name required")
	}
	
	// Simulate DB call
	result, err := h.db.Exec("INSERT INTO users (name) VALUES (?)", user.Name)
	if err != nil {
		return err
	}
	
	return c.JSON(201, result)
}

// Complex handler
func (h *UserHandler) ListUsers(c echo.Context) error {
	page := c.QueryParam("page")
	limit := c.QueryParam("limit")
	
	// Complex business logic
	users := []User{}
	rows, err := h.db.Query("SELECT * FROM users WHERE active = ? LIMIT ? OFFSET ?", 
		true, limit, page)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name); err != nil {
			continue
		}
		users = append(users, user)
	}
	
	// More business logic
	for i, user := range users {
		users[i].DisplayName = strings.ToUpper(user.Name)
	}
	
	return c.JSON(200, users)
}

type User struct {
	ID          int
	Name        string
	DisplayName string
}`,

		"middleware/auth.go": `package middleware

import "github.com/labstack/echo/v4"

func AuthMiddleware() echo.HandlerFunc {
	return func(c echo.Context) error {
		token := c.Request().Header.Get("Authorization")
		if token == "" {
			return c.JSON(401, "unauthorized")
		}
		
		c.Set("user_id", "123")
		return nil
	}
}`,

		"services/product_service.go": `package services

// This file doesn't use Echo
type ProductService struct {
	db *sql.DB
}

func (s *ProductService) GetProduct(id string) (*Product, error) {
	// Business logic only
	return &Product{ID: id}, nil
}

type Product struct {
	ID string
}`,
	}

	// Write test files
	for filename, content := range testFiles {
		fullPath := filepath.Join(tmpDir, filename)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	t.Run("analyze project", func(t *testing.T) {
		analyzer := NewAnalyzer()
		report, err := analyzer.AnalyzeProject(tmpDir)
		require.NoError(t, err)

		// Verify basic counts
		assert.Equal(t, tmpDir, report.ProjectPath)
		assert.Equal(t, 4, report.TotalFiles)       // Excludes test files
		// The actual analyzed files count includes the services/product_service.go file
		// even though it doesn't have Echo imports, because it's still parsed
		assert.Equal(t, 4, report.AnalyzedFiles)    // All Go files are analyzed
		assert.Equal(t, 4, len(report.EchoHandlers)) // 4 handlers total

		// Verify handler detection
		handlerNames := make(map[string]bool)
		for _, h := range report.EchoHandlers {
			handlerNames[h.FunctionName] = true
		}

		assert.True(t, handlerNames["handleHome"])
		assert.True(t, handlerNames["GetUser"])
		assert.True(t, handlerNames["CreateUser"])
		assert.True(t, handlerNames["ListUsers"])

		// Verify complexity analysis
		complexities := make(map[string]string)
		for _, h := range report.EchoHandlers {
			complexities[h.FunctionName] = h.Complexity
		}

		assert.Equal(t, "simple", complexities["handleHome"])
		assert.Equal(t, "simple", complexities["GetUser"])
		assert.Equal(t, "complex", complexities["CreateUser"])  // Has DB call
		assert.Equal(t, "complex", complexities["ListUsers"])    // Has DB call and loops

		// Verify receiver type detection
		for _, h := range report.EchoHandlers {
			if h.FunctionName != "handleHome" {
				assert.Equal(t, "UserHandler", h.ReceiverType)
			}
		}

		// Verify migration effort
		assert.Equal(t, "medium", report.MigrationEffort) // 50% complex handlers
	})

	t.Run("detect handler issues", func(t *testing.T) {
		analyzer := NewAnalyzer()
		report, err := analyzer.AnalyzeProject(tmpDir)
		require.NoError(t, err)

		// Find handlers with issues
		issuesByHandler := make(map[string][]string)
		for _, h := range report.EchoHandlers {
			if len(h.Issues) > 0 {
				issuesByHandler[h.FunctionName] = h.Issues
			}
		}

		// Verify issue detection
		assert.Contains(t, issuesByHandler["CreateUser"], "Uses c.Bind() - consider parameter binding")
		assert.Contains(t, issuesByHandler["ListUsers"], "Uses query parameters - consider struct binding")
	})

	t.Run("generate suggestions", func(t *testing.T) {
		analyzer := NewAnalyzer()
		report, err := analyzer.AnalyzeProject(tmpDir)
		require.NoError(t, err)

		// Verify suggestions were generated
		assert.Greater(t, len(report.Suggestions), 0)

		// Check suggestion types
		suggestionTypes := make(map[string]int)
		for _, s := range report.Suggestions {
			suggestionTypes[s.Type]++
		}

		assert.Greater(t, suggestionTypes["structure"], 0)
		assert.Greater(t, suggestionTypes["middleware"], 0)
		assert.Greater(t, suggestionTypes["handler"], 0)

		// Verify UserHandler grouping suggestion
		hasGroupingSuggestion := false
		for _, s := range report.Suggestions {
			if strings.Contains(s.Description, "UserHandler") &&
				strings.Contains(s.Description, "grouping") {
				hasGroupingSuggestion = true
				break
			}
		}
		assert.True(t, hasGroupingSuggestion)
	})
}

func TestAnalyzerEdgeCases(t *testing.T) {
	t.Run("empty project", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		analyzer := NewAnalyzer()
		report, err := analyzer.AnalyzeProject(tmpDir)
		require.NoError(t, err)

		assert.Equal(t, 0, report.TotalFiles)
		assert.Equal(t, 0, len(report.EchoHandlers))
		assert.Equal(t, "none", report.MigrationEffort)
	})

	t.Run("project without Echo", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create a Go file without Echo
		content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`
		err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644)
		require.NoError(t, err)

		analyzer := NewAnalyzer()
		report, err := analyzer.AnalyzeProject(tmpDir)
		require.NoError(t, err)

		assert.Equal(t, 1, report.TotalFiles)
		assert.Equal(t, 1, report.AnalyzedFiles)  // File is analyzed even without Echo
		assert.Equal(t, 0, len(report.EchoHandlers))
		assert.Equal(t, "none", report.MigrationEffort)
	})

	t.Run("skip vendor directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		
		// Create vendor directory with Echo file
		vendorDir := filepath.Join(tmpDir, "vendor", "test")
		err := os.MkdirAll(vendorDir, 0755)
		require.NoError(t, err)
		
		content := `package test
import "github.com/labstack/echo/v4"
func Handler(c echo.Context) error { return nil }`
		
		err = os.WriteFile(filepath.Join(vendorDir, "handler.go"), []byte(content), 0644)
		require.NoError(t, err)

		analyzer := NewAnalyzer()
		report, err := analyzer.AnalyzeProject(tmpDir)
		require.NoError(t, err)

		assert.Equal(t, 0, report.TotalFiles) // Should skip vendor
		assert.Equal(t, 0, len(report.EchoHandlers))
	})
}