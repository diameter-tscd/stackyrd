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
    -ldflags="-s -w -buildid=" \
    -trimpath \
    -o stackyrd-nano ./cmd/app

# Test stage
FROM builder AS test

# Run tests
RUN go test ./...

# Production stage (Alpine - ~50MB, Python plugin support)
FROM alpine:3.23 AS prod

# Install ca-certificates for HTTPS and Python 3 for external (Python) plugins
RUN apk --no-cache add ca-certificates python3 py3-pip && \
    pip3 install --no-cache-dir grpcio protobuf

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/stackyrd-nano .

# Copy config
COPY --from=builder /app/config.yaml .

# Copy Python plugin host scripts (for external/ext: plugins)
COPY --from=builder /app/pkg/plugin/python ./pkg/plugin/python/

# Create plugin store directory for writable overlay (uploaded scripts, etc.)
RUN mkdir -p store/plugins

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Run the application
CMD ["./stackyrd-nano", "-env", "production"]

# Slim production stage (Ubuntu minimal - ~40MB, Python plugin support)
FROM ubuntu:24.04 AS prod-slim

WORKDIR /root/

# Install minimal runtime dependencies and Python 3 for external plugins
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    python3 \
    python3-pip \
    python3-venv \
    && pip3 install --no-cache-dir grpcio protobuf \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from builder stage
COPY --from=builder /app/stackyrd-nano .

# Copy config
COPY --from=builder /app/config.yaml .

# Copy Python plugin host scripts (for external/ext: plugins)
COPY --from=builder /app/pkg/plugin/python ./pkg/plugin/python/

# Create plugin store directory for writable overlay
RUN mkdir -p store/plugins

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Run the application
CMD ["./stackyrd-nano", "-env", "production"]

# Minimal production stage (Distroless - ultra-minimal, TS/Go plugins only)
# NOTE: Python/external (ext:) plugins are not supported in this stage
# because distroless/static does not include a Python runtime.
# TypeScript (ts:) and Go (go:) plugins work fine (compiled into binary).
FROM gcr.io/distroless/static:nonroot AS prod-distroless

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /app/stackyrd-nano /stackyrd-nano

# Copy config
COPY --from=builder /app/config.yaml .

# Copy Python host scripts (in case Python is layered on top)
COPY --from=builder /app/pkg/plugin/python /pkg/plugin/python/

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Run the application
CMD ["/stackyrd-nano", "-env", "production"]

# Development stage (full toolchain + Python plugin support)
FROM golang:1.25.5-alpine3.23 AS dev

WORKDIR /app

# Install Python 3 for external plugin development
RUN apk --no-cache add python3 py3-pip && \
    pip3 install --no-cache-dir grpcio protobuf

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o stackyrd-nano ./cmd/app

# Create plugin store directory
RUN mkdir -p store/plugins

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server
EXPOSE 8080

# Run the application
CMD ["./stackyrd-nano", "-env", "development"]
