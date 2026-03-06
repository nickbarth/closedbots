package tasklog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePath(t *testing.T) {
	wd := t.TempDir()
	got := ResolvePath(wd)
	want := filepath.Join(wd, "task.log")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := ResolvePath(""); !strings.HasSuffix(got, "task.log") {
		t.Fatalf("blank ResolvePath=%q", got)
	}
}

func TestLoggerLogWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "task.log")
	l := New(path)
	l.Log("hello")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "hello") || !strings.Contains(s, "Z ") {
		t.Fatalf("unexpected log: %q", s)
	}
}

func TestLoggerNoPathAndNil(t *testing.T) {
	var l *Logger
	l.Log("x")
	New("   ").Log("x")
}

func TestLoggerMkdirErrorPath(t *testing.T) {
	// /dev/null is a file, so MkdirAll("/dev/null", ...) fails.
	New("/dev/null/task.log").Log("ignored")
}
