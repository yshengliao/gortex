package migrate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// Analyzer analyzes Go projects for Echo handlers
type Analyzer struct {
	fset *token.FileSet
}

// NewAnalyzer creates a new project analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		fset: token.NewFileSet(),
	}
}

// Report contains the analysis results
type Report struct {
	ProjectPath     string
	TotalFiles      int
	AnalyzedFiles   int
	EchoHandlers    []HandlerInfo
	Suggestions     []Suggestion
	MigrationEffort string // "low", "medium", "high"
}

// HandlerInfo contains information about an Echo handler
type HandlerInfo struct {
	File         string
	Line         int
	FunctionName string
	ReceiverType string
	HTTPMethod   string
	Route        string
	UsesContext  bool
	Complexity   string // "simple", "medium", "complex"
	Issues       []string
}

// Suggestion contains migration suggestions
type Suggestion struct {
	Type        string // "handler", "middleware", "structure"
	File        string
	Line        int
	Description string
	Example     string
}

// AnalyzeProject analyzes a Go project for Echo handlers
func (a *Analyzer) AnalyzeProject(projectPath string) (*Report, error) {
	report := &Report{
		ProjectPath:  projectPath,
		EchoHandlers: []HandlerInfo{},
		Suggestions:  []Suggestion{},
	}

	// Walk through all Go files
	err := filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and hidden directories
		if d.IsDir() && (strings.Contains(path, "vendor") || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}

		// Process only Go files
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		report.TotalFiles++

		// Parse the file
		if err := a.analyzeFile(path, report); err != nil {
			// Log error but continue processing
			fmt.Printf("Warning: failed to analyze %s: %v\n", path, err)
		} else {
			report.AnalyzedFiles++
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk project: %w", err)
	}

	// Calculate migration effort
	report.MigrationEffort = a.calculateEffort(report)

	// Generate suggestions
	a.generateSuggestions(report)

	return report, nil
}

// analyzeFile analyzes a single Go file
func (a *Analyzer) analyzeFile(filename string, report *Report) error {
	src, err := parser.ParseFile(a.fset, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	// Check imports for Echo
	hasEcho := false
	for _, imp := range src.Imports {
		if imp.Path.Value == `"github.com/labstack/echo/v4"` {
			hasEcho = true
			break
		}
	}

	if !hasEcho {
		return nil
	}

	// Find Echo handlers
	ast.Inspect(src, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if handler := a.analyzeFunction(node, filename); handler != nil {
				report.EchoHandlers = append(report.EchoHandlers, *handler)
			}
		}
		return true
	})

	return nil
}

// analyzeFunction analyzes a function to determine if it's an Echo handler
func (a *Analyzer) analyzeFunction(fn *ast.FuncDecl, filename string) *HandlerInfo {
	// Check if it has echo.Context parameter
	if !a.hasEchoContext(fn) {
		return nil
	}

	handler := &HandlerInfo{
		File:         filename,
		Line:         a.fset.Position(fn.Pos()).Line,
		FunctionName: fn.Name.Name,
		UsesContext:  true,
		Issues:       []string{},
	}

	// Get receiver type if it's a method
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		if starExpr, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				handler.ReceiverType = ident.Name
			}
		}
	}

	// Analyze function complexity
	handler.Complexity = a.analyzeFunctionComplexity(fn)

	// Check for common patterns and issues
	a.checkHandlerPatterns(fn, handler)

	return handler
}

// hasEchoContext checks if function has echo.Context parameter
func (a *Analyzer) hasEchoContext(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}

	for _, param := range fn.Type.Params.List {
		if selector, ok := param.Type.(*ast.SelectorExpr); ok {
			if ident, ok := selector.X.(*ast.Ident); ok {
				if ident.Name == "echo" && selector.Sel.Name == "Context" {
					return true
				}
			}
		}
	}

	return false
}

