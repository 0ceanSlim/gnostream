package nostr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcutil/bech32"
	"golang.org/x/net/websocket"

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

// DecodeNsec decodes a Bech32 encoded nsec to its corresponding hex private key
func DecodeNsec(nsec string) (string, error) {
	log.Printf("🔑 Decoding nsec private key...")

	hrp, data, err := bech32.Decode(nsec)
	if err != nil {
		return "", fmt.Errorf("failed to decode bech32 nsec: %w", err)
	}

	if hrp != "nsec" {
		return "", errors.New("invalid hrp, expected 'nsec'")
	}

	decodedData, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", fmt.Errorf("failed to convert bits: %w", err)
	}

	if len(decodedData) != 32 {
		return "", fmt.Errorf("invalid private key length: got %d, expected 32", len(decodedData))
	}

	privateKey := strings.ToLower(hex.EncodeToString(decodedData))
	log.Printf("🔑 Successfully decoded nsec to hex private key")

	return privateKey, nil
}

// DerivePublicKey derives a public key from a private key
func DerivePublicKey(privateKeyHex string) (string, error) {
	if len(privateKeyHex) != 64 {
		return "", fmt.Errorf("private key must be 64 hex characters")
	}

	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid hex private key: %w", err)
	}

	_, publicKey := btcec.PrivKeyFromBytes(privateKeyBytes)
	publicKeyBytes := schnorr.SerializePubKey(publicKey)
	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	return publicKeyHex, nil
}

// Client handles Nostr relay communication
type Client struct {
	privateKey *btcec.PrivateKey
	publicKey  string
	relays     []string
}

// NewClient creates a new Nostr client
func NewClient(cfg *config.NostrRelayConfig) (*Client, error) {
	// Check for placeholder values
	if cfg.PrivateKey == "your-nostr-private-key-nsec" || cfg.PrivateKey == "" {
		return &Client{
			privateKey: nil,
			publicKey:  "",
			relays:     cfg.Relays,
		}, nil // Return a disabled client
	}

	// Decode nsec private key to hex
	privateKeyHex, err := DecodeNsec(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nsec private key: %w", err)
	}

	// Parse hex private key
	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex private key: %w", err)
	}

	privateKey, _ := btcec.PrivKeyFromBytes(keyBytes)

	// Derive public key from private key
	publicKeyHex, err := DerivePublicKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	// Store derived public key in config for reference
	cfg.PublicKey = publicKeyHex

	log.Printf("🔑 Nostr keys initialized successfully")
	log.Printf("🔑 Public key: %s", publicKeyHex)

	return &Client{
		privateKey: privateKey,
		publicKey:  publicKeyHex,
		relays:     cfg.Relays,
	}, nil
}

// BroadcastStartEvent broadcasts a stream start event
func (c *Client) BroadcastStartEvent(metadata *config.StreamMetadata) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream start event to Nostr relays")

	tags := [][]string{
		{"d", metadata.Dtag},
		{"title", metadata.Title},
		{"summary", metadata.Summary},
		{"streaming", metadata.StreamURL},
		{"recording", metadata.RecordingURL},
		{"starts", metadata.Starts},
		{"status", "live"},
	}

	if metadata.Image != "" {
		tags = append(tags, []string{"image", metadata.Image})
	}

	// Add tags
	for _, tag := range metadata.Tags {
		tags = append(tags, []string{"t", tag})
	}

	event, err := c.createEvent(30311, "", tags)
	if err != nil {
		log.Printf("Failed to create start event: %v", err)
		return
	}

	c.publishEvent(event)
}

// BroadcastStartEventWithResponse broadcasts a stream start event and returns event info
func (c *Client) BroadcastStartEventWithResponse(metadata *config.StreamMetadata) (string, []string) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return "", []string{}
	}

	log.Println("📡 Broadcasting stream start event to Nostr relays")

	tags := [][]string{
		{"d", metadata.Dtag},
		{"title", metadata.Title},
		{"summary", metadata.Summary},
		{"streaming", metadata.StreamURL},
		{"recording", metadata.RecordingURL},
		{"starts", metadata.Starts},
		{"status", "live"},
	}

	if metadata.Image != "" {
		tags = append(tags, []string{"image", metadata.Image})
	}

	// Add tags
	for _, tag := range metadata.Tags {
		tags = append(tags, []string{"t", tag})
	}

	event, err := c.createEvent(30311, "", tags)
	if err != nil {
		log.Printf("Failed to create start event: %v", err)
		return "", []string{}
	}

	// Convert event to JSON
	eventJSON, _ := json.Marshal(event)

	successfulRelays := c.publishEvent(event)
	return string(eventJSON), successfulRelays
}

