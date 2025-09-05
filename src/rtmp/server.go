package rtmp

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"gnostream/src/config"
)

// Server represents a simple RTMP-like server that uses FFmpeg for RTMP handling
type Server struct {
	config        *config.Config
	listener      net.Listener
	activeStreams map[string]*StreamContext
	mutex         sync.RWMutex
	onStreamStart func(streamKey string)
	onStreamStop  func(streamKey string)
	ctx           context.Context
	cancel        context.CancelFunc
}

// StreamContext holds information about an active stream
type StreamContext struct {
	StreamKey string
	StartTime time.Time
	FFmpegCmd *exec.Cmd
}

// NewServer creates a new RTMP server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		config:        cfg,
		activeStreams: make(map[string]*StreamContext),
	}
}

// SetStreamHandlers sets callbacks for stream start/stop events
func (s *Server) SetStreamHandlers(onStart, onStop func(string)) {
	s.onStreamStart = onStart
	s.onStreamStop = onStop
}

// Start starts the RTMP server using FFmpeg as RTMP input
func (s *Server) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	rtmpDefaults := s.config.GetRTMPDefaults()
	log.Printf("🎬 RTMP server (FFmpeg-based) starting on port %d", rtmpDefaults.Port)

	// Start FFmpeg RTMP server immediately to listen for connections
	go s.startRTMPToHLSConversion("default")

	// Wait for context cancellation
	<-s.ctx.Done()
	return s.Stop()
}

// Stop stops the RTMP server
func (s *Server) Stop() error {
	log.Println("🛑 Stopping RTMP server...")

	if s.cancel != nil {
		s.cancel()
	}

	// Stop all active streams
	s.mutex.Lock()
	for streamKey, stream := range s.activeStreams {
		s.stopStreamProcessing(streamKey, stream)
	}
	s.activeStreams = make(map[string]*StreamContext)
	s.mutex.Unlock()

	return nil
}


// startRTMPToHLSConversion starts FFmpeg to receive RTMP and convert to HLS
func (s *Server) startRTMPToHLSConversion(streamKey string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.activeStreams[streamKey]; exists {
		log.Printf("⚠️ RTMP server already running for stream: %s", streamKey)
		return nil // Stream already processing
	}

	log.Printf("🎥 Starting RTMP server for stream: %s", streamKey)

	// Get defaults
	streamDefaults := s.config.GetStreamDefaults()
	rtmpDefaults := s.config.GetRTMPDefaults()

	// Ensure output directory exists
	if err := os.MkdirAll(streamDefaults.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Output path for HLS
	outputPath := filepath.Join(streamDefaults.OutputDir, "output.m3u8")

	// Use a simple "live" path - no complex stream key needed for personal server
	rtmpURL := fmt.Sprintf("rtmp://%s:%d/live", rtmpDefaults.Host, rtmpDefaults.Port)
	
	// Get HLS config from stream info
	hlsConfig := s.config.GetHLSConfig()

	// Start FFmpeg as an RTMP server that accepts connections and converts to HLS
	cmd := exec.CommandContext(s.ctx, "ffmpeg",
		"-f", "flv",
		"-listen", "1",
		"-i", rtmpURL,
		"-c:v", "libx264",
		"-crf", "18",
		"-preset", "veryfast",
		"-c:a", "aac",
		"-b:a", "160k",
		"-f", "hls",
		"-hls_time", fmt.Sprintf("%d", hlsConfig.SegmentTime),
		"-hls_list_size", fmt.Sprintf("%d", hlsConfig.PlaylistSize),
		"-hls_flags", "delete_segments",
		"-y",
		outputPath,
	)
	
	log.Printf("✅ RTMP server listening on %s", rtmpURL)

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg RTMP server: %w", err)
	}

	log.Printf("✅ FFmpeg RTMP server started, waiting for connection on %s", rtmpURL)

	// Store stream context
	s.activeStreams[streamKey] = &StreamContext{
		StreamKey: streamKey,
		StartTime: time.Now(),
		FFmpegCmd: cmd,
	}

	// Monitor FFmpeg process and HLS output to detect when stream actually starts/stops
	go func() {
		streamStarted := false
		lastHLSUpdate := time.Time{}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				currentHLSActive := s.hasActiveHLSOutput(outputPath)

				// Check if stream just started
				if !streamStarted && currentHLSActive {
					streamStarted = true
					lastHLSUpdate = time.Now()
					log.Printf("🔴 RTMP stream connected for: %s", streamKey)
					if s.onStreamStart != nil {
						go s.onStreamStart(streamKey)
					}
				}

				// Check if stream is active and update last seen time
				if streamStarted && currentHLSActive {
					lastHLSUpdate = time.Now()
				}

				// Check if stream has ended (no HLS updates for 15 seconds)
				if streamStarted && !currentHLSActive && time.Since(lastHLSUpdate) > 15*time.Second {
					log.Printf("⚫ RTMP stream ended (no HLS activity): %s", streamKey)
					if s.onStreamStop != nil {
						go s.onStreamStop(streamKey)
					}
					
					// Force kill FFmpeg first, then restart
					log.Printf("🔄 Killing FFmpeg and restarting RTMP server for: %s", streamKey)
					if cmd.Process != nil {
						cmd.Process.Kill()
					}
					s.stopStreamProcessing(streamKey, s.activeStreams[streamKey])
					
					// Restart RTMP server automatically after a brief delay
					go func() {
						time.Sleep(3 * time.Second) // Longer delay to ensure port is freed
						log.Printf("🔄 Restarting RTMP server for: %s", streamKey)
						s.startRTMPToHLSConversion(streamKey)
					}()
					return
				}

				// Check if FFmpeg process has ended
				if cmd.ProcessState != nil {
					if streamStarted {
						log.Printf("⚫ RTMP stream ended (FFmpeg stopped): %s", streamKey)
						if s.onStreamStop != nil {
							go s.onStreamStop(streamKey)
						}
					} else {
						log.Printf("📡 RTMP server stopped (no stream received): %s", streamKey)
					}
					s.stopStreamProcessing(streamKey, s.activeStreams[streamKey])
					
					// Restart RTMP server automatically after a brief delay
					go func() {
						log.Printf("🔄 Restarting RTMP server for: %s", streamKey)
						time.Sleep(2 * time.Second)
						s.startRTMPToHLSConversion(streamKey)
					}()
					return
				}
			}
		}
	}()

	return nil
}

