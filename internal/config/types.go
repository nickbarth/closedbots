package config

import "strings"

const SchemaVersionSettings = 1

const (
	RunnerCodexCLI  = "codex_cli"
	RunnerClaudeCLI = "claude_cli"
	RunnerOllamaCLI = "ollama_cli"
)

const legacyRunnerOpenAI = "openai"

const DefaultStopHotkey = "Ctrl+Shift+S"

type Settings struct {
	SchemaVersion int    `json:"schema_version"`
	Runner        string `json:"runner"`
	StopHotkey    string `json:"stop_hotkey"`
}

func DefaultSettings() Settings {
	return Settings{
		SchemaVersion: SchemaVersionSettings,
		Runner:        RunnerCodexCLI,
		StopHotkey:    DefaultStopHotkey,
	}
}

func normalize(in Settings) Settings {
	out := in
	if out.SchemaVersion == 0 {
		out.SchemaVersion = SchemaVersionSettings
	}
	switch strings.ToLower(strings.TrimSpace(out.Runner)) {
	case "", legacyRunnerOpenAI:
		out.Runner = RunnerCodexCLI
	}
	if strings.TrimSpace(out.StopHotkey) == "" {
		out.StopHotkey = DefaultStopHotkey
	}
	return out
}

func ValidateSettings(s Settings) error {
	switch strings.TrimSpace(s.Runner) {
	case RunnerCodexCLI, RunnerClaudeCLI, RunnerOllamaCLI:
	default:
		return ErrInvalidRunner
	}
	if strings.TrimSpace(s.StopHotkey) == "" {
		return ErrInvalidHotkey
	}
	return nil
}
