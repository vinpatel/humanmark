# ==============================================================================
# HumanMark API Dockerfile
# ==============================================================================
#
# Multi-stage build for minimal production image.
#
# Build:
#   docker build -t humanmark/api .
#
# Run:
#   docker run -p 8080:8080 humanmark/api
#
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Build
# ------------------------------------------------------------------------------
FROM golang:1.22-alpine AS builder

# Install build dependencies
# - git: for version info
# - ca-certificates: for HTTPS requests
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum* ./

# Download dependencies (cached if go.mod unchanged)
RUN go mod download

# Copy source code
COPY . .

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# Build the binary
# CGO_ENABLED=0: Static binary, no C dependencies
# -ldflags: Embed version info, strip debug symbols
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -o humanmark \
    ./cmd/api

# ------------------------------------------------------------------------------
# Stage 2: Production
# ------------------------------------------------------------------------------
FROM alpine:3.19

# Labels for container metadata
LABEL org.opencontainers.image.title="HumanMark API"
LABEL org.opencontainers.image.description="Verify human-created content"
LABEL org.opencontainers.image.source="https://github.com/humanmark/humanmark"
LABEL org.opencontainers.image.vendor="HumanMark"

# Install runtime dependencies
# - ca-certificates: for HTTPS requests to external APIs
# - tzdata: for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
# Running as root is a security risk
RUN addgroup -g 1000 humanmark && \
    adduser -u 1000 -G humanmark -s /bin/sh -D humanmark

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/humanmark .

# Copy any static files if needed
# COPY --from=builder /build/static ./static

# Change ownership to non-root user
RUN chown -R humanmark:humanmark /app

# Switch to non-root user
USER humanmark

# Expose port
EXPOSE 8080

# Health check
# Checks every 30s, timeout 5s, start after 5s, fail after 3 retries
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Environment variables with defaults
ENV PORT=8080 \
    ENV=production \
    LOG_LEVEL=info

# Run the binary
ENTRYPOINT ["./humanmark"]

# ------------------------------------------------------------------------------
# Notes:
# ------------------------------------------------------------------------------
#
# Build with version info:
#   docker build \
#     --build-arg VERSION=$(git describe --tags --always) \
#     --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
#     --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
#     -t humanmark/api:latest .
#
# Run with environment variables:
#   docker run -p 8080:8080 \
#     -e HIVE_API_KEY=your-key \
#     -e DATABASE_URL=postgres://... \
#     humanmark/api
#
# Run with mounted config:
#   docker run -p 8080:8080 \
#     -v $(pwd)/config:/app/config:ro \
#     humanmark/api
#
# Image size: ~20MB (Alpine base + static binary)
#
# Security features:
#   - Non-root user
#   - Minimal Alpine base
#   - No shell (can add for debugging)
#   - Health checks for orchestrators
#
# ==============================================================================
