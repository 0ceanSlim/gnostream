package commands

import (
	"fmt"
	"strconv"
	"strings"

	"gnostream/src/config"
)

// ConfigCommand handles configuration management
type ConfigCommand struct {
	config *config.Config
}

// NewConfigCommand creates a new config command
func NewConfigCommand(cfg *config.Config) *ConfigCommand {
	return &ConfigCommand{config: cfg}
}

// Execute runs the config command
func (c *ConfigCommand) Execute(args []string) error {
	if len(args) == 0 {
		c.printUsage()
		return nil
	}

	subcommand := args[0]

	switch subcommand {
	case "get":
		return c.handleGet(args[1:])
	case "set":
		return c.handleSet(args[1:])
	case "list":
		return c.handleList()
	case "show":
		return c.handleShow()
	case "reload":
		return c.handleReload()
	case "--help", "help":
		c.printUsage()
		return nil
	default:
		fmt.Printf("Unknown config subcommand: %s\n\n", subcommand)
		c.printUsage()
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// printUsage prints config command usage
func (c *ConfigCommand) printUsage() {
	fmt.Println(`CONFIG MANAGEMENT

USAGE:
    gnostream config <SUBCOMMAND> [OPTIONS]

SUBCOMMANDS:
    get <key>           Get configuration value
    set <key> <value>   Set configuration value  
    list               List all configuration keys
    show               Show current configuration
    reload             Reload configuration from file

CONFIGURATION KEYS:
    recording          Enable/disable recording (true/false)
    segment_time       HLS segment duration in seconds
    playlist_size      HLS playlist size (number of segments)
    title              Stream title
    summary            Stream summary/description
    image              Stream thumbnail image URL
    tags               Stream tags (comma-separated)
    server.port        Server port
    server.host        Server host
    rtmp.port          RTMP server port

EXAMPLES:
    gnostream config get recording
    gnostream config set recording true
    gnostream config set title "My Stream"
    gnostream config set tags "gaming,live,test"
    gnostream config show
    gnostream config reload`)
}

// handleGet gets a configuration value
func (c *ConfigCommand) handleGet(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing configuration key")
	}

	key := args[0]
	value, err := c.getConfigValue(key)
	if err != nil {
		return err
	}

	fmt.Printf("%s = %v\n", key, value)
	return nil
}

// handleSet sets a configuration value
func (c *ConfigCommand) handleSet(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("missing configuration key or value")
	}

	key := args[0]
	value := strings.Join(args[1:], " ")

	if err := c.setConfigValue(key, value); err != nil {
		return err
	}

	fmt.Printf("✅ Set %s = %s\n", key, value)
	return nil
}

// handleList lists all configuration keys
func (c *ConfigCommand) handleList() error {
	fmt.Println("CONFIGURATION KEYS:")
	keys := []string{
		"recording", "segment_time", "playlist_size",
		"title", "summary", "image", "tags",
		"server.port", "server.host", "rtmp.port",
	}

	for _, key := range keys {
		value, _ := c.getConfigValue(key)
		fmt.Printf("  %-15s %v\n", key, value)
	}

	return nil
}

