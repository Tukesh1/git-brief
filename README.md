<h1 align="center"><code>git-brief</code></h1>
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

<p align="center">A CLI tool that writes your daily standup for you using your local Git history and GitHub activity.</p>

```text
$ git brief

📋 brief - Thursday, June 26

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

## How it works

1. It scans your local git commits (defaulting to yesterday, and automatically skipping weekends).
2. It fetches PRs you merged or reviewed via the GitHub API.
3. It bundles that data into a strict prompt and sends it to your LLM provider.
4. It outputs your standup to the terminal and copies it to your clipboard.

It makes exactly one API call per run. No chat interfaces, no wasted tokens.

## Features that catch edge cases

Traditional `git log` tools usually miss half of what you actually do in a day. `git-brief` is built to catch the edge cases:

* **Uncommitted work:** It runs `git status` to include files you are actively modifying.
* **Git stashes:** It checks your recent stashes for work you had to pause.
* **Pair programming:** It parses `Co-authored-by:` tags in commit messages so you get credit when pairing.
* **Rebase-aware:** It strictly looks at the commit date (`%cI`), meaning rebased commits aren't lost if the author date is old.
* **Draft PRs & Issues:** It tracks open PRs and issues you've commented on, not just what was merged.

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

Run the setup wizard to configure your workspaces and API keys:
```sh
git brief init
```

Then generate your standup:
```sh
git brief
```

## Advanced Usage

```sh
git brief                     # Generate today's standup
git brief init                # Run the setup wizard
git brief config              # Show your config (keys masked)
git brief version             # Print the installed version

# Useful Overrides
git brief --since "monday"    # Custom time range
git brief --days 3            # Look back 3 days
git brief -w ~/projects       # Scan a specific directory instead of config defaults
git brief --no-clipboard      # Print only, skip clipboard copy
```

## Slack Integration

If you want `git-brief` to post directly to Slack instead of copying to your clipboard, you have two options:

### 1. Background Send (API Token)
Set a Slack user token (`xoxp-...`) and a channel in your config (`git brief init`). It will post the standup as you.
```sh
git brief            # Prompts you to confirm, then posts
git brief --slack    # Posts immediately without confirmation
git brief --no-slack # Skips Slack entirely
```

### 2. Open Hand-off (Browser)
If you don't want to use an API token, you can just set a channel link in your config.
```sh
git brief --slack-open
```
This copies the brief to your clipboard and automatically opens the Slack app to your specified channel so you can hit paste.

## Supported AI providers

Because `git-brief` only sends commit metadata (and never your source code), context windows are extremely small and cheap.

| Provider | Model | Cost per standup |
|---|---|---|
| Google Gemini | `gemini-2.5-flash` | Free tier available |
| Anthropic | `claude-3.5-haiku` | ~$0.001 |
| OpenAI | `gpt-4o-mini` | ~$0.001 |

You bring your own API key. There is no subscription and no backend.

## Development

```sh
make build     # Compile the binary
make install   # Compile and copy to ~/.local/bin
make lint      # Format and vet the code
make clean     # Remove build artifacts
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[MIT](LICENSE)
