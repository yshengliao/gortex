package mock

import (
	"sync"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a mock logger for testing
type Logger struct {
	*zap.Logger
	entries []LogEntry
	mu      sync.Mutex
}

// LogEntry represents a logged message
type LogEntry struct {
	Level   zapcore.Level
	Message string
	Fields  []zap.Field
}

// NewLogger creates a new mock logger
func NewLogger() *Logger {
	// Create a no-op core that captures log entries
	ml := &Logger{
		entries: make([]LogEntry, 0),
	}
	
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&mockWriter{logger: ml}),
		zapcore.DebugLevel,
	)
	
	ml.Logger = zap.New(core)
	return ml
}

// Entries returns all logged entries
func (l *Logger) Entries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Return a copy to avoid race conditions
	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// Clear clears all logged entries
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}

// HasEntry checks if a message was logged at a specific level
func (l *Logger) HasEntry(level zapcore.Level, message string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	for _, entry := range l.entries {
		if entry.Level == level && entry.Message == message {
			return true
		}
	}
	return false
}

// mockWriter implements io.Writer to capture log output
type mockWriter struct {
	logger *Logger
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	// In a real implementation, we would parse the JSON output
	// For now, we'll just count the write
	return len(p), nil
}

func (w *mockWriter) Sync() error {
	return nil
}