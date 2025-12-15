.PHONY: build run fmt tidy clean test help dev swagger build-embeddings fmt-embeddings lint-embeddings run-embeddings build-import-emails import-emails import-emails-eml import-emails-mbox test-race test-all test-short test-package bench bench-package coverage-report test-clean test-e2e test-e2e-headless test-e2e-quick

# Build configuration
BINARY_NAME=server
BUILD_DIR=bin
CMD_DIR=./cmd/server
EMBEDDINGS_CMD_DIR=./cmd/init-embeddings-write
IMPORT_EMAILS_CMD_DIR=./cmd/import-emails

# Default target
all: build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build the embeddings command
build-embeddings:
	@echo "Building init-embeddings-write..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/init-embeddings-write $(EMBEDDINGS_CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/init-embeddings-write"

# Build the email import command
build-import-emails:
	@echo "Building import-emails..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/import-emails $(IMPORT_EMAILS_CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/import-emails"

# Import emails from EML files or directory
import-emails-eml: build-import-emails
	@if [ -z "$(PATH_TO_EMAILS)" ]; then \
		echo "Error: PATH_TO_EMAILS not specified"; \
		echo "Usage: make import-emails-eml PATH_TO_EMAILS=/path/to/emails"; \
		exit 1; \
	fi
	@echo "Importing EML files from: $(PATH_TO_EMAILS)"
	@./$(BUILD_DIR)/import-emails -eml $(PATH_TO_EMAILS)

# Import emails from MBOX file
import-emails-mbox: build-import-emails
	@if [ -z "$(PATH_TO_MBOX)" ]; then \
		echo "Error: PATH_TO_MBOX not specified"; \
		echo "Usage: make import-emails-mbox PATH_TO_MBOX=/path/to/mailbox.mbox"; \
		exit 1; \
	fi
	@echo "Importing MBOX file: $(PATH_TO_MBOX)"
	@./$(BUILD_DIR)/import-emails -mbox $(PATH_TO_MBOX)

# Import emails (auto-detect EML directory or MBOX file)
import-emails: build-import-emails
	@if [ -z "$(EMAIL_PATH)" ]; then \
		echo "Error: EMAIL_PATH not specified"; \
		echo "Usage:"; \
		echo "  make import-emails EMAIL_PATH=/path/to/emails       # For EML files/directory"; \
		echo "  make import-emails EMAIL_PATH=/path/to/file.mbox    # For MBOX file"; \
		exit 1; \
	fi
	@if [ -f "$(EMAIL_PATH)" ] && echo "$(EMAIL_PATH)" | grep -q "\.mbox$$"; then \
		echo "Detected MBOX file: $(EMAIL_PATH)"; \
		./$(BUILD_DIR)/import-emails -mbox $(EMAIL_PATH); \
	elif [ -f "$(EMAIL_PATH)" ] && echo "$(EMAIL_PATH)" | grep -q "\.eml$$"; then \
		echo "Detected EML file: $(EMAIL_PATH)"; \
		./$(BUILD_DIR)/import-emails -eml $(EMAIL_PATH); \
	elif [ -d "$(EMAIL_PATH)" ]; then \
		echo "Detected directory: $(EMAIL_PATH)"; \
		./$(BUILD_DIR)/import-emails -eml $(EMAIL_PATH); \
	else \
		echo "Error: $(EMAIL_PATH) is not a valid file or directory"; \
		exit 1; \
	fi

# Run the application
run: build
	@echo "Starting $(BINARY_NAME)..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

# Run the application in development mode (without building binary)
dev:
	@echo "Running in development mode..."
	@go run $(CMD_DIR)

# Format all Go code
fmt:
	@echo "Formatting Go code..."
	@go fmt ./...

# Format embeddings command specifically
fmt-embeddings:
	@echo "Formatting init-embeddings-write..."
	@go fmt $(EMBEDDINGS_CMD_DIR)
	@echo "Formatting complete for init-embeddings-write"

# Tidy up modules
tidy:
	@echo "Tidying modules..."
	@go mod tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	@go test -v -race ./...

# Run tests with race detection and coverage
test-all:
	@echo "Running comprehensive tests..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run short tests only (skip long-running tests)
test-short:
	@echo "Running short tests..."
	@go test -v -short ./...

# Run tests for specific package
test-package:
	@echo "Running tests for package: $(PKG)"
	@go test -v ./$(PKG)

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Run benchmarks for specific package
bench-package:
	@echo "Running benchmarks for package: $(PKG)"
	@go test -bench=. -benchmem ./$(PKG)

# Show test coverage in terminal
coverage-report:
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@echo ""
	@echo "For detailed HTML report, open coverage.html"

# Clean test cache
test-clean:
	@echo "Cleaning test cache..."
	@go clean -testcache
	@echo "Test cache cleaned"

# Run E2E tests (with visible browser)
test-e2e:
	@echo "Running E2E tests with visible browser..."
	@E2E_HEADLESS=false E2E_BASE_URL=$(E2E_BASE_URL) go test -v -timeout 10m ./e2e/...

# Run E2E tests (headless mode)
test-e2e-headless:
	@echo "Running E2E tests in headless mode..."
	@E2E_HEADLESS=true E2E_BASE_URL=$(E2E_BASE_URL) go test -v -timeout 10m ./e2e/...