// handleShow shows the current configuration
func (c *ConfigCommand) handleShow() error {
	fmt.Println("CURRENT CONFIGURATION:")
	fmt.Println()

	fmt.Println("📺 STREAM INFO:")
	if c.config.StreamInfo != nil {
		fmt.Printf("  Title:       %s\n", c.config.StreamInfo.Title)
		fmt.Printf("  Summary:     %s\n", c.config.StreamInfo.Summary)
		fmt.Printf("  Image:       %s\n", c.config.StreamInfo.Image)
		fmt.Printf("  Tags:        %v\n", c.config.StreamInfo.Tags)
		fmt.Printf("  Recording:   %t\n", c.config.StreamInfo.Record)
		fmt.Println()
		fmt.Printf("  HLS Settings:\n")
		fmt.Printf("    Segment Time:   %d seconds\n", c.config.StreamInfo.HLS.SegmentTime)
		fmt.Printf("    Playlist Size:  %d segments\n", c.config.StreamInfo.HLS.PlaylistSize)
	}

	fmt.Println()
	fmt.Println("🌐 SERVER:")
	fmt.Printf("  Host:        %s\n", c.config.Server.Host)
	fmt.Printf("  Port:        %d\n", c.config.Server.Port)
	fmt.Printf("  External URL: %s\n", c.config.Server.ExternalURL)

	fmt.Println()
	fmt.Println("📡 RTMP:")
	rtmpDefaults := c.config.GetRTMPDefaults()
	fmt.Printf("  Host:        %s\n", rtmpDefaults.Host)
	fmt.Printf("  Port:        %d\n", rtmpDefaults.Port)
	fmt.Printf("  Enabled:     %t\n", rtmpDefaults.Enabled)

	fmt.Println()
	fmt.Println("🔗 NOSTR:")
	fmt.Printf("  Relays:      %v\n", c.config.Nostr.Relays)
	fmt.Printf("  Public Key:  %s\n", c.config.Nostr.PublicKey)
	fmt.Printf("  Delete Non-Recorded: %t\n", c.config.Nostr.DeleteNonRecorded)

	return nil
}

// handleReload reloads configuration from file
func (c *ConfigCommand) handleReload() error {
	_, changed, err := c.config.CheckAndReloadStreamInfo()
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	if changed {
		fmt.Println("✅ Configuration reloaded successfully")
	} else {
		fmt.Println("ℹ️  No configuration changes detected")
	}

	return nil
}

// getConfigValue gets a configuration value by key
func (c *ConfigCommand) getConfigValue(key string) (interface{}, error) {
	if c.config.StreamInfo == nil {
		return nil, fmt.Errorf("stream info not loaded")
	}

	switch key {
	case "recording":
		return c.config.StreamInfo.Record, nil
	case "segment_time":
		return c.config.StreamInfo.HLS.SegmentTime, nil
	case "playlist_size":
		return c.config.StreamInfo.HLS.PlaylistSize, nil
	case "title":
		return c.config.StreamInfo.Title, nil
	case "summary":
		return c.config.StreamInfo.Summary, nil
	case "image":
		return c.config.StreamInfo.Image, nil
	case "tags":
		return strings.Join(c.config.StreamInfo.Tags, ","), nil
	case "server.port":
		return c.config.Server.Port, nil
	case "server.host":
		return c.config.Server.Host, nil
	case "rtmp.port":
		return c.config.GetRTMPDefaults().Port, nil
	default:
		return nil, fmt.Errorf("unknown configuration key: %s", key)
	}
}

// setConfigValue sets a configuration value by key
func (c *ConfigCommand) setConfigValue(key, value string) error {
	if c.config.StreamInfo == nil {
		return fmt.Errorf("stream info not loaded")
	}

	switch key {
	case "recording":
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
		c.config.StreamInfo.Record = boolVal
	case "segment_time":
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", value)
		}
		c.config.StreamInfo.HLS.SegmentTime = intVal
	case "playlist_size":
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", value)
		}
		c.config.StreamInfo.HLS.PlaylistSize = intVal
	case "title":
		c.config.StreamInfo.Title = value
	case "summary":
		c.config.StreamInfo.Summary = value
	case "image":
		c.config.StreamInfo.Image = value
	case "tags":
		c.config.StreamInfo.Tags = strings.Split(value, ",")
		// Trim whitespace from each tag
		for i, tag := range c.config.StreamInfo.Tags {
			c.config.StreamInfo.Tags[i] = strings.TrimSpace(tag)
		}
	default:
		return fmt.Errorf("configuration key '%s' is not settable via CLI", key)
	}

	// Save the updated stream info back to file
	return config.SaveStreamInfo(c.config.StreamInfoPath, c.config.StreamInfo)
}