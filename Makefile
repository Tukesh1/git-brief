BINARY    := git-brief
MODULE    := github.com/tukesh1/git-brief
CMD_PKG   := .

# Installation directory — prefers ~/.local/bin so no sudo is needed.
INSTALL_DIR ?= $(HOME)/.local/bin

# Version: use git tag if available, otherwise the short SHA.
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X $(MODULE)/cmd.Version=$(VERSION)

BIN_DIR   := ./bin

.PHONY: all build install uninstall fmt vet clean help

all: build ## Default: build the binary

## ── Build ────────────────────────────────────────────────────────────────────

build: ## Compile the binary into ./bin/git-brief
	@mkdir -p $(BIN_DIR)
	@go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD_PKG)

## ── Install ──────────────────────────────────────────────────────────────────

install: build ## Install git-brief to $(INSTALL_DIR)
	@mkdir -p $(INSTALL_DIR)
	@cp $(BIN_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "✅ Installed. Next step: run 'git brief init'"

uninstall: ## Remove git-brief from $(INSTALL_DIR)
	@rm -f $(INSTALL_DIR)/$(BINARY)

## ── Code quality ─────────────────────────────────────────────────────────────

fmt: ## Format all Go source files
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: fmt vet ## Run formatter + vet

## ── Clean ────────────────────────────────────────────────────────────────────

clean: ## Remove the ./bin directory
	rm -rf $(BIN_DIR)

## ── Help ─────────────────────────────────────────────────────────────────────

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
