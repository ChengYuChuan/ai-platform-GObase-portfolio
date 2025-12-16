# LLM Gateway Makefile

# Variables
APP_NAME := llm-gateway
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.1.0")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-w -s -X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOLINT := golangci-lint

# Docker parameters
DOCKER := docker
DOCKER_IMAGE := $(APP_NAME)
DOCKER_TAG := $(VERSION)

# Directories
CMD_DIR := ./cmd/gateway
BUILD_DIR := ./build

.PHONY: all build clean test lint run docker-build docker-run help

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## all: Build and test
all: lint test build

## build: Build the binary
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)
	@echo "Binary built: $(BUILD_DIR)/$(APP_NAME)"

## build-linux: Build for Linux (cross-compile)
build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux $(CMD_DIR)

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## tidy: Tidy go modules
tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

## run: Run the application locally
run:
	@echo "Running $(APP_NAME)..."
	$(GOCMD) run $(CMD_DIR)

## run-dev: Run with hot reload (requires air)
run-dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Running without hot reload..."; \
		$(GOCMD) run $(CMD_DIR); \
	fi

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	$(DOCKER) build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	$(DOCKER) tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	$(DOCKER) run --rm -p 8080:8080 \
		--env-file .env \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## docker-push: Push Docker image to registry
docker-push:
	@echo "Pushing Docker image..."
	$(DOCKER) push $(DOCKER_IMAGE):$(DOCKER_TAG)
	$(DOCKER) push $(DOCKER_IMAGE):latest

## compose-up: Start all services with docker-compose
compose-up:
	$(DOCKER) compose up -d

## compose-down: Stop all services
compose-down:
	$(DOCKER) compose down

## compose-logs: View docker-compose logs
compose-logs:
	$(DOCKER) compose logs -f

## proto: Generate protobuf code (when implementing gRPC)
proto:
	@echo "Generating protobuf code..."
	@if [ -f proto/gateway.proto ]; then \
		protoc --go_out=. --go-grpc_out=. proto/gateway.proto; \
	else \
		echo "No proto files found"; \
	fi

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

## version: Show version
version:
	@echo "$(APP_NAME) version $(VERSION) ($(GIT_COMMIT))"
