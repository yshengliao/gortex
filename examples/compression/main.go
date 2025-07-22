package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/yshengliao/gortex/app"
	"github.com/yshengliao/gortex/response"
	"go.uber.org/zap"
)

// HandlersManager defines application handlers
type HandlersManager struct {
	API   *APIHandlers   `url:"/api"`
	Files *FileHandlers `url:"/files"`
}

// APIHandlers provides API endpoints
type APIHandlers struct {
	Logger *zap.Logger
}

func (h *APIHandlers) GET(c echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{
		"message": "Compression Example API",
		"version": "1.0.0",
	})
}

func (h *APIHandlers) Large(c echo.Context) error {
	// Generate large JSON response to demonstrate compression
	data := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		data[i] = map[string]interface{}{
			"id":          i,
			"name":        fmt.Sprintf("Item %d", i),
			"description": "This is a long description that will be repeated many times to create a large response suitable for compression testing.",
			"timestamp":   time.Now().Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
			"metadata": map[string]interface{}{
				"category": "test",
				"tags":     []string{"compression", "example", "gortex"},
				"nested": map[string]interface{}{
					"level1": map[string]interface{}{
						"level2": map[string]interface{}{
							"value": "deep nested value for compression",
						},
					},
				},
			},
		}
	}
	
	h.Logger.Info("Serving large JSON response", 
		zap.Int("items", len(data)),
		zap.String("accept_encoding", c.Request().Header.Get("Accept-Encoding")),
	)
	
	return response.Success(c, http.StatusOK, data)
}

func (h *APIHandlers) Text(c echo.Context) error {
	// Generate large text response
	text := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 500)
	
	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	return c.String(http.StatusOK, text)
}

// FileHandlers demonstrates compression for different content types
type FileHandlers struct {
	Logger *zap.Logger
}

func (h *FileHandlers) CSS(c echo.Context) error {
	// Simulate a large CSS file
	css := `
/* Large CSS file for compression testing */
body { margin: 0; padding: 0; font-family: Arial, sans-serif; }
.container { max-width: 1200px; margin: 0 auto; padding: 20px; }
.header { background-color: #333; color: white; padding: 20px; }
.nav { display: flex; justify-content: space-between; align-items: center; }
.nav-item { margin: 0 10px; text-decoration: none; color: white; }
.content { padding: 40px 0; }
.footer { background-color: #f0f0f0; padding: 20px; text-align: center; }
`
	// Repeat to make it larger
	css = strings.Repeat(css, 100)
	
	c.Response().Header().Set("Content-Type", "text/css; charset=utf-8")
	return c.String(http.StatusOK, css)
}

func (h *FileHandlers) JavaScript(c echo.Context) error {
	// Simulate a large JavaScript file
	js := `
// Large JavaScript file for compression testing
(function() {
    'use strict';
    
    function initializeApp() {
        console.log('Application initialized');
        
        const elements = document.querySelectorAll('.clickable');
        elements.forEach(element => {
            element.addEventListener('click', function(e) {
                e.preventDefault();
                console.log('Element clicked:', this.textContent);
            });
        });
    }
    
    document.addEventListener('DOMContentLoaded', initializeApp);
})();
`
	// Repeat to make it larger
	js = strings.Repeat(js, 100)
	
	c.Response().Header().Set("Content-Type", "application/javascript; charset=utf-8")
	return c.String(http.StatusOK, js)
}

