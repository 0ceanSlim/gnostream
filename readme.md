# gnostream

Live streaming server with embedded RTMP and cyberpunk web interface. Built in Go with automatic Nostr broadcasting.

## Features

- Embedded RTMP server (no external dependencies)
- Live HLS streaming with archive system
- Terminal-style cyberpunk web interface
- Automatic Nostr event broadcasting
- Real-time stream detection and monitoring

## Quick Start

1. **Build and run**
   ```bash
   go mod tidy
   go build -o gnostream
   ./gnostream
   ```

2. **Stream to** `rtmp://localhost:1935/live/default`

3. **View at** `http://localhost:8080`

## Requirements

- Go 1.21+
- FFmpeg (for HLS conversion)

Binary releases available on GitHub releases page.
