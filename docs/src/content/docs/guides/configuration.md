---
title: Configuration
description: Manual overrides and JSON schema.
---

The setup wizard writes your configuration to:
`~/.config/git-brief/config.json`

If you need to add multiple workspaces or manually update your API keys, you can edit this file directly.

## JSON Schema

```json
{
  "workspaces": [
    "/home/user/projects",
    "/home/user/go/src"
  ],
  "llm_provider": "gemini",
  "openai_api_key": "",
  "anthropic_api_key": "",
  "gemini_api_key": "your-key-here",
  "github_token": "ghp_your_token_here",
  "github_username": "your-username",
  "author": "Your Name",
  "email": "youremail@example.com"
}
```

## Config Keys

| Key | Description |
|---|---|
| `workspaces` | An array of absolute paths. The git collector scans these. |
| `llm_provider` | Must be `gemini`, `anthropic`, or `openai`. |
| `github_token` | Optional. Used to fetch PRs and Issues. |
| `github_username` | Optional. Used to scope GitHub API queries to your activity. |
| `author` & `email` | Optional overrides. If empty, `git-brief` reads your global `~/.gitconfig`. |

## Viewing Config Safely

To verify your configuration without dumping raw API keys to your terminal, use:

```bash
git brief config
```

This will print the file path and mask your secrets (e.g., `sk-ant...a42`).