func (h *FileHandlers) XML(c echo.Context) error {
	// Generate large XML response
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<root>
    <items>`
	
	for i := 0; i < 500; i++ {
		xml += fmt.Sprintf(`
        <item id="%d">
            <name>Item %d</name>
            <description>This is a description for item %d that will be compressed</description>
            <metadata>
                <created>%s</created>
                <updated>%s</updated>
            </metadata>
        </item>`, i, i, i, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
	}
	
	xml += `
    </items>
</root>`
	
	c.Response().Header().Set("Content-Type", "application/xml; charset=utf-8")
	return c.String(http.StatusOK, xml)
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create configuration with advanced compression settings
	cfg := &app.Config{}
	cfg.Server.Address = ":8080"
	cfg.Logger.Level = "debug" // Enable development mode for monitoring
	
	// Configure advanced compression
	cfg.Server.Compression.Enabled = true
	cfg.Server.Compression.Level = "default" // Options: default, speed, best
	cfg.Server.Compression.MinSize = 512     // Compress responses larger than 512 bytes
	cfg.Server.Compression.EnableBrotli = true // Enable Brotli compression
	cfg.Server.Compression.PreferBrotli = true // Prefer Brotli over gzip when both are supported
	cfg.Server.Compression.ContentTypes = []string{
		"text/html",
		"text/css",
		"text/plain",
		"text/javascript",
		"application/javascript",
		"application/json",
		"application/xml",
		"application/rss+xml",
		"image/svg+xml",
	}

	// Create handlers
	handlers := &HandlersManager{
		API: &APIHandlers{
			Logger: logger,
		},
		Files: &FileHandlers{
			Logger: logger,
		},
	}

	// Create application
	application, err := app.NewApp(
		app.WithConfig(cfg),
		app.WithLogger(logger),
		app.WithHandlers(handlers),
		app.WithDevelopmentMode(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start server
	go func() {
		logger.Info("Starting compression example server", 
			zap.String("address", cfg.Server.Address),
		)
		
		fmt.Println("\n=== COMPRESSION MIDDLEWARE EXAMPLE ===")
		fmt.Println("\nConfiguration:")
		fmt.Printf("  Compression Enabled: %v\n", cfg.Server.Compression.Enabled)
		fmt.Printf("  Compression Level: %s\n", cfg.Server.Compression.Level)
		fmt.Printf("  Minimum Size: %d bytes\n", cfg.Server.Compression.MinSize)
		fmt.Printf("  Brotli Enabled: %v\n", cfg.Server.Compression.EnableBrotli)
		fmt.Printf("  Prefer Brotli: %v\n", cfg.Server.Compression.PreferBrotli)
		
		fmt.Println("\nEndpoints:")
		fmt.Println("  GET /api - API root")
		fmt.Println("  POST /api/large - Large JSON response (1000 items)")
		fmt.Println("  POST /api/text - Large text response")
		fmt.Println("  POST /files/css - Large CSS file")
		fmt.Println("  POST /files/javascript - Large JavaScript file")
		fmt.Println("  POST /files/xml - Large XML response")
		
		fmt.Println("\nMonitoring:")
		fmt.Println("  GET /_monitor - View compression status")
		
		fmt.Println("\nTest Commands:")
		fmt.Println("  # Test gzip compression")
		fmt.Println("  curl -H 'Accept-Encoding: gzip' http://localhost:8080/api/large -X POST -s -o /dev/null -w '%{size_download} bytes (compressed)\\n'")
		fmt.Println()
		fmt.Println("  # Test brotli compression")
		fmt.Println("  curl -H 'Accept-Encoding: br' http://localhost:8080/api/large -X POST -s -o /dev/null -w '%{size_download} bytes (compressed)\\n'")
		fmt.Println()
		fmt.Println("  # Test without compression")
		fmt.Println("  curl http://localhost:8080/api/large -X POST -s -o /dev/null -w '%{size_download} bytes (uncompressed)\\n'")
		fmt.Println()
		fmt.Println("  # Compare compression ratios")
		fmt.Println("  echo '=== Compression Test ==='")
		fmt.Println("  echo -n 'Uncompressed: '; curl http://localhost:8080/api/large -X POST -s -o /dev/null -w '%{size_download} bytes\\n'")
		fmt.Println("  echo -n 'Gzip: '; curl -H 'Accept-Encoding: gzip' http://localhost:8080/api/large -X POST -s -o /dev/null -w '%{size_download} bytes\\n'")
		fmt.Println("  echo -n 'Brotli: '; curl -H 'Accept-Encoding: br' http://localhost:8080/api/large -X POST -s -o /dev/null -w '%{size_download} bytes\\n'")
		fmt.Println()
		fmt.Println("  # View compression configuration")
		fmt.Println("  curl http://localhost:8080/_monitor | jq .compression")
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop...")
		fmt.Println()
		
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	logger.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}