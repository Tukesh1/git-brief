---
title: Installation
description: How to install git-brief on your machine.
---

`git-brief` is a single compiled binary written in Go.

## Method 1: Go Install (Recommended)

If you have Go installed on your machine, you can install the latest version directly:

```bash
go install github.com/tukesh1/git-brief@latest
```

Make sure your Go `bin` directory is in your `PATH`. If your terminal says `git-brief: command not found`, add this to your `~/.bashrc` or `~/.zshrc`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Method 2: Build from Source

If you want to build the binary yourself:

```bash
git clone https://github.com/tukesh1/git-brief.git
cd git-brief
make install
```

This compiles the binary and copies it to `~/.local/bin/git-brief`. 

Make sure `~/.local/bin` is in your `PATH`:

```bash
export PATH="$PATH:$HOME/.local/bin"
```

## Verify Installation

```bash
git brief --version
```
