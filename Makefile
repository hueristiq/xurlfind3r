# Go(Golang) Options
GOCMD=go
GOMOD=$(GOCMD) mod
GOTEST=$(GOCMD) test
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOFLAGS := -v 
LDFLAGS := -s -w

# Golangci Options
GOLANGCILINTCMD=golangci-lint
GOLANGCILINTRUN=$(GOLANGCILINTCMD) run

ifneq ($(shell go env GOOS),darwin)
LDFLAGS := -extldflags "-static"
endif

tidy:
	$(GOMOD) tidy
lint:
	$(GOLANGCILINTRUN) ./...
test:
	$(GOTEST) $(GOFLAGS) ./...
build:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/xurlfind3r cmd/xurlfind3r/main.go
install:
	$(GOINSTALL) $(GOFLAGS) ./...
