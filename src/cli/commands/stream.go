package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gnostream/src/config"
)

// StreamCommand handles stream management and debugging
type StreamCommand struct {
	config *config.Config
}

// NewStreamCommand creates a new stream command
func NewStreamCommand(cfg *config.Config) *StreamCommand {
	return &StreamCommand{config: cfg}
}

// Execute runs the stream command
func (s *StreamCommand) Execute(args []string) error {
	if len(args) == 0 {
		s.printUsage()
		return nil
	}

	subcommand := args[0]

	switch subcommand {
	case "status":
		return s.handleStatus()
	case "info":
		return s.handleInfo()
	case "debug":
		return s.handleDebug()
	case "files":
		return s.handleFiles()
	case "logs":
		return s.handleLogs(args[1:])
	case "--help", "help":
		s.printUsage()
		return nil
	default:
		fmt.Printf("Unknown stream subcommand: %s\n\n", subcommand)
		s.printUsage()
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// printUsage prints stream command usage
func (s *StreamCommand) printUsage() {
	fmt.Println(`STREAM MANAGEMENT & DEBUGGING

USAGE:
    gnostream stream <SUBCOMMAND> [OPTIONS]

SUBCOMMANDS:
    status              Show current stream status
    info                Show detailed stream information
    debug               Show debug information
    files               List stream files and sizes
    logs                Show recent log entries

EXAMPLES:
    gnostream stream status
    gnostream stream info
    gnostream stream debug
    gnostream stream files`)
}

// handleStatus shows current stream status
func (s *StreamCommand) handleStatus() error {
	fmt.Println("📺 STREAM STATUS")
	fmt.Println()

	// Check if stream is active
	streamDefaults := s.config.GetStreamDefaults()
	playlistPath := filepath.Join(streamDefaults.OutputDir, "stream.m3u8")

	if _, err := os.Stat(playlistPath); err != nil {
		fmt.Println("🔴 OFFLINE - No active stream")
		return nil
	}

	fmt.Println("🟢 ONLINE - Stream is active")
	fmt.Printf("📁 Output Directory: %s\n", streamDefaults.OutputDir)

	// Check for metadata
	metadataPath := filepath.Join(streamDefaults.OutputDir, "metadata.json")
	if _, err := os.Stat(metadataPath); err == nil {
		fmt.Println("📄 Metadata: Available")
	} else {
		fmt.Println("📄 Metadata: Not found")
	}

	// Show recording status
	if s.config.StreamInfo != nil {
		if s.config.StreamInfo.Record {
			fmt.Println("💾 Recording: ENABLED")
		} else {
			fmt.Println("💾 Recording: DISABLED")
		}
	}

	return nil
}

// handleInfo shows detailed stream information
func (s *StreamCommand) handleInfo() error {
	fmt.Println("📺 DETAILED STREAM INFORMATION")
	fmt.Println()

	if s.config.StreamInfo == nil {
		fmt.Println("❌ No stream configuration loaded")
		return nil
	}

	// Basic stream info
	fmt.Println("📋 STREAM DETAILS:")
	fmt.Printf("  Title:       %s\n", s.config.StreamInfo.Title)
	fmt.Printf("  Summary:     %s\n", s.config.StreamInfo.Summary)
	fmt.Printf("  Tags:        %v\n", s.config.StreamInfo.Tags)
	fmt.Printf("  Recording:   %t\n", s.config.StreamInfo.Record)
	fmt.Println()

	// HLS Configuration
	fmt.Println("🎬 HLS SETTINGS:")
	fmt.Printf("  Segment Time:   %d seconds\n", s.config.StreamInfo.HLS.SegmentTime)
	fmt.Printf("  Playlist Size:  %d segments\n", s.config.StreamInfo.HLS.PlaylistSize)
	fmt.Println()

	// Stream paths
	streamDefaults := s.config.GetStreamDefaults()
	fmt.Println("📁 PATHS:")
	fmt.Printf("  Output Dir:   %s\n", streamDefaults.OutputDir)
	fmt.Printf("  Archive Dir:  %s\n", streamDefaults.ArchiveDir)
	fmt.Println()

	// Server info
	fmt.Println("🌐 SERVER:")
	fmt.Printf("  Host:         %s\n", s.config.Server.Host)
	fmt.Printf("  Port:         %d\n", s.config.Server.Port)
	fmt.Printf("  External URL: %s\n", s.config.Server.ExternalURL)
	fmt.Println()

	// RTMP info
	rtmpDefaults := s.config.GetRTMPDefaults()
	fmt.Println("📡 RTMP:")
	fmt.Printf("  Host:         %s\n", rtmpDefaults.Host)
	fmt.Printf("  Port:         %d\n", rtmpDefaults.Port)
	fmt.Printf("  Enabled:      %t\n", rtmpDefaults.Enabled)

	return nil
}

// handleDebug shows debug information
func (s *StreamCommand) handleDebug() error {
	fmt.Println("🔍 DEBUG INFORMATION")
	fmt.Println()

	// Check file system state
	streamDefaults := s.config.GetStreamDefaults()
	
	fmt.Println("📁 FILE SYSTEM STATUS:")
	dirs := []string{streamDefaults.OutputDir, streamDefaults.ArchiveDir}
	
	for _, dir := range dirs {
		if stat, err := os.Stat(dir); err != nil {
			fmt.Printf("  ❌ %s: Not found\n", dir)
		} else if !stat.IsDir() {
			fmt.Printf("  ⚠️  %s: Not a directory\n", dir)
		} else {
			fmt.Printf("  ✅ %s: OK\n", dir)
		}
	}
	fmt.Println()

	// Check for active stream files
	fmt.Println("🎬 STREAM FILES:")
	streamFiles := []string{"stream.m3u8", "metadata.json"}
	
	for _, file := range streamFiles {
		path := filepath.Join(streamDefaults.OutputDir, file)
		if stat, err := os.Stat(path); err != nil {
			fmt.Printf("  ❌ %s: Not found\n", file)
		} else {
			fmt.Printf("  ✅ %s: %d bytes\n", file, stat.Size())
		}
	}
	fmt.Println()

	// Check metadata content
	metadataPath := filepath.Join(streamDefaults.OutputDir, "metadata.json")
	if data, err := os.ReadFile(metadataPath); err == nil {
		var metadata map[string]interface{}
		if json.Unmarshal(data, &metadata) == nil {
			fmt.Println("📄 METADATA CONTENT:")
			for key, value := range metadata {
				fmt.Printf("  %s: %v\n", key, value)
			}
		}
	}

	return nil
}

// handleFiles lists stream files and their sizes
func (s *StreamCommand) handleFiles() error {
	fmt.Println("📁 STREAM FILES")
	fmt.Println()

	streamDefaults := s.config.GetStreamDefaults()
	
	// List output directory
	fmt.Printf("📂 Output Directory (%s):\n", streamDefaults.OutputDir)
	if err := s.listDirectory(streamDefaults.OutputDir); err != nil {
		fmt.Printf("   ❌ Error reading directory: %v\n", err)
	}
	fmt.Println()

	// List archive directory
	fmt.Printf("📦 Archive Directory (%s):\n", streamDefaults.ArchiveDir)
	if err := s.listDirectory(streamDefaults.ArchiveDir); err != nil {
		fmt.Printf("   ❌ Error reading directory: %v\n", err)
	}

	return nil
}

// handleLogs shows recent log entries (placeholder - would need log file integration)
func (s *StreamCommand) handleLogs(args []string) error {
	fmt.Println("📋 RECENT LOG ENTRIES")
	fmt.Println()
	fmt.Println("⚠️  Note: Log integration not yet implemented")
	fmt.Println("💡 This feature requires implementing log file monitoring")
	
	// In a real implementation, you would:
	// - Check common log locations (/var/log/, ./logs/, etc.)
	// - Parse log files for gnostream entries
	// - Filter by timestamp/severity
	// - Format and display recent entries
	
	return nil
}

// listDirectory lists files in a directory with sizes
func (s *StreamCommand) listDirectory(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("   📭 Directory is empty")
		return nil
	}

	totalSize := int64(0)
	fileCount := 0

	for _, entry := range entries {
		path := filepath.Join(dirPath, entry.Name())
		
		if entry.IsDir() {
			fmt.Printf("   📁 %s/\n", entry.Name())
		} else {
			if stat, err := os.Stat(path); err == nil {
				size := stat.Size()
				totalSize += size
				fileCount++
				
				// Format file size
				sizeStr := formatFileSize(size)
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				
				var icon string
				switch ext {
				case ".m3u8":
					icon = "🎬"
				case ".ts":
					icon = "🎞️"
				case ".json":
					icon = "📄"
				default:
					icon = "📄"
				}
				
				fmt.Printf("   %s %s (%s)\n", icon, entry.Name(), sizeStr)
			}
		}
	}

	if fileCount > 0 {
		fmt.Printf("   📊 Total: %d files, %s\n", fileCount, formatFileSize(totalSize))
	}

	return nil
}

