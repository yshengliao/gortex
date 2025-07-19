package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newGenerateHandlerCmd() *cobra.Command {
	var handlerType string
	var methods []string

	cmd := &cobra.Command{
		Use:   "handler [name]",
		Short: "Generate a new handler",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return generateHandler(name, handlerType, methods)
		},
	}

	cmd.Flags().StringVarP(&handlerType, "type", "t", "http", "handler type (http, websocket)")
	cmd.Flags().StringSliceVarP(&methods, "methods", "m", []string{"GET", "POST"}, "HTTP methods to generate")

	return cmd
}

func newGenerateServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service [name]",
		Short: "Generate a new service interface and implementation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return generateService(name)
		},
	}

	return cmd
}

func newGenerateModelCmd() *cobra.Command {
	var fields []string

	cmd := &cobra.Command{
		Use:   "model [name]",
		Short: "Generate a new model struct",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return generateModel(name, fields)
		},
	}

	cmd.Flags().StringSliceVarP(&fields, "fields", "f", []string{}, "model fields (format: name:type)")

	return cmd
}

func generateHandler(name, handlerType string, methods []string) error {
	// Ensure name is capitalized
	handlerName := strings.Title(strings.ToLower(name))
	fileName := strings.ToLower(name) + ".go"
	
	moduleName, err := getModuleName()
	if err != nil {
		return err
	}

	var template string
	data := map[string]interface{}{
		"ModuleName":  moduleName,
		"HandlerName": handlerName,
		"Methods":     methods,
	}

	switch handlerType {
	case "websocket", "ws":
		template = websocketHandlerGenerateTemplate
	default:
		template = httpHandlerGenerateTemplate
	}

	path := filepath.Join("handlers", fileName)
	if err := generateFile(path, template, data); err != nil {
		return err
	}

	fmt.Printf("✅ Generated handler: %s\n", path)
	fmt.Printf("\nDon't forget to:\n")
	fmt.Printf("1. Add the handler to HandlersManager in handlers/manager.go\n")
	fmt.Printf("2. Initialize it in cmd/server/main.go\n")

	return nil
}

func generateService(name string) error {
	serviceName := strings.Title(strings.ToLower(name))
	interfaceFileName := strings.ToLower(name) + "_service.go"
	implFileName := strings.ToLower(name) + "_service_impl.go"
	
	moduleName, err := getModuleName()
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"ModuleName":   moduleName,
		"ServiceName":  serviceName,
		"ServiceLower": strings.ToLower(name),
	}

	// Generate interface
	interfacePath := filepath.Join("services", interfaceFileName)
	if err := generateFile(interfacePath, serviceInterfaceTemplate, data); err != nil {
		return err
	}

	// Generate implementation
	implPath := filepath.Join("services", implFileName)
	if err := generateFile(implPath, serviceImplTemplate, data); err != nil {
		return err
	}

	fmt.Printf("✅ Generated service interface: %s\n", interfacePath)
	fmt.Printf("✅ Generated service implementation: %s\n", implPath)
	fmt.Printf("\nDon't forget to:\n")
	fmt.Printf("1. Add the service to your dependency injection\n")
	fmt.Printf("2. Inject it into your handlers\n")

	return nil
}

func generateModel(name string, fields []string) error {
	modelName := strings.Title(strings.ToLower(name))
	fileName := strings.ToLower(name) + ".go"
	
	moduleName, err := getModuleName()
	if err != nil {
		return err
	}

	// Parse fields
	var parsedFields []map[string]string
	for _, field := range fields {
		parts := strings.Split(field, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid field format: %s (expected name:type)", field)
		}
		parsedFields = append(parsedFields, map[string]string{
			"Name": strings.Title(parts[0]),
			"Type": parts[1],
			"JSON": strings.ToLower(parts[0]),
		})
	}

	// Add default fields if none provided
	if len(parsedFields) == 0 {
		parsedFields = []map[string]string{
			{"Name": "ID", "Type": "string", "JSON": "id"},
			{"Name": "CreatedAt", "Type": "time.Time", "JSON": "created_at"},
			{"Name": "UpdatedAt", "Type": "time.Time", "JSON": "updated_at"},
		}
	}

	data := map[string]interface{}{
		"ModuleName": moduleName,
		"ModelName":  modelName,
		"Fields":     parsedFields,
	}

	path := filepath.Join("models", fileName)
	if err := generateFile(path, modelTemplate, data); err != nil {
		return err
	}

	fmt.Printf("✅ Generated model: %s\n", path)

	return nil
}