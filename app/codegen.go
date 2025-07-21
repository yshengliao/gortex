package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
)

// HandlerInfo represents information about a handler
type HandlerInfo struct {
	Name        string            // Field name in HandlersManager
	URLPattern  string            // URL pattern from tag
	IsWebSocket bool              // Whether it's a WebSocket handler
	Methods     []MethodInfo      // HTTP methods and custom endpoints
	TypeName    string            // Go type name
}

// MethodInfo represents a handler method
type MethodInfo struct {
	Name       string // Method name
	HTTPMethod string // GET, POST, etc. or "CUSTOM" for custom endpoints
	Path       string // Full path including custom endpoint conversion
}

// RouteCodeGenerator generates static route registration code
type RouteCodeGenerator struct {
	PackageName string
	Handlers    []HandlerInfo
}

// NewRouteCodeGenerator creates a new code generator
func NewRouteCodeGenerator(packageName string) *RouteCodeGenerator {
	return &RouteCodeGenerator{
		PackageName: packageName,
		Handlers:    []HandlerInfo{},
	}
}

// AnalyzeHandlersFromAST analyzes HandlersManager from AST
func (g *RouteCodeGenerator) AnalyzeHandlersFromAST(filename string) error {
	// Parse the Go file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Find HandlersManager struct
	var handlersManagerStruct *ast.StructType
	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.Name == "HandlersManager" || strings.HasSuffix(typeSpec.Name.Name, "HandlersManager") {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					handlersManagerStruct = structType
					return false // Found it, stop searching
				}
			}
		}
		return true
	})

	if handlersManagerStruct == nil {
		return fmt.Errorf("HandlersManager struct not found in %s", filename)
	}

	// Analyze each field in HandlersManager
	for _, field := range handlersManagerStruct.Fields.List {
		if len(field.Names) == 0 {
			continue // Skip anonymous fields
		}

		fieldName := field.Names[0].Name
		
		// Get URL tag
		var urlTag, hijackTag string
		if field.Tag != nil {
			tagValue := field.Tag.Value
			urlTag = extractTag(tagValue, "url")
			hijackTag = extractTag(tagValue, "hijack")
		}

		if urlTag == "" {
			continue // Skip fields without url tag
		}

		// Get handler type name
		var typeName string
		if starExpr, ok := field.Type.(*ast.StarExpr); ok {
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				typeName = ident.Name
			}
		}

		handler := HandlerInfo{
			Name:        fieldName,
			URLPattern:  urlTag,
			IsWebSocket: hijackTag == "ws",
			TypeName:    typeName,
			Methods:     []MethodInfo{},
		}

		g.Handlers = append(g.Handlers, handler)
	}

	return nil
}

// AnalyzeHandlerMethods analyzes methods for a given handler type using reflection
func (g *RouteCodeGenerator) AnalyzeHandlerMethods(handlerInstance interface{}) error {
	handlerType := reflect.TypeOf(handlerInstance)
	if handlerType == nil {
		return fmt.Errorf("handler instance is nil")
	}

	// Keep the original type (with pointer) for method lookup
	originalType := handlerType
	
	// Get struct name for type matching
	structType := handlerType
	if handlerType.Kind() == reflect.Ptr {
		structType = handlerType.Elem()
	}
	typeName := structType.Name()

	// Find all handlers of this type
	var matchingHandlers []*HandlerInfo
	for i := range g.Handlers {
		if g.Handlers[i].TypeName == typeName {
			matchingHandlers = append(matchingHandlers, &g.Handlers[i])
		}
	}

	if len(matchingHandlers) == 0 {
		return fmt.Errorf("handler type %s not found in handlers list", typeName)
	}

	// Process all matching handlers
	for _, handlerInfo := range matchingHandlers {
		// If it's a WebSocket handler, just add the HandleConnection method
		if handlerInfo.IsWebSocket {
			handlerInfo.Methods = append(handlerInfo.Methods, MethodInfo{
				Name:       "HandleConnection",
				HTTPMethod: "GET",
				Path:       handlerInfo.URLPattern,
			})
			continue
		}

		// Standard HTTP methods (use original pointer type for method lookup)
		httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
		
		for _, method := range httpMethods {
			if _, ok := originalType.MethodByName(method); ok {
				handlerInfo.Methods = append(handlerInfo.Methods, MethodInfo{
					Name:       method,
					HTTPMethod: method,
					Path:       handlerInfo.URLPattern,
				})
			}
		}

		// Custom methods (sub-routes) - use original type for method enumeration
		for i := 0; i < originalType.NumMethod(); i++ {
			method := originalType.Method(i)
			methodName := method.Name

			// Skip standard HTTP methods and unexported methods
			if contains(httpMethods, methodName) || !method.IsExported() || methodName == "HandleConnection" {
				continue
			}

			// Convert method name to route path
			routePath := camelToKebab(methodName)
			fullPath := strings.TrimSuffix(handlerInfo.URLPattern, "/") + "/" + routePath

			handlerInfo.Methods = append(handlerInfo.Methods, MethodInfo{
				Name:       methodName,
				HTTPMethod: "POST", // Custom methods default to POST
				Path:       fullPath,
			})
		}
	}

	return nil
}