// BroadcastUpdateEvent broadcasts a stream metadata update
func (c *Client) BroadcastUpdateEvent(metadata *config.StreamMetadata) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream update event to Nostr relays")

	tags := [][]string{
		{"d", metadata.Dtag},
		{"title", metadata.Title},
		{"summary", metadata.Summary},
		{"streaming", metadata.StreamURL},
		{"recording", metadata.RecordingURL},
		{"starts", metadata.Starts},
		{"status", metadata.Status},
	}

	if metadata.Image != "" {
		tags = append(tags, []string{"image", metadata.Image})
	}

	if metadata.Ends != "" {
		tags = append(tags, []string{"ends", metadata.Ends})
	}

	// Add tags
	for _, tag := range metadata.Tags {
		tags = append(tags, []string{"t", tag})
	}

	event, err := c.createEvent(30311, "", tags)
	if err != nil {
		log.Printf("Failed to create update event: %v", err)
		return
	}

	c.publishEvent(event)
}

// BroadcastUpdateEventWithResponse broadcasts a stream metadata update and returns event info
func (c *Client) BroadcastUpdateEventWithResponse(metadata *config.StreamMetadata) (string, []string) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return "", []string{}
	}

	log.Println("📡 Broadcasting stream update event to Nostr relays")

	tags := [][]string{
		{"d", metadata.Dtag},
		{"title", metadata.Title},
		{"summary", metadata.Summary},
		{"streaming", metadata.StreamURL},
		{"recording", metadata.RecordingURL},
		{"starts", metadata.Starts},
		{"status", metadata.Status},
	}

	if metadata.Image != "" {
		tags = append(tags, []string{"image", metadata.Image})
	}

	if metadata.Ends != "" {
		tags = append(tags, []string{"ends", metadata.Ends})
	}

	// Add tags
	for _, tag := range metadata.Tags {
		tags = append(tags, []string{"t", tag})
	}

	event, err := c.createEvent(30311, "", tags)
	if err != nil {
		log.Printf("Failed to create update event: %v", err)
		return "", []string{}
	}

	// Convert event to JSON
	eventJSON, _ := json.Marshal(event)

	successfulRelays := c.publishEvent(event)
	return string(eventJSON), successfulRelays
}

// BroadcastEndEvent broadcasts a stream end event
func (c *Client) BroadcastEndEvent(metadata *config.StreamMetadata) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream end event to Nostr relays")

	tags := [][]string{
		{"d", metadata.Dtag},
		{"title", metadata.Title},
		{"summary", metadata.Summary},
		{"streaming", metadata.StreamURL},
		{"recording", metadata.RecordingURL},
		{"starts", metadata.Starts},
		{"ends", metadata.Ends},
		{"status", "ended"},
	}

	if metadata.Image != "" {
		tags = append(tags, []string{"image", metadata.Image})
	}

	// Add tags
	for _, tag := range metadata.Tags {
		tags = append(tags, []string{"t", tag})
	}

	event, err := c.createEvent(30311, "", tags)
	if err != nil {
		log.Printf("Failed to create end event: %v", err)
		return
	}

	c.publishEvent(event)
}

// BroadcastEndEventWithResponse broadcasts a stream end event and returns event info
func (c *Client) BroadcastEndEventWithResponse(metadata *config.StreamMetadata) (string, []string) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return "", []string{}
	}

	log.Println("📡 Broadcasting stream end event to Nostr relays")

	tags := [][]string{
		{"d", metadata.Dtag},
		{"title", metadata.Title},
		{"summary", metadata.Summary},
		{"streaming", metadata.StreamURL},
		{"recording", metadata.RecordingURL},
		{"starts", metadata.Starts},
		{"ends", metadata.Ends},
		{"status", "ended"},
	}

	if metadata.Image != "" {
		tags = append(tags, []string{"image", metadata.Image})
	}

	// Add tags
	for _, tag := range metadata.Tags {
		tags = append(tags, []string{"t", tag})
	}

	event, err := c.createEvent(30311, "", tags)
	if err != nil {
		log.Printf("Failed to create end event: %v", err)
		return "", []string{}
	}

	// Convert event to JSON
	eventJSON, _ := json.Marshal(event)

	successfulRelays := c.publishEvent(event)
	return string(eventJSON), successfulRelays
}

// BroadcastCancelEvent broadcasts an event to cancel/end an incorrect stream event
func (c *Client) BroadcastCancelEvent(dtag string) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Println("📡 Broadcasting stream cancellation event to Nostr relays")

	tags := [][]string{
		{"d", dtag},
		{"status", "ended"},
		{"summary", "Stream was incorrectly marked as live"},
	}

	event, err := c.createEvent(30311, "", tags)
	if err != nil {
		log.Printf("Failed to create cancel event: %v", err)
		return
	}

	c.publishEvent(event)
}

// BroadcastDeletionEvent broadcasts a NIP-09 deletion request event
func (c *Client) BroadcastDeletionEvent(eventID string, reason string) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return
	}

	log.Printf("🗑️ Broadcasting NIP-09 deletion request for event: %s", eventID)

	tags := [][]string{
		{"e", eventID},
		{"k", "30311"}, // kind 30311 (live streaming event)
	}

	content := reason
	if content == "" {
		content = "Stream ended without recording"
	}

	event, err := c.createEvent(5, content, tags) // kind 5 = deletion request
	if err != nil {
		log.Printf("Failed to create deletion event: %v", err)
		return
	}

	successfulRelays := c.publishEvent(event)
	log.Printf("🗑️ Deletion request sent to %d relays", len(successfulRelays))
}

