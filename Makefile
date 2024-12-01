SHELL = /bin/sh

PROJECT = xurlfind3r

# --------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Prepare | Setup ------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: prepare
prepare:
	@# Install the latest version of Lefthook (a Git hooks manager) and set it up.
	go install github.com/evilmartians/lefthook@latest && lefthook install

# --------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Go (Golang) ----------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------

GOCMD=go
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOTEST=$(GOCMD) test
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install

GOFLAGS := -v
LDFLAGS := -s -w
ifneq ($(shell go env GOOS),darwin)
	LDFLAGS := -extldflags "-static"
endif

GOLANGCILINTCMD=golangci-lint
GOLANGCILINTRUN=$(GOLANGCILINTCMD) run

.PHONY: go-mod-clean
go-mod-clean:
	$(GOCLEAN) -modcache

.PHONY: go-mod-tidy
go-mod-tidy:
	$(GOMOD) tidy

.PHONY: go-mod-update
go-mod-update:
	@# Update test dependencies.
	$(GOGET) -f -t -u ./...
	@# Update all other dependencies.
	$(GOGET) -f -u ./...

.PHONY: go-fmt
go-fmt:
	$(GOFMT) ./...

.PHONY: go-lint
go-lint: go-fmt
	$(GOLANGCILINTRUN) $(GOLANGCILINT) ./...

.PHONY: go-test
go-test:
	$(GOTEST) $(GOFLAGS) ./...

.PHONY: go-build
go-build:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(PROJECT) cmd/$(PROJECT)/main.go

.PHONY: go-install
go-install:
	$(GOINSTALL) $(GOFLAGS) ./...

# --------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Docker ---------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------

DOCKERCMD = docker
DOCKERBUILD = $(DOCKERCMD) build

DOCKERFILE := ./Dockerfile

IMAGE_NAME = hueristiq/$(PROJECT)
IMAGE_TAG = $(shell cat internal/configuration/configuration.go | grep "VERSION =" | sed 's/.*VERSION = "\([0-9.]*\)".*/\1/')
IMAGE = $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: docker-build
docker-build:
	@$(DOCKERBUILD) -f $(DOCKERFILE) -t $(IMAGE) -t $(IMAGE_NAME):latest .

# --------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Help -----------------------------------------------------------------------------------------------------------------------------------------------------
# ---------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: help
help:
	@echo ""
	@echo "*****************************************************************************"
	@echo ""
	@echo "PROJECT : $(PROJECT)"
	@echo ""
	@echo "*****************************************************************************"
	@echo ""
	@echo "Available commands:"
	@echo ""
	@echo " Preparation | Setup:"
	@echo "  prepare .................. prepare repository."
	@echo ""
	@echo " Go Commands:"
	@echo "  go-mod-clean ............. Clean Go module cache."
	@echo "  go-mod-tidy .............. Tidy Go modules."
	@echo "  go-mod-update ............ Update Go modules."
	@echo "  go-fmt ................... Format Go code."
	@echo "  go-lint .................. Lint Go code."
	@echo "  go-test .................. Run Go tests."
	@echo "  go-build ................. Build Go program."
	@echo "  go-install ............... Install Go program."
	@echo ""
	@echo " Docker Commands:"
	@echo "  docker-build ............. Build Docker image."
	@echo ""
	@echo " Help Commands:"
	@echo "  help ..................... Display this help information."
	@echo ""

.DEFAULT_GOAL = help