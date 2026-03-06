package runner

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/nickbarth/closedbots/internal/domain"
)

type deterministicStepKind string

const (
	deterministicStepNone               deterministicStepKind = ""
	deterministicStepOpenLauncher       deterministicStepKind = "open_launcher"
	deterministicStepLaunchChrome       deterministicStepKind = "launch_chrome"
	deterministicStepLaunchApp          deterministicStepKind = "launch_app"
	deterministicStepLauncherSearch     deterministicStepKind = "launcher_search_launch"
	deterministicStepOpenURL            deterministicStepKind = "open_url"
	deterministicStepCalculatorEntry    deterministicStepKind = "calculator_entry"
	deterministicStepCalculatorOperator deterministicStepKind = "calculator_operator"
)

const deterministicStepWait = 2 * time.Second
const deterministicLauncherSettle = 800 * time.Millisecond
const deterministicCalculatorKeyDelay = 75 * time.Millisecond
const deterministicCalculatorSettle = 250 * time.Millisecond

var urlPattern = regexp.MustCompile(`https?://[^\s"')]+`)
var quotedDigitSequencePattern = regexp.MustCompile(`["']([^"']*\d[^"']*)["']`)
var digitTokenPattern = regexp.MustCompile(`\d+`)

type deterministicStepIntent struct {
	Kind        deterministicStepKind
	URL         string
	App         string
	Text        string
	PressEquals bool
}

func deriveDeterministicStepIntent(task domain.Task, step domain.Step) deterministicStepIntent {
	raw := strings.TrimSpace(step.Instruction)
	lower := strings.ToLower(raw)
	targetApp := inferTargetApp(task)

	if isGenericLauncherStep(lower) {
		return deterministicStepIntent{
			Kind: deterministicStepOpenLauncher,
		}
	}

	if appName := chromeAppFromInstruction(lower); appName != "" {
		return deterministicStepIntent{
			Kind: deterministicStepLaunchChrome,
			App:  appName,
		}
	}

	if isLaunchAppStep(lower, "calculator") {
		if hasCompletedLauncherStepImmediatelyBefore(task, step) {
			return deterministicStepIntent{
				Kind: deterministicStepLauncherSearch,
				App:  "calculator",
			}
		}
		return deterministicStepIntent{
			Kind: deterministicStepLaunchApp,
			App:  "calculator",
		}
	}

	if targetApp == "calculator" && isCalculatorNumberEntryStep(lower) {
		if digits := extractDigitSequence(raw); digits != "" {
			return deterministicStepIntent{
				Kind:        deterministicStepCalculatorEntry,
				Text:        digits,
				PressEquals: shouldPressCalculatorEquals(lower),
			}
		}
	}

	if targetApp == "calculator" && isCalculatorOperatorStep(lower) {
		if key := extractCalculatorOperatorKey(lower); key != "" {
			return deterministicStepIntent{
				Kind: deterministicStepCalculatorOperator,
				Text: key,
			}
		}
	}

	if strings.Contains(lower, "navigate") || strings.Contains(lower, "go to") || strings.Contains(lower, "open ") {
		if url := extractFirstURL(raw); url != "" {
			return deterministicStepIntent{
				Kind: deterministicStepOpenURL,
				URL:  url,
			}
		}
	}

	return deterministicStepIntent{}
}

func extractFirstURL(text string) string {
	m := urlPattern.FindString(strings.TrimSpace(text))
	if m == "" {
		return ""
	}
	return strings.TrimRight(m, ".,;")
}

func inferTargetApp(task domain.Task) string {
	summary := strings.ToLower(strings.TrimSpace(task.Summary))
	if strings.Contains(summary, "calculator") {
		return "calculator"
	}
	for _, step := range task.Steps {
		lower := strings.ToLower(strings.TrimSpace(step.Instruction))
		if strings.Contains(lower, "calculator") {
			return "calculator"
		}
	}
	return ""
}

func isGenericLauncherStep(lowerInstruction string) bool {
	if strings.TrimSpace(lowerInstruction) == "" {
		return false
	}
	opensLauncher := (strings.Contains(lowerInstruction, "open") || strings.Contains(lowerInstruction, "launch")) &&
		(strings.Contains(lowerInstruction, "app launcher") ||
			strings.Contains(lowerInstruction, "application launcher") ||
			strings.Contains(lowerInstruction, "search bar") ||
			strings.Contains(lowerInstruction, "start menu"))
	return opensLauncher
}

