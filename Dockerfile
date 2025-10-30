# Build stage
FROM golang:1.25-alpine AS builder

# Install git and ca-certificates for building
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Install swag tool for Swagger generation
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy source code
COPY . .

# Generate Swagger documentation
RUN swag init -g cmd/server/main.go -o docs/

# Build the main application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

# Build the init-embeddings-write command
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o init-embeddings-write ./cmd/init-embeddings-write

# Build the update-embeddings command
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o update-embeddings ./cmd/update-embeddings

# Final stage
FROM alpine:latest

# Install ca-certificates for SSL connections
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 appuser && \
  adduser -u 1001 -G appuser -s /bin/sh -D appuser

# Set working directory
WORKDIR /home/appuser

# Copy the binaries from builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/init-embeddings-write .
COPY --from=builder /app/update-embeddings .

# Copy static files for the frontend
COPY --from=builder /app/static ./static

# Change ownership to non-root user
RUN chown -R appuser:appuser /home/appuser

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/healthz || exit 1

# Run the application
CMD ["./main"]
