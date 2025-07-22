package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestInitProject_GoModVersion(t *testing.T) {
	tmpDir := t.TempDir()

	rootCmd = &cobra.Command{Version: "v0.1.10"}

	err := initProject(tmpDir, "testapp", false)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	require.NoError(t, err)

	require.Contains(t, string(data), "github.com/yshengliao/gortex v0.1.10")
}
