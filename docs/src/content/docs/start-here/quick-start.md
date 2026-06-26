---
title: Quick Start
description: Get your first standup generated in 60 seconds.
---

## 1. Run the wizard

Initialize `git-brief` by running the setup wizard:

```bash
git brief init
```

This will ask you three things:
1. **Your workspace directory:** (e.g., `~/projects`). This is where it will scan for your git repos.
2. **Your LLM provider:** Choose Gemini (free), Anthropic, or OpenAI, and paste your API key.
3. **GitHub integration:** (Optional) Provide a GitHub Personal Access Token so it can fetch your PRs and Issues.

*Note: Your API keys are stored securely on your local filesystem at `~/.config/git-brief/config.json`.*

## 2. Generate your standup

Just run the command from anywhere on your machine:

```bash
git brief
```

```text
📋 brief — Thursday, June 26
─────────────────────────────────

Yesterday:
  • Fixed auth token expiry bug in /api/refresh (PR #234 merged)
  • Reviewed 2 PRs: payment-service, user-onboarding

Today:
  • Finishing Redis TTL fallback in rate limiter

Blockers:
  None

─────────────────────────────────
📋 Copied to clipboard. Paste into Slack.
```

Your standup is now in your clipboard. Paste it into Slack. You're done.
