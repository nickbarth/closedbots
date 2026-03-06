package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultAndNormalizeAndValidate(t *testing.T) {
	def := DefaultSettings()
	if def.Runner != RunnerCodexCLI {
		t.Fatalf("default runner = %q", def.Runner)
	}
	n := normalize(Settings{Runner: "openai", StopHotkey: ""})
	if n.Runner != RunnerCodexCLI {
		t.Fatalf("normalized runner = %q", n.Runner)
	}
	if n.StopHotkey != DefaultStopHotkey {
		t.Fatalf("normalized hotkey = %q", n.StopHotkey)
	}
	if err := ValidateSettings(Settings{Runner: "bad", StopHotkey: "Ctrl+X"}); err != ErrInvalidRunner {
		t.Fatalf("expected ErrInvalidRunner, got %v", err)
	}
	if err := ValidateSettings(Settings{Runner: RunnerCodexCLI, StopHotkey: " "}); err != ErrInvalidHotkey {
		t.Fatalf("expected ErrInvalidHotkey, got %v", err)
	}
	if err := ValidateSettings(Settings{Runner: RunnerClaudeCLI, StopHotkey: "Ctrl+Shift+S"}); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestStoreLoadMissingReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := NewStore(path)
	got, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Runner != RunnerCodexCLI {
		t.Fatalf("runner = %q", got.Runner)
	}
}

func TestStoreSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "settings.json")
	s := NewStore(path)
	in := Settings{Runner: RunnerOllamaCLI, StopHotkey: "Ctrl+K"}
	if err := s.Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Runner != in.Runner || got.StopHotkey != in.StopHotkey {
		t.Fatalf("got %+v, want %+v", got, in)
	}
}

func TestStoreLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s := NewStore(path)
	if _, err := s.Load(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestStoreLoadInvalidSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	body := `{"schema_version":1,"runner":"bad","stop_hotkey":"Ctrl+Shift+S"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s := NewStore(path)
	if _, err := s.Load(); err != ErrInvalidRunner {
		t.Fatalf("expected ErrInvalidRunner, got %v", err)
	}
}

func TestStoreSaveValidationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := NewStore(path)
	err := s.Save(Settings{Runner: "bad", StopHotkey: "Ctrl+X"})
	if err != ErrInvalidRunner {
		t.Fatalf("expected ErrInvalidRunner, got %v", err)
	}
}

func TestWriteAtomicErrorsWhenDirMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "file.json")
	err := writeAtomic(path, []byte("{}"), 0o644)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
