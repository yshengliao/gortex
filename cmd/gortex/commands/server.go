package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// execCommand is used to execute external commands. It is defined as a
// variable so tests can replace it with a mock implementation.
var execCommand = exec.Command

func runDevServer(configPath, port string) error {
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Watch directories
	watchDirs := []string{".", "handlers", "services", "models", "config"}
	for _, dir := range watchDirs {
		if err := addWatchDir(watcher, dir); err != nil {
			fmt.Printf("Warning: failed to watch directory %s: %v\n", dir, err)
		}
	}

	// Server management
	var (
		cmd       *exec.Cmd
		cmdMutex  sync.Mutex
		lastBuild time.Time
	)

	startServer := func() {
		cmdMutex.Lock()
		defer cmdMutex.Unlock()

		if cmd != nil && cmd.Process != nil {
			fmt.Println("🔄 Stopping server...")
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}

		// Build
		fmt.Println("🔨 Building...")
		buildCmd := execCommand("go", "build", "-o", ".gortex-dev-server", "./cmd/server")
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			fmt.Printf("❌ Build failed: %v\n", err)
			return
		}

		// Run
		fmt.Printf("🚀 Starting server on port %s...\n", port)
		cmd = execCommand("./.gortex-dev-server")
		cmd.Env = append(os.Environ(), fmt.Sprintf("GORTEX_SERVER_ADDRESS=:%s", port))
		if configPath != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("GORTEX_CONFIG_PATH=%s", configPath))
		}

		// Pipe output
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)

		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed to start server: %v\n", err)
			return
		}

		lastBuild = time.Now()
	}

	// Initial start
	startServer()

	// File change debouncer
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case <-sigChan:
			fmt.Println("\n👋 Shutting down...")
			cmdMutex.Lock()
			if cmd != nil && cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGTERM)
				cmd.Wait()
			}
			cmdMutex.Unlock()
			os.Remove(".gortex-dev-server")
			return nil

		case event := <-watcher.Events:
			// Skip non-Go files and temporary files
			if !strings.HasSuffix(event.Name, ".go") ||
				strings.Contains(event.Name, ".gortex-dev-server") ||
				strings.HasPrefix(filepath.Base(event.Name), ".") {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				fmt.Printf("📝 File changed: %s\n", event.Name)

				// Cancel previous timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				// Set new timer
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					if time.Since(lastBuild) > debounceDuration {
						startServer()
					}
				})
			}

		case err := <-watcher.Errors:
			fmt.Printf("❌ Watcher error: %v\n", err)
		}
	}
}

func addWatchDir(watcher *fsnotify.Watcher, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and vendor
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor") {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return watcher.Add(path)
		}

		return nil
	})
}
