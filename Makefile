.PHONY: help build clean test lint mocks install uninstall vendor dev air

GO ?= go
BINARY_NAME=amaru_agent
BUILD_DIR=bin
VERSION ?= $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
LDFLAGS := -X "main.Version=$(VERSION)"
CONFIG ?= config.toml

help: ## Show all available targets with descriptions
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-15s %s\n", $$1, $$2}'

build: ## Build application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) build -v -ldflags '-s -w $(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/amaru_agent

install: build ## Install to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@mkdir -p $(GOPATH)/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Please add $(GOPATH)/bin to your PATH if it is not already there."

uninstall: ## Remove from $GOPATH/bin
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f $(GOPATH)/bin/$(BINARY_NAME)

clean: ## Clean artifacts
	@echo "Cleaning up build artifacts..."
	@rm -rf $(BUILD_DIR)

test: ## Run tests
	@echo "Running tests..."
	@$(GO) test -v ./...

cover: ## Test coverage stats
	@echo "Running tests with coverage..."
	@$(GO) test -cover -v ./...
	@echo "\nDetailed coverage by package:"
	@$(GO) test -cover ./... | grep -v "no test files" | sort -k 5 -n

cover-html: ## Generate HTML coverage report
	@echo "Generating HTML coverage report..."
	@mkdir -p $(BUILD_DIR)/coverage
	@$(GO) test -coverprofile=$(BUILD_DIR)/coverage/coverage.out ./...
	@$(GO) tool cover -html=$(BUILD_DIR)/coverage/coverage.out -o $(BUILD_DIR)/coverage/coverage.html
	@echo "HTML coverage report generated at $(BUILD_DIR)/coverage/coverage.html"

mocks: ## Generate interface mocks
	@echo "Generating mocks..."
	@$(GO) install go.uber.org/mock/mockgen@latest
	@$(shell go env GOPATH)/bin/mockgen -destination=internal/transport/mocks/mock_connection.go -package=mocks erlang-solutions.com/amaru_agent/internal/transport Connection

lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Please install it from https://golangci-lint.run/"; \
	fi

fmt: ## Format code
	@echo "Formatting code..."
	@$(GO) fmt ./...

vendor: ## Tidy and verify dependencies
	@echo 'Tidying and verifying module dependencies...'
	@$(GO) mod tidy
	@$(GO) mod verify

air: ## Install air for hot reload
	@if ! command -v air > /dev/null 2>&1; then \
		echo "Installing air for hot reload..."; \
		$(GO) install github.com/air-verse/air@latest; \
	else \
		echo "air is already installed"; \
	fi

dev: air ## Hot reload development (requires CONFIG=file.toml)
	@echo "Starting development server with config: $(CONFIG)"
	@air --build.cmd "make build" --build.bin ./$(BUILD_DIR)/$(BINARY_NAME) -- -config $(CONFIG)

.DEFAULT_GOAL := help
