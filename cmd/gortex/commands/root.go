package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func Execute(version string) error {
	rootCmd = &cobra.Command{
		Use:   "gortex",
		Short: "Gortex - High-Performance Go Web Framework CLI",
		Long: `Gortex (Go + Vortex) is a high-performance web framework that creates 
a powerful vortex of connectivity between HTTP and WebSocket protocols.

This CLI tool helps you quickly scaffold new projects and generate code.`,
		Version: version,
	}

	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newServerCmd())

	return rootCmd.Execute()
}

func newInitCmd() *cobra.Command {
	var projectName string
	var withExamples bool

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new Gortex project",
		Long:  "Initialize a new Gortex project with basic structure and configuration",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			if projectName == "" {
				projectName = "myapp"
			}

			fmt.Printf("Initializing new Gortex project '%s' in %s...\n", projectName, path)
			return initProject(path, projectName, withExamples)
		},
	}

	cmd.Flags().StringVarP(&projectName, "name", "n", "", "project name")
	cmd.Flags().BoolVarP(&withExamples, "with-examples", "e", false, "include example code")

	return cmd
}

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate code from templates",
		Long:  "Generate handlers, services, and other components",
	}

	cmd.AddCommand(newGenerateHandlerCmd())
	cmd.AddCommand(newGenerateServiceCmd())
	cmd.AddCommand(newGenerateModelCmd())

	return cmd
}

func newServerCmd() *cobra.Command {
	var configPath string
	var port string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run development server",
		Long:  "Run a development server with hot reload",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Starting development server on port %s...\n", port)
			fmt.Println("Watching for file changes...")
			return runDevServer(configPath, port)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "config file path")
	cmd.Flags().StringVarP(&port, "port", "p", "8080", "server port")

	return cmd
}