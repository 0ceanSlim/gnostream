# gnostream

Live streaming server with RTMP support and automatic Nostr broadcasting. Built in Go with configurable recording and real-time metadata updates.

## Features

- **RTMP Streaming**: Built-in RTMP server for live streaming
- **HLS Conversion**: Automatic conversion to HLS format for web playback
- **Nostr Broadcasting**: Automatic event publishing to Nostr relays
- **Real-time Updates**: Edit stream info while live streaming
- **Recording Control**: Toggle recording on/off per stream
- **Archive System**: Automatic archival of recorded streams
- **Web Interface**: Simple web viewer for your streams

## Requirements

- **Go 1.21+** (for building from source)
- **FFmpeg** (required for RTMP processing and HLS conversion)

### Installing FFmpeg

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install ffmpeg
```

**macOS:**
```bash
brew install ffmpeg
```

**Windows:**
- Download from [ffmpeg.org](https://ffmpeg.org/download.html)
- Add to PATH environment variable

## Quick Start

1. **Clone and build**
   ```bash
   git clone https://github.com/your-repo/gnostream.git
   cd gnostream
   go mod tidy
   go build -o gnostream
   ```

2. **Copy example configs**
   ```bash
   cp config.example.yml config.yml
   cp stream-info.example.yml stream-info.yml
   ```

3. **Configure**
   - Edit `config.yml` with your Nostr private key and server settings
   - Edit `stream-info.yml` with your stream details

4. **Run**
   ```bash
   ./gnostream
   ```

5. **Start streaming**
   - **RTMP URL**: `rtmp://your-server-ip:1935/live`
   - **Web viewer**: `http://your-server-ip:8080`

## Configuration

### config.yml

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  external_url: "https://live.yourdomain.com"

rtmp:
  port: 1935
  host: "0.0.0.0"

nostr:
  private_key: "nsec1abc..."  # Your Nostr private key
  delete_non_recorded: false  # Auto-delete events for streams without recordings
  relays:
    - "wss://relay.damus.io"
    - "wss://wheat.happytavern.co"
    - "wss://relay.nostr.band"

stream_info_path: "stream-info.yml"
```

### stream-info.yml

```yaml
title: "My Live Stream"
summary: "Stream description here"
image: "https://example.com/thumbnail.jpg"
tags: ["live", "gaming", "chill"]

# Recording (true = save for later, false = live only)
record: false

# HLS Settings
hls:
  segment_time: 10    # Seconds per segment
  playlist_size: 10   # Segments to keep in playlist
```

## Usage

- **Live streaming**: Connect to RTMP - stream starts automatically
- **Live updates**: Edit `stream-info.yml` while streaming to update title, description, and tags
- **Recording control**: Set `record: true/false` to save streams or stream live-only
- **Nostr events**: Automatic start/update/end events broadcast to configured relays
- **Event cleanup**: Enable `delete_non_recorded` to automatically remove Nostr events for streams that weren't recorded
