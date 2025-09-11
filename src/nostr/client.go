package nostr

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"

	"gnostream/src/config"
)

// Event represents a Nostr event
type Event struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

// Client interface defines the Nostr client contract
type Client interface {
	BroadcastStartEvent(metadata *config.StreamMetadata)
	BroadcastStartEventWithResponse(metadata *config.StreamMetadata) (string, []string)
	BroadcastUpdateEvent(metadata *config.StreamMetadata)
	BroadcastUpdateEventWithResponse(metadata *config.StreamMetadata) (string, []string)
	BroadcastEndEvent(metadata *config.StreamMetadata)
	BroadcastEndEventWithResponse(metadata *config.StreamMetadata) (string, []string)
	BroadcastCancelEvent(dtag string)
	BroadcastDeletionEvent(eventID string, reason string)
	BroadcastDeletionEventWithResponse(eventID string, reason string) (string, []string)
	IsEnabled() bool
	GetConnectedRelays() []string
	Close() error
}

// GrainClient wraps Grain's Nostr client with gnostream-specific functionality
type GrainClient struct {
	client      *core.Client
	signer      *core.EventSigner
	userSession *session.UserSession
	config      *config.NostrRelayConfig
	publicKey   string
	isEnabled   bool
}

// NewClient creates a new Nostr client (uses Grain implementation)
func NewClient(cfg *config.NostrRelayConfig) (Client, error) {
	return NewGrainClient(cfg)
}

