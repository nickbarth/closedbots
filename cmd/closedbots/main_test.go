package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainSuccessDoesNotExit(t *testing.T) {
	prevRun := runMain
	prevExit := exitMain
	prevErr := stderrMain
	defer func() {
		runMain = prevRun
		exitMain = prevExit
		stderrMain = prevErr
	}()

	exitCode := -1
	runMain = func() error { return nil }
	exitMain = func(code int) { exitCode = code }
	stderrMain = &bytes.Buffer{}

	main()
	if exitCode != -1 {
		t.Fatalf("unexpected exit code %d", exitCode)
	}
}

func TestMainFailureWritesErrorAndExits(t *testing.T) {
	prevRun := runMain
	prevExit := exitMain
	prevErr := stderrMain
	defer func() {
		runMain = prevRun
		exitMain = prevExit
		stderrMain = prevErr
	}()

	var buf bytes.Buffer
	exitCode := -1
	runMain = func() error { return errors.New("boom") }
	exitMain = func(code int) { exitCode = code }
	stderrMain = &buf

	main()
	if exitCode != 1 {
		t.Fatalf("exit code=%d", exitCode)
	}
	if !strings.Contains(buf.String(), "closedbots: boom") {
		t.Fatalf("stderr=%q", buf.String())
	}
}

func TestRunGetwdError(t *testing.T) {
	prev := getwdMain
	defer func() { getwdMain = prev }()
	getwdMain = func() (string, error) { return "", errors.New("wd") }
	if err := run(); err == nil || !strings.Contains(err.Error(), "wd") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunEnsureTasksDirError(t *testing.T) {
	wd := t.TempDir()
	if err := os.WriteFile(filepath.Join(wd, "tasks"), []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write tasks file: %v", err)
	}

	prev := getwdMain
	defer func() { getwdMain = prev }()
	getwdMain = func() (string, error) { return wd, nil }

	err := run()
	if err == nil {
		t.Fatalf("expected tasks dir error")
	}
}

func TestRunLoadSettingsError(t *testing.T) {
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wd, "config", "settings.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	prev := getwdMain
	defer func() { getwdMain = prev }()
	getwdMain = func() (string, error) { return wd, nil }

	err := run()
	if err == nil || !strings.Contains(err.Error(), "load settings") {
		t.Fatalf("expected load settings error, got %v", err)
	}
}
