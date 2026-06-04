// Command contextcheck runs the context propagation analyzer
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/yshengliao/gortex/tools/analyzer"
)

func main() {
	singlechecker.Main(analyzer.ContextChecker)
}
