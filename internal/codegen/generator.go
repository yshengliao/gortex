package codegen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the configuration for the code generator
type Config struct {
	InputDir    string
	OutputDir   string
	PackageName string
	Recursive   bool
	DryRun      bool
	Logger      *log.Logger
}

// Generator is the main code generator
type Generator struct {
	config    *Config
	fileSet   *token.FileSet
	foundSpecs []HandlerSpec
}

// HandlerSpec represents a handler method to generate
type HandlerSpec struct {
	PackageName  string
	StructName   string
	MethodName   string
	FilePath     string
	LineNumber   int
	Comment      string
	Method       *ast.FuncDecl
	ReceiverType string
}

// NewGenerator creates a new code generator
func NewGenerator(config *Config) *Generator {
	return &Generator{
		config:  config,
		fileSet: token.NewFileSet(),
	}
}

// Generate scans for handler methods and generates code
func (g *Generator) Generate() error {
	g.log("Starting code generation...")
	g.log("Input directory: %s", g.config.InputDir)
	g.log("Output directory: %s", g.config.OutputDir)

	// Scan for handler methods
	if err := g.scanDirectory(g.config.InputDir); err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	g.log("Found %d handler methods to generate", len(g.foundSpecs))

	// Generate code for each found method
	for _, spec := range g.foundSpecs {
		if err := g.generateHandler(spec); err != nil {
			return fmt.Errorf("failed to generate handler for %s.%s: %w", 
				spec.StructName, spec.MethodName, err)
		}
	}

	return nil
}

// scanDirectory recursively scans a directory for Go files
func (g *Generator) scanDirectory(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories if not recursive
		if d.IsDir() && path != dir && !g.config.Recursive {
			return filepath.SkipDir
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip generated files
		if strings.Contains(path, ".gen.go") || strings.Contains(path, "_gen.go") {
			return nil
		}

		g.log("Scanning file: %s", path)
		return g.scanFile(path)
	})
}

// scanFile scans a single Go file for handler methods
func (g *Generator) scanFile(path string) error {
	// Parse the file
	file, err := parser.ParseFile(g.fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", path, err)
	}

	// Look for methods with //go:generate gortex-gen handler comments
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			if g.shouldGenerateHandler(fn, file) {
				spec := g.createHandlerSpec(fn, file, path)
				g.foundSpecs = append(g.foundSpecs, spec)
				g.log("Found handler: %s.%s", spec.StructName, spec.MethodName)
			}
		}
	}

	return nil
}

// shouldGenerateHandler checks if a method should have a handler generated
func (g *Generator) shouldGenerateHandler(fn *ast.FuncDecl, file *ast.File) bool {
	// Check for //go:generate gortex-gen handler comment
	if fn.Doc != nil {
		for _, comment := range fn.Doc.List {
			text := strings.TrimSpace(comment.Text)
			if strings.HasPrefix(text, "//go:generate gortex-gen handler") ||
			   strings.HasPrefix(text, "// gortex:handler") {
				return true
			}
		}
	}

	// Also check the line immediately before the function
	pos := g.fileSet.Position(fn.Pos())
	for _, cg := range file.Comments {
		for _, comment := range cg.List {
			cpos := g.fileSet.Position(comment.Pos())
			if cpos.Line == pos.Line-1 {
				text := strings.TrimSpace(comment.Text)
				if strings.HasPrefix(text, "//go:generate gortex-gen handler") ||
				   strings.HasPrefix(text, "// gortex:handler") {
					return true
				}
			}
		}
	}

	return false
}

// createHandlerSpec creates a handler specification from an AST function
func (g *Generator) createHandlerSpec(fn *ast.FuncDecl, file *ast.File, path string) HandlerSpec {
	spec := HandlerSpec{
		PackageName: file.Name.Name,
		MethodName:  fn.Name.Name,
		FilePath:    path,
		Method:      fn,
	}

	// Get receiver type
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := fn.Recv.List[0]
		switch t := recv.Type.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				spec.StructName = ident.Name
				spec.ReceiverType = "*" + ident.Name
			}
		case *ast.Ident:
			spec.StructName = t.Name
			spec.ReceiverType = t.Name
		}
	}

	// Get line number
	pos := g.fileSet.Position(fn.Pos())
	spec.LineNumber = pos.Line

	// Get comment if any
	if fn.Doc != nil && len(fn.Doc.List) > 0 {
		spec.Comment = fn.Doc.Text()
	}

	return spec
}

// generateHandler generates the HTTP handler for a method
func (g *Generator) generateHandler(spec HandlerSpec) error {
	g.log("Generating handler for %s.%s", spec.StructName, spec.MethodName)

	// Analyze the method
	methodInfo, err := AnalyzeMethod(spec.Method)
	if err != nil {
		return fmt.Errorf("failed to analyze method: %w", err)
	}

	// Generate handler code
	code, err := GenerateHandler(spec, methodInfo)
	if err != nil {
		return fmt.Errorf("failed to generate handler: %w", err)
	}

	// Determine output file path
	outputFile := g.getOutputFilePath(spec)
	
	if g.config.DryRun {
		g.log("[DRY RUN] Would write handler to %s", outputFile)
		g.log("[DRY RUN] Generated code:\n%s", code)
		return nil
	}

	// Write the generated code
	if err := g.writeGeneratedFile(outputFile, code); err != nil {
		return fmt.Errorf("failed to write generated file: %w", err)
	}

	g.log("Generated handler written to %s", outputFile)
	return nil
}

// getOutputFilePath determines the output file path for a handler
func (g *Generator) getOutputFilePath(spec HandlerSpec) string {
	// Convert service name to handler name
	handlerName := strings.ToLower(spec.StructName) + "_handler_gen.go"
	
	// Ensure output directory exists
	outputDir := g.config.OutputDir
	if outputDir == "" {
		outputDir = filepath.Dir(spec.FilePath)
	}
	
	return filepath.Join(outputDir, handlerName)
}

// writeGeneratedFile writes the generated code to a file
func (g *Generator) writeGeneratedFile(path string, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}

// log logs a message if verbose mode is enabled
func (g *Generator) log(format string, args ...interface{}) {
	if g.config.Logger != nil {
		g.config.Logger.Printf(format, args...)
	}
}