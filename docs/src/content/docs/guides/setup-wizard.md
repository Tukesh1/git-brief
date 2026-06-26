---
title: Setup Wizard
description: Running the interactive init command.
---

`git-brief` requires a one-time configuration to know where your repositories live and which LLM provider to use.

You run the wizard by executing:

```bash
git brief init
```

## The Prompts

1. **Workspace Path**: The root directory where your code lives (e.g., `~/projects`). The Git collector will recursively scan this folder to find all your local repositories.
2. **LLM Provider**: Choose between Gemini, Anthropic, or OpenAI.
3. **API Key**: Paste your respective API key. The input is masked in your terminal.
4. **GitHub Token (Optional)**: If you want PR and Issue tracking, paste a Classic Personal Access Token (with `repo` scope).

Once completed, it writes your settings to a local JSON file. You never need to run this command again unless you want to change your provider or add a new workspace.
