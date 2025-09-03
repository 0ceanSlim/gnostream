# gnostream

Live streaming server with RTMP support and automatic Nostr broadcasting. Built in Go with configurable recording and real-time metadata updates.

## Features

- **RTMP Streaming**: Built-in RTMP server for live streaming
- **HLS Conversion**: Automatic conversion to HLS format for web playback
- **Nostr Broadcasting**: Automatic event publishing to Nostr relays
- **Live Recording Control**: Toggle recording on/off per stream
- **Real-time Updates**: Edit stream info while live streaming
- **Archive System**: Automatic archival of recorded streams
- **Relay Verification**: Track which Nostr relays accept your events

## Requirements

- **Go 1.21+** (for building from source)
- **FFmpeg** (required for RTMP processing and HLS conversion)

### Installing FFmpeg

**Windows:**

- Download from [ffmpeg.org](https://ffmpeg.org/download.html)
- Add to PATH environment variable

**macOS:**

```bash
brew install ffmpeg
```

**Linux (Ubuntu/Debian):**

```bash
sudo apt update
sudo apt install ffmpeg
```

## Quick Start

1. **Install dependencies**

   ```bash
   # Make sure FFmpeg is installed and in PATH
   ffmpeg -version
   ```

2. **Build and run**

   ```bash
   go mod tidy
   go build -o gnostream
   ./gnostream
   ```

3. **Configure your stream**

   - Edit `config.yml` with your Nostr keys
   - Edit `stream-info.yml` with your stream details

4. **Start streaming**
   - Stream to: `rtmp://localhost:1935/live/stream`
   - View at: `http://localhost:8080`

## Configuration

### config.yml

```yaml
server:
  port: 8080 # Web server port
  host: "127.0.0.1" # Web server host

hls:
  segment_time: 10 # Seconds per video segment
  playlist_size: 10 # Number of segments to keep

stream_info_path: "stream-info.yml" # Path to stream metadata

nostr:
  public_key: "your-nostr-public-key-hex"
  private_key: "your-nostr-private-key-hex"
  relays:
    - "wss://relay.damus.io"
    - "wss://nos.lol"
```

### stream-info.yml

```yaml
title: "My Live Stream"
summary: "Stream description here"
image: "https://example.com/thumbnail.jpg"
tags: ["live", "coding", "tech"]
record: true # true = save recordings, false = live only
```

## Usage

- **Live streaming**: Stream appears automatically when you connect to RTMP
- **Recording control**: Set `record: false` for live-only (saves disk space)
- **Live updates**: Edit `stream-info.yml` while streaming to update metadata
- **Nostr events**: Automatic start/update/end events broadcast to configured relays

## Binary Releases Planned

Pre-built binaries will be available on the [GitHub releases page](https://github.com/your-repo/gnostream/releases) when I've completed testing and written a build process.
