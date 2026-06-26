---
title: AI Generation
description: The constraints and rules applied to the LLM.
---

Once the collectors bundle your git metadata and GitHub activity, `git-brief` injects it into a highly constrained System Prompt.

## The Bottleneck

Language models are verbose. If you pass a raw git log to an LLM, it will output a 3-paragraph essay about how "The developer significantly improved the authentication module by refactoring...".

That is useless for a daily standup. Standups require punchy, action-oriented bullet points.

## The System Prompt Rules

To enforce a high-signal, low-noise output, the embedded system prompt enforces strict rules on the LLM:

* **Action Verbs Only:** Pronouns ("I") are forbidden. Bullets must start with verbs like "Fixed", "Reviewed", or "Pushed".
* **Extreme Specificity:** It must extract exact PR numbers (e.g., `PR #234`), branch names, or file paths from the context bundle.
* **Timestamp Enforcement:** It is strictly forbidden from placing a commit dated "Today" into the "Yesterday" bucket.
* **Length Limits:** Maximum 3 bullet points per section. The total output is hard-capped at 100 words.

## Token Costs

Because the prompt relies exclusively on metadata (commit messages, PR titles, uncommitted file paths) and *never* your actual source code, the context window is tiny.

A typical `git brief` run consumes ~500 input tokens and ~100 output tokens.

| Provider | Model | Cost per Run |
|---|---|---|
| Google Gemini | `gemini-2.5-flash` | Free |
| Anthropic | `claude-3-5-haiku-latest` | ~$0.001 |
| OpenAI | `gpt-4o-mini` | ~$0.001 |
