package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
	"github.com/nickbarth/closedbots/internal/osctrl"
)

type recordingExecutor struct {
	actions []domain.Action
	failAt  int
	count   int
}

func (r *recordingExecutor) BackendName() string { return "test" }
func (r *recordingExecutor) Execute(ctx context.Context, action domain.Action) error {
	r.actions = append(r.actions, action)
	if r.failAt >= 0 && r.count == r.failAt {
		r.count++
		return errors.New("boom")
	}
	r.count++
	return nil
}

type recordingDriver struct {
	url string
	err error
}

func (d *recordingDriver) Name() string { return "driver" }
func (d *recordingDriver) StartGlobalStopHotkey(combo string, onPress func()) (osctrl.HotkeyHandle, error) {
	return nil, nil
}
func (d *recordingDriver) MinimizeMainWindow() {}
func (d *recordingDriver) RestoreMainWindow()  {}
func (d *recordingDriver) LaunchBrowser(url string) error {
	d.url = url
	return d.err
}
func (d *recordingDriver) OpenPath(path string) error { return nil }

func TestDeterministicIntentHelpers(t *testing.T) {
	if got := extractFirstURL(`go to "https://example.com/test."`); got != "https://example.com/test" {
		t.Fatalf("url=%q", got)
	}
	task := domain.Task{
		Summary: "Use Calculator",
		Steps: []domain.Step{
			{ID: "s1", Instruction: "Open the app launcher", Status: domain.StepCompleted},
			{ID: "s2", Instruction: `Search for "Calculator" and launch`, Status: domain.StepPending},
		},
	}
	if got := inferTargetApp(task); got != "calculator" {
		t.Fatalf("target app=%q", got)
	}
	if !isGenericLauncherStep("open app launcher now") {
		t.Fatalf("expected generic launcher")
	}
	if !isLaunchAppStep("type calculator in launcher", "calculator") {
		t.Fatalf("expected launch app")
	}
	if !isCalculatorNumberEntryStep(`enter "1234" on calculator buttons`) {
		t.Fatalf("expected number entry")
	}
	if isCalculatorNumberEntryStep("verify 1337 appears") {
		t.Fatalf("verify instruction should not be number entry")
	}
	if !isCalculatorOperatorStep("press plus (+)") {
		t.Fatalf("expected operator step")
	}
	if got := extractCalculatorOperatorKey("press divide /"); got != "/" {
		t.Fatalf("operator=%q", got)
	}
	if !shouldPressCalculatorEquals("enter 1 then equals") {
		t.Fatalf("expected equals")
	}
	if !hasCompletedLauncherStepImmediatelyBefore(task, task.Steps[1]) {
		t.Fatalf("expected previous launcher step")
	}
	if taskStepIndex(task, domain.Step{ID: "missing", Instruction: "none"}) != -1 {
		t.Fatalf("expected -1")
	}
	if chromeAppFromInstruction("open chrome") != "google chrome" {
		t.Fatalf("unexpected chrome app parse")
	}
	if extractDigitSequence(`Type number sequence "1 2 3 4"`) != "1234" {
		t.Fatalf("unexpected digit extraction")
	}
}

