# Go(Golang) Options
GOCMD=go
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
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

.PHONY: tidy
tidy:
	$(GOMOD) tidy

.PHONY: update-deps
update-deps:
	$(GOGET) -f -t -u ./...
	$(GOGET) -f -u ./...

.PHONY: format
format:
	$(GOFMT) ./...

.PHONY: lint
lint:
	$(GOLANGCILINTRUN) ./...

.PHONY: test
test:
	$(GOTEST) $(GOFLAGS) ./...

.PHONY: build
build:
	$(GOBUILD) $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/xurlfind3r cmd/xurlfind3r/main.go

.PHONY: install
install:
	$(GOINSTALL) $(GOFLAGS) ./...