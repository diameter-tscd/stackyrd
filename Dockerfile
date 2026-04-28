# Multi-stage Dockerfile for development, testing, and production

# Build stage - optimized for smaller size
FROM golang:1.25.5-alpine3.23 AS builder

WORKDIR /app

# # Install build dependencies and UPX for compression
# RUN apk add --no-cache upx

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
    -o main ./cmd/app

# # Compress binary with UPX (ultra-brute for maximum compression)
# RUN upx --ultra-brute main

# Test stage
FROM builder AS test

# Run tests
RUN go test ./...

# Ultra test stage (ultra-minimal - Distroless)
FROM gcr.io/distroless/static:latest AS ultra-test

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /app/main /main

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server 
EXPOSE 8080 9090

# Use non-root user (already set by distroless)
USER nonroot:nonroot

# Run the application
CMD ["/main"]

# Development stage
FROM golang:1.25.5-alpine3.23 AS dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . . .

# Build the binary
RUN go build -o main ./cmd/app

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server 
EXPOSE 8080 9090

# Run the application
CMD ["./main"]

# Ultra development stage (ultra-minimal - Distroless)
FROM gcr.io/distroless/static:latest AS ultra-dev

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /app/main /main

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server 
EXPOSE 8080 9090

# Use non-root user (already set by distroless)
USER nonroot:nonroot

# Run the application
CMD ["/main"]

# Production stage (Alpine - ~50MB)
FROM alpine:latest AS prod

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .


# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server 
EXPOSE 8080

# Run the application
CMD ["./main"]

# Slim production stage (Ubuntu minimal - ~30-40MB, more secure than Alpine)
FROM ubuntu:24.04 AS prod-slim

WORKDIR /root/

# Install minimal runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server 
EXPOSE 8080

# Run the application
CMD ["./main"]

# Minimal production stage (BusyBox - ~10-20MB)
FROM busybox:1.36-glibc AS prod-minimal

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy CA certificates for HTTPS
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server 
EXPOSE 8080

# Run the application
CMD ["./main"]

# Ultra-minimal production stage (alternative - even smaller)
FROM gcr.io/distroless/static:latest AS ultra-prod

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /app/main /main

# Configure for Docker environment
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false

# Expose ports for main API server 
EXPOSE 8080

# Use non-root user (already set by distroless)
USER nonroot:nonroot

# Run the application
CMD ["/main"]
