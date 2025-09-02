# Multi-stage build for secure Go application
FROM golang:1.23.3-alpine3.20 AS builder

# Security: Update packages and install minimal dependencies
RUN apk update && apk upgrade && \
    apk add --no-cache --update git ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

# Security: Create non-root user for build
RUN adduser -D -s /bin/sh -u 1001 appuser

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./

# Security: Download dependencies as non-root
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Security: Build with security flags and strip debugging info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o stream-server ./cmd/server

# Security: Use distroless base image instead of alpine
FROM gcr.io/distroless/static-debian12:nonroot

# Security: Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Security: Set up non-root user (distroless provides nonroot user)
USER nonroot:nonroot

WORKDIR /app

# Security: Copy binary with correct ownership
COPY --from=builder --chown=nonroot:nonroot /app/stream-server ./stream-server

# Copy static files with correct ownership  
COPY --from=builder --chown=nonroot:nonroot /app/www ./www
COPY --from=builder --chown=nonroot:nonroot /app/configs ./configs

# Security: Create directories with correct permissions
USER root
RUN mkdir -p /app/streams/live /app/streams/archive && \
    chown -R nonroot:nonroot /app/streams
USER nonroot:nonroot

# Security: Use non-privileged port
EXPOSE 8080

# Security: Set resource limits and read-only filesystem
LABEL \
    org.opencontainers.image.title="Stream Node Server" \
    org.opencontainers.image.description="Cyberpunk-themed live streaming server" \
    org.opencontainers.image.version="2.1.4" \
    org.opencontainers.image.vendor="Neural Interface" \
    org.opencontainers.image.licenses="MIT"

# Security: Use exec form and run as non-root
ENTRYPOINT ["./stream-server"]
