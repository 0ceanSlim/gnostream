package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/core"
	nostrTypes "github.com/0ceanslim/grain/server/types"
	"github.com/gorilla/websocket"

	"gnostream/src/config"
	"gnostream/src/nostr"
)

// WebSocketManager handles live chat WebSocket connections
type WebSocketManager struct {
	config       *config.Config
	monitor      StreamMonitor
	clients      map[*websocket.Conn]*ChatClient
	clientsMux   sync.RWMutex
	broadcast    chan ChatMessage
	register     chan *ChatClient
	unregister   chan *ChatClient
	nostrClient  nostr.Client
	nostrSub     *core.Subscription
	currentATag  string
	// Message cache for HTTP API
	messageCache []ChatMessage
	cacheMux     sync.RWMutex
}

// ChatClient represents a connected WebSocket client
type ChatClient struct {
	conn     *websocket.Conn
	send     chan ChatMessage
	manager  *WebSocketManager
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager(cfg *config.Config, monitor StreamMonitor, nostrClient nostr.Client) *WebSocketManager {
	return &WebSocketManager{
		config:       cfg,
		monitor:      monitor,
		clients:      make(map[*websocket.Conn]*ChatClient),
		broadcast:    make(chan ChatMessage, 256),
		register:     make(chan *ChatClient),
		unregister:   make(chan *ChatClient),
		nostrClient:  nostrClient,
		messageCache: make([]ChatMessage, 0),
	}
}

// Run starts the WebSocket manager
func (wsm *WebSocketManager) Run() {
	// Create a ticker to check for stream changes every 30 seconds
	streamCheckTicker := time.NewTicker(30 * time.Second)
	defer streamCheckTicker.Stop()

	for {
		select {
		case client := <-wsm.register:
			wsm.clientsMux.Lock()
			wsm.clients[client.conn] = client
			wsm.clientsMux.Unlock()
			log.Printf("💬 WebSocket client connected (%d total)", len(wsm.clients))

			// Subscription is now handled by StartInitialSubscription(), not here

		case client := <-wsm.unregister:
			wsm.clientsMux.Lock()
			if _, ok := wsm.clients[client.conn]; ok {
				delete(wsm.clients, client.conn)
				close(client.send)
			}
			wsm.clientsMux.Unlock()
			log.Printf("💬 WebSocket client disconnected (%d total)", len(wsm.clients))

			// Subscription stays active regardless of client count

		case message := <-wsm.broadcast:
			wsm.clientsMux.RLock()
			for _, client := range wsm.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(wsm.clients, client.conn)
				}
			}
			wsm.clientsMux.RUnlock()

		case <-streamCheckTicker.C:
			// Stream change checking is now handled by StartInitialSubscription()
		}
	}
}

// HandleWebSocket handles WebSocket connection requests
func (wsm *WebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed: %v", err)
		return
	}

	client := &ChatClient{
		conn:    conn,
		send:    make(chan ChatMessage, 256),
		manager: wsm,
	}

	client.manager.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// startNostrSubscription starts subscribing to nostr relays for chat messages
func (wsm *WebSocketManager) startNostrSubscription() {
	if wsm.nostrClient == nil || !wsm.nostrClient.IsEnabled() {
		log.Printf("📝 Nostr client not available for WebSocket subscription")
		return
	}

	// Get current stream metadata
	metadata, err := wsm.getCurrentStreamMetadata()
	if err != nil {
		log.Printf("❌ Failed to get stream metadata for WebSocket: %v", err)
		return
	}

	if metadata.Dtag == "offline" {
		log.Printf("📝 Stream offline, not starting nostr subscription")
		return
	}

	// Construct a tag using event ID (stored in LastNostrEvent) instead of pubkey
	aTag := "30311:" + metadata.LastNostrEvent + ":" + metadata.Dtag

	// Check if already subscribed to this stream
	if wsm.currentATag == aTag && wsm.nostrSub != nil {
		log.Printf("📡 Already subscribed to stream: %s", aTag)
		return
	}

	wsm.currentATag = aTag
	log.Printf("📡 Starting real-time nostr subscription for: %s", aTag)

	// Create subscription filter - grain client has issues with tag filters, so just filter by kind
	// We'll do client-side filtering since relay filtering isn't working
	filters := []nostrTypes.Filter{
		{
			Kinds: []int{1311}, // Kind 1311 = live chat message
			// Note: No tag filter due to grain client issues - using client-side filtering instead
		},
	}

	subscription, err := wsm.nostrClient.Subscribe(filters, nil)
	if err != nil {
		log.Printf("❌ Failed to create nostr subscription: %v", err)
		return
	}

	log.Printf("✅ Nostr subscription created successfully")
	wsm.nostrSub = subscription

	// Grain automatically starts the subscription, no need to call Start()

	// Listen for events using grain's event channel
	go wsm.listenForEvents()
}

// stopNostrSubscription stops the nostr subscription
func (wsm *WebSocketManager) stopNostrSubscription() {
	if wsm.nostrSub != nil {
		log.Printf("📡 Stopping nostr subscription")
		wsm.nostrSub.Close()
		wsm.nostrSub = nil
		wsm.currentATag = ""
	}
}

