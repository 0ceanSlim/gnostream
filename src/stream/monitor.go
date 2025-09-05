package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"gnostream/src/config"
	"gnostream/src/nostr"
)

// Monitor manages stream monitoring and HLS conversion
type Monitor struct {
	config       *config.Config
	streamConfig *config.StreamDefaults
	metadata     *config.StreamMetadata
	nostrClient  *nostr.Client
	ffmpegCmd    *exec.Cmd
	mutex        sync.RWMutex
	isActive     bool
	streamKey    string // Current active stream key
}

// NewMonitor creates a new stream monitor
func NewMonitor(cfg *config.Config) (*Monitor, error) {
	// Initialize Nostr client with integrated config
	nostrClient, err := nostr.NewClient(&cfg.Nostr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize nostr client: %w", err)
	}

	monitor := &Monitor{
		config:       cfg,
		streamConfig: cfg.GetStreamDefaults(),
		nostrClient:  nostrClient,
	}

	// Check if there's any existing metadata that indicates a "live" stream that shouldn't be
	// This helps clean up any incorrect live events from previous runs
	go monitor.cleanupIncorrectLiveEvents()

	return monitor, nil
}

// cleanupIncorrectLiveEvents cancels any live events that shouldn't exist
func (m *Monitor) cleanupIncorrectLiveEvents() {
	// Check if there are any HLS files that might indicate a false live status
	// This is mainly for cleanup from previous runs, since stream details are now separate

	// Also check if there are any HLS files that might indicate a false live status
	metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
	if _, err := os.Stat(metadataPath); err == nil {
		// Remove old metadata file to prevent confusion
		if err := os.Remove(metadataPath); err != nil {
			log.Printf("Warning: couldn't remove old metadata file: %v", err)
		}
	}
}

// Start begins monitoring the RTMP stream
func (m *Monitor) Start(ctx context.Context) error {
	log.Println("🎬 Stream monitor started")

	// Start stream info watcher in a separate goroutine
	go m.watchStreamInfo(ctx)

	// Check if RTMP is enabled - if so, only do file watching, not stream detection
	rtmpDefaults := m.config.GetRTMPDefaults()
	if rtmpDefaults.Enabled {
		log.Println("📡 RTMP mode: Only running file watcher (stream detection handled by RTMP server)")
		// Just wait for context cancellation - RTMP server handles stream detection
		<-ctx.Done()
		log.Println("📁 File watcher stopping...")
		return nil
	}

	// Traditional mode: do active stream detection
	ticker := time.NewTicker(m.streamConfig.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("📡 Stream monitor stopping...")
			if m.isActive {
				m.stopStream()
			}
			return nil
		case <-ticker.C:
			if err := m.checkStream(); err != nil {
				log.Printf("Stream check error: %v", err)
			}
		}
	}
}

// checkStream checks if the RTMP stream is active
func (m *Monitor) checkStream() error {
	streamActive := m.isStreamActive()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if streamActive && !m.isActive {
		// Stream just started
		log.Println("🔴 Stream detected - starting HLS conversion")
		return m.startStream()
	} else if !streamActive && m.isActive {
		// Stream just stopped
		log.Println("⚫ Stream stopped - stopping HLS conversion")
		return m.stopStream()
	}

	return nil
}

// startStream begins HLS conversion and Nostr broadcasting
func (m *Monitor) startStream() error {
	// Use stream details from config
	metadata := m.config.GetStreamMetadata()

	// Generate unique stream identifier
	metadata.Dtag = fmt.Sprintf("%d", rand.Intn(900000)+100000)
	metadata.Status = "live"
	metadata.Starts = fmt.Sprintf("%d", time.Now().Unix())
	metadata.Ends = ""
	// Use external URL if configured, otherwise use localhost
	baseURL := m.config.Server.ExternalURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", m.config.Server.Port)
	}
	
	metadata.StreamURL = fmt.Sprintf("%s/live/output.m3u8", baseURL)

	// Only set recording URL if recording is enabled
	if m.config.StreamInfo.Record {
		metadata.RecordingURL = fmt.Sprintf("%s/past-streams/%s-%s/output.m3u8",
			baseURL,
			time.Now().Format("1-2-2006"),
			metadata.Dtag)
	} else {
		metadata.RecordingURL = "" // No recording URL when recording disabled
	}

	m.metadata = metadata

	// Ensure output directory exists
	if err := os.MkdirAll(m.streamConfig.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save metadata to JSON
	metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
	if err := config.SaveStreamMetadata(metadataPath, metadata); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Start FFmpeg HLS conversion
	if err := m.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Broadcast Nostr start event and capture response
	go func() {
		eventJSON, successfulRelays := m.nostrClient.BroadcastStartEventWithResponse(metadata)
		m.mutex.Lock()
		m.metadata.LastNostrEvent = eventJSON
		m.metadata.SuccessfulRelays = successfulRelays
		m.mutex.Unlock()

		// Save updated metadata with Nostr info
		metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
		config.SaveStreamMetadata(metadataPath, m.metadata)
	}()

	m.isActive = true
	log.Println("✅ Stream started successfully")
	return nil
}

