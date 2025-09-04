# TODO - Stream Info Updates Issue

## Problem
Stream info updates from `stream-info.yml` are not being detected or broadcast to Nostr when the file is modified during an active stream.

## What Works ✅
- RTMP server reconnection - OBS can connect multiple times without restarting service
- External URL configuration - Nostr events use `https://live.happytavern.co` instead of localhost  
- Configurable RTMP host - Server binds to specific IP (10.1.10.5:1935)
- Simplified stream key - Just use `rtmp://10.1.10.5:1935/live` (no stream key needed)

## Investigation Steps 🔍

1. **Debug why stream info watcher isn't starting properly**
   - Add debug logging to `watchStreamInfo` function in `src/stream/monitor.go:513`
   - Confirm the "👁️ Stream info watcher started" message appears in logs

2. **Check if checkStreamInfoChanges is being called**
   - Add debug logging to `checkStreamInfoChanges` function
   - Verify it runs every 2 seconds during active streams

3. **Verify file change detection**
   - Check if `CheckAndReloadStreamInfo` in `src/config/config.go:289` detects file modification time changes
   - Test by modifying `stream-info.yml` and checking logs

4. **Test update broadcasting**
   - Confirm that when changes are detected, Nostr update events are broadcast
   - Look for "📡 Broadcasting stream update event" messages in logs

## Files to Check
- `src/stream/monitor.go` - Lines 75, 513, 533
- `src/config/config.go` - Line 289
- `stream-info.yml` - Target file for updates

## Test Scenario
1. Start streaming from OBS
2. Modify `stream-info.yml` (change title/summary)
3. Check logs for detection and broadcast messages
4. Verify Nostr event is updated with new info

## Current Status
- Stream starts with old info even after YAML changes
- No "Stream info reloaded" messages appearing in logs
- Watcher may not be starting or file changes not detected