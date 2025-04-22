.PHONY: build clean test lint

BINARY_NAME=cortex_agent
BUILD_DIR=bin

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/cortex_agent

clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)

test:
	@echo "Running tests..."
	@go test -v ./...

cover:
	@echo "Running tests with coverage..."
	@go test -cover -v ./...
	@echo "\nDetailed coverage by package:"
	@go test -cover ./... | grep -v "no test files" | sort -k 5 -n

cover-html:
	@echo "Generating HTML coverage report..."
	@mkdir -p $(BUILD_DIR)/coverage
	@go test -coverprofile=$(BUILD_DIR)/coverage/coverage.out ./...
	@go tool cover -html=$(BUILD_DIR)/coverage/coverage.out -o $(BUILD_DIR)/coverage/coverage.html
	@echo "HTML coverage report generated at $(BUILD_DIR)/coverage/coverage.html"

lint:
	@echo "Running linter..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Please install it from https://golangci-lint.run/usage/install/"; \
	fi

fmt:
	@echo "Formatting code..."
	@go fmt ./...

.DEFAULT_GOAL := build