func isLaunchAppStep(lowerInstruction, appName string) bool {
	if appName == "" {
		return false
	}
	if !strings.Contains(lowerInstruction, appName) {
		return false
	}
	isTypeToLaunch := strings.Contains(lowerInstruction, "type ") &&
		(strings.Contains(lowerInstruction, "launcher") || strings.Contains(lowerInstruction, "search"))
	return strings.Contains(lowerInstruction, "open") ||
		strings.Contains(lowerInstruction, "launch") ||
		isTypeToLaunch
}

func isCalculatorNumberEntryStep(lowerInstruction string) bool {
	trimmed := strings.TrimSpace(lowerInstruction)
	if trimmed == "" || !digitTokenPattern.MatchString(trimmed) {
		return false
	}

	if strings.Contains(trimmed, "confirm") || strings.Contains(trimmed, "verify") || strings.Contains(trimmed, "check") {
		return false
	}

	if strings.Contains(trimmed, "number sequence") || strings.Contains(trimmed, "on-screen") ||
		strings.Contains(trimmed, "calculator button") || strings.Contains(trimmed, "keypad") {
		return strings.Contains(trimmed, "enter") || strings.Contains(trimmed, "type")
	}

	return strings.Contains(trimmed, "enter") || strings.Contains(trimmed, "type")
}

func isCalculatorOperatorStep(lowerInstruction string) bool {
	trimmed := strings.TrimSpace(lowerInstruction)
	if trimmed == "" {
		return false
	}
	if !strings.Contains(trimmed, "press") &&
		!strings.Contains(trimmed, "tap") &&
		!strings.Contains(trimmed, "hit") &&
		!strings.Contains(trimmed, "click") {
		return false
	}
	return extractCalculatorOperatorKey(trimmed) != ""
}

func extractCalculatorOperatorKey(lowerInstruction string) string {
	trimmed := strings.TrimSpace(lowerInstruction)
	if trimmed == "" {
		return ""
	}
	switch {
	case strings.Contains(trimmed, "plus"), strings.Contains(trimmed, "(+)"), strings.Contains(trimmed, " + "):
		return "+"
	case strings.Contains(trimmed, "minus"), strings.Contains(trimmed, "(-)"), strings.Contains(trimmed, " - "):
		return "-"
	case strings.Contains(trimmed, "multiply"), strings.Contains(trimmed, "times"), strings.Contains(trimmed, "(x)"), strings.Contains(trimmed, "(*)"), strings.Contains(trimmed, " * "):
		return "*"
	case strings.Contains(trimmed, "divide"), strings.Contains(trimmed, "(/)"), strings.Contains(trimmed, " / "):
		return "/"
	case strings.Contains(trimmed, "equals"), strings.Contains(trimmed, "equal"), strings.Contains(trimmed, "(=)"), strings.Contains(trimmed, " = "):
		return "="
	default:
		return ""
	}
}

func shouldPressCalculatorEquals(lowerInstruction string) bool {
	trimmed := strings.TrimSpace(lowerInstruction)
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "equals") ||
		strings.Contains(trimmed, "equal") ||
		strings.Contains(trimmed, "(=)") ||
		strings.Contains(trimmed, " = ")
}

func hasCompletedLauncherStepImmediatelyBefore(task domain.Task, step domain.Step) bool {
	idx := taskStepIndex(task, step)
	if idx <= 0 {
		return false
	}
	prev := task.Steps[idx-1]
	if prev.Status != domain.StepCompleted {
		return false
	}
	return isGenericLauncherStep(strings.ToLower(strings.TrimSpace(prev.Instruction)))
}

func taskStepIndex(task domain.Task, step domain.Step) int {
	if strings.TrimSpace(step.ID) != "" {
		for i := range task.Steps {
			if task.Steps[i].ID == step.ID {
				return i
			}
		}
	}
	needle := strings.TrimSpace(step.Instruction)
	if needle == "" {
		return -1
	}
	for i := range task.Steps {
		if strings.TrimSpace(task.Steps[i].Instruction) == needle {
			return i
		}
	}
	return -1
}

