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

<p align="center">A CLI that writes your daily Slack standup from local git activity, GitHub PRs, and in-progress workspace work.</p>

```text
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

📋 Copied to clipboard
✅ Posted to Slack
```

## How it works

1. Scans local git commits (defaults to yesterday; skips weekends automatically), plus uncommitted and stashed work.
2. Optionally fetches PRs you merged or reviewed via the GitHub API.
3. Builds a teammate-ready Yesterday / Today / Blockers standup (one AI call, with a data-backed fallback).
4. Copies a Slack-ready brief to the clipboard, then posts as you or opens the channel to paste.

One AI call per run. No chat interfaces, no wasted tokens.

## Features that catch edge cases

Traditional `git log` tools usually miss half of what you actually do in a day. `git-brief` is built to catch the edge cases:

* **Uncommitted work:** Includes files you are actively modifying (`git status`), ignoring tooling/build noise.
* **Git stashes:** Checks recent stashes for paused work.
* **Pair programming:** Parses `Co-authored-by:` tags so you get credit when pairing.
* **Rebase-aware:** Uses committer date (`%cI`) so rebased commits are not lost.
* **Draft PRs & Issues:** Tracks open/draft PRs and issue activity, not just merges.
* **Slack delivery:** Posts as you with a user token, or opens the channel for a manual paste.

## Installation

**Using Go (Recommended)**
```sh
go install github.com/tukesh1/git-brief@latest
```

**Build from source**
```sh
git clone https://github.com/tukesh1/git-brief.git
cd git-brief
make install
```
*Note: The binary installs to `~/.local/bin/git-brief`. Make sure that folder is in your `$PATH`.*

## Quick start

```sh
git brief init   # workspaces, LLM key, optional GitHub + Slack
git brief        # generate today's standup
```

## Usage

```sh
git brief                     # Generate today's standup
git brief init                # Run the setup wizard
git brief config              # Show config path + masked contents
git brief version             # Print the installed version

# Overrides
git brief --since "monday"    # Custom time range
git brief --days 3            # Look back 3 days
git brief -w ~/projects       # Scan a specific directory
git brief --no-clipboard      # Print only, skip clipboard

# Slack
git brief --slack             # Deliver without interactive confirm
git brief --no-slack          # Skip Slack for this run
git brief --slack-open        # Open Slack to paste/send (no API post)
```

## Slack integration

### 1. Background send (API token)
Set a Slack user token (`xoxp-...` with `chat:write`) and a channel via `git brief init`. Posts the standup as you.
```sh
git brief            # Confirm, then post
git brief --slack    # Post immediately
git brief --no-slack # Skip Slack
```

### 2. Open hand-off (no token)
Set a channel link or ID in config. Copies the brief and opens Slack so you paste and send yourself.
```sh
git brief --slack-open
```

## Supported AI providers

| Provider | Model | Cost per standup |
|---|---|---|
| Google Gemini | `gemini-2.5-flash` | Free tier available |
| Anthropic | `claude-3.5-haiku` | ~$0.001 |
| OpenAI | `gpt-4o-mini` | ~$0.001 |

You bring your own API key. There is no subscription and no backend.

## Development

```sh
make build     # Build bin/git-brief with version info
make install   # Build + install to ~/.local/bin
make lint      # Format + vet + test
make test      # go test ./...
make fmt       # gofmt -w .
make clean     # Remove build artifacts
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
