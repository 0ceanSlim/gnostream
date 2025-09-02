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

	"stream-server/internal/config"
	"stream-server/internal/nostr"
)

// Monitor manages stream monitoring and HLS conversion
type Monitor struct {
	config      *config.Config
	metadata    *config.StreamMetadata
	nostrClient *nostr.Client
	ffmpegCmd   *exec.Cmd
	mutex       sync.RWMutex
	isActive    bool
}

// NewMonitor creates a new stream monitor
func NewMonitor(cfg *config.Config) (*Monitor, error) {
	// Load Nostr configuration
	nostrCfg, err := config.LoadNostrConfig(cfg.Nostr.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load nostr config: %w", err)
	}

	// Initialize Nostr client
	nostrClient, err := nostr.NewClient(nostrCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize nostr client: %w", err)
	}

	return &Monitor{
		config:      cfg,
		nostrClient: nostrClient,
	}, nil
}

// Start begins monitoring the RTMP stream
func (m *Monitor) Start(ctx context.Context) error {
	log.Println("🎬 Stream monitor started")

	ticker := time.NewTicker(m.config.Stream.CheckInterval)
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
	// Load stream metadata
	metadata, err := config.LoadStreamMetadata(m.config.Metadata.ConfigFile)
	if err != nil {
		log.Printf("Failed to load stream metadata: %v", err)
		metadata = &config.StreamMetadata{
			Title:   "Live Stream",
			Summary: "Currently streaming live",
		}
	}

	// Generate unique stream identifier
	metadata.Dtag = fmt.Sprintf("%d", rand.Intn(900000)+100000)
	metadata.Status = "live"
	metadata.Starts = fmt.Sprintf("%d", time.Now().Unix())
	metadata.Ends = ""
	metadata.StreamURL = fmt.Sprintf("http://localhost:%d/live/output.m3u8", m.config.Server.Port)
	metadata.RecordingURL = fmt.Sprintf("http://localhost:%d/past-streams/%s-%s/output.m3u8",
		m.config.Server.Port,
		time.Now().Format("1-2-2006"),
		metadata.Dtag)

	m.metadata = metadata

	// Ensure output directory exists
	if err := os.MkdirAll(m.config.Stream.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save metadata to JSON
	metadataPath := filepath.Join(m.config.Stream.OutputDir, "metadata.json")
	if err := config.SaveStreamMetadata(metadataPath, metadata); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Start FFmpeg HLS conversion
	if err := m.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Broadcast Nostr start event
	go m.nostrClient.BroadcastStartEvent(metadata)

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
		metadataPath := filepath.Join(m.config.Stream.OutputDir, "metadata.json")
		config.SaveStreamMetadata(metadataPath, m.metadata)

		// Archive the stream
		if err := m.archiveStream(); err != nil {
			log.Printf("Error archiving stream: %v", err)
		}

		// Broadcast Nostr end event
		go m.nostrClient.BroadcastEndEvent(m.metadata)
	}

	m.isActive = false
	log.Println("✅ Stream stopped and archived")
	return nil
}

// startFFmpeg starts the FFmpeg HLS conversion process
func (m *Monitor) startFFmpeg() error {
	outputPath := filepath.Join(m.config.Stream.OutputDir, "output.m3u8")

	m.ffmpegCmd = exec.Command("ffmpeg",
		"-i", m.config.Stream.RTMPUrl,
		"-c:v", "libx264",
		"-crf", "18",
		"-preset", "veryfast",
		"-c:a", "aac",
		"-b:a", "160k",
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", m.config.HLS.SegmentTime),
		"-hls_list_size", fmt.Sprintf("%d", m.config.HLS.PlaylistSize),
		"-hls_flags", "delete_segments",
		outputPath,
	)

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
	archiveDir := filepath.Join(m.config.Stream.ArchiveDir,
		fmt.Sprintf("%s-%s",
			time.Now().Format("1-2-2006"),
			m.metadata.Dtag))

	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Move all files from output directory to archive
	files, err := filepath.Glob(filepath.Join(m.config.Stream.OutputDir, "*"))
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
		"-i", m.config.Stream.RTMPUrl,
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