// stopStream stops HLS conversion and archives the stream
func (m *Monitor) stopStream() error {
	if m.ffmpegCmd != nil {
		// Stop FFmpeg
		if err := m.ffmpegCmd.Process.Kill(); err != nil {
			log.Printf("Error killing FFmpeg: %v", err)
		}
		m.ffmpegCmd.Wait()
		m.ffmpegCmd = nil
	}

	if m.metadata != nil {
		// Update metadata
		m.metadata.Status = "ended"
		m.metadata.Ends = fmt.Sprintf("%d", time.Now().Unix())

		// Save final metadata
		metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
		config.SaveStreamMetadata(metadataPath, m.metadata)

		// Archive the stream only if recording is enabled
		if m.config.StreamInfo.Record {
			if err := m.archiveStream(); err != nil {
				log.Printf("Error archiving stream: %v", err)
			}
		} else {
			log.Println("📡 Recording disabled - skipping archive process")
		}

		// Broadcast Nostr end event and capture response
		go func() {
			// Store original event ID before sending end event
			var originalEventID string
			if m.metadata.LastNostrEvent != "" {
				if id, err := nostr.ExtractEventID(m.metadata.LastNostrEvent); err == nil {
					originalEventID = id
				}
			}

			eventJSON, successfulRelays := m.nostrClient.BroadcastEndEventWithResponse(m.metadata)
			m.mutex.Lock()
			m.metadata.LastNostrEvent = eventJSON
			m.metadata.SuccessfulRelays = successfulRelays
			m.mutex.Unlock()

			// Check if we should send a deletion request for non-recorded streams
			if m.config.Nostr.DeleteNonRecorded && m.metadata.RecordingURL == "" && originalEventID != "" {
				log.Printf("🗑️ Stream ended without recording - sending deletion request")
				deletionJSON, deletionRelays := m.nostrClient.BroadcastDeletionEventWithResponse(
					originalEventID, 
					"Stream ended without recording - removing temporary live event",
				)
				log.Printf("🗑️ Deletion request sent: %s to %d relays", deletionJSON, len(deletionRelays))
			}

			// Save final metadata with Nostr info
			metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
			config.SaveStreamMetadata(metadataPath, m.metadata)
		}()
	}

	m.isActive = false
	if m.config.StreamInfo.Record {
		log.Println("✅ Stream stopped and archived")
	} else {
		log.Println("✅ Stream stopped")
	}
	return nil
}

// startFFmpeg starts the FFmpeg HLS conversion process
func (m *Monitor) startFFmpeg() error {
	outputPath := filepath.Join(m.streamConfig.OutputDir, "output.m3u8")

	// Get HLS config from stream info
	hlsConfig := m.config.GetHLSConfig()

	// Build FFmpeg arguments
	args := []string{
		"-i", m.streamConfig.RTMPUrl,
		"-c:v", "libx264",
		"-crf", "18",
		"-preset", "veryfast",
		"-c:a", "aac",
		"-b:a", "160k",
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", hlsConfig.SegmentTime),
	}

	// Configure HLS behavior based on recording setting
	if m.config.StreamInfo.Record {
		// Recording enabled: keep all segments, don't delete
		args = append(args, "-hls_list_size", "0") // 0 = unlimited playlist size
		// Don't add delete_segments flag - keep all segments for archival
	} else {
		// Live only: use playlist size limit and delete old segments
		args = append(args,
			"-hls_list_size", fmt.Sprintf("%d", hlsConfig.PlaylistSize),
			"-hls_flags", "delete_segments",
		)
	}

	args = append(args, outputPath)
	m.ffmpegCmd = exec.Command("ffmpeg", args...)

	if err := m.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	log.Println("🎥 FFmpeg HLS conversion started")
	return nil
}

