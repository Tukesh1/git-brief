---
title: Supported AI Providers
description: Token costs and API key generation.
---

`git-brief` supports three major AI providers. Because the tool relies entirely on local Git metadata (commit messages, file paths) and *not* your source code, context windows are incredibly small. 

A typical `git brief` execution consumes roughly **~500 input tokens** and **~100 output tokens**.

## Google Gemini (Recommended)

Google's `gemini-2.5-flash` model is incredibly fast and completely free for this level of usage.

* **Cost per run:** $0.00 (Free Tier)
* **Get an API Key:** [Google AI Studio](https://aistudio.google.com/app/apikey)

## Anthropic Claude

Anthropic's `claude-3-5-haiku-latest` is their fastest, most cost-effective model, explicitly tuned for rapid text processing.

* **Cost per run:** ~$0.00015 (Input) + ~$0.000125 (Output) = **~$0.00027**
* **Cost for 1 year of daily standups:** **~$0.07**
* **Get an API Key:** [Anthropic Console](https://console.anthropic.com/settings/keys)

## OpenAI

OpenAI's `gpt-4o-mini` is their flagship small model.

* **Cost per run:** ~$0.000075 (Input) + ~$0.00006 (Output) = **~$0.000135**
* **Cost for 1 year of daily standups:** **~$0.03**
* **Get an API Key:** [OpenAI Platform](https://platform.openai.com/api-keys)
