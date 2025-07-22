package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	execMu    sync.Mutex
	execCalls [][]string
)

func fakeExecCommand(name string, args ...string) *exec.Cmd {
	execMu.Lock()
	execCalls = append(execCalls, append([]string{name}, args...))
	execMu.Unlock()

	cs := []string{"-test.run=TestHelperProcess", "--", name}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestRunDevServer(t *testing.T) {
	tmpDir := t.TempDir()
	dirs := []string{"handlers", "services", "models", "config", "cmd/server"}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, d), 0755))
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(cwd)

	execMu.Lock()
	execCalls = nil
	execMu.Unlock()
	execCommand = fakeExecCommand
	defer func() { execCommand = exec.Command }()

	done := make(chan error)
	go func() {
		done <- runDevServer("", "1234")
	}()

	time.Sleep(100 * time.Millisecond)
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)

	err = <-done
	require.NoError(t, err)

	execMu.Lock()
	calls := append([][]string(nil), execCalls...)
	execMu.Unlock()

	require.Len(t, calls, 2)
	require.Equal(t, []string{"go", "build", "-o", ".gortex-dev-server", "./cmd/server"}, calls[0])
	require.Equal(t, []string{"./.gortex-dev-server"}, calls[1])
}