func chromeAppFromInstruction(lowerInstruction string) string {
	switch {
	case strings.Contains(lowerInstruction, "open ungoogled chromium"),
		strings.Contains(lowerInstruction, "launch ungoogled chromium"):
		return "ungoogled chromium"
	case strings.Contains(lowerInstruction, "open chromium"),
		strings.Contains(lowerInstruction, "launch chromium"):
		return "chromium"
	case strings.Contains(lowerInstruction, "open google chrome"),
		strings.Contains(lowerInstruction, "launch google chrome"),
		strings.Contains(lowerInstruction, "open chrome"),
		strings.Contains(lowerInstruction, "launch chrome"):
		return "google chrome"
	default:
		return ""
	}
}

func extractDigitSequence(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	quotedMatches := quotedDigitSequencePattern.FindAllStringSubmatch(trimmed, -1)
	for _, m := range quotedMatches {
		if len(m) < 2 {
			continue
		}
		if seq := strings.Join(digitTokenPattern.FindAllString(m[1], -1), ""); seq != "" {
			return seq
		}
	}

	lower := strings.ToLower(trimmed)
	if idx := strings.Index(lower, "sequence"); idx >= 0 {
		suffix := trimmed[idx+len("sequence"):]
		if tokens := digitTokenPattern.FindAllString(suffix, -1); len(tokens) > 0 {
			return strings.Join(tokens, "")
		}
	}

	tokens := digitTokenPattern.FindAllString(trimmed, -1)
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, "")
}

func (r *Runner) executeDeterministicStep(ctx context.Context, task domain.Task, step domain.Step) ([]domain.Action, bool, error) {
	intent := deriveDeterministicStepIntent(task, step)
	if intent.Kind == deterministicStepNone {
		return nil, false, nil
	}
	if r.Executor == nil {
		return nil, true, fmt.Errorf("executor is required for deterministic execution")
	}

	actions := make([]domain.Action, 0, 8)
	var err error
	postWait := time.Duration(0)
	postWaitReason := ""

	switch intent.Kind {
	case deterministicStepOpenLauncher:
		actions, err = buildOpenLauncherActions(runtime.GOOS)
		if err != nil {
			return nil, true, err
		}
		if err := r.executeDeterministicActions(ctx, actions); err != nil {
			return nil, true, err
		}
	case deterministicStepLaunchChrome:
		appName := strings.TrimSpace(intent.App)
		if appName == "" {
			appName = "google chrome"
		}
		actions, err = buildAppLaunchActions(appName, runtime.GOOS)
		if err != nil {
			return nil, true, err
		}
		if err := r.executeDeterministicActions(ctx, actions); err != nil {
			return nil, true, err
		}
	case deterministicStepLaunchApp:
		actions, err = buildAppLaunchActions(intent.App, runtime.GOOS)
		if err != nil {
			return nil, true, err
		}
		if err := r.executeDeterministicActions(ctx, actions); err != nil {
			return nil, true, err
		}
	case deterministicStepLauncherSearch:
		actions, err = buildLauncherSearchLaunchActions(intent.App)
		if err != nil {
			return nil, true, err
		}
		if err := r.executeDeterministicActions(ctx, actions); err != nil {
			return nil, true, err
		}
	case deterministicStepOpenURL:
		if r.Driver == nil {
			return nil, true, fmt.Errorf("driver is required for deterministic URL launch")
		}
		if err := r.Driver.LaunchBrowser(intent.URL); err != nil {
			return nil, true, fmt.Errorf("open url in browser: %w", err)
		}
		postWait = deterministicStepWait
		postWaitReason = "deterministic os launch"
	case deterministicStepCalculatorEntry:
		digitActions, err := r.executeDeterministicCalculatorEntry(ctx, intent.Text, intent.PressEquals)
		if err != nil {
			return nil, true, err
		}
		actions = append(actions, digitActions...)
		postWait = deterministicCalculatorSettle
		postWaitReason = "deterministic calculator input settle"
	case deterministicStepCalculatorOperator:
		operatorKey := strings.TrimSpace(intent.Text)
		if operatorKey == "" {
			return nil, true, fmt.Errorf("deterministic calculator operator is empty")
		}
		operatorAction := domain.Action{
			Type:   domain.ActionSendKey,
			Key:    operatorKey,
			Reason: "deterministic calculator operator input",
		}
		if err := r.Executor.Execute(ctx, operatorAction); err != nil {
			return nil, true, err
		}
		actions = append(actions, operatorAction)
		postWait = deterministicCalculatorSettle
		postWaitReason = "deterministic calculator input settle"
	default:
		return nil, false, nil
	}

	if postWait > 0 {
		if err := waitWithContext(ctx, postWait); err != nil {
			return nil, true, err
		}
		actions = append(actions, domain.Action{
			Type:   domain.ActionWait,
			Reason: postWaitReason,
			MS:     int(postWait / time.Millisecond),
		})
	}

	return actions, true, nil
}

