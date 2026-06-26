<h1 align="center"><code>git brief</code></h1>
<p align="center">
  <img
    alt="Platform"
    src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue?style=flat-square"
  />
  <a href="LICENSE"
    ><img
      alt="License"
      src="https://img.shields.io/github/license/tukesh1/git-brief?style=flat-square"
  /></a>
</p>

<h3 align="center">Stop writing standup. You already logged the work..</h3>

<br />

```
$ git brief

📋 brief — Thursday, June 26

Yesterday:
  • Fixed auth token expiry bug in /api/refresh (PR #234 merged)
  • Reviewed 2 PRs: payment-service, user-onboarding
  • Pushed rate limiter skeleton to feature/rate-limit

Today:
  • Finishing Redis TTL fallback in rate limiter
  • Pick up caching ticket (#301)

Blockers:
  None

📋 Copied to clipboard. Paste into Slack.
```

One command. Three seconds. Your standup is written and copied to clipboard.

## How it works

```
  git brief runs
       │
       ├── scans your git commits from the last 24hrs
       │   across all your local repositories
       │
       ├── fetches PRs you merged or reviewed on GitHub
       │
       └── sends one prompt to your configured AI provider
                │
                └── AI writes Yesterday / Today / Blockers
                         │
                         └── output printed + copied to clipboard
```

One AI call per run. No chat, no back-and-forth, no wasted tokens.

### ✨ What it catches that others miss:
- **Uncommitted work:** Captures modified files from your IDE using `git status`.
- **Git Stashes:** Finds recent work you had to `git stash` and switch away from.
- **Pair Programming:** Parses `Co-authored-by:` tags so you get credit when pairing.
- **Rebase-aware:** Uses `Commit Date` instead of just Author Date so rebased commits aren't lost.
- **GitHub Issues & Draft PRs:** Finds Open/Draft PRs you're working on and issues you've commented on, not just merged PRs.

## Install

### Go install (recommended)

```sh
go install github.com/tukesh1/git-brief@latest
```

### Build from source

```sh
git clone https://github.com/tukesh1/git-brief.git
cd git-brief
make install
```

The binary is installed to `~/.local/bin/git-brief`. Make sure `~/.local/bin` is in your PATH.

## Quick Start

```sh
# One-time setup (takes 60 seconds)
$ git brief init

  Welcome to git-brief setup!

  ? LLM provider: Google 
  ? Gemini API Key: ********
  ? Enable GitHub PR integration? Yes
  ? GitHub Personal Access Token: ********
  ? GitHub username: tukesh1

  ✅ Setup complete!

# Generate your standup every morning
$ git brief
```

## Usage

```sh
git brief                     # Generate today's standup
git brief init                # Run the setup wizard
git brief config              # Show config path + masked contents
git brief version             # Print version
git brief --version           # Same

# Overrides
git brief --since "monday"    # Custom time range
git brief --days 3            # Last 3 days (returning from PTO)
git brief -w ~/projects       # Override workspace directory
git brief --no-clipboard      # Print only, skip clipboard
```

## Supported AI Providers

| Provider | Model | Cost per standup |
|---|---|---|
| **Google Gemini** | gemini-2.5-flash | Free tier available |
| **Anthropic** | claude-3.5-haiku | ~$0.001 |
| **OpenAI** | gpt-4o-mini | ~$0.001 |

You bring your own API key. No subscription, no backend, no server costs.

## Configuration

Config is stored at `~/.config/git-brief/config.json`.

```sh
# View your current config (API keys are masked)
$ git brief config

# Re-run the setup wizard
$ git brief init
```

## Development

```sh
make build     # Build bin/git-brief with version info
make install   # Build + install to ~/.local/bin
make lint      # Format + vet
make fmt       # gofmt -w .
make clean     # Remove build artifacts
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full contributor guide.

## License

[MIT](LICENSE)