// hasActiveHLSOutput checks if HLS files are being actively created
func (s *Server) hasActiveHLSOutput(outputPath string) bool {
	// Check if the m3u8 file exists and has recent modification time
	if info, err := os.Stat(outputPath); err == nil {
		// If file was modified within the last 8 seconds, stream is likely active
		if time.Since(info.ModTime()) < 8*time.Second {
			return true
		}
	}

	// Also check for .ts segment files which are created more frequently
	dir := filepath.Dir(outputPath)
	if files, err := filepath.Glob(filepath.Join(dir, "*.ts")); err == nil && len(files) > 0 {
		// Check if any .ts file was modified recently
		for _, file := range files {
			if info, err := os.Stat(file); err == nil {
				if time.Since(info.ModTime()) < 8*time.Second {
					return true
				}
			}
		}
	}

	return false
}

// stopStreamProcessing stops FFmpeg processing for a stream
func (s *Server) stopStreamProcessing(streamKey string, stream *StreamContext) {
	if stream == nil {
		return
	}

	log.Printf("⏹️ Stopping stream processing for: %s", streamKey)

	// Kill FFmpeg process
	if stream.FFmpegCmd != nil && stream.FFmpegCmd.Process != nil {
		if err := stream.FFmpegCmd.Process.Kill(); err != nil {
			log.Printf("Error killing FFmpeg process: %v", err)
		}
	}

	// Remove from active streams
	s.mutex.Lock()
	delete(s.activeStreams, streamKey)
	s.mutex.Unlock()

	// Notify stream stop
	if s.onStreamStop != nil {
		go s.onStreamStop(streamKey)
	}

	log.Printf("✅ Stream processing stopped for: %s", streamKey)
}

// GetActiveStreams returns a list of currently active stream keys
func (s *Server) GetActiveStreams() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	keys := make([]string, 0, len(s.activeStreams))
	for key := range s.activeStreams {
		keys = append(keys, key)
	}
	return keys
}

// IsStreamActive checks if a specific stream is active
func (s *Server) IsStreamActive(streamKey string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	_, exists := s.activeStreams[streamKey]
	return exists
}
