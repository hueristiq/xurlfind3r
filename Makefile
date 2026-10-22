SHELL = /bin/sh

MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

COLOR_BLUE := \033[34m
COLOR_BOLD := \033[1m
COLOR_CYAN := \033[36m
COLOR_GREEN := \033[32m
COLOR_RED := \033[31m
COLOR_RESET := \033[0m
COLOR_YELLOW := \033[33m

ICON_HEADER := >
ICON_NONE := ◌
ICON_INFO := 🛈
ICON_WARNING := !
ICON_ERROR := ✗
ICON_SUCCESS := ✓

define log_header
	echo "$(COLOR_CYAN)$(ICON_HEADER) $(1)$(COLOR_RESET)"
endef

define log_info
	echo "$(COLOR_BLUE)$(ICON_INFO) $(1)$(COLOR_RESET)"
endef

define log_warning
	echo "$(COLOR_YELLOW)$(ICON_WARNING) $(1)$(COLOR_RESET)"
endef

define log_error
	echo "$(COLOR_RED)$(ICON_ERROR) $(1)$(COLOR_RESET)"
endef

define log_success
	echo "$(COLOR_GREEN)$(ICON_SUCCESS) $(1)$(COLOR_RESET)"
endef

BIN_DIR = $(CURDIR)/.bin

GOLANGCI_LINT_BIN_VER = 2.12.0
GOLANGCI_LINT_BIN_DIR = $(BIN_DIR)/golangci-lint/$(GOLANGCI_LINT_BIN_VER)

GOLANGCI_LINT_BIN = $(GOLANGCI_LINT_BIN_DIR)/golangci-lint

LEFTHOOK_BIN_VER = 2.1.6
LEFTHOOK_BIN_DIR = $(BIN_DIR)/lefthook/$(LEFTHOOK_BIN_VER)

LEFTHOOK_BIN = $(LEFTHOOK_BIN_DIR)/lefthook

NAME := xurlfind3r

SEMVER ?= $(shell { git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0"; } | sed 's/^v//')
BRANCH ?= $(shell { git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "branch"; } | sed 's/^HEAD$$/main/')
SHA ?= $(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "00000000")
VERSION ?= v$(SEMVER)-$(BRANCH)-$(SHA)

.PHONY: install-golangci-lint install-lefthook

install-golangci-lint:
	@$(call log_header,Installing golangci-lint...)
	@if command -v $(GOLANGCI_LINT_BIN) >/dev/null 2>&1; \
	then \
		$(call log_warning,Skipped Installing golangci-lint: already installed!); \
	else \
		[ -d "$(GOLANGCI_LINT_BIN_DIR)" ] || mkdir -p $(GOLANGCI_LINT_BIN_DIR); \
		TMP_DIR=$$(mktemp -d); \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		ARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
		curl -sSL "https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_BIN_VER)/golangci-lint-$(GOLANGCI_LINT_BIN_VER)-$$OS-$$ARCH.tar.gz" -o "$$TMP_DIR/golangci-lint.tar.gz" && \
		tar -xzf "$$TMP_DIR/golangci-lint.tar.gz" -C "$$TMP_DIR" && \
		mv "$$TMP_DIR/golangci-lint-$(GOLANGCI_LINT_BIN_VER)-$$OS-$$ARCH/golangci-lint" "$(GOLANGCI_LINT_BIN)" && \
		chmod +x "$(GOLANGCI_LINT_BIN)" && \
		rm -rf "$$TMP_DIR"; \
	fi
	@$(call log_success,Installing golangci-lint...done!)

install-lefthook:
	@$(call log_header,Installing lefthook...)
	@if command -v $(LEFTHOOK_BIN) >/dev/null 2>&1; \
	then \
		$(call log_warning,Skipped Installing lefthook: already installed!); \
	else \
		[ -d "$(LEFTHOOK_BIN_DIR)" ] || mkdir -p $(LEFTHOOK_BIN_DIR); \
		curl -sSL "https://github.com/evilmartians/lefthook/releases/download/v$(LEFTHOOK_BIN_VER)/lefthook_$(LEFTHOOK_BIN_VER)_$$(uname -s)_$$(uname -m | sed 's/arm64/aarch64/')" -o "$(LEFTHOOK_BIN)" && \
		chmod +x "$(LEFTHOOK_BIN)"; \
	fi
	@$(call log_success,Installing lefthook...done!)

.PHONY: lefthook-initialize

lefthook-initialize:
	@$(call log_header,Initializing lefthook...)
	@if git rev-parse --git-dir >/dev/null 2>&1; \
	then \
		$(LEFTHOOK_BIN) install; \
	else \
		$(call log_warning,Skipped Initializing lefthook: git repository not found!); \
	fi
	@$(call log_success,Initializing lefthook...done!)

.PHONY: go-mod-clean go-mod-tidy go-mod-update go-fmt go-lint go-test go-build go-install

go-mod-clean:
	@$(call log_header,Cleaning Go module cache...)
	@go clean -modcache || ( $(call log_error,Failed Cleaning Go module cache) && exit 1 )
	@$(call log_success,Cleaning Go module cache...done!)

go-mod-tidy:
	@$(call log_header,Tidying Go modules...)
	@go mod tidy || ( $(call log_error,Failed Tidying Go modules!) && exit 1 )
	@$(call log_success,Tidying Go modules...done!)

go-mod-update:
	@$(call log_header,Updating Go modules...)
	@go get -f -t -u ./... || ( $(call log_error,Failed Updating Go modules (step 1)!) && exit 1 )
	@go get -f -u ./... || ( $(call log_error,Failed Updating Go modules (step 2)!) && exit 1 )
	@$(call log_success,Updating Go modules...done!)

go-fmt: install-golangci-lint
	@$(call log_header,Formatting Go code...)
	@$(GOLANGCI_LINT_BIN) fmt ./... || ( $(call log_error,Failed Formatting Go code!) && exit 1 )
	@$(call log_success,Formatting Go code...done!)

go-lint: go-fmt
	@$(call log_header,Linting Go code...)
	@$(GOLANGCI_LINT_BIN) run ./... || ( $(call log_error,Failed Linting Go code!) && exit 1 )
	@$(call log_success,Linting Go code...done!)

go-test:
	@$(call log_header,Running Go tests...)
	@go test -v -race ./... || ( $(call log_error,Failed Running Go tests!) && exit 1 )
	@$(call log_success,Running Go tests...done!)

go-build:
	@$(call log_header,Building Go program...)
	@go build -v -ldflags "-s -w -X github.com/hueristiq/$(NAME)/internal/configuration.Version=$(VERSION)" -o bin/$(NAME) ./cmd/$(NAME) || ( $(call log_error,Failed Building Go program!) && exit 1 )
	@$(call log_success,Building Go program...done!)

go-install:
	@$(call log_header,Installing Go program...)
	@go install -v ./... || ( $(call log_error,Failed Installing Go program!) && exit 1 )
	@$(call log_success,Installing Go program...done!)

DOCKERFILE := ./Dockerfile

IMAGE_NAME = hueristiq/$(NAME)
IMAGE_TAG = $(VERSION)
IMAGE = $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: docker-build

docker-build:
	@$(call log_header,Building Docker image...)
	@docker build -f $(DOCKERFILE) -t $(IMAGE) -t $(IMAGE_NAME):latest . || ( $(call log_error,Failed Building Docker image!) && exit 1 )
	@$(call log_success,Building Docker image...done!)

.PHONY: help
help:
	@echo ""
	@echo "$(COLOR_BOLD)$(COLOR_CYAN)Usage:$(COLOR_RESET)"
	@echo ""
	@echo "  make <target>"
	@echo ""
	@echo "$(COLOR_BOLD)$(COLOR_CYAN)Targets:$(COLOR_RESET)"
	@echo " $(COLOR_BOLD)$(COLOR_CYAN)...install:$(COLOR_RESET)"
	@echo "   $(COLOR_GREEN)install-golangci-lint$(COLOR_RESET) ....... Install golangci-lint."
	@echo "   $(COLOR_GREEN)install-lefthook$(COLOR_RESET) ............ Install lefthook."
	@echo ""
	@echo " $(COLOR_BOLD)$(COLOR_CYAN)...lefthook:$(COLOR_RESET)"
	@echo "   $(COLOR_GREEN)lefthook-initialize$(COLOR_RESET) ......... Initialize lefthook git hooks."
	@echo ""
	@echo " $(COLOR_BOLD)$(COLOR_CYAN)...go:$(COLOR_RESET)"
	@echo "   $(COLOR_GREEN)go-mod-clean$(COLOR_RESET) ................ Clean Go module cache."
	@echo "   $(COLOR_GREEN)go-mod-tidy$(COLOR_RESET) ................. Tidy Go modules."
	@echo "   $(COLOR_GREEN)go-mod-update$(COLOR_RESET) ............... Update Go modules."
	@echo "   $(COLOR_GREEN)go-fmt$(COLOR_RESET) ...................... Format Go code."
	@echo "   $(COLOR_GREEN)go-lint$(COLOR_RESET) ..................... Lint Go code."
	@echo "   $(COLOR_GREEN)go-test$(COLOR_RESET) ..................... Run Go tests."
	@echo "   $(COLOR_GREEN)go-build$(COLOR_RESET) .................... Build Go program."
	@echo "   $(COLOR_GREEN)go-install$(COLOR_RESET) .................. Install Go program."
	@echo ""
	@echo " $(COLOR_BOLD)$(COLOR_CYAN)...docker:$(COLOR_RESET)"
	@echo "   $(COLOR_GREEN)docker-build$(COLOR_RESET) ................. Build Docker image."

.DEFAULT_GOAL = help