// listenForEvents listens for incoming nostr events using grain's channels
func (wsm *WebSocketManager) listenForEvents() {
	if wsm.nostrSub == nil {
		return
	}

	seenEventIDs := make(map[string]bool)

	for {
		select {
		case event := <-wsm.nostrSub.Events:
			if event != nil {
				// Skip duplicates (grain may send same event from multiple relays)
				if seenEventIDs[event.ID] {
					continue
				}
				seenEventIDs[event.ID] = true

				// Verify this event is actually for our stream
				isForOurStream := false
				for _, tag := range event.Tags {
					if len(tag) >= 2 && tag[0] == "a" && tag[1] == wsm.currentATag {
						isForOurStream = true
						break
					}
				}

				if !isForOurStream {
					continue
				}

				// Convert to chat message
				chatMsg := wsm.eventToChatMessage(event)
				if chatMsg != nil {

					// Fetch user profile for the message using grain client
					if chatMsg.Profile == nil {
						chatMsg.Profile = wsm.fetchUserProfile(event.PubKey)
					}

					// Add to message cache for HTTP API
					wsm.addToCache(*chatMsg)

					// Broadcast to all connected WebSocket clients
					select {
					case wsm.broadcast <- *chatMsg:
					default:
						// Channel full, drop message silently
					}
				}
			}

		case err := <-wsm.nostrSub.Errors:
			if err != nil {
				log.Printf("⚠️ Nostr subscription error: %v", err)
			}

		case <-wsm.nostrSub.Done:
			log.Printf("📡 Nostr subscription closed")
			return
		}
	}
}

// getCurrentStreamMetadata gets current stream metadata (uses same logic as chat.go)
func (wsm *WebSocketManager) getCurrentStreamMetadata() (*config.StreamMetadata, error) {
	// Try monitor first
	if wsm.monitor != nil {
		metadata := wsm.monitor.GetCurrentMetadata()
		if metadata != nil && metadata.Dtag != "" && metadata.Pubkey != "" {
			log.Printf("🔍 Monitor provided valid metadata: dtag=%s, status=%s", metadata.Dtag, metadata.Status)
			return metadata, nil
		} else {
			if metadata != nil {
				log.Printf("⚠️ Monitor metadata incomplete: dtag='%s', pubkey='%s', falling back to file", metadata.Dtag, metadata.Pubkey)
			} else {
				log.Printf("⚠️ Monitor returned nil metadata, falling back to file")
			}
		}
	} else {
		log.Printf("⚠️ No monitor available, reading file directly")
	}

	// Fallback to reading metadata file directly
	metadataFile := "www/live/metadata.json"

	// Check if metadata file exists
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		// Return default if no metadata file - but this means no chat available
		log.Printf("📝 No stream metadata file found - stream is offline")
		return &config.StreamMetadata{
			Dtag:   "offline",
			Pubkey: wsm.config.Nostr.PublicKey,
			Title:  "Stream Offline",
			Status: "offline",
		}, nil
	}

	// Read the metadata file
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Parse the JSON metadata
	var metadata struct {
		Dtag             string   `json:"dtag"`
		Title            string   `json:"title"`
		Status           string   `json:"status"`
		LastNostrEvent   string   `json:"last_nostr_event"`
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	log.Printf("🔍 Raw metadata: dtag=%s, status=%s, last_nostr_event_length=%d",
		metadata.Dtag, metadata.Status, len(metadata.LastNostrEvent))

	// Extract dtag, pubkey, and event ID from the last nostr event
	dtag := metadata.Dtag
	pubkey := wsm.config.Nostr.PublicKey
	eventID := ""

	if metadata.LastNostrEvent != "" {
		var event struct {
			ID     string     `json:"id"`
			PubKey string     `json:"pubkey"`
			Tags   [][]string `json:"tags"`
		}
		if err := json.Unmarshal([]byte(metadata.LastNostrEvent), &event); err != nil {
			log.Printf("❌ Failed to parse last_nostr_event: %v", err)
		} else {
		
			// Get event ID (this is what we need for the a tag)
			eventID = event.ID

			// Get pubkey from event
			if event.PubKey != "" {
				pubkey = event.PubKey
			}

			// Get dtag from the 'd' tag in the event (this is the authoritative source)
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "d" {
					dtag = tag[1]
					break
				}
			}
		}
	} else {
		log.Printf("⚠️ No last_nostr_event found in metadata")
	}


	result := &config.StreamMetadata{
		Dtag:   dtag,
		Pubkey: pubkey,
		Title:  metadata.Title,
		Status: metadata.Status,
	}

	// Store event ID in LastNostrEvent field for a tag construction (temporary solution)
	if eventID != "" {
		result.LastNostrEvent = eventID
	}

	return result, nil
}

