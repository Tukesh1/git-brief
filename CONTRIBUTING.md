# Contributing to git-brief

Thank you for considering contributing to `git-brief`!

## Getting Started

1. **Fork & clone** the repository.
2. **Build** the project:
   ```bash
   make build
   ```
3. **Run locally** without installing:
   ```bash
   ./bin/git-brief --help
   ./bin/git-brief init
   ```
4. **Install** for local testing:
   ```bash
   make install
   git brief --version
   ```

## Development Workflow

```bash
# Format code
make fmt

# Run linter
make lint

# Build
make build

# Clean build artifacts
make clean
```

## Project Architecture

The project is built in **Go** with the standard project layout:

| Directory | Purpose |
|---|---|
| `cmd/` | Cobra CLI commands (root, init, config, version) |
| `internal/config/` | Viper configuration + path/date helpers |
| `internal/collector/` | Git log reader + GitHub PR fetcher |
| `internal/ai/` | LLM provider adapters (Anthropic, Gemini, OpenAI) |
| `internal/output/` | Terminal formatting + clipboard |

See [AGENTS.md](AGENTS.md) for detailed design constraints and layer boundaries.

## Pull Request Guidelines

- Run `make lint` before pushing.
- Run `go build ./...` to verify the build.
- Keep changes focused — one feature or fix per PR.
- If adding a new dependency, document it in `AGENTS.md`.
