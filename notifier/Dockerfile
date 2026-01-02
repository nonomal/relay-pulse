# Build stage
FROM golang:1.24-bookworm AS builder

WORKDIR /build

# Copy go mod files first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Install Playwright driver and Chromium (before copying source for cache optimization)
# Pin to specific version matching go.mod playwright-go dependency
RUN go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.1 install --with-deps chromium

# Copy source and build
COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o notifier ./cmd/notifier

# Runtime stage - use Debian for Playwright/Chromium compatibility
FROM debian:bookworm-slim

# Prevent tzdata interactive prompts
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Shanghai

# Install runtime dependencies for Chromium
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    wget \
    # Core libraries
    libglib2.0-0 \
    libfontconfig1 \
    # Playwright/Chromium dependencies
    libnss3 \
    libnspr4 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libcups2 \
    libdrm2 \
    libdbus-1-3 \
    libxkbcommon0 \
    libatspi2.0-0 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libgbm1 \
    libasound2 \
    libpango-1.0-0 \
    libcairo2 \
    libx11-6 \
    libx11-xcb1 \
    libxcb1 \
    libxext6 \
    libxi6 \
    libxrender1 \
    libxss1 \
    libxtst6 \
    # Chinese font support
    fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary
COPY --from=builder /build/notifier .

# Copy Playwright driver and browsers from builder
COPY --from=builder /root/.cache/ms-playwright /root/.cache/ms-playwright

# Copy default config template
COPY config.yaml.example config.yaml

# Create data directory
RUN mkdir -p /app/data

# Expose API port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8081/health || exit 1

ENTRYPOINT ["./notifier"]