// eventToChatMessage converts nostr event to ChatMessage (simplified)
func (wsm *WebSocketManager) eventToChatMessage(event *nostrTypes.Event) *ChatMessage {
	if event.Kind != 1311 {
		return nil
	}

	chatMsg := &ChatMessage{
		ID:        event.ID,
		PubKey:    event.PubKey,
		CreatedAt: event.CreatedAt,
		Content:   event.Content,
		Tags:      event.Tags,
		Sig:       event.Sig,
	}

	// Check for reply
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			chatMsg.ReplyTo = tag[1]
			break
		}
	}

	return chatMsg
}

// fetchUserProfile fetches user profile using the nostr client
func (wsm *WebSocketManager) fetchUserProfile(pubkey string) *UserProfile {
	if wsm.nostrClient == nil || !wsm.nostrClient.IsEnabled() {
		return &UserProfile{
			Name: pubkey[:8] + "...",
		}
	}

	// Use the nostr client to fetch user profile
	profileEvent, err := wsm.nostrClient.GetUserProfile(pubkey, nil)
	if err != nil {
		log.Printf("⚠️ Failed to fetch profile for %s: %v", pubkey[:8], err)
		return &UserProfile{
			Name: pubkey[:8] + "...",
		}
	}

	if profileEvent == nil {
		return &UserProfile{
			Name: pubkey[:8] + "...",
		}
	}

	// Parse the profile content (kind 0 event contains JSON metadata)
	var profileData map[string]interface{}
	if err := json.Unmarshal([]byte(profileEvent.Content), &profileData); err != nil {
		return &UserProfile{
			Name: pubkey[:8] + "...",
		}
	}

	profile := &UserProfile{
		Name: pubkey[:8] + "...",
	}

	if name, ok := profileData["name"].(string); ok && name != "" {
		profile.Name = name
	}
	if displayName, ok := profileData["display_name"].(string); ok && displayName != "" {
		profile.DisplayName = displayName
	}
	if picture, ok := profileData["picture"].(string); ok && picture != "" {
		profile.Picture = picture
	}

	return profile
}

// addToCache adds a message to the cache (thread-safe)
func (wsm *WebSocketManager) addToCache(message ChatMessage) {
	wsm.cacheMux.Lock()
	defer wsm.cacheMux.Unlock()

	// Check for duplicates
	for _, existing := range wsm.messageCache {
		if existing.ID == message.ID {
			return // Already cached
		}
	}

	// Add to cache
	wsm.messageCache = append(wsm.messageCache, message)

	// Keep cache size reasonable (last 100 messages)
	if len(wsm.messageCache) > 100 {
		wsm.messageCache = wsm.messageCache[len(wsm.messageCache)-100:]
	}

}

// GetCachedMessages returns cached messages (thread-safe)
func (wsm *WebSocketManager) GetCachedMessages() []ChatMessage {
	wsm.cacheMux.RLock()
	defer wsm.cacheMux.RUnlock()

	// Return a copy to avoid race conditions
	messages := make([]ChatMessage, len(wsm.messageCache))
	copy(messages, wsm.messageCache)
	return messages
}

// ClearCache clears the message cache (when stream changes)
func (wsm *WebSocketManager) ClearCache() {
	wsm.cacheMux.Lock()
	defer wsm.cacheMux.Unlock()
	wsm.messageCache = make([]ChatMessage, 0)
}

// checkStreamChange checks if the stream has changed and restarts subscription if needed
func (wsm *WebSocketManager) checkStreamChange() {
	metadata, err := wsm.getCurrentStreamMetadata()
	if err != nil {
		log.Printf("⚠️ Failed to check stream metadata: %v", err)
		return
	}

	if metadata.Dtag == "offline" {
		// Stream went offline - stop subscription
		if wsm.nostrSub != nil {
			log.Printf("📴 Stream went offline - stopping subscription")
			wsm.stopNostrSubscription()
		}
		return
	}

	newATag := "30311:" + metadata.Pubkey + ":" + metadata.Dtag

	// If stream changed, restart subscription
	if wsm.currentATag != newATag {
		log.Printf("🔄 Stream changed: %s → %s", wsm.currentATag, newATag)

		// Stop old subscription
		if wsm.nostrSub != nil {
			wsm.stopNostrSubscription()
		}

		// Clear old messages
		wsm.ClearCache()

		// Start new subscription
		wsm.startNostrSubscription()
	}
}

// StartInitialSubscription starts nostr subscription immediately on server startup
func (wsm *WebSocketManager) StartInitialSubscription() {
	log.Printf("🚀 Starting initial nostr subscription on server startup")

	// Start with a small delay to let server finish initializing
	time.Sleep(2 * time.Second)

	// Ensure no existing subscription
	if wsm.nostrSub != nil {
		log.Printf("🔄 Found existing subscription, stopping it first")
		wsm.stopNostrSubscription()
	}

	// Clear any existing cache from wrong messages
	wsm.ClearCache()

	// Start the subscription immediately
	wsm.startNostrSubscription()

	// Then continue with periodic checks for stream changes
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wsm.checkStreamChange()
		}
	}
}

// Client read pump
func (c *ChatClient) readPump() {
	defer func() {
		c.manager.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// Client write pump
func (c *ChatClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send message as JSON
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}