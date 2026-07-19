BINARY   := deplens
CMD      := ./cmd/deplens
LDFLAGS  := -ldflags="-s -w"

.PHONY: help build run run-json run-without-dependencies test test-verbose test-pkg test-single vet clean install

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build the binary
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

run: build ## Scan current directory (human output)
	./$(BINARY) .

run-json: build ## Scan current directory (JSON output)
	./$(BINARY) --json .

run-without-dependencies: build ## Scan current directory, include sources without dependencies
	./$(BINARY) --show-without-dependencies .

test: ## Run all tests
	go test ./...

test-verbose: ## Run all tests with verbose output
	go test -v ./...

test-pkg: ## Run tests for a package  usage: make test-pkg PKG=./internal/analyze/
	go test $(PKG)

test-single: ## Run a single named test  usage: make test-single PKG=./internal/analyze/ TEST=TestGoMod
	go test -run $(TEST) $(PKG)

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -f $(BINARY)

install: ## Install binary to GOPATH/bin
	go install $(LDFLAGS) $(CMD)
