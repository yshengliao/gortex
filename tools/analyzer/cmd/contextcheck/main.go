// Command contextcheck runs the context propagation analyzer
package main

import (
	"github.com/yshengliao/gortex/tools/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(analyzer.ContextChecker)
}