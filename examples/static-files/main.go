package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/middleware/static"
	"go.uber.org/zap"
)

type HandlersManager struct {
	// No handlers needed for static file example
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create public directory structure for demo
	publicDir := "public"
	createDemoFiles(publicDir)

	// Create configuration
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug"

	// Create empty handlers
	handlers := &HandlersManager{}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
	)
	if err != nil {
		logger.Fatal("Failed to create application", zap.Error(err))
	}

	// Add static file middleware
	e := application.Echo()
	
	// Basic static file serving
	e.Use(static.Static(publicDir))
	
	// Advanced configuration example (commented out)
	// e.Use(static.StaticWithConfig(static.Config{
	// 	Root:         publicDir,
	// 	Browse:       true,           // Enable directory browsing
	// 	HTML5:        true,           // Enable HTML5 mode (SPA support)
	// 	EnableCache:  true,           // Enable cache headers
	// 	CacheMaxAge:  3600,          // 1 hour cache
	// 	EnableETag:   true,           // Enable ETag generation
	// 	EnableGzip:   true,           // Serve .gz files if available
	// 	EnableBrotli: true,           // Serve .br files if available
	// }))

	logger.Info("Static file server example started", 
		zap.String("address", cfg.Server.Address),
		zap.String("root", publicDir))
	logger.Info("Try these URLs:",
		zap.String("home", "http://localhost:8080/"),
		zap.String("css", "http://localhost:8080/css/style.css"),
		zap.String("js", "http://localhost:8080/js/app.js"),
		zap.String("image", "http://localhost:8080/images/logo.png"))

	// Start server
	if err := application.Run(); err != nil {
		logger.Fatal("Server failed", zap.Error(err))
	}
}

func createDemoFiles(root string) {
	// Create directory structure
	dirs := []string{
		root,
		filepath.Join(root, "css"),
		filepath.Join(root, "js"),
		filepath.Join(root, "images"),
	}

	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}

	// Create demo files
	files := map[string]string{
		"index.html": `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Gortex Static Files Example</title>
    <link rel="stylesheet" href="/css/style.css">
</head>
<body>
    <div class="container">
        <h1>Welcome to Gortex</h1>
        <p>This is a static file server example.</p>
        <img src="/images/logo.png" alt="Logo" style="width: 100px;">
        <script src="/js/app.js"></script>
    </div>
</body>
</html>`,
		"css/style.css": `body {
    font-family: Arial, sans-serif;
    margin: 0;
    padding: 20px;
    background-color: #f5f5f5;
}

.container {
    max-width: 800px;
    margin: 0 auto;
    background: white;
    padding: 20px;
    border-radius: 8px;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

h1 {
    color: #333;
}`,
		"js/app.js": `console.log('Gortex static file server is working!');

// Example of fetching data
fetch('/api/status')
    .then(res => res.text())
    .catch(err => console.log('API not available in this example'));`,
		"images/logo.png": string([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}), // PNG header
	}

	for path, content := range files {
		fullPath := filepath.Join(root, path)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	// Create pre-compressed versions for demonstration
	os.WriteFile(filepath.Join(root, "css/style.css.gz"), []byte("GZIP_COMPRESSED_CSS"), 0644)
	os.WriteFile(filepath.Join(root, "js/app.js.br"), []byte("BROTLI_COMPRESSED_JS"), 0644)

	fmt.Println("Demo files created in", root)
}