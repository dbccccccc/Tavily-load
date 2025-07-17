# Tavily Load Balancer Makefile

# Variables
APP_NAME := tavily-load
VERSION := 1.0.0
BUILD_DIR := build
BINARY_NAME := $(APP_NAME)
MAIN_PATH := ./cmd/$(APP_NAME)

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -ldflags "-X main.AppVersion=$(VERSION) -s -w"
BUILD_FLAGS := -v $(LDFLAGS)

# Default target
.PHONY: all
all: clean build

# Help target
.PHONY: help
help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Build targets
.PHONY: build
build: build-frontend ## Build the binary and frontend
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-backend
build-backend: ## Build only the backend binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-frontend
build-frontend: ## Build the frontend
	@echo "Building frontend..."
	@if [ -d "web" ]; then \
		cd web && npm install && npm run build; \
	else \
		echo "Frontend directory not found, skipping..."; \
	fi

.PHONY: build-all
build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# Windows
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	GOOS=windows GOARCH=386 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-386.exe $(MAIN_PATH)
	
	# Linux
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=386 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-386 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	
	# macOS
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	
	@echo "Cross-compilation complete"

.PHONY: clean
clean: ## Clean build files
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Run targets
.PHONY: run
run: ## Run the server
	@echo "Starting $(APP_NAME)..."
	$(GOCMD) run $(MAIN_PATH)

.PHONY: dev
dev: ## Run in development mode with race detection
	@echo "Starting $(APP_NAME) in development mode..."
	$(GOCMD) run -race $(MAIN_PATH)

# Test targets
.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./...

.PHONY: coverage
coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: bench
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Code quality targets
.PHONY: lint
lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

.PHONY: tidy
tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Management targets
.PHONY: health
health: ## Check server health
	@echo "Checking server health..."
	@curl -s http://localhost:3000/health | jq . || echo "Server not running or jq not installed"

.PHONY: stats
stats: ## View server statistics
	@echo "Getting server statistics..."
	@curl -s http://localhost:3000/stats | jq . || echo "Server not running or jq not installed"

.PHONY: blacklist
blacklist: ## View blacklisted keys
	@echo "Getting blacklisted keys..."
	@curl -s http://localhost:3000/blacklist | jq . || echo "Server not running or jq not installed"

.PHONY: reset-keys
reset-keys: ## Reset all keys
	@echo "Resetting all keys..."
	@curl -s http://localhost:3000/reset-keys | jq . || echo "Server not running or jq not installed"

# Docker targets
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest

.PHONY: docker-run
docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -d -p 3000:3000 \
		-v $(PWD)/keys.txt:/app/keys.txt:ro \
		--name $(APP_NAME) \
		$(APP_NAME):latest

.PHONY: docker-stop
docker-stop: ## Stop Docker container
	@echo "Stopping Docker container..."
	docker stop $(APP_NAME) || true
	docker rm $(APP_NAME) || true

.PHONY: docker-logs
docker-logs: ## View Docker container logs
	docker logs -f $(APP_NAME)

# Setup targets
.PHONY: setup
setup: ## Setup development environment
	@echo "Setting up development environment..."
	@cp .env.example .env
	@cp keys.txt.example keys.txt
	@echo "Please edit .env and keys.txt files with your configuration"

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOGET) -d ./...
	$(GOMOD) tidy



# Install target
.PHONY: install
install: build ## Install binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Version target
.PHONY: version
version: ## Show version
	@echo "$(APP_NAME) version $(VERSION)"