// BroadcastDeletionEventWithResponse broadcasts a NIP-09 deletion request and returns event info
func (c *Client) BroadcastDeletionEventWithResponse(eventID string, reason string) (string, []string) {
	if c.privateKey == nil {
		log.Println("⚠️ Nostr broadcasting disabled - keys not configured")
		return "", []string{}
	}

	log.Printf("🗑️ Broadcasting NIP-09 deletion request for event: %s", eventID)

	tags := [][]string{
		{"e", eventID},
		{"k", "30311"}, // kind 30311 (live streaming event)
	}

	content := reason
	if content == "" {
		content = "Stream ended without recording"
	}

	event, err := c.createEvent(5, content, tags) // kind 5 = deletion request
	if err != nil {
		log.Printf("Failed to create deletion event: %v", err)
		return "", []string{}
	}

	// Convert event to JSON
	eventJSON, _ := json.Marshal(event)

	successfulRelays := c.publishEvent(event)
	log.Printf("🗑️ Deletion request sent to %d relays", len(successfulRelays))
	
	return string(eventJSON), successfulRelays
}

// ExtractEventID extracts the event ID from a JSON event string
func ExtractEventID(eventJSON string) (string, error) {
	if eventJSON == "" {
		return "", errors.New("empty event JSON")
	}

	var event Event
	if err := json.Unmarshal([]byte(eventJSON), &event); err != nil {
		return "", fmt.Errorf("failed to parse event JSON: %w", err)
	}

	if event.ID == "" {
		return "", errors.New("no event ID found")
	}

	return event.ID, nil
}

// createEvent creates and signs a Nostr event
func (c *Client) createEvent(kind int, content string, tags [][]string) (*Event, error) {
	event := Event{
		PubKey:    c.publicKey,
		CreatedAt: time.Now().Unix(),
		Kind:      kind,
		Tags:      tags,
		Content:   content,
	}

	// Create serialization for ID calculation
	serializedData := []interface{}{
		0,
		event.PubKey,
		event.CreatedAt,
		event.Kind,
		event.Tags,
		event.Content,
	}

	// Serialize to JSON
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(serializedData)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize event: %w", err)
	}

	// Remove trailing newline
	serialized := bytes.TrimSpace(buffer.Bytes())

	// Calculate ID (SHA256 hash)
	hash := sha256.Sum256(serialized)
	event.ID = fmt.Sprintf("%x", hash[:])

	// Sign the event
	sig, err := schnorr.Sign(c.privateKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	event.Sig = hex.EncodeToString(sig.Serialize())

	return &event, nil
}

// publishEvent publishes an event to all configured relays and returns successful relay URLs
func (c *Client) publishEvent(event *Event) []string {
	if len(c.relays) == 0 {
		log.Println("⚠️ No Nostr relays configured")
		return []string{}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successfulRelays []string

	for _, relayURL := range c.relays {
		wg.Add(1)
		go func(relay string) {
			defer wg.Done()

			success, err := c.publishToRelay(event, relay)
			if err != nil {
				log.Printf("❌ Failed to publish to %s: %v", relay, err)
			} else if success {
				log.Printf("✅ Published to %s", relay)
				mu.Lock()
				successfulRelays = append(successfulRelays, relay)
				mu.Unlock()
			} else {
				log.Printf("❌ Rejected by %s", relay)
			}
		}(relayURL)
	}

	wg.Wait()
	log.Printf("📡 Published event to %d/%d relays", len(successfulRelays), len(c.relays))
	return successfulRelays
}

// publishToRelay publishes an event to a specific relay and returns success status
func (c *Client) publishToRelay(event *Event, relayURL string) (bool, error) {
	// Parse URL
	u, err := url.Parse(relayURL)
	if err != nil {
		return false, fmt.Errorf("invalid relay URL: %w", err)
	}

	// Connect to relay
	conn, err := websocket.Dial(u.String(), "", "http://localhost/")
	if err != nil {
		return false, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Set connection timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Create EVENT message
	message := []interface{}{"EVENT", event}
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return false, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send event
	if _, err := conn.Write(messageBytes); err != nil {
		return false, fmt.Errorf("failed to send event: %w", err)
	}

	// Read and parse response
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		log.Printf("📥 No response from %s: %v", relayURL, err)
		return false, nil // Consider no response as failure
	}

	responseStr := string(response[:n])
	log.Printf("📥 Response from %s: %s", relayURL, responseStr)

	// Parse the response to check for success
	// Nostr relay responses for EVENT messages: ["OK", <event_id>, <true|false>, <message>]
	var relayResponse []interface{}
	if err := json.Unmarshal(response[:n], &relayResponse); err != nil {
		log.Printf("📥 Failed to parse response from %s: %v", relayURL, err)
		return false, nil
	}

	// Check if it's an OK response and successful
	if len(relayResponse) >= 3 {
		if msgType, ok := relayResponse[0].(string); ok && msgType == "OK" {
			if success, ok := relayResponse[2].(bool); ok && success {
				return true, nil // Success!
			}
		}
	}

	return false, nil // Not a successful OK response
}