func (r *Runner) executeDeterministicActions(ctx context.Context, actions []domain.Action) error {
	for _, action := range actions {
		if err := r.Executor.Execute(ctx, action); err != nil {
			return err
		}
	}
	return nil
}

func buildOpenLauncherActions(goos string) ([]domain.Action, error) {
	keys, err := launcherHotkeyKeys(goos)
	if err != nil {
		return nil, err
	}
	return []domain.Action{
		{
			Type:   domain.ActionHotkey,
			Keys:   keys,
			Reason: "deterministic launcher open",
		},
		{
			Type:   domain.ActionWait,
			MS:     int(deterministicLauncherSettle / time.Millisecond),
			Reason: "deterministic launcher settle",
		},
	}, nil
}

func buildAppLaunchActions(appName, goos string) ([]domain.Action, error) {
	name := strings.TrimSpace(appName)
	if name == "" {
		return nil, fmt.Errorf("app name is empty")
	}

	launcherActions, err := buildOpenLauncherActions(goos)
	if err != nil {
		return nil, err
	}
	actions := append([]domain.Action{}, launcherActions...)
	searchLaunchActions, err := buildLauncherSearchLaunchActions(name)
	if err != nil {
		return nil, err
	}
	actions = append(actions, searchLaunchActions...)
	return actions, nil
}

func buildLauncherSearchLaunchActions(appName string) ([]domain.Action, error) {
	name := strings.TrimSpace(appName)
	if name == "" {
		return nil, fmt.Errorf("app name is empty")
	}
	return []domain.Action{
		{
			Type:   domain.ActionTypeText,
			Text:   name,
			Reason: "deterministic launcher search",
		},
		{
			Type:   domain.ActionSendKey,
			Key:    "ENTER",
			Reason: "deterministic launcher confirm",
		},
		{
			Type:   domain.ActionWait,
			MS:     int(deterministicStepWait / time.Millisecond),
			Reason: "deterministic os launch",
		},
	}, nil
}

func launcherHotkeyKeys(goos string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "windows":
		return []string{"WINDOWS"}, nil
	case "darwin":
		return []string{"CMD", "SPACE"}, nil
	case "linux":
		return []string{"SUPER_L"}, nil
	default:
		return nil, fmt.Errorf("unsupported OS for launcher-based app launch: %q", goos)
	}
}

func (r *Runner) executeDeterministicCalculatorEntry(ctx context.Context, digits string, pressEquals bool) ([]domain.Action, error) {
	trimmed := strings.TrimSpace(digits)
	if trimmed == "" {
		return nil, fmt.Errorf("deterministic calculator input is empty")
	}

	chars := []rune(trimmed)
	actions := make([]domain.Action, 0, len(chars)*2)
	validDigits := 0
	for i, ch := range chars {
		if ch < '0' || ch > '9' {
			continue
		}

		keyAction := domain.Action{
			Type:   domain.ActionSendKey,
			Key:    string(ch),
			Reason: "deterministic calculator digit input",
		}
		if err := r.Executor.Execute(ctx, keyAction); err != nil {
			return nil, err
		}
		actions = append(actions, keyAction)
		validDigits++

		if i < len(chars)-1 {
			waitAction := domain.Action{
				Type:   domain.ActionWait,
				MS:     int(deterministicCalculatorKeyDelay / time.Millisecond),
				Reason: "deterministic key spacing",
			}
			if err := r.Executor.Execute(ctx, waitAction); err != nil {
				return nil, err
			}
			actions = append(actions, waitAction)
		}
	}
	if validDigits == 0 {
		return nil, fmt.Errorf("deterministic calculator input has no digits")
	}
	if pressEquals {
		equalsAction := domain.Action{
			Type:   domain.ActionSendKey,
			Key:    "=",
			Reason: "deterministic calculator equals input",
		}
		if err := r.Executor.Execute(ctx, equalsAction); err != nil {
			return nil, err
		}
		actions = append(actions, equalsAction)
	}

	return actions, nil
}

func waitWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
