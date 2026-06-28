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
```

---

## Design Constraints

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

---

## Cursor Cloud specific instructions

- Go `1.22.x` is pre-installed in the base image; the startup update script only runs `go mod download`. There are no system services, databases, or containers to start.
- There are no automated tests (`*_test.go`) in this repo. "Testing" here means `make lint` (`gofmt -w .` + `go vet ./...`) and `make build` / `go build ./...`, matching CI in `.github/workflows/go.yml`.
- Run the CLI locally without installing via `./bin/git-brief <cmd>` after `make build`. `version`, `--version`, `config`, and `--help` work with no configuration.
- Config and secrets are NOT read from environment variables. The app reads only `~/.config/git-brief/config.json` (see `internal/config/config.go`). To use an LLM/GitHub key, either run the interactive `git brief init` wizard or write the JSON file directly (keys: `gemini_api_key`, `anthropic_api_key`, `openai_api_key`, `github_token`, plus `llm_provider`). Adding a shell env secret alone will not be picked up.
- `git brief init` uses interactive `survey` prompts and will fail without a TTY; in a non-interactive cloud shell, write `config.json` directly instead of running the wizard.
- A bare `git brief` (no API key configured) auto-launches the interactive wizard, which blocks/fails in a non-interactive shell. Pre-populate `config.json` first to avoid this.
- The standup-brief flow (`git brief`) makes a live network call to the configured LLM provider (Gemini/Anthropic/OpenAI). Generating an actual brief requires a valid third-party API key; without one the git/GitHub collection still runs but the run ends with an `AI: ... API key not valid` error at the AI step.
- The collector filters commits to the configured `author`/`email` (falling back to local `git config user.name/email`). When testing collection, set those to match the repo's commit authors or it will report "No commits ... found".
