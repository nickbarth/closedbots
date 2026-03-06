package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nickbarth/closedbots/internal/ai"
	"github.com/nickbarth/closedbots/internal/automation"
	"github.com/nickbarth/closedbots/internal/config"
	"github.com/nickbarth/closedbots/internal/osctrl"
	"github.com/nickbarth/closedbots/internal/runner"
	"github.com/nickbarth/closedbots/internal/scheduler"
	"github.com/nickbarth/closedbots/internal/store"
	"github.com/nickbarth/closedbots/internal/tasklog"
	"github.com/nickbarth/closedbots/internal/ui"
)

var runMain = run
var exitMain = os.Exit
var stderrMain io.Writer = os.Stderr
var getwdMain = os.Getwd

func main() {
	if err := runMain(); err != nil {
		fmt.Fprintf(stderrMain, "closedbots: %v\n", err)
		exitMain(1)
	}
}

func run() error {
	wd, err := getwdMain()
	if err != nil {
		return err
	}
	tasksDir := filepath.Join(wd, "tasks")
	runsDir := filepath.Join(wd, "runs")
	settingsPath := filepath.Join(wd, "config", "settings.json")

	taskStore := store.NewTaskStore(tasksDir)
	if err := taskStore.EnsureDir(); err != nil {
		return err
	}
	settingsStore := config.NewStore(settingsPath)

	settings, err := settingsStore.Load()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}

	registry := ai.NewRegistry()
	provider, err := registry.Build(settings.Runner, wd)
	if err != nil {
		settings.Runner = config.RunnerCodexCLI
		_ = settingsStore.Save(settings)
		provider, err = registry.Build(settings.Runner, wd)
		if err != nil {
			return fmt.Errorf("build provider: %w", err)
		}
	}

	driver := osctrl.NewDriver()

	runEngine := &runner.Runner{
		Provider: provider,
		Capturer: &automation.ScreenshotCapturer{},
		Executor: automation.NewExecutor(),
		Driver:   driver,
		RunsDir:  runsDir,
		SaveTask: taskStore.Save,
	}

	switchRunner := func(providerID string) (ai.Provider, error) {
		p, err := registry.Build(providerID, wd)
		if err != nil {
			return nil, err
		}
		runEngine.Provider = p
		return p, nil
	}

	taskLogPath := tasklog.ResolvePath(wd)
	tasklog.New(taskLogPath).Log(fmt.Sprintf("executor backend=%s", runEngine.Executor.BackendName()))
	app := ui.New(ui.Options{
		TaskStore:     taskStore,
		SettingsStore: settingsStore,
		Provider:      provider,
		SwitchRunner:  switchRunner,
		Settings:      settings,
		TaskLogPath:   taskLogPath,
		Driver:        driver,
	})

	manager := scheduler.NewManager(taskStore, runEngine, app.OnSchedulerEvent)
	app.SetManager(manager)

	hotkeyListener, hotkeyErr := driver.StartGlobalStopHotkey(settings.StopHotkey, func() {
		_ = manager.Stop("", "stopped by user")
	})
	app.SetHotkey(hotkeyListener, hotkeyErr)
	defer func() {
		if hotkeyListener != nil {
			hotkeyListener.Stop()
		}
	}()

	return app.Run(context.Background())
}