// analyzeFunctionComplexity determines the complexity of a handler
func (a *Analyzer) analyzeFunctionComplexity(fn *ast.FuncDecl) string {
	lineCount := 0
	hasBusinessLogic := false
	hasDBCalls := false

	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// Check for database calls
			if selector, ok := node.Fun.(*ast.SelectorExpr); ok {
				methodName := selector.Sel.Name
				if strings.Contains(strings.ToLower(methodName), "query") ||
					strings.Contains(strings.ToLower(methodName), "exec") ||
					strings.Contains(strings.ToLower(methodName), "find") {
					hasDBCalls = true
				}
			}
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt:
			hasBusinessLogic = true
		case *ast.BlockStmt:
			lineCount += len(node.List)
		}
		return true
	})

	if lineCount > 50 || hasDBCalls {
		return "complex"
	} else if lineCount > 20 || hasBusinessLogic {
		return "medium"
	}
	return "simple"
}

// checkHandlerPatterns checks for common patterns and issues
func (a *Analyzer) checkHandlerPatterns(fn *ast.FuncDecl, handler *HandlerInfo) {
	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if selector, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := selector.X.(*ast.Ident); ok {
					if ident.Name == "c" {
						// Check Echo context method calls
						switch selector.Sel.Name {
						case "Bind":
							handler.Issues = append(handler.Issues, "Uses c.Bind() - consider parameter binding")
						case "QueryParam", "QueryParams":
							handler.Issues = append(handler.Issues, "Uses query parameters - consider struct binding")
						case "FormValue", "FormFile":
							handler.Issues = append(handler.Issues, "Uses form handling - needs migration")
						case "Get", "Set":
							handler.Issues = append(handler.Issues, "Uses context storage - consider dependency injection")
						}
					}
				}
			}
		}
		return true
	})
}

// calculateEffort calculates the overall migration effort
func (a *Analyzer) calculateEffort(report *Report) string {
	if len(report.EchoHandlers) == 0 {
		return "none"
	}

	complexCount := 0
	for _, handler := range report.EchoHandlers {
		if handler.Complexity == "complex" {
			complexCount++
		}
	}

	ratio := float64(complexCount) / float64(len(report.EchoHandlers))
	if ratio > 0.5 {
		return "high"
	} else if ratio > 0.2 || len(report.EchoHandlers) > 20 {
		return "medium"
	}
	return "low"
}

// generateSuggestions generates migration suggestions
func (a *Analyzer) generateSuggestions(report *Report) {
	// Group handlers by receiver type
	handlerGroups := make(map[string][]HandlerInfo)
	for _, handler := range report.EchoHandlers {
		if handler.ReceiverType != "" {
			handlerGroups[handler.ReceiverType] = append(handlerGroups[handler.ReceiverType], handler)
		}
	}

	// Suggest service-oriented structure
	for receiverType, handlers := range handlerGroups {
		report.Suggestions = append(report.Suggestions, Suggestion{
			Type:        "structure",
			Description: fmt.Sprintf("Convert %s to a service with business logic methods", receiverType),
			Example: fmt.Sprintf(`// Before: Echo handler
func (h *%s) GetUser(c echo.Context) error {
    id := c.Param("id")
    // ... handler logic
}

// After: Business logic method
func (s *%sService) GetUser(ctx context.Context, id string) (*User, error) {
    // ... business logic
}

// Generated handler will call the service method`, receiverType, receiverType),
		})

		// Check for handlers that can be grouped
		if len(handlers) >= 3 {
			report.Suggestions = append(report.Suggestions, Suggestion{
				Type:        "handler",
				Description: fmt.Sprintf("Consider grouping %d handlers in %s using Gortex's declarative routing", len(handlers), receiverType),
				Example:     "Use struct tags like `url:\"/users/:id\"` for automatic route registration",
			})
		}
	}

	// Suggest middleware migration
	report.Suggestions = append(report.Suggestions, Suggestion{
		Type:        "middleware",
		Description: "Review middleware usage and convert to Gortex middleware pattern",
		Example: `// Gortex supports both Echo-compatible and native middleware
// Consider using built-in middleware for auth, CORS, rate limiting`,
	})

	// Suggest using code generation
	if report.MigrationEffort != "low" {
		report.Suggestions = append(report.Suggestions, Suggestion{
			Type:        "handler",
			Description: "Use gortex-gen to automatically generate HTTP handlers from service methods",
			Example: `//go:generate gortex-gen handler
type UserService struct {
    db *sql.DB
}

// This method will have an HTTP handler generated automatically
func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    // Business logic only
}`,
		})
	}
}