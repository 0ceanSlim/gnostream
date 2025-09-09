# Gnostream CLI Usage Guide

Gnostream now includes comprehensive CLI tooling for configuration management, stream debugging, and maintenance operations. The CLI provides powerful command-line access to all streaming functionality while maintaining full server compatibility.

## Installation & Basic Usage

```bash
# Build gnostream with CLI support
go build -o gnostream .

# Show help
./gnostream help

# Start server (default mode)
./gnostream
# or explicitly
./gnostream server

# Use CLI commands
./gnostream config show
./gnostream stream status
```

## Command Structure

Gnostream uses a hierarchical command structure:
```
gnostream <COMMAND> <SUBCOMMAND> [OPTIONS]
```

## Available Commands

### 🛠️ Configuration Management (`config`)

Manage stream settings and server configuration.

```bash
# Show complete configuration
./gnostream config show

# Get specific values
./gnostream config get recording
./gnostream config get title
./gnostream config get server.port

# Set configuration values
./gnostream config set recording true
./gnostream config set title "My Gaming Stream"
./gnostream config set tags "gaming,live,halo"
./gnostream config set segment_time 15

# List all available keys
./gnostream config list

# Reload configuration from file (hot reload)
./gnostream config reload
```

**Available Configuration Keys:**
- `recording` - Enable/disable recording (true/false)
- `segment_time` - HLS segment duration in seconds
- `playlist_size` - HLS playlist size (number of segments)  
- `title` - Stream title
- `summary` - Stream summary/description
- `image` - Stream thumbnail image URL
- `tags` - Stream tags (comma-separated)
- `server.port` - Server port
- `server.host` - Server host
- `rtmp.port` - RTMP server port

### 📺 Stream Management (`stream`)

Debug and monitor active streams.

```bash
# Check if stream is active
./gnostream stream status

# Show detailed stream information
./gnostream stream info

# Debug information (file system, metadata)
./gnostream stream debug

# List stream files with sizes
./gnostream stream files
```

**Stream Status Output:**
- 🟢 **ONLINE** - Stream is active with HLS playlist
- 🔴 **OFFLINE** - No active stream detected
- 💾 **Recording status** - Shows if recording is enabled
- 📄 **Metadata availability** - Shows if metadata.json exists

### 🌐 Nostr Event Management (`events`)

Manage Nostr protocol stream events.

```bash
# List your stream events
./gnostream events list
./gnostream events list --limit 50 --recent
./gnostream events list --status live

# Search events by content
./gnostream events search "gaming"
./gnostream events search "halo campaign"

# Show detailed event information
./gnostream events show 1234567890abcdef

# Delete specific events
./gnostream events delete 1234567890abcdef

# Publish new events
./gnostream events publish start
./gnostream events publish end
./gnostream events publish update
```

**Event Types:**
- `start` - Publish stream start event
- `end` - Publish stream end event  
- `update` - Publish stream update event

**Search & Filter Options:**
- `--limit <n>` - Limit number of results (default: 20)
- `--status <status>` - Filter by status (live|ended)
- `--recent` - Show only recent events (last 24h)

### 🧹 Cleanup & Maintenance (`cleanup`)

Clean up old files and stale events.

```bash
# Show what would be cleaned (safe preview)
./gnostream cleanup dry-run

# Clean old HLS segments
./gnostream cleanup segments
./gnostream cleanup segments --older-than 30
./gnostream cleanup segments --older-than 7 --confirm

# Clean old archives  
./gnostream cleanup archives --older-than 90

# Clean stale Nostr events
./gnostream cleanup stale

# Run all cleanup operations
./gnostream cleanup all --confirm
```

**Cleanup Options:**
- `--older-than <days>` - Only clean files older than N days (default: 7)
- `--confirm` - Skip confirmation prompts
- `dry-run` - Preview what would be cleaned without doing it

**Cleanup Operations:**
- `segments` - Remove old HLS .ts files (non-recorded streams)
- `archives` - Clean old archived streams
- `stale` - Clean up stale Nostr live events (WIP)
- `all` - Run all cleanup operations

### ℹ️ System Information

```bash
# Show version
./gnostream version

# Show help
./gnostream help
./gnostream <command> --help
```