// archiveStream moves stream files to archive directory
func (m *Monitor) archiveStream() error {
	if m.metadata == nil {
		return fmt.Errorf("no metadata available for archiving")
	}

	// Create archive directory
	archiveDir := filepath.Join(m.streamConfig.ArchiveDir,
		fmt.Sprintf("%s-%s",
			time.Now().Format("1-2-2006"),
			m.metadata.Dtag))

	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Move all files from output directory to archive
	files, err := filepath.Glob(filepath.Join(m.streamConfig.OutputDir, "*"))
	if err != nil {
		return fmt.Errorf("failed to list output files: %w", err)
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		destPath := filepath.Join(archiveDir, fileName)

		if err := os.Rename(file, destPath); err != nil {
			log.Printf("Failed to move file %s: %v", file, err)
		}
	}

	log.Printf("📁 Stream archived to: %s", archiveDir)
	return nil
}

// isStreamActive checks if the RTMP stream is currently active
func (m *Monitor) isStreamActive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ffprobe",
		"-i", m.streamConfig.RTMPUrl,
		"-show_streams",
		"-select_streams", "v",
		"-show_entries", "stream=codec_name",
		"-of", "json",
		"-v", "quiet",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// Parse JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return false
	}

	// Check if streams exist
	streams, ok := result["streams"].([]interface{})
	if !ok || len(streams) == 0 {
		return false
	}

	// Look for video stream with codec
	for _, stream := range streams {
		streamMap, ok := stream.(map[string]interface{})
		if !ok {
			continue
		}
		if codecName, exists := streamMap["codec_name"].(string); exists && codecName != "" {
			return true
		}
	}

	return false
}

// GetCurrentMetadata returns the current stream metadata
func (m *Monitor) GetCurrentMetadata() *config.StreamMetadata {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.metadata != nil {
		return m.metadata
	}

	// Return offline status if no active stream
	return &config.StreamMetadata{
		Status:  "offline",
		Title:   "Stream Offline",
		Summary: "The stream is currently offline",
	}
}

// IsActive returns whether the stream is currently active
func (m *Monitor) IsActive() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isActive
}

// HandleStreamStart handles when an RTMP stream starts
func (m *Monitor) HandleStreamStart(streamKey string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.isActive {
		log.Printf("Stream already active, ignoring new stream: %s", streamKey)
		return
	}

	log.Printf("🔴 RTMP stream started: %s", streamKey)
	m.streamKey = streamKey

	// Start stream processing
	if err := m.startStreamsrc(); err != nil {
		log.Printf("Failed to start stream processing: %v", err)
		return
	}

	m.isActive = true
}

// HandleStreamStop handles when an RTMP stream stops
func (m *Monitor) HandleStreamStop(streamKey string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isActive || m.streamKey != streamKey {
		return
	}

	log.Printf("⚫ RTMP stream stopped: %s", streamKey)

	// Stop stream processing
	if err := m.stopStreamsrc(); err != nil {
		log.Printf("Failed to stop stream processing: %v", err)
	}

	m.isActive = false
	m.streamKey = ""
}

// startStreamsrc starts stream processing without checking RTMP
func (m *Monitor) startStreamsrc() error {
	// Use stream details from config
	metadata := m.config.GetStreamMetadata()

	// Generate unique stream identifier
	metadata.Dtag = fmt.Sprintf("%d", rand.Intn(900000)+100000)
	metadata.Status = "live"
	metadata.Starts = fmt.Sprintf("%d", time.Now().Unix())
	metadata.Ends = ""
	// Use external URL if configured, otherwise use localhost
	baseURL := m.config.Server.ExternalURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", m.config.Server.Port)
	}
	
	metadata.StreamURL = fmt.Sprintf("%s/live/output.m3u8", baseURL)

	// Only set recording URL if recording is enabled
	if m.config.StreamInfo.Record {
		// Create archive directory name that will be used later for consistent naming
		archiveDirName := fmt.Sprintf("%s-%s", time.Now().Format("1-2-2006"), metadata.Dtag)
		metadata.RecordingURL = fmt.Sprintf("%s/archive/%s/output.m3u8",
			baseURL,
			archiveDirName)
	} else {
		metadata.RecordingURL = "" // No recording URL when recording disabled
	}

	m.metadata = metadata

	// Save metadata to JSON
	metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
	if err := config.SaveStreamMetadata(metadataPath, metadata); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Broadcast Nostr start event and capture response
	go func() {
		eventJSON, successfulRelays := m.nostrClient.BroadcastStartEventWithResponse(metadata)
		m.mutex.Lock()
		m.metadata.LastNostrEvent = eventJSON
		m.metadata.SuccessfulRelays = successfulRelays
		m.mutex.Unlock()

		// Save updated metadata with Nostr info
		metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
		config.SaveStreamMetadata(metadataPath, m.metadata)
	}()

	log.Println("✅ Stream started successfully")
	return nil
}

