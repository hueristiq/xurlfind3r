# Set the default shell to `/bin/sh` for executing commands in the Makefile.
# `/bin/sh` is used as it is lightweight and widely available across UNIX systems.
SHELL = /bin/sh

# Define the project name for easy reference throughout the Makefile.
# This helps in maintaining a consistent project name and avoiding hardcoding it in multiple places.
PROJECT = "xurlfind3r"

# The default target that gets executed when the `make` command is run without arguments.
# In this case, it will trigger the `go-build` target.
all: go-build

# --- Prepare | Setup -------------------------------------------------------------------------------

.PHONY: prepare
prepare:
	@# Install the latest version of Lefthook (a Git hooks manager) and set it up.
	go install github.com/evilmartians/lefthook@latest && lefthook install

# --- Go(Golang) ------------------------------------------------------------------------------------

# Define common Go commands with variables for reusability and easier updates.
GOCMD=go
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOTEST=$(GOCMD) test
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install

# Define Go build flags for verbosity and linking.
# Verbose flag for Go commands, helpful for debugging and understanding output.
GOFLAGS := -v
# Linker flags:
# - `-s` removes the symbol table for a smaller binary size.
# - `-w` removes DWARF debugging information.
LDFLAGS := -s -w

# Enable static linking on non-macOS platforms.
# This embeds all dependencies directly into the binary, making it more portable.
ifneq ($(shell go env GOOS),darwin)
	LDFLAGS := -extldflags "-static"
endif

# Define Golangci-lint command for linting Go code.
GOLANGCILINTCMD=golangci-lint
GOLANGCILINTRUN=$(GOLANGCILINTCMD) run

# --- Go Module Management

# Tidy Go modules
# This cleans up `go.mod` and `go.sum` by removing unused dependencies
# and ensuring that only the required packages are listed.
.PHONY: go-mod-tidy
go-mod-tidy:
	$(GOMOD) tidy

# Update Go modules
# Updates all Go dependencies to their latest versions, including both direct and indirect dependencies.
# Useful for staying up-to-date with upstream changes and bug fixes.
.PHONY: go-mod-update
go-mod-update:
	@# Update test dependencies.
	$(GOGET) -f -t -u ./...
	@# Update all other dependencies.
	$(GOGET) -f -u ./...

# --- Go Code Quality and Testing

# Format Go code
# Formats all Go source files in the current module according to Go's standard rules.
# Consistent formatting is crucial for code readability and collaboration.
.PHONY: go-fmt
go-fmt:
	$(GOFMT) ./...

# Lint Go code
# Runs static analysis checks on the Go code using Golangci-lint.
# Ensures the code adheres to best practices and is free from common issues.
# This target also runs `go-fmt` beforehand to ensure the code is formatted.
.PHONY: go-lint
go-lint: go-fmt
	$(GOLANGCILINTRUN) $(GOLANGCILINT) ./...

# Run Go tests
# Executes all unit tests in the module with detailed output.
# The `GOFLAGS` variable is used to enable verbosity, making it easier to debug test results.
.PHONY: go-test
go-test:
	$(GOTEST) $(GOFLAGS) ./...

# --- Go Build and Install

# Build Go program
# This target compiles the Go source code and generates a binary in the `bin/` directory.
# The output binary is named after the project (`xsubfind3r`), and the source entry point is the main file in `cmd/$(PROJECT)/main.go`.
# The `LDFLAGS` flag is passed to optimize the binary size by stripping debug information.
.PHONY: go-build
go-build:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(PROJECT) cmd/$(PROJECT)/main.go

# Install Go program
# This target installs the Go program by compiling and placing it in the system's Go bin directory.
# Use this to make the application globally available on the system.
.PHONY: go-install
go-install:
	$(GOINSTALL) $(GOFLAGS) ./...

# --- Docker ------------------------------------------------------------------------------------

# Define common Docker commands with variables for reusability.
DOCKERCMD = docker
DOCKERBUILD = $(DOCKERCMD) build

# Define the path to the Dockerfile.
# The Dockerfile is located in the root directory by default.
DOCKERFILE := ./Dockerfile

# Define the Docker image name and tag.
# The image name is based on the project name, and the tag is extracted from the version in the configuration file.
IMAGE_NAME = hueristiq/$(PROJECT)
IMAGE_TAG = $(shell cat internal/configuration/configuration.go | grep "VERSION =" | sed 's/.*VERSION = "\([0-9.]*\)".*/\1/')
IMAGE = $(IMAGE_NAME):$(IMAGE_TAG)

# Build Docker image
# This target builds the Docker image using the Dockerfile.
# It tags the image with both the specific version and `latest` for convenience.
.PHONY: docker-build
docker-build:
	@$(DOCKERBUILD) \
		-f $(DOCKERFILE) \
		-t $(IMAGE) \
		-t $(IMAGE_NAME):latest \
		.

# --- Help -----------------------------------------------------------------------------------------

# Display help information
# This target prints out a detailed list of all available Makefile commands for ease of use.
# It's a helpful reference for developers using the Makefile.
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
	@echo "  help ..................... Display this help information"
	@echo ""