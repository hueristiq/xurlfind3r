# Specifies the shell to be used for executing commands. In this case, it's set to `/bin/bash`.
# Bash is chosen for its advanced scripting capabilities, including string manipulation and conditional checks.
SHELL = /bin/bash

# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Git Hooks ------------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: git-hooks-install
# Target: git-hooks-install
# Purpose:
#   Installs and configures Git hooks using Lefthook—a tool for managing Git hooks.
# Process:
#   1. Checks if the `lefthook` command exists in the system PATH.
#   2. If not found, installs the latest version via the Go package installer.
#   3. Finally, runs `lefthook install` to apply the hook configuration defined in the repo.
git-hooks-install:
	@command -v lefthook || go install github.com/evilmartians/lefthook@latest; lefthook install

# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Go (Golang) ----------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: go-mod-clean
# Target: go-mod-clean
# Purpose:
#   Cleans the Go module cache, removing any cached module files.
# Rationale:
#   Useful when facing issues with outdated or corrupt module caches.
go-mod-clean:
	go clean -modcache

.PHONY: go-mod-tidy
# Target: go-mod-tidy
# Purpose:
#   Tidies the go.mod file by ensuring it reflects the actual dependencies.
# Process:
#   - Adds any missing module requirements.
#   - Removes modules that are no longer used in the project.
go-mod-tidy:
	go mod tidy

.PHONY: go-mod-update
# Target: go-mod-update
# Purpose:
#   Updates all Go modules to their latest versions.
# Process:
#   - First command updates test dependencies using flags:
#     - `-f` forces updates, 
#     - `-t` includes test packages, and 
#     - `-u` requests updates.
#   - Second command updates the rest of the dependencies.
go-mod-update:
	go get -f -t -u ./...
	go get -f -u ./...

.PHONY: go-fmt
# Target: go-fmt
# Purpose:
#   Formats all Go source code files to enforce a consistent code style.
# Details:
#   - Uses the built-in `go fmt` command which traverses all packages in the module.
go-fmt:
	go fmt ./...

.PHONY: go-lint
# Target: go-lint
# Purpose:
#   Analyzes Go source code to identify potential issues, coding style violations,
#   or other anomalies that might affect code quality.
# Process:
#   1. Calls the `go-fmt` target to ensure that code is properly formatted.
#   2. Checks if `golangci-lint` is available; if not, installs a specific version.
#   3. Runs the linter across all packages in the project.
go-lint: go-fmt
	(command -v golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5) && golangci-lint run ./...

.PHONY: go-test
# Target: go-test
# Purpose:
#   Executes the test suite for the Go project.
# Details:
#   - The tests are run in verbose mode (`-v`) to provide detailed output.
#   - The `-race` flag is used to enable detection of race conditions.
#   - This target applies to all packages within the module..
go-test:
	go test -v -race ./...

.PHONY: go-build
# Target: go-build
# Purpose:
#   Builds the Go program into an executable.
# Details:
#   - Uses `-v` for verbose build output.
#   - `-ldflags '-s -w'` strips debugging information to reduce the executable size.
#   - The built binary is output to the `bin` directory with a specified name.
go-build:
	go build -v -ldflags '-s -w' -o bin/xurlfind3r cmd/xurlfind3r/main.go

.PHONY: go-install
# Target: go-install
# Purpose:
#   Installs the Go program and its dependencies.
# Details:
#   - Uses `go install` to compile and install the binary to the Go workspace.
go-install:
	go install -v ./...

# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Docker ---------------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------

DOCKERFILE := ./Dockerfile

IMAGE_NAME = hueristiq/xurlfind3r
IMAGE_TAG = $(shell cat internal/configuration/configuration.go | grep "VERSION =" | sed 's/.*VERSION = "\([0-9.]*\)".*/\1/')
IMAGE = $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: docker-build
# Target: docker-build
# Purpose:
#   Builds the Docker image for the project.
# Process:
#   - Invokes the Docker build command using the specified Dockerfile.
#   - Tags the image with both the version-specific tag and "latest" for ease of use.
docker-build:
	docker build -f $(DOCKERFILE) -t $(IMAGE) -t $(IMAGE_NAME):latest .

# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------
# --- Help -----------------------------------------------------------------------------------------------------------------------------------------------------------------
# --------------------------------------------------------------------------------------------------------------------------------------------------------------------------

.PHONY: help
# Target: help
# Purpose:
#   Displays an overview of available targets along with their descriptions.
# Details:
#   - When no target is provided, the default action (set by .DEFAULT_GOAL) is to show this help text.
help:
	@echo ""
	@echo "Available commands:"
	@echo ""
	@echo " Git Hooks:"
	@echo "  git-hooks-install ........ Install Git hooks."
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

# Set the default target to "help".
# This ensures that running `make` without specifying a target will display the help text.
.DEFAULT_GOAL = help