// NewGrainClient creates a new Grain-based Nostr client
func NewGrainClient(cfg *config.NostrRelayConfig) (*GrainClient, error) {
	// Check for placeholder values
	if cfg.PrivateKey == "your-nostr-private-key-nsec" || cfg.PrivateKey == "" {
		log.Println("⚠️ Nostr keys not configured, running in disabled mode")
		return &GrainClient{
			config:    cfg,
			isEnabled: false,
		}, nil
	}

	log.Println("🔑 Initializing Grain Nostr client...")

	// Create Grain client with configuration
	grainConfig := &core.Config{
		DefaultRelays:     cfg.Relays,
		ConnectionTimeout: 15 * time.Second,
		ReadTimeout:       45 * time.Second,
		WriteTimeout:      15 * time.Second,
		MaxConnections:    20,
		RetryAttempts:     3,
		RetryDelay:        2 * time.Second,
		UserAgent:         "gnostream/1.0",
	}

	client := core.NewClient(grainConfig)

	// Connect to relays
	if err := client.ConnectToRelaysWithRetry(cfg.Relays, 3); err != nil {
		log.Printf("⚠️ Some relays failed to connect: %v", err)
	}

	connectedCount := len(client.GetConnectedRelays())
	log.Printf("🌐 Connected to %d/%d Nostr relays", connectedCount, len(cfg.Relays))

	// Decode private key
	privateKeyHex, err := DecodeNsec(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nsec: %w", err)
	}

	// Create signer
	signer, err := core.NewEventSigner(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	// Derive public key
	publicKey, err := tools.DerivePublicKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	// Create user session
	userSession := &session.UserSession{
		PublicKey:       publicKey,
		LastActive:      time.Now(),
		Mode:            session.WriteMode,
		SigningMethod:   session.BrowserExtension, // We'll update this when we find the right constant
		ConnectedRelays: cfg.Relays,
	}

	// Update config with derived public key
	cfg.PublicKey = publicKey

	log.Printf("🔑 Grain client initialized successfully")
	log.Printf("🔑 Public key: %s", publicKey)

	return &GrainClient{
		client:      client,
		signer:      signer,
		userSession: userSession,
		config:      cfg,
		publicKey:   publicKey,
		isEnabled:   true,
	}, nil
}

// ensureConnections ensures all relays are connected before publishing
func (gc *GrainClient) ensureConnections() {
	if err := gc.client.ConnectToRelaysWithRetry(gc.config.Relays, 3); err != nil {
		log.Printf("⚠️ Some relays failed to reconnect: %v", err)
	}
}

// Helper method to build streaming event
func (gc *GrainClient) buildStreamingEvent(metadata *config.StreamMetadata, status string) *nostr.Event {
	eventBuilder := core.NewEventBuilder(30311).
		Content("").
		DTag(metadata.Dtag).
		Tag("title", metadata.Title).
		Tag("summary", metadata.Summary).
		Tag("streaming", metadata.StreamURL).
		Tag("recording", metadata.RecordingURL).
		Tag("starts", metadata.Starts).
		Tag("status", status)

	if metadata.Image != "" {
		eventBuilder = eventBuilder.Tag("image", metadata.Image)
	}

	if metadata.Ends != "" && status != "live" {
		eventBuilder = eventBuilder.Tag("ends", metadata.Ends)
	}

	// Add hashtags
	for _, tag := range metadata.Tags {
		eventBuilder = eventBuilder.TTag(tag)
	}

	return eventBuilder.Build()
}

// GetUserSession returns the current user session
func (gc *GrainClient) GetUserSession() *session.UserSession {
	return gc.userSession
}

// GetClient returns the underlying Grain client
func (gc *GrainClient) GetClient() *core.Client {
	return gc.client
}

// IsEnabled returns whether the client is enabled
func (gc *GrainClient) IsEnabled() bool {
	return gc.isEnabled
}

// GetConnectedRelays returns list of connected relay URLs
func (gc *GrainClient) GetConnectedRelays() []string {
	if !gc.isEnabled {
		return []string{}
	}
	return gc.client.GetConnectedRelays()
}

// BroadcastStartEvent broadcasts a stream start event using Grain
func (gc *GrainClient) BroadcastStartEvent(metadata *config.StreamMetadata) {
	if !gc.isEnabled {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream start event via Grain...")

	event := gc.buildStreamingEvent(metadata, "live")

	if err := gc.signer.SignEvent(event); err != nil {
		log.Printf("❌ Failed to sign start event: %v", err)
		return
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		log.Printf("❌ Failed to publish start event: %v", err)
		return
	}

	summary := core.SummarizeBroadcast(results)
	log.Printf("📡 Start event published to %d/%d relays (%.1f%% success)",
		summary.Successful, summary.TotalRelays, summary.SuccessRate)
}

// BroadcastStartEventWithResponse broadcasts a start event and returns event info
func (gc *GrainClient) BroadcastStartEventWithResponse(metadata *config.StreamMetadata) (string, []string) {
	if !gc.isEnabled {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return "", []string{}
	}

	event := gc.buildStreamingEvent(metadata, "live")

	if err := gc.signer.SignEvent(event); err != nil {
		log.Printf("❌ Failed to sign start event: %v", err)
		return "", []string{}
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		log.Printf("❌ Failed to publish start event: %v", err)
		return "", []string{}
	}

	eventJSON, _ := json.Marshal(event)
	var successfulRelays []string
	for _, result := range results {
		if result.Success {
			successfulRelays = append(successfulRelays, result.RelayURL)
		}
	}

	summary := core.SummarizeBroadcast(results)
	log.Printf("📡 Start event published to %d/%d relays", summary.Successful, summary.TotalRelays)

	return string(eventJSON), successfulRelays
}

// BroadcastUpdateEvent broadcasts a stream metadata update
func (gc *GrainClient) BroadcastUpdateEvent(metadata *config.StreamMetadata) {
	if !gc.isEnabled {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream update event via Grain...")

	event := gc.buildStreamingEvent(metadata, metadata.Status)

	if err := gc.signer.SignEvent(event); err != nil {
		log.Printf("❌ Failed to sign update event: %v", err)
		return
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		log.Printf("❌ Failed to publish update event: %v", err)
		return
	}

	summary := core.SummarizeBroadcast(results)
	log.Printf("📡 Update event published to %d/%d relays (%.1f%% success)",
		summary.Successful, summary.TotalRelays, summary.SuccessRate)
}

// BroadcastUpdateEventWithResponse broadcasts an update event and returns event info
func (gc *GrainClient) BroadcastUpdateEventWithResponse(metadata *config.StreamMetadata) (string, []string) {
	if !gc.isEnabled {
		return "", []string{}
	}

	event := gc.buildStreamingEvent(metadata, metadata.Status)

	if err := gc.signer.SignEvent(event); err != nil {
		return "", []string{}
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		return "", []string{}
	}

	eventJSON, _ := json.Marshal(event)
	var successfulRelays []string
	for _, result := range results {
		if result.Success {
			successfulRelays = append(successfulRelays, result.RelayURL)
		}
	}

	return string(eventJSON), successfulRelays
}

// BroadcastEndEvent broadcasts a stream end event
func (gc *GrainClient) BroadcastEndEvent(metadata *config.StreamMetadata) {
	if !gc.isEnabled {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream end event via Grain...")

	event := gc.buildStreamingEvent(metadata, "ended")

	if err := gc.signer.SignEvent(event); err != nil {
		log.Printf("❌ Failed to sign end event: %v", err)
		return
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		log.Printf("❌ Failed to publish end event: %v", err)
		return
	}

	summary := core.SummarizeBroadcast(results)
	log.Printf("📡 End event published to %d/%d relays (%.1f%% success)",
		summary.Successful, summary.TotalRelays, summary.SuccessRate)
}

// BroadcastEndEventWithResponse broadcasts an end event and returns event info
func (gc *GrainClient) BroadcastEndEventWithResponse(metadata *config.StreamMetadata) (string, []string) {
	if !gc.isEnabled {
		return "", []string{}
	}

	event := gc.buildStreamingEvent(metadata, "ended")

	if err := gc.signer.SignEvent(event); err != nil {
		return "", []string{}
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		return "", []string{}
	}

	eventJSON, _ := json.Marshal(event)
	var successfulRelays []string
	for _, result := range results {
		if result.Success {
			successfulRelays = append(successfulRelays, result.RelayURL)
		}
	}

	return string(eventJSON), successfulRelays
}

// BroadcastCancelEvent broadcasts a cancellation event
func (gc *GrainClient) BroadcastCancelEvent(dtag string) {
	if !gc.isEnabled {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream cancellation event via Grain...")

	event := core.NewEventBuilder(30311).
		Content("").
		DTag(dtag).
		Tag("status", "ended").
		Tag("summary", "Stream was incorrectly marked as live").
		Build()

	if err := gc.signer.SignEvent(event); err != nil {
		log.Printf("❌ Failed to sign cancel event: %v", err)
		return
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		log.Printf("❌ Failed to publish cancel event: %v", err)
		return
	}

	summary := core.SummarizeBroadcast(results)
	log.Printf("📡 Cancel event published to %d/%d relays", summary.Successful, summary.TotalRelays)
}

// BroadcastDeletionEvent broadcasts a NIP-09 deletion request event
func (gc *GrainClient) BroadcastDeletionEvent(eventID string, reason string) {
	if !gc.isEnabled {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Printf("🗑️ Broadcasting NIP-09 deletion request for event: %s", eventID)

	content := reason
	if content == "" {
		content = "Stream ended without recording"
	}

	event := core.NewEventBuilder(5). // kind 5 = deletion request
					Content(content).
					ETag(eventID, "", "").
					Tag("k", "30311"). // kind 30311 (live streaming event)
					Build()

	if err := gc.signer.SignEvent(event); err != nil {
		log.Printf("❌ Failed to sign deletion event: %v", err)
		return
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		log.Printf("❌ Failed to publish deletion event: %v", err)
		return
	}

	summary := core.SummarizeBroadcast(results)
	log.Printf("🗑️ Deletion request sent to %d/%d relays", summary.Successful, summary.TotalRelays)
}

// BroadcastDeletionEventWithResponse broadcasts a deletion request and returns event info
func (gc *GrainClient) BroadcastDeletionEventWithResponse(eventID string, reason string) (string, []string) {
	if !gc.isEnabled {
		return "", []string{}
	}

	content := reason
	if content == "" {
		content = "Stream ended without recording"
	}

	event := core.NewEventBuilder(5).
		Content(content).
		ETag(eventID, "", "").
		Tag("k", "30311").
		Build()

	if err := gc.signer.SignEvent(event); err != nil {
		return "", []string{}
	}

	gc.ensureConnections()

	results, err := gc.client.PublishEvent(event, nil)
	if err != nil {
		return "", []string{}
	}

	eventJSON, _ := json.Marshal(event)
	var successfulRelays []string
	for _, result := range results {
		if result.Success {
			successfulRelays = append(successfulRelays, result.RelayURL)
		}
	}

	return string(eventJSON), successfulRelays
}

// Subscribe creates a subscription to query events
func (gc *GrainClient) Subscribe(filters []nostr.Filter, relayHints []string) (*core.Subscription, error) {
	if !gc.isEnabled {
		return nil, fmt.Errorf("nostr client not enabled")
	}

	return gc.client.Subscribe(filters, relayHints)
}

// GetUserProfile fetches a user's profile metadata
func (gc *GrainClient) GetUserProfile(pubkey string, relayHints []string) (*nostr.Event, error) {
	if !gc.isEnabled {
		return nil, fmt.Errorf("nostr client not enabled")
	}

	return gc.client.GetUserProfile(pubkey, relayHints)
}

// Close closes all relay connections
func (gc *GrainClient) Close() error {
	if gc.client != nil {
		return gc.client.Close()
	}
	return nil
}

// ExtractEventID extracts the event ID from a JSON string
func ExtractEventID(eventJSON string) (string, error) {
	var event Event
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		return "", fmt.Errorf("failed to parse event JSON: %w", err)
	}
	return event.ID, nil
}

// DecodeNsec decodes an nsec key to hex format
func DecodeNsec(nsec string) (string, error) {
	if !strings.HasPrefix(nsec, "nsec1") {
		return "", fmt.Errorf("invalid nsec format: must start with 'nsec1'")
	}

	// Remove the nsec1 prefix and decode bech32
	data := nsec[5:] // Remove "nsec1" prefix
	
	// Simple base32 decode for nsec (this is a simplified implementation)
	// In production, you should use a proper bech32 decoder
	decoded := make([]byte, 32)
	if err := decodeBech32(data, decoded); err != nil {
		return "", fmt.Errorf("failed to decode bech32: %w", err)
	}
	
	return hex.EncodeToString(decoded), nil
}

// Simple bech32 decoder (minimal implementation for nsec)
func decodeBech32(data string, output []byte) error {
	// This is a very basic implementation - in production use a proper bech32 library
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	
	values := make([]int, len(data))
	for i, c := range data {
		pos := strings.IndexRune(charset, c)
		if pos == -1 {
			return fmt.Errorf("invalid character: %c", c)
		}
		values[i] = pos
	}
	
	// Convert from 5-bit to 8-bit groups
	var acc, bits int
	for i := 0; i < len(values)-6; i++ { // -6 for checksum
		acc = (acc << 5) | values[i]
		bits += 5
		if bits >= 8 {
			bits -= 8
			if len(output) > 0 {
				output[0] = byte(acc >> bits)
				output = output[1:]
			}
		}
	}
	
	return nil
}