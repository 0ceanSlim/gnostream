package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the main application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Stream   StreamConfig   `yaml:"stream"`
	HLS      HLSConfig      `yaml:"hls"`
	Nostr    NostrConfig    `yaml:"nostr"`
	Metadata MetadataConfig `yaml:"metadata"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// StreamConfig holds stream monitoring configuration
type StreamConfig struct {
	RTMPUrl       string        `yaml:"rtmp_url"`
	OutputDir     string        `yaml:"output_dir"`
	ArchiveDir    string        `yaml:"archive_dir"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

// HLSConfig holds HLS conversion settings
type HLSConfig struct {
	SegmentTime  int `yaml:"segment_time"`
	PlaylistSize int `yaml:"playlist_size"`
}

// NostrConfig holds Nostr integration settings
type NostrConfig struct {
	ConfigFile string `yaml:"config_file"`
}

// MetadataConfig holds stream metadata settings
type MetadataConfig struct {
	ConfigFile string `yaml:"config_file"`
}

// StreamMetadata represents the stream information
type StreamMetadata struct {
	Title        string   `yaml:"title" json:"title"`
	Summary      string   `yaml:"summary" json:"summary"`
	Image        string   `yaml:"image" json:"image"`
	Tags         []string `yaml:"tags" json:"tags"`
	Pubkey       string   `yaml:"pubkey" json:"pubkey"`
	Dtag         string   `yaml:"dtag" json:"dtag"`
	StreamURL    string   `yaml:"stream_url" json:"stream_url"`
	RecordingURL string   `yaml:"recording_url" json:"recording_url"`
	Starts       string   `yaml:"starts" json:"starts"`
	Ends         string   `yaml:"ends" json:"ends"`
	Status       string   `yaml:"status" json:"status"`
}

// NostrRelayConfig represents Nostr configuration
type NostrRelayConfig struct {
	PublicKey  string   `yaml:"public_key"`
	PrivateKey string   `yaml:"private_key"`
	Relays     []string `yaml:"relays"`
}

// Load reads and parses the main configuration file
func Load(path string) (*Config, error) {
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
	if cfg.Stream.CheckInterval == 0 {
		cfg.Stream.CheckInterval = 5 * time.Second
	}
	if cfg.Stream.OutputDir == "" {
		cfg.Stream.OutputDir = "web/live"
	}
	if cfg.Stream.ArchiveDir == "" {
		cfg.Stream.ArchiveDir = "web/live/past-streams"
	}
	if cfg.HLS.SegmentTime == 0 {
		cfg.HLS.SegmentTime = 10
	}
	if cfg.HLS.PlaylistSize == 0 {
		cfg.HLS.PlaylistSize = 10
	}

	return &cfg, nil
}

// LoadStreamMetadata loads stream metadata from YAML file
func LoadStreamMetadata(path string) (*StreamMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read stream metadata file %s: %w", path, err)
	}

	var metadata StreamMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse stream metadata: %w", err)
	}

	return &metadata, nil
}

// LoadNostrConfig loads Nostr configuration from YAML file
func LoadNostrConfig(path string) (*NostrRelayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read nostr config file %s: %w", path, err)
	}

	var nostrCfg NostrRelayConfig
	if err := yaml.Unmarshal(data, &nostrCfg); err != nil {
		return nil, fmt.Errorf("failed to parse nostr config: %w", err)
	}

	return &nostrCfg, nil
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
