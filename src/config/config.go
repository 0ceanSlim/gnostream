package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the main application configuration
type Config struct {
	Server               ServerConfig     `yaml:"server"`
	RTMP                 RTMPConfig       `yaml:"rtmp"`
	HLS                  HLSConfig        `yaml:"hls"`
	Nostr                NostrRelayConfig `yaml:"nostr"`
	StreamInfoPath    string      `yaml:"stream_info_path"`
	StreamInfo        *StreamInfo `yaml:"-"`    // Not stored in main config, loaded separately
	streamInfoModTime time.Time   `yaml:"-"`    // Track file modification time
	streamInfoMutex   sync.RWMutex `yaml:"-"`    // Protect concurrent access
}

// GetStreamDefaults returns hardcoded stream configuration defaults
func (cfg *Config) GetStreamDefaults() *StreamDefaults {
	return &StreamDefaults{
		RTMPUrl:       "rtmp://localhost:1935/live/stream",
		OutputDir:     "www/live",
		ArchiveDir:    "www/live/archive", 
		CheckInterval: 5 * time.Second,
	}
}

// GetRTMPDefaults returns RTMP configuration with defaults
func (cfg *Config) GetRTMPDefaults() *RTMPDefaults {
	port := cfg.RTMP.Port
	if port == 0 {
		port = 1935
	}
	
	host := cfg.RTMP.Host
	if host == "" {
		host = "0.0.0.0"
	}
	
	return &RTMPDefaults{
		Port:    port,
		Host:    host,
		Enabled: true,
	}
}

// StreamDefaults holds hardcoded stream configuration
type StreamDefaults struct {
	RTMPUrl       string
	OutputDir     string
	ArchiveDir    string
	CheckInterval time.Duration
}

// RTMPConfig holds RTMP configuration from YAML
type RTMPConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// RTMPDefaults holds RTMP configuration with defaults applied
type RTMPDefaults struct {
	Port    int
	Host    string
	Enabled bool
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port        int    `yaml:"port"`
	Host        string `yaml:"host"`
	ExternalURL string `yaml:"external_url"`
}

// HLSConfig holds HLS conversion settings
type HLSConfig struct {
	SegmentTime  int `yaml:"segment_time"`
	PlaylistSize int `yaml:"playlist_size"`
}


// StreamInfo represents the user-configurable stream information
type StreamInfo struct {
	Title  string   `yaml:"title"`
	Summary string   `yaml:"summary"`
	Image  string   `yaml:"image"`
	Tags   []string `yaml:"tags"`
	Record bool     `yaml:"record"` // Whether to record/archive the stream
}

// StreamMetadata represents the complete stream information (user info + runtime data)
type StreamMetadata struct {
	Title            string   `yaml:"title" json:"title"`
	Summary          string   `yaml:"summary" json:"summary"`
	Image            string   `yaml:"image" json:"image"`
	Tags             []string `yaml:"tags" json:"tags"`
	Pubkey           string   `yaml:"pubkey" json:"pubkey"`
	Dtag             string   `yaml:"dtag" json:"dtag"`
	StreamURL        string   `yaml:"stream_url" json:"stream_url"`
	RecordingURL     string   `yaml:"recording_url" json:"recording_url"`
	Starts           string   `yaml:"starts" json:"starts"`
	Ends             string   `yaml:"ends" json:"ends"`
	Status           string   `yaml:"status" json:"status"`
	LastNostrEvent   string   `yaml:"last_nostr_event" json:"last_nostr_event"`       // Raw JSON of last published event
	SuccessfulRelays []string `yaml:"successful_relays" json:"successful_relays"`     // Relays that accepted the event
}

// NostrRelayConfig represents Nostr configuration
type NostrRelayConfig struct {
	PublicKey  string   `yaml:"public_key"`
	PrivateKey string   `yaml:"private_key"`
	Relays     []string `yaml:"relays"`
}

