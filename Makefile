SHELL = /bin/bash

MODULE := github.com/hueristiq/xurlfind3r
BINARY := xurlfind3r
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0")

BIN_DIR = $(PWD)/.bin

GOLANGCI_LINT_BIN_VER = 2.12.0
GOLANGCI_LINT_BIN_DIR = $(BIN_DIR)/golangci-lint/$(GOLANGCI_LINT_BIN_VER)

GOLANGCI_LINT_BIN = $(GOLANGCI_LINT_BIN_DIR)/golangci-lint

LEFTHOOK_BIN_VER = 2.1.6
LEFTHOOK_BIN_DIR = $(BIN_DIR)/lefthook/$(LEFTHOOK_BIN_VER)

LEFTHOOK_BIN = $(LEFTHOOK_BIN_DIR)/lefthook

# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Setup ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: install-lefthook install-golangci-lint lefthook-initialize

install-golangci-lint:
	@if command -v $(GOLANGCI_LINT_BIN) >/dev/null 2>&1; \
	then \
		echo "golangci-lint already installed!"; \
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

install-lefthook:
	@if command -v $(LEFTHOOK_BIN) >/dev/null 2>&1; \
	then \
		echo "lefthook already installed!"; \
	else \
		[ -d "$(LEFTHOOK_BIN_DIR)" ] || mkdir -p $(LEFTHOOK_BIN_DIR); \
		curl -sSL "https://github.com/evilmartians/lefthook/releases/download/v$(LEFTHOOK_BIN_VER)/lefthook_$(LEFTHOOK_BIN_VER)_$$(uname -s)_$$(uname -m | sed 's/arm64/aarch64/')" -o "$(LEFTHOOK_BIN)" && \
		chmod +x "$(LEFTHOOK_BIN)"; \
	fi

lefthook-initialize:
	@if git rev-parse --git-dir >/dev/null 2>&1; \
	then \
		$(LEFTHOOK_BIN) install; \
	else \
		echo "Skipped lefthook install: git repository not found!"; \
	fi

# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Go (Golang) ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: go-mod-clean go-mod-tidy go-mod-update go-fmt go-lint go-test go-build go-install

go-mod-clean:
	go clean -modcache

go-mod-tidy:
	go mod tidy

go-mod-update:
	go get -f -t -u ./...
	go get -f -u ./...

go-fmt: install-golangci-lint
	$(GOLANGCI_LINT_BIN) fmt ./...

go-lint: go-fmt
	$(GOLANGCI_LINT_BIN) run ./...

go-test:
	go test -v -race ./...

go-build:
	go build -v -ldflags "-s -w -X $(MODULE)/internal/configuration.Version=$(VERSION)" -o bin/$(BINARY) cmd/$(BINARY)/main.go

go-install:
	go install -v ./...

# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Docker ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

DOCKERFILE := ./Dockerfile

IMAGE_NAME = hueristiq/xurlfind3r
IMAGE_TAG = $(VERSION)
IMAGE = $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: docker-build

docker-build:
	docker build -f $(DOCKERFILE) -t $(IMAGE) -t $(IMAGE_NAME):latest .


# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Help -----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: help

help:
	@echo ""
	@echo "Available commands:"
	@echo ""
	@echo " Setup:"
	@echo ""
	@echo "  install-lefthook ......... Install lefthook."
	@echo "  install-golangci-lint .... Install golangci-lint."
	@echo "  lefthook-initialize ...... Initialize lefthook git hooks."
	@echo ""
	@echo " Go (Golang):"
	@echo ""
	@echo "  go-mod-clean ............. Clean Go module cache."
	@echo "  go-mod-tidy .............. Tidy Go modules."
	@echo "  go-mod-update ............ Update Go modules."
	@echo "  go-fmt ................... Format Go code."
	@echo "  go-lint .................. Lint Go code."
	@echo "  go-test .................. Run Go tests."
	@echo "  go-build ................. Build Go program."
	@echo "  go-install ............... Install Go program."
	@echo ""
	@echo " Docker:"
	@echo ""
	@echo "  docker-build ............. Build Docker image."
	@echo ""

	@echo " Help:"
	@echo ""
	@echo "  help ..................... Display this help information."
	@echo ""

.DEFAULT_GOAL = help
