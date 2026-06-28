# AGENTS.md

This file is for agentic coding tools and contributors working in this repository.

This repository is a Go CLI tool named `git-brief`.
The binary is named `git-brief` so that `git brief` works as a git sub-command.

---

## Environment

- Go version: `1.22.2` (from `go.mod`)
- Build tooling: standard Go toolchain + `Makefile`
- CLI framework: `spf13/cobra`
- Config storage: `spf13/viper` → `~/.config/git-brief/config.json`
- GitHub API: `google/go-github/v62`
- AI providers: `liushuangls/go-anthropic/v2`, `google/generative-ai-go`, `sashabaranov/go-openai`
- Clipboard: `atotto/clipboard`
- Terminal colour: `fatih/color`
- Interactive prompts: `AlecAivazis/survey/v2`
- Terminal detection: `golang.org/x/term` (TTY check before interactive prompts)
- Slack hand-off: standard library `net/http` (no SDK)

---

## Project Layout

```
git-brief/
├── main.go                     # Entry point — calls cmd.Execute()
├── cmd/
│   ├── root.go                 # Root cobra command (git brief)
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
│   ├── slack/
│   │   ├── slack.go            # Channel resolve + deep links + open client
│   │   └── slack_test.go       # httptest tests (SLACK_API_BASE override)
│   ├── ai/
│   │   └── summarize.go        # Anthropic / Gemini / OpenAI adapters
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

# Format + vet
make lint

# Plain format
make fmt                           # gofmt -w .

# Vet only
make vet                           # go vet ./...

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
git brief --slack                 # Open configured Slack channel to post (no prompt)
git brief --no-slack              # Never open Slack
```

---

## Design Constraints

### Slack hand-off (manual send)
- `git-brief` MUST NOT post to Slack itself. The flow is: copy the brief to the
  clipboard → open the user's Slack client in the target channel → the user
  pastes and presses send. This keeps messages posted **as the user, never as a
  bot**, and gated behind explicit manual approval.
- The Slack token is **optional and read-only**. Posting uses the user's
  existing Slack session (they paste + press send), so no bot, no `chat:write`,
  and no workspace-admin app-install is required to post.
- Preferred config is a pasted channel link (`…/archives/C…`) or a bare channel
  ID; `slack.ParseChannelURL` extracts the IDs with no API call, so the
  non-admin/no-token path is first-class. A token is only needed to resolve a
  `#name`.
- `internal/slack` uses the user token (`xoxp-…`) only for lookups
  (`auth.test` for the team ID, `conversations.list` to resolve a `#name` to an
  ID). It never calls `chat.postMessage`.
- Slack deep links cannot prefill the compose box (confirmed via Slack docs);
  the clipboard is the transport, the deep link only navigates.
- The Slack API base URL is overridable via the `SLACK_API_BASE` env var so the
  integration is testable against an `httptest` mock server.
- The interactive "open Slack?" prompt only runs on a real TTY (checked with
  `golang.org/x/term`); use `--slack` to opt in non-interactively.

### Context & Cancellation
- Thread `context.Context` through all long-running work.
- Use `exec.CommandContext` for all subprocess calls (git, etc.) so they are
  cancelled when the parent context expires.
- AI calls use a 60-second timeout context.
- The root command uses a 2-minute top-level timeout.

### Layer Boundaries
- `internal/collector` must not import `internal/output` or `fatih/color`.
  Warnings are returned as `collector.Warning` values or printed via `fmt`.
- `internal/ai` imports `internal/collector` for its data types — this is
  intentional and acceptable.
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
- API keys are never printed in plain text; use `maskKey()` in cmd/config.go.

### Error Handling
- Prefer `RunE` (returns `error`) over `Run` in cobra commands.
- Do NOT call `os.Exit` inside library functions or survey handlers.
- Wrap errors with context: `fmt.Errorf("git log: %w", err)`.

---

## When Making Changes

- Run `make lint` before committing.
- Run `go build ./...` to verify the build.
- If adding a new dependency, document it in this file and in `go.mod`.
