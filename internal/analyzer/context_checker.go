// Package analyzer provides static analysis tools for the Gortex framework
package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// ContextChecker is an analyzer that checks for proper context.Context propagation
var ContextChecker = &analysis.Analyzer{
	Name:     "contextcheck",
	Doc:      "check that context.Context is properly propagated and used",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runContextCheck,
}

// Issue represents a context propagation issue
type Issue struct {
	Pos     token.Pos
	Message string
}

func runContextCheck(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	
	// Node types we're interested in
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.CallExpr)(nil),
	}
	
	// Track functions that accept context
	contextFuncs := make(map[string]bool)
	
	// First pass: identify functions that accept context.Context
	inspect.Preorder(nodeFilter[:1], func(n ast.Node) {
		funcDecl := n.(*ast.FuncDecl)
		if hasContextParam(funcDecl) {
			contextFuncs[funcDecl.Name.Name] = true
		}
	})
	
	// Second pass: check function calls
	inspect.Preorder(nodeFilter[1:], func(n ast.Node) {
		call := n.(*ast.CallExpr)
		checkContextPropagation(pass, call, contextFuncs)
	})
	
	// Check for long-running operations without context checks
	inspect.Preorder([]ast.Node{
		(*ast.ForStmt)(nil),
		(*ast.RangeStmt)(nil),
	}, func(n ast.Node) {
		checkLongRunningOps(pass, n)
	})
	
	return nil, nil
}

// hasContextParam checks if a function has a context.Context parameter
func hasContextParam(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Params == nil {
		return false
	}
	
	for _, param := range funcDecl.Type.Params.List {
		if isContextType(param.Type) {
			return true
		}
	}
	return false
}

// isContextType checks if a type is context.Context
func isContextType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "context" && t.Sel.Name == "Context"
		}
	case *ast.Ident:
		// Could be an alias or embedded type
		return t.Name == "Context" || strings.Contains(t.Name, "Context")
	}
	return false
}

// checkContextPropagation checks if context is properly passed to function calls
func checkContextPropagation(pass *analysis.Pass, call *ast.CallExpr, contextFuncs map[string]bool) {
	// Get the function being called
	var funcName string
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		funcName = fun.Name
	case *ast.SelectorExpr:
		funcName = fun.Sel.Name
	default:
		return
	}
	
	// Check if this function expects context
	if !contextFuncs[funcName] && !isKnownContextFunc(funcName) {
		return
	}
	
	// Check if context is passed
	hasContext := false
	for _, arg := range call.Args {
		if isContextExpr(arg) {
			hasContext = true
			break
		}
	}
	
	if !hasContext {
		pass.Reportf(call.Pos(), "function %s expects context.Context but none was passed", funcName)
	}
}

// isContextExpr checks if an expression is a context value
func isContextExpr(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == "ctx" || strings.Contains(e.Name, "context") || strings.Contains(e.Name, "Context")
	case *ast.CallExpr:
		// Check for context.Background(), context.TODO(), etc.
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "context" {
				return true
			}
		}
	}
	return false
}

// isKnownContextFunc checks if a function is known to accept context
func isKnownContextFunc(name string) bool {
	contextFuncs := []string{
		"WithContext", "NewRequestWithContext", "ExecContext", "QueryContext",
		"QueryRowContext", "GetContext", "PostContext", "DoWithContext",
	}
	
	for _, cf := range contextFuncs {
		if strings.Contains(name, cf) {
			return true
		}
	}
	return false
}

// checkLongRunningOps checks for long-running operations without context checks
func checkLongRunningOps(pass *analysis.Pass, n ast.Node) {
	// Check if this loop/range is inside a function with context parameter
	var hasContextParam bool
	var contextParamName string
	
	// Find the enclosing function by traversing the AST
	var enclosingFunc *ast.FuncDecl
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if fn, ok := node.(*ast.FuncDecl); ok {
				// Check if this function contains our node
				if fn.Pos() <= n.Pos() && n.End() <= fn.End() {
					enclosingFunc = fn
					return false
				}
			}
			return true
		})
		if enclosingFunc != nil {
			break
		}
	}
	
	if enclosingFunc != nil && enclosingFunc.Type.Params != nil {
		for _, param := range enclosingFunc.Type.Params.List {
			if isContextType(param.Type) && len(param.Names) > 0 {
				hasContextParam = true
				contextParamName = param.Names[0].Name
				break
			}
		}
	}
	
	if !hasContextParam {
		return
	}
	
	// Check if the loop body contains context.Done() check
	hasContextCheck := false
	ast.Inspect(n, func(node ast.Node) bool {
		if hasContextCheck {
			return false
		}
		
		// Look for select statements with ctx.Done()
		if sel, ok := node.(*ast.SelectStmt); ok {
			hasContextCheck = checkSelectForContextDone(sel, contextParamName)
			return false
		}
		
		// Look for if statements checking ctx.Err()
		if ifStmt, ok := node.(*ast.IfStmt); ok {
			hasContextCheck = checkIfForContextErr(ifStmt, contextParamName)
		}
		
		return true
	})
	
	if !hasContextCheck {
		var loopType string
		switch n.(type) {
		case *ast.ForStmt:
			loopType = "for loop"
		case *ast.RangeStmt:
			loopType = "range loop"
		}
		
		pass.Reportf(n.Pos(), "%s in function with context parameter should check for context cancellation", loopType)
	}
}

// checkSelectForContextDone checks if a select statement has ctx.Done() case
func checkSelectForContextDone(sel *ast.SelectStmt, ctxName string) bool {
	for _, clause := range sel.Body.List {
		commClause, ok := clause.(*ast.CommClause)
		if !ok {
			continue
		}
		
		// Check for case <-ctx.Done():
		if commClause.Comm != nil {
			switch comm := commClause.Comm.(type) {
			case *ast.ExprStmt:
				if unary, ok := comm.X.(*ast.UnaryExpr); ok && unary.Op == token.ARROW {
					if call, ok := unary.X.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == ctxName && sel.Sel.Name == "Done" {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// checkIfForContextErr checks if an if statement checks ctx.Err()
func checkIfForContextErr(ifStmt *ast.IfStmt, ctxName string) bool {
	// Look for patterns like: if ctx.Err() != nil
	if binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr); ok && binExpr.Op == token.NEQ {
		if call, ok := binExpr.X.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == ctxName && sel.Sel.Name == "Err" {
					return true
				}
			}
		}
	}
	return false
}