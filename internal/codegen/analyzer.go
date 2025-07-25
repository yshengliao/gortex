package codegen

import (
	"fmt"
	"go/ast"
	"strings"
)

// MethodInfo contains analyzed information about a method
type MethodInfo struct {
	Name       string
	Receiver   ReceiverInfo
	Params     []ParamInfo
	Results    []ResultInfo
	HasContext bool
	HTTPMethod string // Detected HTTP method (GET, POST, etc.)
}

// ReceiverInfo contains information about the method receiver
type ReceiverInfo struct {
	Name    string
	Type    string
	IsPtr   bool
}

// ParamInfo contains information about a method parameter
type ParamInfo struct {
	Name     string
	Type     string
	IsPtr    bool
	IsStruct bool
	Package  string // Package if it's an imported type
}

// ResultInfo contains information about a method result
type ResultInfo struct {
	Type    string
	IsError bool
}

// AnalyzeMethod analyzes a method's signature and extracts relevant information
func AnalyzeMethod(fn *ast.FuncDecl) (*MethodInfo, error) {
	info := &MethodInfo{
		Name: fn.Name.Name,
	}

	// Analyze receiver
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := fn.Recv.List[0]
		info.Receiver = analyzeReceiver(recv)
	}

	// Detect HTTP method from function name
	info.HTTPMethod = detectHTTPMethod(fn.Name.Name)

	// Analyze parameters
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			paramType := extractType(field.Type)
			
			// Check if it's echo.Context
			if paramType == "echo.Context" || strings.HasSuffix(paramType, ".Context") {
				info.HasContext = true
			}

			// Get parameter names
			if len(field.Names) == 0 {
				// Unnamed parameter
				info.Params = append(info.Params, ParamInfo{
					Type:     paramType,
					IsPtr:    isPointerType(field.Type),
					IsStruct: isStructType(field.Type),
				})
			} else {
				// Named parameters
				for _, name := range field.Names {
					info.Params = append(info.Params, ParamInfo{
						Name:     name.Name,
						Type:     paramType,
						IsPtr:    isPointerType(field.Type),
						IsStruct: isStructType(field.Type),
					})
				}
			}
		}
	}

	// Analyze results
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			resultType := extractType(field.Type)
			info.Results = append(info.Results, ResultInfo{
				Type:    resultType,
				IsError: resultType == "error",
			})
		}
	}

	return info, nil
}

// analyzeReceiver analyzes the method receiver
func analyzeReceiver(recv *ast.Field) ReceiverInfo {
	info := ReceiverInfo{}

	// Get receiver name
	if len(recv.Names) > 0 {
		info.Name = recv.Names[0].Name
	}

	// Get receiver type
	switch t := recv.Type.(type) {
	case *ast.StarExpr:
		info.IsPtr = true
		if ident, ok := t.X.(*ast.Ident); ok {
			info.Type = ident.Name
		}
	case *ast.Ident:
		info.Type = t.Name
	}

	return info
}

// extractType extracts the type name from an AST expression
func extractType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + extractType(t.X)
	case *ast.SelectorExpr:
		pkg := extractType(t.X)
		return pkg + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + extractType(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", extractType(t.Key), extractType(t.Value))
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + extractType(t.Value)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// isPointerType checks if an expression is a pointer type
func isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// isStructType checks if an expression is likely a struct type
func isStructType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.StructType:
		return true
	case *ast.StarExpr:
		return isStructType(t.X)
	case *ast.Ident:
		// Assume capitalized names are struct types (heuristic)
		return t.Name != "" && t.Name[0] >= 'A' && t.Name[0] <= 'Z' &&
			t.Name != "Context" // Exclude echo.Context
	case *ast.SelectorExpr:
		// For imported types, check the selector
		return t.Sel.Name != "" && t.Sel.Name[0] >= 'A' && t.Sel.Name[0] <= 'Z' &&
			t.Sel.Name != "Context"
	}
	return false
}

// detectHTTPMethod detects the HTTP method from the function name
func detectHTTPMethod(name string) string {
	upperName := strings.ToUpper(name)
	
	// Check for exact matches first
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range httpMethods {
		if upperName == method {
			return method
		}
	}

	// Check for prefixes
	for _, method := range httpMethods {
		if strings.HasPrefix(upperName, method) {
			return method
		}
	}

	// Default to POST for custom methods
	return "POST"
}

// GetRoutePattern generates a route pattern from the method name
func GetRoutePattern(methodName string) string {
	// Convert method name to route pattern
	// Examples: GetUser -> /user, CreateUser -> /user, ListUsers -> /users
	
	name := methodName
	
	// Special case for exact HTTP method names
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range httpMethods {
		if strings.ToUpper(name) == method {
			return "/"
		}
	}
	
	// Remove HTTP method prefixes
	httpPrefixes := []string{"Get", "Post", "Put", "Delete", "Patch", "Create", "Update", "List"}
	for _, prefix := range httpPrefixes {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}

	// Convert to kebab-case
	if name == "" {
		return "/"
	}

	// Convert camelCase to kebab-case
	var result strings.Builder
	var prevWasUpper bool
	for i, r := range name {
		isUpper := r >= 'A' && r <= 'Z'
		
		// Add hyphen before uppercase letters, but not for consecutive uppercase
		if i > 0 && isUpper && !prevWasUpper {
			result.WriteRune('-')
		}
		
		result.WriteRune(r)
		prevWasUpper = isUpper
	}
	
	route := strings.ToLower(result.String())
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	
	return route
}