func TestDeterministicBuildersAndWaitWithContext(t *testing.T) {
	if _, err := launcherHotkeyKeys("unknown"); err == nil {
		t.Fatalf("expected unsupported os")
	}
	if _, err := buildOpenLauncherActions("unknown"); err == nil {
		t.Fatalf("expected open launcher error")
	}
	if _, err := buildAppLaunchActions("", "linux"); err == nil {
		t.Fatalf("expected empty app name")
	}
	if _, err := buildLauncherSearchLaunchActions(""); err == nil {
		t.Fatalf("expected empty app name")
	}
	if err := waitWithContext(context.Background(), 0); err != nil {
		t.Fatalf("wait 0: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitWithContext(ctx, 10*time.Millisecond); err == nil {
		t.Fatalf("expected canceled")
	}
}

func TestExecuteDeterministicCalculatorEntry(t *testing.T) {
	exec := &recordingExecutor{failAt: -1}
	r := &Runner{Executor: exec}
	actions, err := r.executeDeterministicCalculatorEntry(context.Background(), "12", true)
	if err != nil {
		t.Fatalf("entry err: %v", err)
	}
	if len(actions) == 0 {
		t.Fatalf("expected actions")
	}
	if _, err := r.executeDeterministicCalculatorEntry(context.Background(), "   ", false); err == nil {
		t.Fatalf("expected empty input error")
	}
	if _, err := r.executeDeterministicCalculatorEntry(context.Background(), "abc", false); err == nil {
		t.Fatalf("expected no digits error")
	}
}

func TestExecuteDeterministicStepPaths(t *testing.T) {
	// No deterministic intent.
	r := &Runner{Executor: &recordingExecutor{failAt: -1}}
	actions, handled, err := r.executeDeterministicStep(context.Background(), domain.Task{}, domain.Step{Instruction: "do something generic"})
	if err != nil || handled || actions != nil {
		t.Fatalf("unexpected generic result actions=%#v handled=%v err=%v", actions, handled, err)
	}

	// Missing executor.
	r = &Runner{}
	_, handled, err = r.executeDeterministicStep(context.Background(), domain.Task{Summary: "calculator"}, domain.Step{Instruction: `enter "123"`})
	if !handled || err == nil {
		t.Fatalf("expected handled error for missing executor")
	}

	// URL step requires driver and launch.
	exec := &recordingExecutor{failAt: -1}
	task := domain.Task{Steps: []domain.Step{{ID: "s1", Instruction: "go to https://example.com"}}}
	step := domain.Step{ID: "s1", Instruction: "go to https://example.com"}
	r = &Runner{Executor: exec}
	_, handled, err = r.executeDeterministicStep(context.Background(), task, step)
	if !handled || err == nil {
		t.Fatalf("expected driver required error")
	}
	drv := &recordingDriver{}
	r = &Runner{Executor: exec, Driver: drv}
	acts, handled, err := r.executeDeterministicStep(context.Background(), task, step)
	if err != nil || !handled || len(acts) == 0 {
		t.Fatalf("url deterministic err=%v handled=%v acts=%#v", err, handled, acts)
	}
	if drv.url != "https://example.com" {
		t.Fatalf("url launch=%q", drv.url)
	}
}

func TestExecuteDeterministicActionsError(t *testing.T) {
	r := &Runner{Executor: &recordingExecutor{failAt: 0}}
	err := r.executeDeterministicActions(context.Background(), []domain.Action{{Type: domain.ActionWait, MS: 100}})
	if err == nil {
		t.Fatalf("expected execute error")
	}
}

func TestDeterministicIntentDerivationVariants(t *testing.T) {
	task := domain.Task{
		Summary: "calculator",
		Steps: []domain.Step{
			{ID: "s1", Instruction: "Open the app launcher", Status: domain.StepCompleted},
			{ID: "s2", Instruction: `Search for "Calculator" and launch`, Status: domain.StepPending},
		},
	}

	intent := deriveDeterministicStepIntent(task, domain.Step{Instruction: "open start menu"})
	if intent.Kind != deterministicStepOpenLauncher {
		t.Fatalf("expected open launcher intent, got %#v", intent)
	}

	intent = deriveDeterministicStepIntent(task, domain.Step{Instruction: "open chrome"})
	if intent.Kind != deterministicStepLaunchChrome || intent.App != "google chrome" {
		t.Fatalf("expected launch chrome intent, got %#v", intent)
	}

	intent = deriveDeterministicStepIntent(task, task.Steps[1])
	if intent.Kind != deterministicStepLauncherSearch || intent.App != "calculator" {
		t.Fatalf("expected launcher search intent, got %#v", intent)
	}

	intent = deriveDeterministicStepIntent(task, domain.Step{Instruction: `launch calculator app now`})
	if intent.Kind != deterministicStepLaunchApp || intent.App != "calculator" {
		t.Fatalf("expected launch app intent, got %#v", intent)
	}

	intent = deriveDeterministicStepIntent(task, domain.Step{Instruction: `enter "1234" on calculator`})
	if intent.Kind != deterministicStepCalculatorEntry || intent.Text != "1234" {
		t.Fatalf("expected calculator entry intent, got %#v", intent)
	}

	intent = deriveDeterministicStepIntent(task, domain.Step{Instruction: "press minus (-)"})
	if intent.Kind != deterministicStepCalculatorOperator || intent.Text != "-" {
		t.Fatalf("expected calculator operator intent, got %#v", intent)
	}

	intent = deriveDeterministicStepIntent(task, domain.Step{Instruction: "go to https://example.com/path"})
	if intent.Kind != deterministicStepOpenURL || intent.URL == "" {
		t.Fatalf("expected open url intent, got %#v", intent)
	}
}

func TestDeterministicHelperEdgeCases(t *testing.T) {
	if got := extractFirstURL("no url here"); got != "" {
		t.Fatalf("url=%q", got)
	}
	if got := inferTargetApp(domain.Task{Steps: []domain.Step{{Instruction: "Open calculator"}}}); got != "calculator" {
		t.Fatalf("target app=%q", got)
	}
	if isGenericLauncherStep(" ") {
		t.Fatalf("blank launcher step should be false")
	}
	if isLaunchAppStep("open app", "") {
		t.Fatalf("empty app name should be false")
	}
	if isLaunchAppStep("launch browser", "calculator") {
		t.Fatalf("missing app keyword should be false")
	}
	if !isLaunchAppStep("type calculator in launcher search", "calculator") {
		t.Fatalf("type-to-launch variant should be true")
	}

	if isCalculatorNumberEntryStep("confirm 1337") {
		t.Fatalf("confirm should not be number-entry step")
	}
	if !isCalculatorNumberEntryStep("type 123 on-screen keypad") {
		t.Fatalf("keypad entry should be number-entry step")
	}
	if isCalculatorOperatorStep("plus symbol") {
		t.Fatalf("operator step requires action verb")
	}

	if got := extractCalculatorOperatorKey("press minus (-)"); got != "-" {
		t.Fatalf("minus key=%q", got)
	}
	if got := extractCalculatorOperatorKey("press multiply (*)"); got != "*" {
		t.Fatalf("multiply key=%q", got)
	}
	if got := extractCalculatorOperatorKey("press equals (=)"); got != "=" {
		t.Fatalf("equals key=%q", got)
	}
	if got := extractCalculatorOperatorKey("n/a"); got != "" {
		t.Fatalf("unexpected operator key=%q", got)
	}
	if shouldPressCalculatorEquals(" ") {
		t.Fatalf("blank should not request equals")
	}

	task := domain.Task{
		Steps: []domain.Step{
			{ID: "s1", Instruction: "Open launcher", Status: domain.StepPending},
			{ID: "s2", Instruction: "Launch app", Status: domain.StepPending},
		},
	}
	if hasCompletedLauncherStepImmediatelyBefore(task, task.Steps[1]) {
		t.Fatalf("previous step is not completed")
	}
	if got := taskStepIndex(task, domain.Step{Instruction: "Launch app"}); got != 1 {
		t.Fatalf("expected instruction lookup index 1, got %d", got)
	}
	if got := taskStepIndex(task, domain.Step{}); got != -1 {
		t.Fatalf("expected -1 for empty step, got %d", got)
	}

	if chromeAppFromInstruction("launch ungoogled chromium") != "ungoogled chromium" {
		t.Fatalf("expected ungoogled chromium parse")
	}
	if chromeAppFromInstruction("open chromium browser") != "chromium" {
		t.Fatalf("expected chromium parse")
	}
	if chromeAppFromInstruction("open firefox") != "" {
		t.Fatalf("non-chrome app should be empty")
	}

	if got := extractDigitSequence("sequence: apples only"); got != "" {
		t.Fatalf("expected empty digit sequence, got %q", got)
	}
	if got := extractDigitSequence("My sequence is 9 8 7"); got != "987" {
		t.Fatalf("expected 987, got %q", got)
	}
}

func TestDeterministicExecutionBranches(t *testing.T) {
	r := &Runner{Executor: &recordingExecutor{failAt: -1}, Driver: &recordingDriver{}}
	task := domain.Task{Summary: "calculator"}

	actions, handled, err := r.executeDeterministicStep(context.Background(), task, domain.Step{Instruction: "open start menu"})
	if err != nil || !handled || len(actions) == 0 {
		t.Fatalf("open launcher err=%v handled=%v actions=%#v", err, handled, actions)
	}

	actions, handled, err = r.executeDeterministicStep(context.Background(), task, domain.Step{Instruction: "launch chrome"})
	if err != nil || !handled || len(actions) == 0 {
		t.Fatalf("launch chrome err=%v handled=%v actions=%#v", err, handled, actions)
	}

	actions, handled, err = r.executeDeterministicStep(context.Background(), task, domain.Step{Instruction: "launch calculator"})
	if err != nil || !handled || len(actions) == 0 {
		t.Fatalf("launch app err=%v handled=%v actions=%#v", err, handled, actions)
	}

	taskWithPrev := domain.Task{
		Summary: "calculator",
		Steps: []domain.Step{
			{ID: "s1", Instruction: "open start menu", Status: domain.StepCompleted},
			{ID: "s2", Instruction: "search calculator and launch", Status: domain.StepPending},
		},
	}
	actions, handled, err = r.executeDeterministicStep(context.Background(), taskWithPrev, taskWithPrev.Steps[1])
	if err != nil || !handled || len(actions) == 0 {
		t.Fatalf("launcher search err=%v handled=%v actions=%#v", err, handled, actions)
	}

	actions, handled, err = r.executeDeterministicStep(context.Background(), task, domain.Step{Instruction: "press plus (+)"})
	if err != nil || !handled || len(actions) == 0 {
		t.Fatalf("operator err=%v handled=%v actions=%#v", err, handled, actions)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, handled, err = r.executeDeterministicStep(ctx, task, domain.Step{Instruction: "go to https://example.com"})
	if !handled || err == nil {
		t.Fatalf("expected handled cancellation error")
	}
}

func TestDeterministicBuilderBranches(t *testing.T) {
	if _, err := buildOpenLauncherActions("linux"); err != nil {
		t.Fatalf("linux launcher actions: %v", err)
	}
	if _, err := buildAppLaunchActions("Calculator", "linux"); err != nil {
		t.Fatalf("app launch actions: %v", err)
	}
	if _, err := buildLauncherSearchLaunchActions("Calculator"); err != nil {
		t.Fatalf("search launch actions: %v", err)
	}
	if _, err := launcherHotkeyKeys("windows"); err != nil {
		t.Fatalf("windows keys: %v", err)
	}
	if _, err := launcherHotkeyKeys("darwin"); err != nil {
		t.Fatalf("darwin keys: %v", err)
	}
}

func TestExecuteDeterministicCalculatorEntryErrorBranches(t *testing.T) {
	r := &Runner{Executor: &recordingExecutor{failAt: 0}}
	if _, err := r.executeDeterministicCalculatorEntry(context.Background(), "12", false); err == nil {
		t.Fatalf("expected key action failure")
	}

	r = &Runner{Executor: &recordingExecutor{failAt: 1}}
	if _, err := r.executeDeterministicCalculatorEntry(context.Background(), "12", false); err == nil {
		t.Fatalf("expected key-spacing wait failure")
	}

	r = &Runner{Executor: &recordingExecutor{failAt: 1}}
	if _, err := r.executeDeterministicCalculatorEntry(context.Background(), "1", true); err == nil {
		t.Fatalf("expected equals action failure")
	}
}
