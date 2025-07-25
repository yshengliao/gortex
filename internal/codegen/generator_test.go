package codegen

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_scanFile(t *testing.T) {
	// Create test file with handler methods
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "test_service.go")
	
	testCode := `package service

import (
	"context"
	"github.com/labstack/echo/v4"
)

type UserService struct {
	db Database
}

// GetUser retrieves a user by ID
//go:generate gortex-gen handler
func (s *UserService) GetUser(ctx context.Context, id int) (*User, error) {
	return s.db.GetUser(ctx, id)
}

// CreateUser creates a new user
// gortex:handler
func (s *UserService) CreateUser(ctx context.Context, user *User) (*User, error) {
	return s.db.CreateUser(ctx, user)
}

// This method should not be generated
func (s *UserService) internalMethod() error {
	return nil
}

type User struct {
	ID   int
	Name string
}

type Database interface {
	GetUser(ctx context.Context, id int) (*User, error)
	CreateUser(ctx context.Context, user *User) (*User, error)
}
`
	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	// Create generator
	config := &Config{
		InputDir:  testDir,
		OutputDir: testDir,
		Logger:    nil,
	}
	gen := NewGenerator(config)

	// Scan the file
	err = gen.scanFile(testFile)
	require.NoError(t, err)

	// Check found methods
	assert.Len(t, gen.foundSpecs, 2)
	
	// Check first method
	assert.Equal(t, "GetUser", gen.foundSpecs[0].MethodName)
	assert.Equal(t, "UserService", gen.foundSpecs[0].StructName)
	assert.Equal(t, "*UserService", gen.foundSpecs[0].ReceiverType)
	
	// Check second method
	assert.Equal(t, "CreateUser", gen.foundSpecs[1].MethodName)
	assert.Equal(t, "UserService", gen.foundSpecs[1].StructName)
}

func TestGenerator_shouldGenerateHandler(t *testing.T) {
	// Create test files with different comment styles
	testCases := []struct {
		name     string
		code     string
		expected int // Expected number of handlers found
	}{
		{
			name: "go:generate comment",
			code: `package test

//go:generate gortex-gen handler
func (s *Service) Method1() error { return nil }
`,
			expected: 1,
		},
		{
			name: "gortex:handler comment",
			code: `package test

// gortex:handler
func (s *Service) Method2() error { return nil }
`,
			expected: 1,
		},
		{
			name: "no generation comment",
			code: `package test

// Regular comment
func (s *Service) Method3() error { return nil }
`,
			expected: 0,
		},
		{
			name: "multiple handlers",
			code: `package test

//go:generate gortex-gen handler
func (s *Service) Method4() error { return nil }

// gortex:handler
func (s *Service) Method5() error { return nil }
`,
			expected: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test file
			testDir := t.TempDir()
			testFile := filepath.Join(testDir, "test.go")
			err := os.WriteFile(testFile, []byte(tc.code), 0644)
			require.NoError(t, err)

			// Create generator
			config := &Config{
				InputDir:  testDir,
				OutputDir: testDir,
				Logger:    nil,
			}
			gen := NewGenerator(config)

			// Scan the file
			err = gen.scanFile(testFile)
			require.NoError(t, err)

			// Check found methods
			assert.Len(t, gen.foundSpecs, tc.expected)
		})
	}
}

func TestGenerator_scanDirectory(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()
	
	// Create main service file
	mainFile := filepath.Join(testDir, "service.go")
	mainCode := `package service

//go:generate gortex-gen handler
func (s *Service) MainMethod() error { return nil }
`
	err := os.WriteFile(mainFile, []byte(mainCode), 0644)
	require.NoError(t, err)

	// Create subdirectory with another service
	subDir := filepath.Join(testDir, "sub")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)
	
	subFile := filepath.Join(subDir, "sub_service.go")
	subCode := `package sub

// gortex:handler
func (s *SubService) SubMethod() error { return nil }
`
	err = os.WriteFile(subFile, []byte(subCode), 0644)
	require.NoError(t, err)

	// Create test file (should be ignored)
	testFile := filepath.Join(testDir, "service_test.go")
	testCode := `package service

//go:generate gortex-gen handler
func (s *Service) TestMethod() error { return nil }
`
	err = os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	// Test with recursive scanning
	t.Run("recursive", func(t *testing.T) {
		config := &Config{
			InputDir:  testDir,
			OutputDir: testDir,
			Recursive: true,
			Logger:    nil,
		}
		gen := NewGenerator(config)

		err = gen.scanDirectory(testDir)
		require.NoError(t, err)

		// Should find both methods (main and sub)
		assert.Len(t, gen.foundSpecs, 2)
		
		methodNames := []string{}
		for _, spec := range gen.foundSpecs {
			methodNames = append(methodNames, spec.MethodName)
		}
		assert.Contains(t, methodNames, "MainMethod")
		assert.Contains(t, methodNames, "SubMethod")
	})

	// Test without recursive scanning
	t.Run("non-recursive", func(t *testing.T) {
		config := &Config{
			InputDir:  testDir,
			OutputDir: testDir,
			Recursive: false,
			Logger:    nil,
		}
		gen := NewGenerator(config)

		err = gen.scanDirectory(testDir)
		require.NoError(t, err)

		// Should only find main method
		assert.Len(t, gen.foundSpecs, 1)
		assert.Equal(t, "MainMethod", gen.foundSpecs[0].MethodName)
	})
}

func TestGenerator_Generate(t *testing.T) {
	// Create test service
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "service.go")
	testCode := `package service

import "context"

type UserService struct{}

//go:generate gortex-gen handler
func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
	return &User{ID: id, Name: "Test"}, nil
}

// gortex:handler
func (s *UserService) CreateUser(ctx context.Context, user *User) (*User, error) {
	return user, nil
}

type User struct {
	ID   string
	Name string
}
`
	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	// Test dry run
	t.Run("dry run", func(t *testing.T) {
		var logOutput strings.Builder
		logger := log.New(&logOutput, "", 0)
		
		config := &Config{
			InputDir:  testDir,
			OutputDir: testDir,
			DryRun:    true,
			Logger:    logger,
		}
		gen := NewGenerator(config)

		err = gen.Generate()
		require.NoError(t, err)

		// Check log output
		output := logOutput.String()
		assert.Contains(t, output, "Found 2 handler methods")
		assert.Contains(t, output, "[DRY RUN]")
	})

	// Test actual generation (when implemented)
	t.Run("generate", func(t *testing.T) {
		config := &Config{
			InputDir:  testDir,
			OutputDir: testDir,
			DryRun:    false,
			Logger:    nil,
		}
		gen := NewGenerator(config)

		err = gen.Generate()
		require.NoError(t, err)
		
		// Check that generation completes without error
		// The actual generation is tested in templates_test.go
	})
}