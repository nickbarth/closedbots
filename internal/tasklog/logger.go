package tasklog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultLogFile = "task.log"

type Logger struct {
	path string
	mu   sync.Mutex
}

func ResolvePath(workDir string) string {
	name := defaultLogFile
	if filepath.IsAbs(name) {
		return name
	}
	base := strings.TrimSpace(workDir)
	if base == "" {
		wd, err := os.Getwd()
		if err == nil {
			base = wd
		}
	}
	if base == "" {
		return name
	}
	return filepath.Join(base, name)
}

func New(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Log(message string) {
	if l == nil || strings.TrimSpace(l.path) == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = f.WriteString(fmt.Sprintf("%s %s\n", ts, message))
}
