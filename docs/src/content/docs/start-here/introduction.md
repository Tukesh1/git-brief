---
title: Introduction
description: Why git-brief exists.
---

## The standup problem

Daily standups are supposed to take 5 minutes, but you spend 15 minutes trying to remember what you actually did yesterday. You scour your Slack messages, your Jira board, and run `git log`. 

But `git log` lies to you.

| What actually happened | What `git log` shows |
|---|---|
| You spent 4 hours in your IDE debugging | Nothing |
| You stashed your WIP to fix a hotfix | Nothing |
| You pair-programmed and your coworker pushed the commit | Nothing |
| You opened 3 Draft PRs | Nothing |

## The solution

`git-brief` sits locally on your machine and generates your standup in 3 seconds. 

It reads your uncommitted files, your git stashes, your `Co-authored-by:` tags, and your GitHub PRs. Then, it uses a local or cheap API LLM to format it into a clean, bulleted list.

It doesn't upload your codebase to a remote server. It just reads your git metadata, bundles it, and writes your standup directly to your clipboard.

```
Yesterday:
  • Fixed auth token expiry bug in /api/refresh (PR #234 merged)
  • Pushed rate limiter skeleton to feature/rate-limit

Today:
  • Finishing Redis TTL fallback in rate limiter
```

Zero overhead. Type `git brief`, paste into Slack, and get back to work.
