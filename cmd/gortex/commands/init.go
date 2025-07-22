package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func initProject(basePath, projectName string, withExamples bool) error {
	// Create project structure
	dirs := []string{
		"cmd/server",
		"handlers",
		"services",
		"models",
		"config",
	}

	for _, dir := range dirs {
		path := filepath.Join(basePath, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	// Generate go.mod
	moduleName := projectName
	if !strings.Contains(moduleName, "/") {
		moduleName = "github.com/yourusername/" + projectName
	}

	if err := generateFile(filepath.Join(basePath, "go.mod"), goModTemplate, map[string]string{
		"ModuleName":    moduleName,
		"GORTEXVersion": rootCmd.Version,
	}); err != nil {
		return err
	}

	// Generate main.go
	if err := generateFile(filepath.Join(basePath, "cmd/server/main.go"), mainTemplate, map[string]string{
		"ModuleName":  moduleName,
		"ProjectName": projectName,
	}); err != nil {
		return err
	}

	// Generate config.yaml
	if err := generateFile(filepath.Join(basePath, "config/config.yaml"), configTemplate, nil); err != nil {
		return err
	}

	// Generate handlers
	if err := generateFile(filepath.Join(basePath, "handlers/manager.go"), handlersManagerTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	if err := generateFile(filepath.Join(basePath, "handlers/health.go"), healthHandlerTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	// Generate services
	if err := generateFile(filepath.Join(basePath, "services/interfaces.go"), servicesInterfacesTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	// Generate README
	if err := generateFile(filepath.Join(basePath, "README.md"), projectReadmeTemplate, map[string]string{
		"ProjectName": projectName,
		"ModuleName":  moduleName,
	}); err != nil {
		return err
	}

	// Generate .gitignore
	if err := generateFile(filepath.Join(basePath, ".gitignore"), gitignoreTemplate, nil); err != nil {
		return err
	}

	// Generate examples if requested
	if withExamples {
		if err := generateExamples(basePath, moduleName); err != nil {
			return err
		}
	}

	fmt.Printf("\n✅ Project '%s' initialized successfully!\n", projectName)
	fmt.Println("\nNext steps:")
	fmt.Println("1. cd " + basePath)
	fmt.Println("2. go mod tidy")
	fmt.Println("3. go run cmd/server/main.go")
	fmt.Println("\nOr use the CLI:")
	fmt.Println("  gortex server")

	return nil
}

func generateExamples(basePath, moduleName string) error {
	// Create example handlers
	exampleHandlersPath := filepath.Join(basePath, "handlers")

	if err := generateFile(filepath.Join(exampleHandlersPath, "user.go"), userHandlerTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	if err := generateFile(filepath.Join(exampleHandlersPath, "auth.go"), authHandlerTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	if err := generateFile(filepath.Join(exampleHandlersPath, "websocket.go"), websocketHandlerTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	// Create example services
	exampleServicesPath := filepath.Join(basePath, "services")

	if err := generateFile(filepath.Join(exampleServicesPath, "user_service.go"), userServiceTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	// Create example models
	exampleModelsPath := filepath.Join(basePath, "models")

	if err := generateFile(filepath.Join(exampleModelsPath, "user.go"), userModelTemplate, map[string]string{
		"ModuleName": moduleName,
	}); err != nil {
		return err
	}

	return nil
}
