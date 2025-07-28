// Command contextcheck runs the context propagation analyzer
package main

import (
	"github.com/yshengliao/gortex/internal/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer.ContextChecker)
}