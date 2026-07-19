# AGENTS.md

This file is for agentic coding tools and contributors working in this repository.

This repository is a Go CLI tool named `git-brief`.
The binary is named `git-brief` so that `git brief` works as a git sub-command.

---

## Environment

- Go version: `1.22.2` (from `go.mod`)
- Build tooling: standard Go toolchain + `Makefile`
- CLI framework: `spf13/cobra`
- Config storage: `spf13/viper` → `~/.config/git-brief/config.json` (mode `0600`)
- GitHub API: `google/go-github/v62`
- AI providers: `liushuangls/go-anthropic/v2`, `google/generative-ai-go`, `sashabaranov/go-openai`
- Slack: stdlib `net/http` client in `internal/slack` (user token optional)
- Clipboard: `atotto/clipboard`
- Terminal colour: `fatih/color`
- Interactive prompts: `AlecAivazis/survey/v2`
- TTY detection: `golang.org/x/term`

---

## Project Layout

```
git-brief/
├── main.go                     # Entry point — calls cmd.Execute()
├── cmd/
│   ├── root.go                 # Root cobra command (git brief) + Slack delivery
│   ├── init.go                 # init sub-command + runInitWizard()
│   ├── config.go               # config sub-command (show/masked config)
│   └── version.go              # version sub-command + --version flag
├── internal/
│   ├── config/
│   │   └── config.go           # Config struct, InitConfig, SaveConfig,
│   │                           #   ConfigPath(), SinceTime()
│   ├── collector/
│   │   ├── git.go              # git log reader (os/exec)
│   │   └── github.go           # GitHub PR fetcher (go-github)
│   ├── ai/
│   │   └── summarize.go        # Anthropic / Gemini / OpenAI adapters
│   ├── slack/
│   │   ├── slack.go            # Slack Web API + channel open hand-off
│   │   └── slack_test.go       # httptest-backed unit tests
│   ├── prompt/
│   │   ├── system_prompt.txt   # Embedded plain-text AI instructions
│   │   └── prompt.go           # go:embed exposure
│   └── output/
│       ├── format.go           # Terminal output with section colouring
│       └── clipboard.go        # clipboard.WriteAll wrapper
├── Makefile
├── go.mod
└── go.sum
```

---

## Primary Commands

```bash
# Build the binary
make build                         # → ./bin/git-brief

# Install to ~/.local/bin (no sudo)
make install

# Uninstall
make uninstall

# Format + vet + test
make lint

# Plain format
make fmt                           # gofmt -w .

# Vet only
make vet                           # go vet ./...

# Unit tests
make test                          # go test ./...

# Clean build output
make clean

# Direct build (plain, no ldflags)
go build -o ./bin/git-brief .
```

---

## User-Facing Commands

```bash
git brief                         # Generate today's standup brief
git brief init                    # Interactive setup wizard
git brief config                  # Show config path + masked contents
git brief version                 # Print version
git brief --version               # Same (cobra convention)
git brief --since "monday"        # Override time range
git brief --days 3                # Last 3 working days
git brief -w ~/projects           # Override workspace directory
git brief --no-clipboard          # Print without copying to clipboard
git brief --slack                 # Slack delivery without interactive confirm
git brief --no-slack              # Skip Slack even if configured
git brief --slack-open            # Force open/paste hand-off (no API post)
```

---

## Design Constraints

### Context & Cancellation
- Thread `context.Context` through all long-running work.
- Use `exec.CommandContext` for all subprocess calls (git, etc.) so they are
  cancelled when the parent context expires.
- AI calls use a 60-second timeout derived from the parent context.
- The root command uses a 2-minute top-level timeout.

### Layer Boundaries
- `internal/collector` must not import `internal/output` or `fatih/color`.
  Warnings are returned as `collector.Warning` values or printed via `fmt`.
- `internal/ai` imports `internal/collector` for its data types — this is
  intentional and acceptable.
- `internal/slack` must not import `internal/output` or `fatih/color`.
- `internal/config` has no dependencies on other internal packages.

### Date Logic
- `config.SinceTime(days int) time.Time` is the single source of truth for
  computing the "since" boundary. Both the git collector and GitHub collector
  use it.
- The `--since` string flag is passed verbatim to `git log --since=` (git
  parses natural-language dates natively). GitHub always uses the computed
  `time.Time` because its API requires an ISO date.

### Config
- Config file lives at `~/.config/git-brief/config.json`.
- `config.ConfigPath()` is the single source of truth for this path.
- `SaveConfig` writes the file with mode `0600`.
- API keys are never printed in plain text; use `maskKey()` in cmd/config.go.

### Slack (primary delivery target)
- Briefs are written as Slack standups (Yesterday / Today / Blockers).
- Clipboard and API post use `output.ForSlack` (section headers as `*Yesterday:*`).
- Channel can be a pasted Slack link, channel ID, or `#name`.
- With an `xoxp-` user token + `chat:write`, background post via
  `chat.postMessage` (as the user). Without a token, open the channel for
  manual paste/send.
- Interactive confirm on TTY unless `--slack`; skipped entirely on non-TTY
  unless `--slack`. `--no-slack` disables delivery.

### Standup quality
- Commits are sorted newest-first, noise-filtered, date-bucketed before the LLM.
- Local uncommitted/stash is first-class Today signal; compressed when commit-rich.
- Temperature 0 on all providers for stable output.

### Error Handling
- Prefer `RunE` (returns `error`) over `Run` in cobra commands.
- Do NOT call `os.Exit` inside library functions or survey handlers.
- Wrap errors with context: `fmt.Errorf("git log: %w", err)`.

---

## When Making Changes

- Run `make lint` before committing.
- Run `go build ./...` to verify the build.
- If adding a new dependency, document it in this file and in `go.mod`.