// Load reads and parses the main configuration file
func Load(path string) (*Config, error) {
	// Check if config file exists, if not try to copy from example
	if _, err := os.Stat(path); os.IsNotExist(err) {
		examplePath := path + ".example"
		if _, err := os.Stat(examplePath); err == nil {
			// Copy example config to regular config
			exampleData, err := os.ReadFile(examplePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read example config %s: %w", examplePath, err)
			}
			
			if err := os.WriteFile(path, exampleData, 0644); err != nil {
				return nil, fmt.Errorf("failed to create config from example: %w", err)
			}
			
			fmt.Printf("📋 Created %s from %s - please edit with your settings\n", path, examplePath)
		}
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.HLS.SegmentTime == 0 {
		cfg.HLS.SegmentTime = 10
	}
	if cfg.HLS.PlaylistSize == 0 {
		cfg.HLS.PlaylistSize = 10
	}
	if cfg.StreamInfoPath == "" {
		cfg.StreamInfoPath = "stream-info.yml"
	}

	// Load stream info from separate file
	streamInfo, modTime, err := LoadStreamInfoWithModTime(cfg.StreamInfoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load stream info: %w", err)
	}
	cfg.StreamInfo = streamInfo
	cfg.streamInfoModTime = modTime

	// Validate configuration and warn about issues
	cfg.validateAndWarn()

	return &cfg, nil
}

// validateAndWarn checks config values and warns about potential issues
func (cfg *Config) validateAndWarn() {
	warnings := []string{}

	// Check Nostr keys
	if cfg.Nostr.PublicKey == "your-nostr-public-key-hex" || cfg.Nostr.PublicKey == "" {
		warnings = append(warnings, "Nostr public key is not configured - Nostr broadcasting will not work")
	}
	
	if cfg.Nostr.PrivateKey == "your-nostr-private-key-hex" || cfg.Nostr.PrivateKey == "" {
		warnings = append(warnings, "Nostr private key is not configured - Nostr broadcasting will not work")
	}

	// Check if keys are valid hex (basic check)
	if cfg.Nostr.PublicKey != "your-nostr-public-key-hex" && cfg.Nostr.PublicKey != "" {
		if len(cfg.Nostr.PublicKey) != 64 {
			warnings = append(warnings, "Nostr public key should be 64 hex characters")
		}
	}
	
	if cfg.Nostr.PrivateKey != "your-nostr-private-key-hex" && cfg.Nostr.PrivateKey != "" {
		if len(cfg.Nostr.PrivateKey) != 64 {
			warnings = append(warnings, "Nostr private key should be 64 hex characters")
		}
	}

	// Check if relays are configured
	if len(cfg.Nostr.Relays) == 0 {
		warnings = append(warnings, "No Nostr relays configured - events will not be published")
	}

	// Print warnings
	if len(warnings) > 0 {
		fmt.Println("⚠️  Configuration Warnings:")
		for _, warning := range warnings {
			fmt.Printf("   • %s\n", warning)
		}
		fmt.Println("   💡 Edit config.yml to fix these issues")
		fmt.Println()
	}
}

// LoadStreamInfo loads stream info from YAML file, creating a default if it doesn't exist
func LoadStreamInfo(path string) (*StreamInfo, error) {
	info, _, err := LoadStreamInfoWithModTime(path)
	return info, err
}

// LoadStreamInfoWithModTime loads stream info and returns modification time
func LoadStreamInfoWithModTime(path string) (*StreamInfo, time.Time, error) {
	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to read stream info file %s: %w", path, err)
	}

	var info StreamInfo
	if err := yaml.Unmarshal(data, &info); err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse stream info: %w", err)
	}

	return &info, fileInfo.ModTime(), nil
}

// SaveStreamInfo saves stream info to YAML file
func SaveStreamInfo(path string, info *StreamInfo) error {
	data, err := yaml.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal stream info: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write stream info file: %w", err)
	}

	return nil
}

// GetStreamMetadata converts StreamInfo to StreamMetadata for runtime use
func (cfg *Config) GetStreamMetadata() *StreamMetadata {
	cfg.streamInfoMutex.RLock()
	defer cfg.streamInfoMutex.RUnlock()
	
	if cfg.StreamInfo == nil {
		return &StreamMetadata{
			Title:   "Stream Offline",
			Summary: "The stream is currently offline",
			Status:  "offline",
		}
	}

	return &StreamMetadata{
		Title:   cfg.StreamInfo.Title,
		Summary: cfg.StreamInfo.Summary,
		Image:   cfg.StreamInfo.Image,
		Tags:    cfg.StreamInfo.Tags,
	}
}

// CheckAndReloadStreamInfo checks if stream info file has been modified and reloads if needed
func (cfg *Config) CheckAndReloadStreamInfo() (*StreamInfo, bool, error) {
	fileInfo, err := os.Stat(cfg.StreamInfoPath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to stat stream info file: %w", err)
	}

	cfg.streamInfoMutex.RLock()
	lastModTime := cfg.streamInfoModTime
	cfg.streamInfoMutex.RUnlock()

	// Check if file has been modified
	if fileInfo.ModTime().After(lastModTime) {
		// File was modified, reload it
		newInfo, newModTime, err := LoadStreamInfoWithModTime(cfg.StreamInfoPath)
		if err != nil {
			return nil, false, fmt.Errorf("failed to reload stream info: %w", err)
		}

		cfg.streamInfoMutex.Lock()
		cfg.StreamInfo = newInfo
		cfg.streamInfoModTime = newModTime
		cfg.streamInfoMutex.Unlock()

		fmt.Printf("📝 Stream info reloaded from: %s\n", cfg.StreamInfoPath)
		return newInfo, true, nil
	}

	return cfg.StreamInfo, false, nil
}

// SaveStreamMetadata saves stream metadata to JSON file
func SaveStreamMetadata(path string, metadata *StreamMetadata) error {
	// Convert to map for JSON serialization with lowercase keys
	data := map[string]interface{}{
		"title":         metadata.Title,
		"summary":       metadata.Summary,
		"image":         metadata.Image,
		"tags":          metadata.Tags,
		"pubkey":        metadata.Pubkey,
		"dtag":          metadata.Dtag,
		"stream_url":    metadata.StreamURL,
		"recording_url": metadata.RecordingURL,
		"starts":        metadata.Starts,
		"ends":          metadata.Ends,
		"status":        metadata.Status,
	}

	return SaveJSON(path, data)
}

// SaveJSON saves data to JSON file with pretty formatting
func SaveJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}
