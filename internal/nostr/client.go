package nostr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"golang.org/x/net/websocket"

	"stream-server/internal/config"
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

// Client handles Nostr relay communication
type Client struct {
	privateKey *btcec.PrivateKey
	publicKey  string
	relays     []string
}

// NewClient creates a new Nostr client
func NewClient(cfg *config.NostrRelayConfig) (*Client, error) {
	// Decode private key
	keyBytes, err := hex.DecodeString(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	privateKey, _ := btcec.PrivKeyFromBytes(keyBytes)

	// Decode public key
	publicKeyBytes, err := hex.DecodeString(cfg.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	publicKey := fmt.Sprintf("%x", publicKeyBytes)

	return &Client{
		privateKey: privateKey,
		publicKey:  publicKey,
		relays:     cfg.Relays,
	}, nil
}

// BroadcastStartEvent broadcasts a stream start event
func (c *Client) BroadcastStartEvent(metadata *config.StreamMetadata) {
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

// BroadcastUpdateEvent broadcasts a stream metadata update
func (c *Client) BroadcastUpdateEvent(metadata *config.StreamMetadata) {
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

// BroadcastEndEvent broadcasts a stream end event
func (c *Client) BroadcastEndEvent(metadata *config.StreamMetadata) {
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

// publishEvent publishes an event to all configured relays
func (c *Client) publishEvent(event *Event) {
	if len(c.relays) == 0 {
		log.Println("⚠️ No Nostr relays configured")
		return
	}

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for _, relayURL := range c.relays {
		wg.Add(1)
		go func(relay string) {
			defer wg.Done()

			if err := c.publishToRelay(event, relay); err != nil {
				log.Printf("❌ Failed to publish to %s: %v", relay, err)
			} else {
				log.Printf("✅ Published to %s", relay)
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(relayURL)
	}

	wg.Wait()
	log.Printf("📡 Published event to %d/%d relays", successCount, len(c.relays))
}

// publishToRelay publishes an event to a specific relay
func (c *Client) publishToRelay(event *Event, relayURL string) error {
	// Parse URL
	u, err := url.Parse(relayURL)
	if err != nil {
		return fmt.Errorf("invalid relay URL: %w", err)
	}

	// Connect to relay
	conn, err := websocket.Dial(u.String(), "", "http://localhost/")
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Set connection timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Create EVENT message
	message := []interface{}{"EVENT", event}
	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send event
	if _, err := conn.Write(messageBytes); err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}

	// Read response (optional)
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err == nil {
		log.Printf("📥 Response from %s: %s", relayURL, string(response[:n]))
	}

	return nil
}
