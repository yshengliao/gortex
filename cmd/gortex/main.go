package main

import (
	"fmt"
	"os"

	"github.com/yshengliao/gortex/cmd/gortex/commands"
)

var version = "v0.1.10"

func main() {
	if err := commands.Execute(version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}