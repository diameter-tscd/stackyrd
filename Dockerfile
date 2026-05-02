# Multi-stage Dockerfile for development, testing, and production

# Build stage - optimized for smaller size
FROM golang:1.25.5-alpine3.23 AS builder

WORKDIR /app

# Install build dependencies if needed
# RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s" \
    -trimpath \
    -o stackyrd ./cmd/app

# Test stage
FROM builder AS test

# Run tests
RUN go test ./...

# Production stage (Alpine - ~50MB)
FROM alpine:latest AS prod

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/stackyrd .

# Copy config and plugins if needed
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/banner.txt .
COPY --from=builder /app/plugins ./plugins/

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Run the application
CMD ["./stackyrd", "-env", "production"]

# Slim production stage (Ubuntu minimal - ~30-40MB, more secure than Alpine)
FROM ubuntu:24.04 AS prod-slim

WORKDIR /root/

# Install minimal runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from builder stage
COPY --from=builder /app/stackyrd .

# Copy config and plugins
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/banner.txt .
COPY --from=builder /app/plugins ./plugins/

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Run the application
CMD ["./stackyrd", "-env", "production"]

# Minimal production stage (Distroless - ultra-minimal)
FROM gcr.io/distroless/static:latest AS prod-distroless

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /app/stackyrd /stackyrd

# Copy config and plugins
COPY --from=builder /app/config.yaml .
COPY --from=builder /app/banner.txt .
COPY --from=builder /app/plugins ./plugins/

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Use non-root user (already set by distroless)
USER nonroot:nonroot

# Run the application
CMD ["/stackyrd", "-env", "production"]

# Development stage
FROM golang:1.25.5-alpine3.23 AS dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o stackyrd ./cmd/app

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Run the application
CMD ["./stackyrd", "-env", "development"]
