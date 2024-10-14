# Set the default shell to `/bin/sh` for executing commands in the Makefile.
SHELL = /bin/sh

# Define the project name for easy reference.
PROJECT = "xurlfind3r"

# The default target that gets executed when the `make` command is run without arguments.
# In this case, it will trigger the `go-build` target.
all: go-build

# --- Go(Golang) ------------------------------------------------------------------------------------

# Define common Go commands with variables for reusability and easier updates.
GOCMD=go # The main Go command.
GOMOD=$(GOCMD) mod # Go mod command for managing modules.
GOGET=$(GOCMD) get # Go get command for retrieving packages.
GOFMT=$(GOCMD) fmt # Go fmt command for formatting Go code.
GOTEST=$(GOCMD) test # Go test command for running tests.
GOBUILD=$(GOCMD) build # Go build command for building binaries.
GOINSTALL=$(GOCMD) install # Go install command for installing packages.

# Define Go build flags for verbosity and linking.
GOFLAGS := -v # Verbose flag for Go commands to print detailed output.
LDFLAGS := -s -w # Linker flags to strip debug information (-s) and reduce binary size (-w).

# Set static linking flags for systems that are not macOS (darwin).
# Static linking allows the binary to include all required libraries in the executable.
ifneq ($(shell go env GOOS),darwin)
	LDFLAGS := -extldflags "-static"
endif

# Define Golangci-lint command for linting Go code.
GOLANGCILINTCMD=golangci-lint
GOLANGCILINTRUN=$(GOLANGCILINTCMD) run

# --- Go Module Management

# Tidy Go modules
# This target cleans up `go.mod` and `go.sum` files by removing any unused dependencies.
# Use this command to ensure that the module files are in a clean state.
.PHONY: go-mod-tidy
go-mod-tidy:
	$(GOMOD) tidy

# Update Go modules
# This target updates the Go module dependencies to their latest versions.
# It fetches and updates all modules, and any indirect dependencies.
.PHONY: go-mod-update
go-mod-update:
	$(GOGET) -f -t -u ./... # Update test dependencies.
	$(GOGET) -f -u ./... # Update other dependencies.

# --- Go Code Quality and Testing

# Format Go code
# This target formats all Go source files according to Go's standard formatting rules using `go fmt`.
.PHONY: go-fmt
go-fmt:
	$(GOFMT) ./...

# Lint Go code
# This target lints the Go source code to ensure it adheres to best practices.
# It uses `golangci-lint` to run various static analysis checks on the code.
# It first runs the `go-fmt` target to ensure the code is properly formatted.
.PHONY: go-lint
go-lint: go-fmt
	$(GOLANGCILINTRUN) $(GOLANGCILINT) ./...

# Run Go tests
# This target runs all Go tests in the current module.
# The `GOFLAGS` flag ensures that tests are run with verbose output, providing more detailed information.
.PHONY: go-test
go-test:
	$(GOTEST) $(GOFLAGS) ./...

# --- Go Build and Install

# Build Go program
# This target compiles the Go source code and generates a binary in the `bin/` directory.
# The output binary is named after the project (`xurlfind3r`), and the source entry point is the main file in `cmd/$(PROJECT)/main.go`.
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
DOCKERCMD = docker # The main Docker command.
DOCKERBUILD = $(DOCKERCMD) build # Docker build command for building images.

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