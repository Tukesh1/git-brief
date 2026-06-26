---
title: Edge Cases
description: How git-brief handles stashes, uncommitted code, and pairing.
---

Standard `git log` commands are blind to a massive chunk of modern developer workflows. `git-brief` is explicitly designed to catch them.

## 1. Pair Programming

If you spend all day pair-programming on a colleague's machine, or they push the commit from their terminal, standard `git log --author=you` will show you doing absolutely nothing that day.

`git-brief` parses commit message bodies for GitHub's standard `Co-authored-by:` trailer. 

If your coworker includes this in their commit message:
```text
feat: rewrite auth flow

Co-authored-by: Your Name <your.email@example.com>
```

`git-brief` will attribute that commit to your daily standup.

## 2. Uncommitted IDE Work

You spent 6 hours refactoring a legacy service. The code isn't compiling yet, so you haven't committed.

Because `git-brief` executes `git status --porcelain` on your workspaces, it sees those modified files and feeds them to the LLM. 

Instead of an empty "Yesterday" section, your brief will automatically say:
`• Continuing refactor in /services/legacy/auth.go`

## 3. Git Stashes

You are halfway through a feature when PagerDuty goes off. You run `git stash`, checkout `main`, and fix the bug.

The next morning, you need to report on both the bug fix *and* the WIP feature you started. `git-brief` parses `git log -g refs/stash` to ensure your shelved work is injected into the prompt.
