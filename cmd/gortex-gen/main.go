package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yshengliao/gortex/internal/codegen"
)

const version = "0.1.0"

func main() {
	var (
		inputDir    = flag.String("input", ".", "Input directory to scan for Go files")
		outputDir   = flag.String("output", ".", "Output directory for generated files")
		packageName = flag.String("package", "", "Package name for generated files (defaults to input package)")
		recursive   = flag.Bool("recursive", true, "Recursively scan subdirectories")
		verbose     = flag.Bool("verbose", false, "Enable verbose logging")
		versionFlag = flag.Bool("version", false, "Print version and exit")
		dryRun      = flag.Bool("dry-run", false, "Show what would be generated without writing files")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGortex code generator for automatic HTTP handler generation.\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -input ./services -output ./handlers\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -input ./internal/services -package handlers -verbose\n", os.Args[0])
	}

	flag.Parse()

	if *versionFlag {
		fmt.Printf("gortex-gen version %s\n", version)
		os.Exit(0)
	}

	// Configure logger
	logger := log.New(os.Stdout, "[gortex-gen] ", log.LstdFlags)
	if !*verbose {
		logger.SetOutput(nil)
	}

	// Create generator config
	config := &codegen.Config{
		InputDir:    *inputDir,
		OutputDir:   *outputDir,
		PackageName: *packageName,
		Recursive:   *recursive,
		DryRun:      *dryRun,
		Logger:      logger,
	}

	// Create and run generator
	generator := codegen.NewGenerator(config)
	if err := generator.Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		fmt.Println("Dry run completed. No files were written.")
	} else {
		fmt.Println("Code generation completed successfully.")
	}
}