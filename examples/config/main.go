package main

import (
	"fmt"
	"log"

	"go.uber.org/zap"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/config"
)

func main() {
	// Current implementation using SimpleLoader
	// This will be replaced with Bofry/config in production
	loader := config.NewSimpleLoader().
		WithYAMLFile("config.yaml").
		WithEnvPrefix("STMP_")

	cfg := &config.Config{}
	if err := loader.Load(cfg); err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Print loaded configuration
	fmt.Printf("Server Address: %s\n", cfg.Server.Address)
	fmt.Printf("Logger Level: %s\n", cfg.Logger.Level)
	fmt.Printf("JWT Issuer: %s\n", cfg.JWT.Issuer)

	// Initialize logger based on config
	var logger *zap.Logger
	var err error
	
	if cfg.Logger.Level == "debug" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	// Create application with loaded config
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
	)
	if err != nil {
		log.Fatal("Failed to create app:", err)
	}

	logger.Info("Configuration loaded successfully",
		zap.String("server", cfg.Server.Address),
		zap.String("log_level", cfg.Logger.Level),
	)

	// Run the application
	if err := application.Run(); err != nil {
		logger.Fatal("Application failed", zap.Error(err))
	}
}

// Migration example: How to use with Bofry/config
// 
// When migrating to Bofry/config, replace the loader section with:
//
// import "github.com/Bofry/config"
//
// func main() {
//     cfg := &config.Config{}
//     
//     // Using Bofry/config
//     err := config.NewConfigurationService(cfg).
//         LoadDotEnv(".env").                    // Load from .env file
//         LoadEnvironmentVariables("STMP").      // Load env vars with STMP_ prefix
//         LoadYamlFile("config.yaml").           // Load from YAML
//         LoadCommandArguments()                 // Load from command line args
//     
//     if err != nil {
//         log.Fatal("Failed to load config:", err)
//     }
//     
//     // Rest of the code remains the same...
// }
//
// The Config struct is already compatible with Bofry/config tags!