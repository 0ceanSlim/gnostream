FROM alpine:latest

# Install dependencies for downloading and extracting releases, plus ffmpeg for stream processing
RUN apk add --no-cache ca-certificates curl jq unzip tar gzip wget ffmpeg

WORKDIR /tmp

# Detect architecture and download appropriate release
RUN ARCH=$(uname -m) && \
    case $ARCH in \
        x86_64) GNOSTREAM_ARCH="amd64" ;; \
        aarch64) GNOSTREAM_ARCH="arm64" ;; \
        *) echo "Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    echo "Detected architecture: $ARCH -> gnostream-linux-$GNOSTREAM_ARCH" && \
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/yourusername/gnostream/releases/latest | jq -r .tag_name) && \
    echo "Downloading GNOSTREAM version: $LATEST_RELEASE for linux-$GNOSTREAM_ARCH" && \
    DOWNLOAD_URL="https://github.com/yourusername/gnostream/releases/download/$LATEST_RELEASE/gnostream-linux-$GNOSTREAM_ARCH.tar.gz" && \
    curl -L "$DOWNLOAD_URL" -o gnostream-release.tar.gz && \
    echo "Extracting release..." && \
    tar -xzf gnostream-release.tar.gz && \
    EXTRACTED_DIR=$(tar -tzf gnostream-release.tar.gz | head -1 | cut -f1 -d"/") && \
    echo "Moving files from $EXTRACTED_DIR to /app..." && \
    mkdir -p /app && \
    mv "$EXTRACTED_DIR"/* /app/ && \
    chmod +x /app/gnostream && \
    rm -rf /tmp/*

WORKDIR /app

# Create non-root user for security
RUN addgroup -g 1001 gnostream && \
    adduser -D -s /bin/sh -u 1001 -G gnostream gnostream && \
    chown -R gnostream:gnostream /app

USER gnostream

EXPOSE 8181

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8181/ || exit 1

CMD ["./gnostream"]