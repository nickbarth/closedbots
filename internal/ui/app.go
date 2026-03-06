package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nickbarth/closedbots/internal/ai"
	"github.com/nickbarth/closedbots/internal/config"
	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/osctrl"
	"github.com/nickbarth/closedbots/internal/scheduler"
	"github.com/nickbarth/closedbots/internal/store"
	"github.com/nickbarth/closedbots/internal/tasklog"
	tk "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure"
)

const (
	eventListboxSelect  = "<<ListboxSelect>>"
	eventComboboxSelect = "<<ComboboxSelected>>"
	progressLabelWrapPx = 420

	defaultMinimizeSettleDelay = 1200 * time.Millisecond

	generateButtonDefaultLabel = "Generate Steps"
	generateSpinnerInterval    = 120 * time.Millisecond
)

var generateSpinnerFrames = []string{"|", "/", "-", `\`}

type Options struct {
	TaskStore     *store.TaskStore
	SettingsStore *config.Store
	Provider      ai.Provider
	SwitchRunner  func(string) (ai.Provider, error)
	Settings      config.Settings
	TaskLogPath   string
	Driver        osctrl.Driver
	HotkeyErr     error
	Hotkey        osctrl.HotkeyHandle
}

type App struct {
	taskStore     *store.TaskStore
	settingsStore *config.Store
	provider      ai.Provider
	switchRunner  func(string) (ai.Provider, error)
	driver        osctrl.Driver
	settings      config.Settings
	taskLogPath   string
	taskLogger    *tasklog.Logger
	manager       *scheduler.Manager
	hotkey        osctrl.HotkeyHandle
	hotkeyErr     error

	tasks        []*domain.Task
	current      *domain.Task
	selectedStep int

	running         bool
	generatingSteps bool
	recurring       bool
	activeTaskID    string

	lastFailureShot  string
	generateSpinStop chan struct{}

	taskList           *tk.ListboxWidget
	notebook           *tk.TNotebookWidget
	editorTab          *tk.TFrameWidget
	currentStepLabel   *tk.TLabelWidget
	stepProgress       *tk.TProgressbarWidget
	stepProgressCounts *tk.TLabelWidget

	summaryEntry *tk.TEntryWidget
	stepText     *tk.TextWidget
	intervalBox  *tk.TComboboxWidget
	runnerBox    *tk.TComboboxWidget

	runBtn      *tk.TButtonWidget
	saveBtn     *tk.TButtonWidget
	newBtn      *tk.TButtonWidget
	selectBtn   *tk.TButtonWidget
	deleteBtn   *tk.TButtonWidget
	importBtn   *tk.TButtonWidget
	exportBtn   *tk.TButtonWidget
	generateBtn *tk.TButtonWidget

	countdownLabel *tk.TLabelWidget
}

func newDraftTask() *domain.Task {
	return &domain.Task{
		Summary: "",
		Steps:   []domain.Step{},
	}
}

func applyStepTextToTask(p *domain.Task, stepText string) {
	if p == nil {
		return
	}
	instructions := domain.ParsePointFormSteps(stepText)
	domain.SetStepsFromInstructions(p, instructions)
}

func stepsTextFromTask(p *domain.Task) string {
	if p == nil {
		return ""
	}
	return domain.FormatPointFormSteps(p.Steps)
}

func minimizeSettleDelay() time.Duration {
	return defaultMinimizeSettleDelay
}

func buildGenerateButtonSpinnerLabel(tick int) string {
	if len(generateSpinnerFrames) == 0 {
		return "Generating Steps..."
	}
	if tick < 0 {
		tick = 0
	}
	frame := generateSpinnerFrames[tick%len(generateSpinnerFrames)]
	return "Generating Steps... " + frame
}

func setGeneratedSteps(p *domain.Task, drafts []domain.StepDraft) {
	if p == nil {
		return
	}
	instructions := make([]string, 0, len(drafts))
	for _, d := range drafts {
		instruction := strings.TrimSpace(d.Instruction)
		if instruction == "" {
			continue
		}
		instructions = append(instructions, instruction)
	}
	domain.SetStepsFromInstructions(p, instructions)
}

func New(opts Options) *App {
	logPath := strings.TrimSpace(opts.TaskLogPath)
	if logPath == "" {
		logPath = tasklog.ResolvePath("")
	}
	return &App{
		taskStore:     opts.TaskStore,
		settingsStore: opts.SettingsStore,
		provider:      opts.Provider,
		switchRunner:  opts.SwitchRunner,
		driver:        opts.Driver,
		settings:      opts.Settings,
		taskLogPath:   logPath,
		taskLogger:    tasklog.New(logPath),
		hotkeyErr:     opts.HotkeyErr,
		hotkey:        opts.Hotkey,
		selectedStep:  -1,
		activeTaskID:  "",
	}
}

func (a *App) SetManager(m *scheduler.Manager) {
	a.manager = m
}

func (a *App) SetHotkey(l osctrl.HotkeyHandle, err error) {
	a.hotkey = l
	a.hotkeyErr = err
}

func (a *App) OnSchedulerEvent(e scheduler.Event) {
	tk.PostEvent(func() {
		a.handleSchedulerEvent(e)
	}, false)
}

func (a *App) Run(ctx context.Context) (err error) {
	_ = ctx
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ui initialization failed: %v (ensure a desktop display/session is available)", r)
		}
	}()

	if a.taskStore == nil || a.settingsStore == nil || a.provider == nil || a.driver == nil {
		return fmt.Errorf("ui dependencies are not fully configured")
	}
	if a.manager == nil {
		return fmt.Errorf("scheduler manager is required")
	}

	a.buildUI()
	if err := a.reloadTasks(""); err != nil {
		a.showFriendlyError("Load Failed", "We couldn't load your tasks.", err)
	}
	if a.hotkeyErr != nil {
		a.logError("global stop hotkey unavailable", a.hotkeyErr)
	}
	a.updateRunUIState()
	a.resetRunIntervalToDefault()

	tk.App.Wait()
	return nil
}

func (a *App) buildUI() {
	tk.ActivateTheme("azure light")
	tk.App.WmTitle(osctrl.AppWindowTitle())
	tk.App.SetResizable(true, true)
	tk.WmProtocol(tk.App, "WM_DELETE_WINDOW", tk.Command(func() {
		if a.running {
			_ = a.manager.Stop("", "stopped by user")
		}
		a.stopHotkey()
		tk.Destroy(tk.App)
	}))

	root := tk.TFrame()
	tk.Pack(root, tk.Fill("both"), tk.Expand(true), tk.Padx("1m"), tk.Pady("1m"))

	header := root.TFrame()
	tk.Pack(header, tk.Fill("x"), tk.Pady("1m"))
	tk.Pack(header.TLabel(tk.Txt(osctrl.AppWindowTitle()+" - Task Automation"), tk.Font("{Helvetica} 14 bold")), tk.Side("left"))

	runnerFrame := header.TFrame()
	tk.Pack(runnerFrame, tk.Side("right"))
	tk.Pack(runnerFrame.TLabel(tk.Txt("Runner")), tk.Side("left"), tk.Padx("0 0.5m"))
	a.runnerBox = runnerFrame.TCombobox(tk.State("readonly"), tk.Width(16), tk.Values(runnerValues()))
	a.runnerBox.Current(runnerIndex(a.settings.Runner))
	tk.Pack(a.runnerBox, tk.Side("left"))
	tk.Bind(a.runnerBox, eventComboboxSelect, tk.Command(a.onRunnerChanged))

	a.notebook = root.TNotebook()
	tk.Pack(a.notebook, tk.Fill("both"), tk.Expand(true), tk.Pady("1m"))

	tasksTab := a.notebook.TFrame(tk.Padding(8))
	a.editorTab = a.notebook.TFrame(tk.Padding(8))
	a.notebook.Add(tasksTab, tk.Txt("Bots"))
	a.notebook.Add(a.editorTab, tk.Txt("Task"))

	taskFrame := tasksTab.TFrame()
	tk.Pack(taskFrame, tk.Fill("x"), tk.Pady("0.5m"))
	tk.Pack(taskFrame.TLabel(tk.Txt("Bots")), tk.Anchor("w"))
	a.taskList = taskFrame.Listbox(tk.Height(10))
	tk.Pack(a.taskList, tk.Fill("x"))
	tk.Bind(a.taskList, eventListboxSelect, tk.Command(func() {
		if a.running {
			return
		}
	}))

	taskHint := tasksTab.TLabel(tk.Txt("Select a Bot and click Open to load it into the Task tab."))
	tk.Pack(taskHint, tk.Anchor("w"), tk.Pady("0.25m"))

	taskButtons := tasksTab.TFrame()
	tk.Pack(taskButtons, tk.Fill("x"), tk.Pady("0.5m"))
	a.newBtn = taskButtons.TButton(tk.Txt("New"), tk.Command(a.onNewTask))
	a.selectBtn = taskButtons.TButton(tk.Txt("Open"), tk.Command(a.onSelectTask))
	a.deleteBtn = taskButtons.TButton(tk.Txt("Delete"), tk.Command(a.onDeleteTask))
	a.importBtn = taskButtons.TButton(tk.Txt("Import"), tk.Command(a.onImportTask))
	a.exportBtn = taskButtons.TButton(tk.Txt("Export"), tk.Command(a.onExportTask))
	tk.Pack(a.newBtn, a.selectBtn, a.deleteBtn, a.importBtn, a.exportBtn, tk.Side("left"), tk.Padx("0.5m"))

	formFrame := a.editorTab.TFrame()
	tk.Pack(formFrame, tk.Fill("x"), tk.Pady("0.5m"))
	tk.Pack(formFrame.TLabel(tk.Txt("Summary")), tk.Anchor("w"))
	a.summaryEntry = formFrame.TEntry(tk.Width(80))
	tk.Pack(a.summaryEntry, tk.Fill("x"))

	generateFrame := a.editorTab.TFrame()
	tk.Pack(generateFrame, tk.Fill("x"), tk.Pady("0.5m"))
	a.generateBtn = generateFrame.TButton(tk.Txt("Generate Steps"), tk.Command(a.onGenerateSteps))
	tk.Pack(a.generateBtn, tk.Side("left"))

	stepEditor := a.editorTab.TFrame()
	tk.Pack(stepEditor, tk.Fill("both"), tk.Expand(true), tk.Pady("0.5m"))
	tk.Pack(stepEditor.TLabel(tk.Txt("Steps (point form)")), tk.Anchor("w"))
	a.stepText = stepEditor.Text(tk.Width(80), tk.Height(10), tk.Wrap("word"))
	tk.Pack(a.stepText, tk.Fill("both"), tk.Expand(true))

	stepFrame := a.editorTab.TFrame()
	tk.Pack(stepFrame, tk.Fill("x"), tk.Pady("0.5m"))
	tk.Pack(stepFrame.TLabel(tk.Txt("Run Progress")), tk.Anchor("w"))
	a.currentStepLabel = stepFrame.TLabel(
		tk.Txt(buildEmptyCurrentStepLabel()),
		tk.Wraplength(progressLabelWrapPx),
		tk.Justify("left"),
	)
	tk.Pack(a.currentStepLabel, tk.Fill("x"), tk.Anchor("w"), tk.Pady("0.2m"))
	a.stepProgress = stepFrame.TProgressbar(tk.Mode("determinate"), tk.Maximum(1), tk.Value(0), tk.Length(420))
	tk.Pack(a.stepProgress, tk.Fill("x"))
	a.stepProgressCounts = stepFrame.TLabel(tk.Txt(buildStepProgressCounts(0, 0)))
	tk.Pack(a.stepProgressCounts, tk.Anchor("w"), tk.Pady("0.2m"))

	runFrame := a.editorTab.TFrame()
	tk.Pack(runFrame, tk.Fill("x"), tk.Pady("1m"))
	tk.Pack(runFrame.TLabel(tk.Txt("Run Interval")), tk.Side("left"))
	a.intervalBox = runFrame.TCombobox(tk.State("readonly"), tk.Width(24), tk.Values(intervalValues()))
	a.intervalBox.Current(0)
	a.runBtn = runFrame.TButton(tk.Txt("Run"), tk.Command(a.onRunClicked))
	tk.Pack(a.intervalBox, a.runBtn, tk.Side("left"), tk.Padx("0.5m"))

	saveFrame := a.editorTab.TFrame()
	tk.Pack(saveFrame, tk.Fill("x"), tk.Pady("0.5m"))
	a.saveBtn = saveFrame.TButton(tk.Txt("Save"), tk.Command(a.onSaveTask))
	tk.Pack(a.saveBtn, tk.Anchor("w"))

	statusFrame := root.TFrame()
	tk.Pack(statusFrame, tk.Fill("x"), tk.Pady("1m"))
	a.countdownLabel = statusFrame.TLabel(tk.Txt(""))
	tk.Pack(a.countdownLabel, tk.Fill("x"), tk.Anchor("w"))
}

func intervalValues() string {
	values := make([]string, 0, len(intervalOptions))
	for _, opt := range intervalOptions {
		values = append(values, strings.ReplaceAll(opt.Label, " ", "\u00A0"))
	}
	return strings.Join(values, " ")
}

func runnerValues() string {
	return strings.Join([]string{"Codex\u00A0CLI", "Claude\u00A0CLI", "Ollama\u00A0CLI"}, " ")
}

func runnerIndex(id string) int {
	switch strings.TrimSpace(id) {
	case config.RunnerClaudeCLI:
		return 1
	case config.RunnerOllamaCLI:
		return 2
	default:
		return 0
	}
}

func (a *App) resetRunIntervalToDefault() {
	if a.intervalBox != nil {
		a.intervalBox.Current(0)
	}
	if a.countdownLabel != nil {
		a.countdownLabel.Configure(tk.Txt(""))
	}
}

func (a *App) onNewTask() {
	if a.running {
		return
	}
	a.current = newDraftTask()
	a.selectedStep = -1
	a.taskList.SelectionClear("0", tk.END)
	a.renderCurrentTask()
	a.resetRunIntervalToDefault()
	if a.notebook != nil && a.editorTab != nil {
		a.notebook.Select(a.editorTab.Window)
	}
}

func (a *App) onSaveTask() {
	if a.current == nil {
		return
	}
	a.syncCurrentTaskFields()
	if err := a.taskStore.Save(a.current); err != nil {
		a.showFriendlyError("Save Failed", "We couldn't save this task.", err)
		return
	}
	_ = a.reloadTasks(a.current.ID)
}

func (a *App) onSelectTask() {
	if a.running {
		return
	}
	i := a.selectedTaskIndex()
	if i < 0 {
		tk.MessageBox(
			tk.Title("Select Task"),
			tk.Msg("Select a task in the list first."),
			tk.Icon("warning"),
			tk.Type("ok"),
		)
		return
	}
	a.selectTaskIndex(i)
	a.resetRunIntervalToDefault()
	if a.notebook != nil && a.editorTab != nil {
		a.notebook.Select(a.editorTab.Window)
	}
}

func (a *App) onDeleteTask() {
	if a.running {
		return
	}
	target := a.current
	if i := a.selectedTaskIndex(); i >= 0 {
		target = a.tasks[i]
	}
	if target == nil || target.ID == "" {
		return
	}
	answer := tk.MessageBox(
		tk.Title("Delete Task"),
		tk.Msg("Delete selected task?"),
		tk.Icon("warning"),
		tk.Type("yesno"),
	)
	if answer != "yes" {
		return
	}
	if err := a.taskStore.Delete(target.ID); err != nil {
		a.showFriendlyError("Delete Failed", "We couldn't delete that task.", err)
		return
	}
	if a.current != nil && a.current.ID == target.ID {
		a.current = newDraftTask()
		a.selectedStep = -1
		a.renderCurrentTask()
		a.resetRunIntervalToDefault()
	}
	_ = a.reloadTasks("")
}

func (a *App) onImportTask() {
	if a.running {
		return
	}
	paths := tk.GetOpenFile(
		tk.Title("Import Task Markdown"),
		tk.Filetypes([]tk.FileType{{TypeName: "Markdown files", Extensions: []string{".md"}}}),
	)
	path := ""
	if len(paths) > 0 {
		path = paths[0]
	}
	if strings.TrimSpace(path) == "" {
		return
	}
	p, err := a.taskStore.Import(path)
	if err != nil {
		a.showFriendlyError("Import Failed", "We couldn't import that task file.", err)
		return
	}
	_ = a.reloadTasks(p.ID)
	a.resetRunIntervalToDefault()
}

func (a *App) onExportTask() {
	targetID := ""
	if i := a.selectedTaskIndex(); i >= 0 {
		targetID = a.tasks[i].ID
	}
	if targetID == "" && a.current != nil {
		targetID = a.current.ID
	}
	if targetID == "" && a.current != nil {
		a.onSaveTask()
		targetID = a.current.ID
	}
	if targetID == "" {
		return
	}
	path := tk.GetSaveFile(
		tk.Title("Export Task Markdown"),
		tk.Confirmoverwrite(true),
		tk.Filetypes([]tk.FileType{{TypeName: "Markdown files", Extensions: []string{".md"}}}),
	)
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := a.taskStore.Export(targetID, path); err != nil {
		a.showFriendlyError("Export Failed", "We couldn't export that task file.", err)
		return
	}
}

func (a *App) onGenerateSteps() {
	if a.current == nil || a.running || a.generatingSteps {
		return
	}
	a.syncCurrentTaskFields()
	summary := strings.TrimSpace(a.current.Summary)
	if summary == "" {
		tk.MessageBox(
			tk.Title("Missing Fields"),
			tk.Msg("Summary is required before generating steps."),
			tk.Icon("warning"),
			tk.Type("ok"),
		)
		return
	}
	a.generatingSteps = true
	a.updateRunUIState()
	a.startGenerateSpinner()
	go func(task *domain.Task) {
		drafts, err := a.provider.GenerateInitialSteps(context.Background(), task.Summary, 7)
		tk.PostEvent(func() {
			a.stopGenerateSpinner()
			a.generatingSteps = false
			a.updateRunUIState()
			if err != nil {
				a.showFriendlyError("Generate Failed", "We couldn't generate steps right now.", err)
				return
			}
			setGeneratedSteps(task, drafts)
			if len(task.Steps) > 0 {
				a.selectedStep = 0
			} else {
				a.selectedStep = -1
			}
			a.renderCurrentTask()
			a.onSaveTask()
		}, false)
	}(a.current)
}

func (a *App) onRunClicked() {
	if a.running {
		go func() { _ = a.manager.Stop("", "stopped by user") }()
		return
	}
	if a.hotkeyErr != nil {
		a.showFriendlyError("Hotkey Unavailable", "We couldn't start the run because the stop hotkey is unavailable.", a.hotkeyErr)
		return
	}
	if a.current == nil {
		return
	}
	a.onSaveTask()
	if a.current == nil || a.current.ID == "" {
		return
	}
	domain.ResetStepStatuses(a.current)
	_ = a.taskStore.Save(a.current)
	if len(a.current.Steps) > 0 {
		a.selectedStep = 0
	} else {
		a.selectedStep = -1
	}
	a.renderSteps()

	i := 0
	if idx, err := strconv.Atoi(a.intervalBox.Current(nil)); err == nil && idx >= 0 && idx < len(intervalOptions) {
		i = idx
	}
	opt := intervalOptions[i]
	settleDelay := minimizeSettleDelay()
	a.running = true
	a.recurring = opt.Recurring
	a.activeTaskID = a.current.ID
	a.updateRunUIState()
	a.countdownLabel.Configure(tk.Txt(""))
	a.driver.MinimizeMainWindow()

	if opt.Recurring {
		go func(taskID string, d, settle time.Duration) {
			if settle > 0 {
				time.Sleep(settle)
			}
			if err := a.manager.StartLoop(context.Background(), taskID, d); err != nil {
				tk.PostEvent(func() {
					a.running = false
					a.recurring = false
					a.activeTaskID = ""
					a.updateRunUIState()
					a.driver.RestoreMainWindow()
					a.showFriendlyError("Run Failed", "We couldn't start this task run.", err)
				}, false)
			}
		}(a.current.ID, opt.Duration, settleDelay)
		return
	}

	go func(taskID string, settle time.Duration) {
		if settle > 0 {
			time.Sleep(settle)
		}
		if err := a.manager.RunNow(context.Background(), taskID); err != nil {
			// Event callbacks drive UI state for run result.
			tk.PostEvent(func() {
				a.logError("run failed", err)
			}, false)
		}
	}(a.current.ID, settleDelay)
}

func (a *App) onRunnerChanged() {
	if a.running {
		return
	}
	if a.switchRunner == nil {
		return
	}
	idx, err := strconv.Atoi(a.runnerBox.Current(nil))
	if err != nil {
		idx = 0
	}
	runnerID := config.RunnerCodexCLI
	switch idx {
	case 1:
		runnerID = config.RunnerClaudeCLI
	case 2:
		runnerID = config.RunnerOllamaCLI
	}
	if runnerID == a.settings.Runner {
		return
	}
	p, err := a.switchRunner(runnerID)
	if err != nil {
		a.showFriendlyError("Runner Error", "We couldn't switch to that runner.", err)
		a.runnerBox.Current(runnerIndex(a.settings.Runner))
		return
	}
	a.provider = p
	a.settings.Runner = runnerID
	_ = a.settingsStore.Save(a.settings)
}

func shouldResetCurrentTaskForEvent(current *domain.Task, eventTaskID string) bool {
	if current == nil {
		return false
	}
	taskID := strings.TrimSpace(eventTaskID)
	return taskID == "" || current.ID == taskID
}

func (a *App) clearActiveRunState(restoreWindow bool) {
	a.running = false
	a.recurring = false
	a.activeTaskID = ""
	if a.countdownLabel != nil {
		a.countdownLabel.Configure(tk.Txt(""))
	}
	if a.runBtn != nil {
		a.updateRunUIState()
	}
	if restoreWindow && a.driver != nil {
		a.driver.RestoreMainWindow()
	}
}

func (a *App) applyStoppedStatus(taskID string) {
	a.clearActiveRunState(true)
	if shouldResetCurrentTaskForEvent(a.current, taskID) {
		a.resetCurrentTaskForReady()
	}
}

func (a *App) handleSchedulerEvent(e scheduler.Event) {
	switch e.Type {
	case scheduler.EventStep:
		if a.current != nil && a.current.ID == e.TaskID && e.CurrentStepIndex >= 0 && e.CurrentStepIndex < len(a.current.Steps) {
			a.current.Steps[e.CurrentStepIndex].Status = e.CurrentStepStatus
			a.current.Steps[e.CurrentStepIndex].LastError = e.Error
			a.selectedStep = e.CurrentStepIndex
			a.renderSteps()
		}
		if e.CurrentStepStatus == domain.StepFailed {
			a.lastFailureShot = e.PostScreenshotPath
		}
	case scheduler.EventSchedule:
		if e.State == "countdown" {
			if a.countdownLabel != nil {
				a.countdownLabel.Configure(tk.Txt(fmt.Sprintf("Next run in %ds", e.CountdownSeconds)))
			}
		}
		if e.State == "stopped" {
			a.applyStoppedStatus(e.TaskID)
		}
	case scheduler.EventStatus:
		switch e.State {
		case "running":
			a.running = true
			a.activeTaskID = e.TaskID
			if a.runBtn != nil {
				a.updateRunUIState()
			}
		case "completed":
			if a.driver != nil {
				a.driver.RestoreMainWindow()
			}
			if !a.recurring {
				a.clearActiveRunState(false)
			}
		case "failed":
			a.clearActiveRunState(true)
			detail := strings.TrimSpace(e.Error)
			if detail == "" {
				detail = strings.TrimSpace(e.Message)
			}
			if detail == "" {
				detail = "run failed"
			}
			if strings.TrimSpace(a.lastFailureShot) != "" {
				detail += " (post-failure screenshot: " + a.lastFailureShot + ")"
			}
			a.showFriendlyError("Run Failed", "The task stopped because a step failed.", errors.New(detail))
		case "stopped":
			a.applyStoppedStatus(e.TaskID)
		}
	}
}

func (a *App) updateRunUIState() {
	runText := "Run"
	if a.running {
		runText = "Stop"
	}
	a.runBtn.Configure(tk.Txt(runText))

	disable := a.running || a.generatingSteps
	state := "normal"
	if disable {
		state = "disabled"
	}
	a.taskList.Configure(tk.State(state))
	a.summaryEntry.Configure(tk.State(state))
	a.stepText.Configure(tk.State(state))
	a.intervalBox.Configure(tk.State(state))
	a.newBtn.Configure(tk.State(state))
	a.selectBtn.Configure(tk.State(state))
	a.saveBtn.Configure(tk.State(state))
	a.deleteBtn.Configure(tk.State(state))
	a.importBtn.Configure(tk.State(state))
	a.exportBtn.Configure(tk.State(state))
	a.generateBtn.Configure(tk.State(state))
	a.runnerBox.Configure(tk.State(state))

	if a.hotkeyErr != nil || a.generatingSteps {
		a.runBtn.Configure(tk.State("disabled"))
	} else {
		a.runBtn.Configure(tk.State("normal"))
	}
	a.refreshStepProgress()
}

func (a *App) startGenerateSpinner() {
	if a.generateBtn == nil {
		return
	}
	stop := make(chan struct{})
	a.generateSpinStop = stop
	a.generateBtn.Configure(tk.Txt(buildGenerateButtonSpinnerLabel(0)))
	go func(stopCh <-chan struct{}) {
		ticker := time.NewTicker(generateSpinnerInterval)
		defer ticker.Stop()
		tick := 1
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				label := buildGenerateButtonSpinnerLabel(tick)
				tick++
				tk.PostEvent(func() {
					if a.generateBtn == nil || !a.generatingSteps {
						return
					}
					a.generateBtn.Configure(tk.Txt(label))
				}, false)
			}
		}
	}(stop)
}

func (a *App) stopGenerateSpinner() {
	if a.generateSpinStop != nil {
		close(a.generateSpinStop)
		a.generateSpinStop = nil
	}
	if a.generateBtn != nil {
		a.generateBtn.Configure(tk.Txt(generateButtonDefaultLabel))
	}
}

func (a *App) resetCurrentTaskForReady() {
	if a.current != nil {
		domain.ResetStepStatuses(a.current)
		if a.taskStore != nil && strings.TrimSpace(a.current.ID) != "" {
			_ = a.taskStore.Save(a.current)
		}
	}
	a.selectedStep = -1
	a.lastFailureShot = ""
	a.refreshStepProgress()
}

func (a *App) logError(context string, err error) {
	if a.taskLogger == nil {
		return
	}
	msg := strings.TrimSpace(context)
	if msg == "" {
		msg = "ui error"
	}
	if err != nil {
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	a.taskLogger.Log(msg)
}

func (a *App) taskLogFilePath() string {
	path := strings.TrimSpace(a.taskLogPath)
	if path == "" {
		path = tasklog.ResolvePath("")
	}
	return path
}

func (a *App) ensureTaskLogFile() (string, error) {
	path := a.taskLogFilePath()
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("task log path is empty")
	}
	dir := filepath.Dir(path)
	if strings.TrimSpace(dir) != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return path, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return path, err
	}
	return path, f.Close()
}

func (a *App) openTaskLog(path string) error {
	if a.driver == nil {
		return fmt.Errorf("os driver unavailable")
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("task log path is empty")
	}
	return a.driver.OpenPath(path)
}

func (a *App) showFriendlyError(title, userMessage string, err error) {
	if err != nil {
		a.logError(title, err)
	} else {
		a.logError(title, errors.New(strings.TrimSpace(userMessage)))
	}

	logPath, ensureErr := a.ensureTaskLogFile()
	if ensureErr != nil {
		a.logError("prepare task log", ensureErr)
	}

	displayPath := ""
	if ensureErr == nil {
		displayPath = logPath
	}
	tk.MessageBox(
		tk.Title(title),
		tk.Msg(buildFriendlyErrorMessage(userMessage, displayPath)),
		tk.Icon("error"),
		tk.Type("ok"),
	)

	if ensureErr != nil {
		return
	}
	if err := a.openTaskLog(logPath); err != nil {
		a.logError("open task log", err)
	}
}

func (a *App) reloadTasks(selectID string) error {
	list, err := a.taskStore.List()
	if err != nil {
		return err
	}
	a.tasks = list
	a.renderTaskList()
	if len(a.tasks) == 0 {
		a.current = newDraftTask()
		a.selectedStep = -1
		a.renderCurrentTask()
		return nil
	}

	target := selectID
	if strings.TrimSpace(target) == "" && a.current != nil {
		target = a.current.ID
	}
	for i, p := range a.tasks {
		if p.ID == target {
			a.selectTaskIndex(i)
			return nil
		}
	}
	a.selectTaskIndex(0)
	return nil
}

func (a *App) renderTaskList() {
	a.taskList.Delete(0, tk.END)
	for _, p := range a.tasks {
		summary := strings.TrimSpace(p.Summary)
		if summary == "" {
			summary = "(No summary)"
		}
		a.taskList.Insert(tk.END, summary)
	}
}

func (a *App) selectedTaskIndex() int {
	if a.taskList == nil {
		return -1
	}
	sel := a.taskList.Curselection()
	if len(sel) == 0 {
		return -1
	}
	i := sel[0]
	if i < 0 || i >= len(a.tasks) {
		return -1
	}
	return i
}

func (a *App) selectTaskIndex(i int) {
	if i < 0 || i >= len(a.tasks) {
		return
	}
	a.current = a.tasks[i]
	a.selectedStep = -1
	a.renderCurrentTask()
	a.taskList.SelectionClear("0", tk.END)
	a.taskList.SelectionSet(i)
}

func (a *App) renderCurrentTask() {
	if a.current == nil {
		return
	}
	a.summaryEntry.Configure(tk.Textvariable(a.current.Summary))
	a.stepText.Clear()
	a.stepText.Insert("1.0", stepsTextFromTask(a.current))
	a.renderSteps()
}

func (a *App) renderSteps() {
	if a.current == nil || len(a.current.Steps) == 0 {
		a.selectedStep = -1
		a.refreshStepProgress()
		return
	}
	a.selectedStep = resolveStepIndex(a.current.Steps, a.selectedStep)
	a.refreshStepProgress()
}

func (a *App) syncCurrentTaskFields() {
	if a.current == nil {
		return
	}
	a.current.Summary = strings.TrimSpace(a.summaryEntry.Textvariable())
	applyStepTextToTask(a.current, a.stepText.Text())
}

func (a *App) stopHotkey() {
	if a.hotkey != nil {
		a.hotkey.Stop()
		a.hotkey = nil
	}
}

func (a *App) refreshStepProgress() {
	if a.currentStepLabel == nil || a.stepProgress == nil || a.stepProgressCounts == nil {
		return
	}

	total := 0
	if a.current != nil {
		total = len(a.current.Steps)
	}
	if !a.running {
		maximum := 1
		if total > 0 {
			maximum = total
		}
		if a.current != nil && total > 0 && allStepsCompleted(a.current.Steps) {
			idx := resolveStepIndex(a.current.Steps, a.selectedStep)
			if idx < 0 {
				idx = total - 1
			}
			a.selectedStep = idx
			step := a.current.Steps[idx]
			a.currentStepLabel.Configure(tk.Txt(buildCurrentStepLabel(step, idx, total)))
			a.stepProgress.Configure(tk.Maximum(total), tk.Value(total))
			a.stepProgressCounts.Configure(tk.Txt(buildStepProgressCounts(total, total)))
			return
		}
		a.currentStepLabel.Configure(tk.Txt(buildEmptyCurrentStepLabel()))
		a.stepProgress.Configure(tk.Maximum(maximum), tk.Value(0))
		a.stepProgressCounts.Configure(tk.Txt(buildStepProgressCounts(0, total)))
		return
	}
	if a.current == nil || total == 0 {
		a.currentStepLabel.Configure(tk.Txt(buildEmptyCurrentStepLabel()))
		a.stepProgress.Configure(tk.Maximum(1), tk.Value(0))
		a.stepProgressCounts.Configure(tk.Txt(buildStepProgressCounts(0, 0)))
		return
	}
	idx := resolveStepIndex(a.current.Steps, a.selectedStep)
	if idx < 0 {
		a.currentStepLabel.Configure(tk.Txt(buildEmptyCurrentStepLabel()))
		a.stepProgress.Configure(tk.Maximum(1), tk.Value(0))
		a.stepProgressCounts.Configure(tk.Txt(buildStepProgressCounts(0, total)))
		return
	}
	a.selectedStep = idx
	step := a.current.Steps[idx]
	value := computeStepProgressValue(idx, total)
	a.currentStepLabel.Configure(tk.Txt(buildCurrentStepLabel(step, idx, total)))
	a.stepProgress.Configure(tk.Maximum(total), tk.Value(value))
	a.stepProgressCounts.Configure(tk.Txt(buildStepProgressCounts(value, total)))
}
