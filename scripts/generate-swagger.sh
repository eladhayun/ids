#!/bin/bash

# Generate Swagger documentation
# This script should be run from the project root directory

echo "Generating Swagger documentation..."

# Install swag if not already installed
if ! command -v swag &> /dev/null; then
    echo "Installing swag..."
    go install github.com/swaggo/swag/cmd/swag@latest
fi

# Generate docs
echo "Running swag init..."
swag init -g cmd/server/main.go -o docs/

echo "Swagger documentation generated successfully!"
echo "You can now access the Swagger UI at: http://localhost:8080/swagger/"
