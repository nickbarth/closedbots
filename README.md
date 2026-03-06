# ClosedBots ⛔🤖 

<img width="1108" height="681" alt="image" src="https://github.com/user-attachments/assets/a8f162a4-decb-4c65-9809-96ec6d5b79cb" />

Closed Bots is a simple open desktop automation app. It's Open, MIT, and cross-platform (Windows, macOS, Linux).
Add a new Bot with a summary and steps, then run it with your choice of LLM runner (Codex CLI, Claude CLI, Ollama CLI).
Sometimes it works! I take no responsibility for what it does to your computer or your privacy (it sends screenshots to
your selected LLM). Use the global stop hotkey if it goes rogue.

## How it works

It manages "tasks" defined as markdown files with a summary and steps. You can edit these tasks in-app or generate steps
from the summary using an LLM runner.

When you run a task, the app executes each step sequentially using the selected runner. It captures screenshots before
and after each step, logs progress, and allows you to stop execution with a global hotkey. Your choses LLM will process the screenshots.

## Features

- Task editor (summary + numbered steps)
- AI step generation from a summary
- Run once or run on a recurring interval
- Per-step progress with `Step X / X`
- Automatic screenshot capture during runs
- Global stop hotkey (default: `Ctrl+Shift+S`)
- Runner switcher in-app:
  - `Codex CLI`
  - `Claude CLI`
  - `Ollama CLI`

## Requirements

- Go `1.26+`
- Interactive desktop session (not headless)
- A working runner CLI on `PATH`:
  - `codex` for Codex CLI runner
  - `claude` for Claude CLI runner
  - `ollama` for Ollama CLI runner (with at least one local model)

## Quick Start

1. Launch Closed Bots executable or build from source with `go build -o closed-bots .` and run `./closed-bots`.
2. In the `Bots` tab, click `New` or `Open` an existing task.
3. In the `Task` tab:
   - Set `Summary`
   - Add/edit `Steps (point form)` or click `Generate Steps`
   - Click `Save`
4. Choose runner from the top-right `Runner` dropdown.
5. Choose a `Run Interval`:
   - `Run Once`
   - or recurring options (every minute/hour/day, etc.)
6. Click `Run`.

Run interval preference is not persisted. It resets to `Run Once` when the app starts and when you create/open/import tasks.

## Task File Format

Tasks are markdown files in `tasks/`:

```md
# Summary
Open Calculator and compute 1336 + 1

# Steps
1. Open Calculator.
2. Enter 1336.
3. Press +.
4. Enter 1 and press =.
5. Verify result is 1337.
```

The app reads/writes these files directly.

## Project Data Layout

- `tasks/`:
  - task markdown files (`*.md`)
- `runs/`:
  - run folders with `run.jsonl`
  - step screenshots (`step_XXX_pre.png`, `step_XXX_post.png`)
- `task.log`:
  - app/provider log stream
- `config/settings.json`:
  - persisted runner + hotkey settings

## Runner Behavior

- `Codex CLI`: uses `codex exec`
- `Claude CLI`: uses `claude -p`
- `Ollama CLI`: uses `ollama run`

## Disclaimer

This is a personal project for fun and learning. It may have bugs, security issues, or cause unintended consequences.
Always review generated steps before running, and use the global stop hotkey if something goes wrong. I am not
responsible for any damage or data loss that may occur.

## License

MIT License
