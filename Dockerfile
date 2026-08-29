# syntax=docker/dockerfile:1.7
# $ docker build --target prod-distroless --no-cache -t diameter-tscd/stackyrd:1.0.4-clover-prd .

ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.21

FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN apk add --no-cache bash && chmod +x ./scripts/yrd && chmod +x ./scripts/dist/* 2>/dev/null || true && \
    CGO_ENABLED=0 ./scripts/yrd build --dev --no-tui
RUN ls -lh dist/stackyrd && test -x dist/stackyrd

FROM builder AS test
RUN go test ./...

FROM alpine:${ALPINE_VERSION} AS prod
RUN apk --no-cache add ca-certificates wget
WORKDIR /app
COPY --from=builder /app/dist/stackyrd ./stackyrd
COPY --from=builder /app/dist/config.yaml ./config.yaml
RUN mkdir -p store/plugins && adduser -D -H appuser && chown -R appuser /app
USER appuser
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false
EXPOSE 8452
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=10s CMD wget --no-verbose --tries=1 --spider http://localhost:8452/health || exit 1
CMD ["./stackyrd", "-env", "production"]

FROM ubuntu:24.04 AS prod-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/dist/stackyrd ./stackyrd
COPY --from=builder /app/dist/config.yaml ./config.yaml
RUN mkdir -p store/plugins && useradd -m appuser && chown -R appuser /app
USER appuser
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false
EXPOSE 8452
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=10s CMD wget --no-verbose --tries=1 --spider http://localhost:8452/health || exit 1
CMD ["./stackyrd", "-env", "production"]

FROM gcr.io/distroless/static:nonroot AS prod-distroless
WORKDIR /app
COPY --from=builder /app/dist/stackyrd ./stackyrd
COPY --from=builder /app/dist/config.yaml ./config.yaml
EXPOSE 8452
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false
CMD ["/app/stackyrd", "-env", "production"]

FROM prod-distroless AS prod-minimal

FROM golang:${GO_VERSION}-alpine AS dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN apk add --no-cache bash && chmod +x ./scripts/yrd && chmod +x ./scripts/dist/* 2>/dev/null || true && \
    CGO_ENABLED=0 ./scripts/yrd build --dev --no-tui && cp dist/stackyrd ./stackyrd
RUN mkdir -p store/plugins
ENV APP_QUIET_STARTUP=false
ENV APP_ENABLE_TUI=false
EXPOSE 8452
CMD ["./stackyrd", "-env", "development"]

FROM prod AS ultra-prod
FROM dev AS ultra-dev
FROM test AS ultra-test
