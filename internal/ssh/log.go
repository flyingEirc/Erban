package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)
// FileLogger is a simple logger that appends logs to a local file (default: log.txt).
type FileLogger struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileLogger creates a file-based logger writing to the given path (default: log.txt).
func NewFileLogger(path string) (*FileLogger, error) {
	if strings.TrimSpace(path) == "" {
		path = "log.txt"
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileLogger{file: f}, nil
}

func (l *FileLogger) write(prefix, format string, args ...any) {
	if l == nil || l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf(format, args...)
	_, _ = l.file.WriteString(fmt.Sprintf("[%s] %s %s\n", ts, prefix, line))
	_ = l.file.Sync()
}

func (l *FileLogger) Infof(format string, args ...any)  { l.write("INFO", format, args...) }
func (l *FileLogger) Errorf(format string, args ...any) { l.write("ERROR", format, args...) }

// ---- Global logger ----
var (
	globalLog *FileLogger
	onceInit  sync.Once
)

// ensureGlobalLogger initializes and returns the package-level logger.
func ensureGlobalLogger() *FileLogger {
	onceInit.Do(func() {
		// Default to log.txt; if open fails, fallback to stderr
		if fl, err := NewFileLogger("log.txt"); err == nil {
			globalLog = fl
		} else {
			globalLog = &FileLogger{file: os.Stderr}
		}
	})
	if globalLog == nil {
		globalLog = &FileLogger{file: os.Stderr}
	}
	return globalLog
}

// SetLogFile switches the global logger to a specific file path.
// If path is empty, defaults to log.txt. Returns error if open fails.
func SetLogFile(path string) error {
	fl, err := NewFileLogger(path)
	if err != nil {
		return err
	}
	// best-effort close previous file if not stderr
	if globalLog != nil && globalLog.file != nil && globalLog.file != os.Stderr {
		_ = globalLog.file.Close()
	}
	globalLog = fl
	return nil
}

// Convenience helpers for global logging.
func LogInfof(format string, args ...any)  { ensureGlobalLogger().Infof(format, args...) }
func LogErrorf(format string, args ...any) { ensureGlobalLogger().Errorf(format, args...) }

// Generic write with custom prefix; used if callers want explicit level text.
func LogWrite(prefix, format string, args ...any) {
	p := strings.ToUpper(strings.TrimSpace(prefix))
	switch p {
	case "INFO":
		LogInfof(format, args...)
	case "ERROR":
		LogErrorf(format, args...)
	default:
		// Unknown level: include prefix in message under INFO
		LogInfof("[%s] %s", p, fmt.Sprintf(format, args...))
	}
}