// stopStreamsrc stops stream processing without checking RTMP
func (m *Monitor) stopStreamsrc() error {
	if m.metadata != nil {
		// Update metadata
		m.metadata.Status = "ended"
		m.metadata.Ends = fmt.Sprintf("%d", time.Now().Unix())

		// Save final metadata
		metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
		config.SaveStreamMetadata(metadataPath, m.metadata)

		// Archive the stream only if recording is enabled
		if m.config.StreamInfo.Record {
			if err := m.archiveStream(); err != nil {
				log.Printf("Error archiving stream: %v", err)
			}
		} else {
			log.Println("📡 Recording disabled - skipping archive process")
		}

		// Broadcast Nostr end event and capture response
		go func() {
			// Store original event ID before sending end event
			var originalEventID string
			if m.metadata.LastNostrEvent != "" {
				if id, err := nostr.ExtractEventID(m.metadata.LastNostrEvent); err == nil {
					originalEventID = id
				}
			}

			eventJSON, successfulRelays := m.nostrClient.BroadcastEndEventWithResponse(m.metadata)
			m.mutex.Lock()
			m.metadata.LastNostrEvent = eventJSON
			m.metadata.SuccessfulRelays = successfulRelays
			m.mutex.Unlock()

			// Check if we should send a deletion request for non-recorded streams
			if m.config.Nostr.DeleteNonRecorded && m.metadata.RecordingURL == "" && originalEventID != "" {
				log.Printf("🗑️ Stream ended without recording - sending deletion request")
				deletionJSON, deletionRelays := m.nostrClient.BroadcastDeletionEventWithResponse(
					originalEventID, 
					"Stream ended without recording - removing temporary live event",
				)
				log.Printf("🗑️ Deletion request sent: %s to %d relays", deletionJSON, len(deletionRelays))
			}

			// Save final metadata with Nostr info
			metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
			config.SaveStreamMetadata(metadataPath, m.metadata)
		}()
	}

	if m.config.StreamInfo.Record {
		log.Println("✅ Stream stopped and archived")
	} else {
		log.Println("✅ Stream stopped")
	}
	return nil
}

// watchStreamInfo monitors the stream info file for changes and broadcasts updates
func (m *Monitor) watchStreamInfo(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()

	log.Println("👁️ Stream info watcher started")

	for {
		select {
		case <-ctx.Done():
			log.Println("📁 Stream info watcher stopping...")
			return
		case <-ticker.C:
			if err := m.checkStreamInfoChanges(); err != nil {
				log.Printf("Stream info check error: %v", err)
			}
		}
	}
}

// checkStreamInfoChanges checks for stream info file changes and broadcasts updates if needed
func (m *Monitor) checkStreamInfoChanges() error {
	_, changed, err := m.config.CheckAndReloadStreamInfo()
	if err != nil {
		return err
	}


	// Only broadcast update if we have an active stream and the info actually changed
	if changed && m.isActive && m.metadata != nil {
		m.mutex.Lock()
		// Update the current stream metadata with new info
		newMetadata := m.config.GetStreamMetadata()

		// Preserve runtime fields from existing metadata
		newMetadata.Dtag = m.metadata.Dtag
		newMetadata.Status = m.metadata.Status
		newMetadata.Starts = m.metadata.Starts
		newMetadata.Ends = m.metadata.Ends
		newMetadata.StreamURL = m.metadata.StreamURL
		newMetadata.RecordingURL = m.metadata.RecordingURL

		m.metadata = newMetadata
		m.mutex.Unlock()

		// Save updated metadata to JSON
		metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
		if err := config.SaveStreamMetadata(metadataPath, m.metadata); err != nil {
			log.Printf("Failed to save updated metadata: %v", err)
		}

		// Broadcast update event to Nostr relays and capture response
		go func() {
			eventJSON, successfulRelays := m.nostrClient.BroadcastUpdateEventWithResponse(m.metadata)
			m.mutex.Lock()
			m.metadata.LastNostrEvent = eventJSON
			m.metadata.SuccessfulRelays = successfulRelays
			m.mutex.Unlock()

			// Save updated metadata with Nostr info
			metadataPath := filepath.Join(m.streamConfig.OutputDir, "metadata.json")
			config.SaveStreamMetadata(metadataPath, m.metadata)
		}()

		log.Println("🔄 Stream info updated and broadcasted to Nostr relays")
	}

	return nil
}
