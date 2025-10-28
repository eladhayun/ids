.PHONY: build run fmt tidy clean test help dev

# Build configuration
BINARY_NAME=server
BUILD_DIR=bin
CMD_DIR=./cmd/server

# Default target
all: build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

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

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install -a github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Lint the code
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Run 'make install-tools' first."; \
		exit 1; \
	fi

# Build for production (with optimizations)
build-prod:
	@echo "Building for production..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Production build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Docker build
docker-build:
	@echo "Building Docker image..."
	@docker build -t ids-api .

# Database management
db-start:
	@echo "Starting MariaDB container..."
	@docker stop mariadb 2>/dev/null || true
	@docker rm mariadb 2>/dev/null || true
	@docker run -d \
		--name mariadb \
		-e MYSQL_ROOT_PASSWORD=my-secret-pw \
		-e MYSQL_DATABASE=isrealde_wp654 \
		-v $(PWD)/isrealde_wp654.sql:/docker-entrypoint-initdb.d/dump.sql \
		-p 3306:3306 \
		mariadb:10.6.23
	@echo "MariaDB container started on port 3306"

db-stop:
	@echo "Stopping MariaDB container..."
	@docker stop mariadb || true
	@echo "MariaDB container stopped"

db-remove:
	@echo "Removing MariaDB container..."
	@docker rm mariadb || true
	@echo "MariaDB container removed"

db-restart: db-stop db-remove db-start

db-status:
	@echo "Checking MariaDB container status..."
	@docker ps -f name=mariadb

db-logs:
	@echo "Showing MariaDB container logs..."
	@docker logs mariadb

# Help
help:
	@echo "Available commands:"
	@echo "  build        - Build the application"
	@echo "  run          - Build and run the application"
	@echo "  dev          - Run in development mode (no binary)"
	@echo "  fmt          - Format Go code"
	@echo "  tidy         - Tidy up Go modules"
	@echo "  deps         - Download dependencies"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean        - Clean build artifacts"
	@echo "  install-tools - Install development tools"
	@echo "  lint         - Lint the code"
	@echo "  build-prod   - Build for production"
	@echo "  docker-build - Build Docker image"
	@echo "  db-start     - Start MariaDB container"
	@echo "  db-stop      - Stop MariaDB container"
	@echo "  db-remove    - Remove MariaDB container"
	@echo "  db-restart   - Restart MariaDB container"
	@echo "  db-status    - Check MariaDB container status"
	@echo "  db-logs      - Show MariaDB container logs"
	@echo "  help         - Show this help message"
