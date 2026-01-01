# ==============================================================================
# HumanMark Makefile
# ==============================================================================
#
# This Makefile provides commands for building, testing, and deploying HumanMark.
#
# Usage:
#   make help      - Show this help message
#   make build     - Build the API binary
#   make test      - Run all tests
#   make run       - Run the API locally
#   make docker    - Build Docker image
#
# ==============================================================================

# Project configuration
PROJECT_NAME := humanmark
BINARY_NAME := humanmark
MAIN_PATH := ./cmd/api

# Go configuration
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Docker configuration
DOCKER_IMAGE := humanmark/api
DOCKER_TAG := $(VERSION)

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo "$(BLUE)HumanMark - Verify human-created content$(NC)"
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make $(GREEN)<target>$(NC)"
	@echo ""
	@echo "$(YELLOW)Targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'

# ==============================================================================
# Development
# ==============================================================================

.PHONY: run
run: ## Run the API locally
	@echo "$(BLUE)Starting HumanMark API...$(NC)"
	$(GO) run $(MAIN_PATH)/main.go

.PHONY: run-watch
run-watch: ## Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
	@command -v air >/dev/null 2>&1 || { echo "$(RED)air not found. Install with: go install github.com/cosmtrek/air@latest$(NC)"; exit 1; }
	air

.PHONY: dev
dev: ## Run in development mode with debug logging
	LOG_LEVEL=debug $(GO) run $(MAIN_PATH)/main.go

# ==============================================================================
# Building
# ==============================================================================

.PHONY: build
build: ## Build the API binary
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	@echo "  Version:    $(VERSION)"
	@echo "  Build Time: $(BUILD_TIME)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Build complete: bin/$(BINARY_NAME)$(NC)"

.PHONY: build-linux
build-linux: ## Build for Linux (useful for Docker)
	@echo "$(BLUE)Building for Linux...$(NC)"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "$(GREEN)Build complete: bin/$(BINARY_NAME)-linux-amd64$(NC)"

.PHONY: build-all
build-all: ## Build for all platforms
	@echo "$(BLUE)Building for all platforms...$(NC)"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "$(GREEN)All builds complete$(NC)"

.PHONY: clean
clean: ## Remove build artifacts
	@echo "$(BLUE)Cleaning...$(NC)"
	rm -rf bin/
	rm -rf tmp/
	$(GO) clean -cache
	@echo "$(GREEN)Clean complete$(NC)"

# ==============================================================================
# Testing
# ==============================================================================

.PHONY: test
test: ## Run all tests
	@echo "$(BLUE)Running tests...$(NC)"
	$(GO) test -race -cover ./...

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	@echo "$(BLUE)Running tests (verbose)...$(NC)"
	$(GO) test -race -cover -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

.PHONY: test-short
test-short: ## Run short tests only (skip integration tests)
	@echo "$(BLUE)Running short tests...$(NC)"
	$(GO) test -short ./...

.PHONY: bench
bench: ## Run benchmarks
	@echo "$(BLUE)Running benchmarks...$(NC)"
	$(GO) test -bench=. -benchmem ./...

# ==============================================================================
# Code Quality
# ==============================================================================

.PHONY: fmt
fmt: ## Format code
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GO) fmt ./...
	@echo "$(GREEN)Format complete$(NC)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GO) vet ./...

.PHONY: lint
lint: ## Run linter (requires golangci-lint)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "$(RED)golangci-lint not found. Install from: https://golangci-lint.run/usage/install/$(NC)"; exit 1; }
	@echo "$(BLUE)Running linter...$(NC)"
	golangci-lint run ./...

.PHONY: check
check: fmt vet test ## Run all checks (format, vet, test)
	@echo "$(GREEN)All checks passed$(NC)"

# ==============================================================================
# Docker
# ==============================================================================

.PHONY: docker
docker: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

.PHONY: docker-run
docker-run: docker ## Build and run Docker container
	@echo "$(BLUE)Running Docker container...$(NC)"
	docker run -it --rm -p 8080:8080 \
		-e ENV=development \
		-e LOG_LEVEL=debug \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-push
docker-push: docker ## Push Docker image to registry
	@echo "$(BLUE)Pushing Docker image...$(NC)"
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

# ==============================================================================
# Dependencies
# ==============================================================================

.PHONY: deps
deps: ## Download dependencies
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod verify
	@echo "$(GREEN)Dependencies downloaded$(NC)"

.PHONY: deps-update
deps-update: ## Update all dependencies
	@echo "$(BLUE)Updating dependencies...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "$(GREEN)Dependencies updated$(NC)"

.PHONY: deps-tidy
deps-tidy: ## Tidy go.mod
	@echo "$(BLUE)Tidying go.mod...$(NC)"
	$(GO) mod tidy
	@echo "$(GREEN)go.mod tidied$(NC)"

# ==============================================================================
# Documentation
# ==============================================================================

.PHONY: docs
docs: ## Generate documentation
	@echo "$(BLUE)Starting documentation server...$(NC)"
	@echo "Open http://localhost:6060/pkg/github.com/humanmark/humanmark/"
	godoc -http=:6060

# ==============================================================================
# API Testing
# ==============================================================================

.PHONY: api-test
api-test: ## Test API endpoints (requires running server)
	@echo "$(BLUE)Testing API endpoints...$(NC)"
	@echo "Health check:"
	curl -s http://localhost:8080/health | jq .
	@echo "\nVerify text:"
	curl -s -X POST http://localhost:8080/verify \
		-H "Content-Type: application/json" \
		-d '{"text": "This is a test to see if the API is working correctly."}' | jq .

.PHONY: api-bench
api-bench: ## Benchmark API (requires hey: go install github.com/rakyll/hey@latest)
	@command -v hey >/dev/null 2>&1 || { echo "$(RED)hey not found. Install with: go install github.com/rakyll/hey@latest$(NC)"; exit 1; }
	@echo "$(BLUE)Benchmarking API...$(NC)"
	hey -n 1000 -c 50 -m POST \
		-H "Content-Type: application/json" \
		-d '{"text": "This is a benchmark test for the HumanMark API."}' \
		http://localhost:8080/verify

# ==============================================================================
# Deployment
# ==============================================================================

.PHONY: deploy
deploy: build-linux docker docker-push ## Build and deploy (full pipeline)
	@echo "$(GREEN)Deployment complete$(NC)"

# ==============================================================================
# Version
# ==============================================================================

.PHONY: version
version: ## Show version information
	@echo "Version:    $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

# Default target
.DEFAULT_GOAL := help