// GenerateCode generates the static route registration code
func (g *RouteCodeGenerator) GenerateCode() string {
	var code strings.Builder

	code.WriteString("//go:build production\n")
	code.WriteString("// +build production\n\n")
	code.WriteString("// This file is generated by Gortex code generator\n")
	code.WriteString("// DO NOT EDIT MANUALLY\n\n")
	code.WriteString("package " + g.PackageName + "\n\n")
	code.WriteString("import (\n")
	code.WriteString("\t\"fmt\"\n")
	code.WriteString("\t\"github.com/labstack/echo/v4\"\n")
	code.WriteString(")\n\n")
	code.WriteString("// RegisterRoutes registers routes from a HandlersManager struct\n")
	code.WriteString("// In development mode (default), this uses reflection for instant feedback\n")  
	code.WriteString("// In production mode (go build -tags production), this uses generated static registration\n")
	code.WriteString("func RegisterRoutes(e *echo.Echo, manager interface{}, ctx *Context) error {\n")
	code.WriteString("\treturn RegisterRoutesStatic(e, manager)\n")
	code.WriteString("}\n\n")
	code.WriteString("// RegisterRoutesStatic registers routes using static registration (production mode)\n")
	code.WriteString("func RegisterRoutesStatic(e *echo.Echo, manager interface{}) error {\n")
	
	// Add type assertion
	code.WriteString("\tm, ok := manager.(*HandlersManager)\n")
	code.WriteString("\tif !ok {\n")
	code.WriteString("\t\treturn fmt.Errorf(\"expected *HandlersManager, got %T\", manager)\n")
	code.WriteString("\t}\n\n")

	// Generate route registration for each handler
	for _, handler := range g.Handlers {
		code.WriteString(fmt.Sprintf("\t// Register routes for %s\n", handler.Name))
		
		if handler.IsWebSocket {
			// WebSocket handler
			for _, method := range handler.Methods {
				code.WriteString(fmt.Sprintf("\te.GET(\"%s\", m.%s.%s)\n", 
					method.Path, handler.Name, method.Name))
			}
		} else {
			// HTTP handler
			for _, method := range handler.Methods {
				switch method.HTTPMethod {
				case "GET":
					code.WriteString(fmt.Sprintf("\te.GET(\"%s\", m.%s.%s)\n", 
						method.Path, handler.Name, method.Name))
				case "POST":
					code.WriteString(fmt.Sprintf("\te.POST(\"%s\", m.%s.%s)\n", 
						method.Path, handler.Name, method.Name))
				case "PUT":
					code.WriteString(fmt.Sprintf("\te.PUT(\"%s\", m.%s.%s)\n", 
						method.Path, handler.Name, method.Name))
				case "DELETE":
					code.WriteString(fmt.Sprintf("\te.DELETE(\"%s\", m.%s.%s)\n", 
						method.Path, handler.Name, method.Name))
				case "PATCH":
					code.WriteString(fmt.Sprintf("\te.PATCH(\"%s\", m.%s.%s)\n", 
						method.Path, handler.Name, method.Name))
				case "HEAD":
					code.WriteString(fmt.Sprintf("\te.HEAD(\"%s\", m.%s.%s)\n", 
						method.Path, handler.Name, method.Name))
				case "OPTIONS":
					code.WriteString(fmt.Sprintf("\te.OPTIONS(\"%s\", m.%s.%s)\n", 
						method.Path, handler.Name, method.Name))
				}
			}
		}
		code.WriteString("\n")
	}

	code.WriteString("\treturn nil\n")
	code.WriteString("}\n")

	return code.String()
}

// WriteToFile writes the generated code to a file
func (g *RouteCodeGenerator) WriteToFile(filename string) error {
	code := g.GenerateCode()
	return os.WriteFile(filename, []byte(code), 0644)
}

// Helper function to extract struct tag values
func extractTag(tagString, key string) string {
	// Remove backticks
	tagString = strings.Trim(tagString, "`")
	
	// Split by spaces to get individual tags
	tags := strings.Fields(tagString)
	
	for _, tag := range tags {
		if strings.HasPrefix(tag, key+":") {
			// Extract the value between quotes
			value := strings.TrimPrefix(tag, key+":")
			value = strings.Trim(value, `"`)
			return value
		}
	}
	
	return ""
}