## Operational Modes

### Server Mode (Default)
When run without arguments or with `server` command, gnostream starts the full streaming server:

```bash
./gnostream          # Default server mode
./gnostream server   # Explicit server mode
```

### CLI Mode  
Any other command activates CLI mode for one-time operations:

```bash
./gnostream config get recording    # CLI mode
./gnostream stream status           # CLI mode  
./gnostream cleanup dry-run         # CLI mode
```

## Example Workflows

### Daily Maintenance
```bash
# Check stream status
./gnostream stream status

# Preview cleanup
./gnostream cleanup dry-run

# Clean old segments (keep 30 days)
./gnostream cleanup segments --older-than 30 --confirm
```

### Stream Configuration
```bash
# Show current settings
./gnostream config show

# Update stream info
./gnostream config set title "Halo 3 Campaign Run"
./gnostream config set tags "halo,campaign,legendary"
./gnostream config set recording true

# Reload config without server restart
./gnostream config reload
```

### Debug Stream Issues
```bash
# Check overall status
./gnostream stream status

# Examine files and metadata
./gnostream stream debug

# List actual stream files
./gnostream stream files
```

### Event Management
```bash
# Start streaming session
./gnostream events publish start

# Update stream info mid-session
./gnostream config set title "Boss Fight!"
./gnostream events publish update

# End streaming session
./gnostream events publish end

# Clean up old events
./gnostream events list --recent
./gnostream cleanup stale
```

## File Size Reporting

The CLI includes intelligent file size formatting:
- **Bytes**: `1,234 B`
- **Kilobytes**: `45.6 KB` 
- **Megabytes**: `123.4 MB`
- **Gigabytes**: `9.6 GB` (as seen in cleanup dry-run)

## Integration with Existing Features

The CLI fully integrates with existing gnostream functionality:

- ✅ **Configuration**: Uses same `config.yml` and `stream-info.yml`
- ✅ **Hot Reload**: Config changes apply immediately to running server
- ✅ **Nostr Integration**: Full access to Nostr client and relay management
- ✅ **File Management**: Works with same output/archive directories
- ✅ **Stream Monitoring**: Accesses same stream metadata and status

## TODO - Planned Features

### 🔄 High Priority
- [ ] **Real Nostr Event Querying** - Currently shows placeholder messages
  - Implement actual relay querying in `fetchStreamEvents()`
  - Add event filtering and search functionality
  - Enable real-time event monitoring

- [ ] **Advanced Log Integration** - Currently shows placeholder  
  - Parse gnostream log files
  - Filter by timestamp and severity
  - Show structured log output with highlighting

- [ ] **Stream Analytics** - Add metrics and statistics
  - Viewer count history
  - Stream duration tracking  
  - Bandwidth usage statistics

### 🎯 Medium Priority  
- [ ] **Enhanced File Management**
  - Archive compression options
  - Selective file cleanup (by stream title/date)
  - Automatic archive organization

- [ ] **Relay Management**
  - Add/remove Nostr relays via CLI
  - Test relay connectivity
  - Monitor relay response times

- [ ] **Backup & Restore**
  - Export/import stream configurations  
  - Backup archive management
  - Configuration versioning

### 🚀 Future Enhancements
- [ ] **Interactive Mode** - TUI interface for common operations
- [ ] **Scripting Support** - JSON output modes for automation
- [ ] **Remote Management** - Manage multiple gnostream instances
- [ ] **Plugin System** - Extensible command architecture

### 🐛 Known Limitations
- **Event Fetching**: Relay querying not implemented (shows mock data)
- **Log Parsing**: Log integration shows placeholder (needs log file discovery)
- **Stale Event Cleanup**: Basic structure exists but needs relay integration
- **Error Handling**: Some operations need more robust error reporting

## Contributing

To add new CLI commands:

1. Create new command file in `src/cli/commands/`
2. Implement `Execute(args []string) error` method
3. Add command to router in `src/cli/cli.go`
4. Update help documentation

Example command structure:
```go
type NewCommand struct {
    config *config.Config
}

func (c *NewCommand) Execute(args []string) error {
    // Command implementation
}
```