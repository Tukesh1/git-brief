---
title: CLI Commands
description: Exhaustive reference for all git-brief commands and flags.
---

## The Root Command

```bash
git brief [flags]
```

Executing `git brief` with no arguments runs the standard data collectors and generates your standup for "Yesterday" and "Today". 

| Flag | Description | Default |
|---|---|---|
| `-w, --workspace` | Override the config file and scan a specific directory. You can pass this multiple times. | `config.json` workspaces |
| `-d, --days` | Number of days to look back for "Yesterday". Automatically extends to 3 on Mondays. | `1` |
| `--since` | Direct override for the Git/GitHub timestamp (e.g., `yesterday`). | Automatically calculated |
| `--no-clipboard` | Print to stdout but do not copy to your OS clipboard. | `false` |

## Subcommands

### `git brief init`

Runs the interactive setup wizard to configure your workspaces, LLM provider, and API keys.

### `git brief config`

Prints your active configuration path and safely masks all API tokens. Use this to verify `git-brief` is reading from the correct workspace.

### `git brief version`

Prints the compiled binary version.

## Advanced Usage Examples

**Generate a brief for a completely unconfigured repository:**
```bash
git brief -w /path/to/some/repo
```

**Generate a brief for multiple distinct folders:**
```bash
git brief -w ~/projects/frontend -w ~/projects/backend
```

**Generate a brief spanning the last week (e.g. returning from vacation):**
```bash
git brief --days 7
```

**Print output to terminal only (useful for CI/CD or scripts):**
```bash
git brief --no-clipboard > standup.txt
```
