package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestInitProjectCreatesAllFiles(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd = &cobra.Command{Version: "v0.0.0"}

	err := initProject(tmpDir, "testapp", false)
	require.NoError(t, err)

	expected := []string{
		"go.mod",
		"cmd/server/main.go",
		"config/config.yaml",
		"handlers/manager.go",
		"handlers/health.go",
		"services/interfaces.go",
		"README.md",
		".gitignore",
	}

	for _, f := range expected {
		_, err := os.Stat(filepath.Join(tmpDir, f))
		require.NoError(t, err, f)
	}
}

func TestInitProjectWithExamples(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd = &cobra.Command{Version: "v0.0.0"}

	err := initProject(tmpDir, "testapp", true)
	require.NoError(t, err)

	expected := []string{
		"handlers/example.go",
		"handlers/health.go",
		"handlers/websocket.go",
		"services/data_service.go",
		"models/example.go",
	}

	for _, f := range expected {
		_, err := os.Stat(filepath.Join(tmpDir, f))
		require.NoError(t, err, f)
	}
}