# Run quick E2E tests (core functionality only)
test-e2e-quick:
	@echo "Running quick E2E tests..."
	@E2E_HEADLESS=true E2E_BASE_URL=$(E2E_BASE_URL) go test -v -timeout 5m -run "TestAppLoads|TestConnectionStatus|TestChatInteraction" ./e2e/...

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install -a github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/swaggo/swag/cmd/swag@latest

# Lint the code (same checks as CI)
lint:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Running staticcheck..."
	@if ! command -v staticcheck &> /dev/null; then \
		echo "Installing staticcheck..."; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
	@staticcheck ./...
	@echo "Linting complete!"

# Lint embeddings command specifically
lint-embeddings:
	@echo "Linting init-embeddings-write..."
	@echo "Running go vet on embeddings command..."
	@go vet $(EMBEDDINGS_CMD_DIR)
	@echo "Running staticcheck on embeddings command..."
	@if ! command -v staticcheck &> /dev/null; then \
		echo "Installing staticcheck..."; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
	@staticcheck $(EMBEDDINGS_CMD_DIR)
	@echo "Linting complete for init-embeddings-write!"

# Format, lint, and build embeddings command
embeddings: fmt-embeddings lint-embeddings build-embeddings
	@echo "Embeddings command ready: $(BUILD_DIR)/init-embeddings-write"

# Run embeddings generation once (loads .env and runs full generation, then exits)
run-embeddings: build-embeddings
	@echo "Running embeddings generation (one-time run)..."
	@if [ ! -f .env ]; then \
		echo "Error: .env file not found in root directory"; \
		exit 1; \
	fi
	@echo "Loading environment variables from .env..."
	@export $$(cat .env | grep -v '^#' | xargs) && ./$(BUILD_DIR)/init-embeddings-write --once
	@echo "Embeddings generation complete!"

# Build for production (with optimizations)
build-prod:
	@echo "Building for production..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Production build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/server/main.go -o docs/; \
		echo "Swagger documentation generated successfully!"; \
		echo "Access Swagger UI at: http://localhost:8080/swagger/"; \
	else \
		echo "swag not found. Run 'make install-tools' first."; \
		exit 1; \
	fi

# Docker build
docker-build:
	@echo "Building Docker image..."
	@echo "Note: Swagger documentation will be generated during Docker build"
	@docker build -t ids-api .

# Docker build with local Swagger generation
docker-build-with-swagger: swagger
	@echo "Building Docker image with pre-generated Swagger docs..."
	@docker build -t ids-api .

# Help
help:
	@echo "Available commands:"
	@echo "  build        - Build the application"
	@echo "  run          - Build and run the application"
	@echo "  dev          - Run in development mode (no binary)"
	@echo "  fmt          - Format Go code"
	@echo "  tidy         - Tidy up Go modules"
	@echo "  deps         - Download dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  install-tools - Install development tools"
	@echo "  lint         - Lint the code"
	@echo "  build-prod   - Build for production"
	@echo "  swagger      - Generate Swagger documentation"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-build-with-swagger - Build Docker image with pre-generated Swagger docs"
	@echo ""
	@echo "Test commands:"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-race    - Run tests with race detection"
	@echo "  test-all     - Run comprehensive tests (race + coverage)"
	@echo "  test-short   - Run only short tests"
	@echo "  test-package - Run tests for specific package (use PKG=<package>)"
	@echo "  bench        - Run benchmarks"
	@echo "  bench-package - Run benchmarks for specific package (use PKG=<package>)"
	@echo "  coverage-report - Show coverage report in terminal"
	@echo "  test-clean   - Clean test cache"
	@echo ""
	@echo "Embeddings commands:"
	@echo "  build-embeddings - Build init-embeddings-write command"
	@echo "  fmt-embeddings   - Format init-embeddings-write command"
	@echo "  lint-embeddings  - Lint init-embeddings-write command"
	@echo "  embeddings       - Format, lint, and build embeddings command"
	@echo "  run-embeddings   - Run embeddings generation once and exit (loads .env)"
	@echo ""
	@echo "Email commands:"
	@echo "  build-import-emails - Build import-emails command"
	@echo "  import-emails       - Import emails (auto-detect format)"
	@echo "                        Usage: make import-emails EMAIL_PATH=/path/to/emails"
	@echo "  import-emails-eml   - Import EML files/directory"
	@echo "                        Usage: make import-emails-eml PATH_TO_EMAILS=/path/to/emails"
	@echo "  import-emails-mbox  - Import MBOX file"
	@echo "                        Usage: make import-emails-mbox PATH_TO_MBOX=/path/to/file.mbox"
	@echo ""
	@echo "E2E test commands:"
	@echo "  test-e2e          - Run E2E tests with visible browser"
	@echo "  test-e2e-headless - Run E2E tests in headless mode"
	@echo "  test-e2e-quick    - Run quick E2E tests (core functionality only)"
	@echo "                      Use E2E_BASE_URL to override target (default: https://ids.jshipster.io)"
	@echo "                      Example: make test-e2e-quick E2E_BASE_URL=http://localhost:8080"
	@echo ""
	@echo "  help         - Show this help message"