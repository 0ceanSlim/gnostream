package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	nostrTypes "github.com/0ceanslim/grain/server/types"

	"gnostream/src/config"
	"gnostream/src/nostr"
)

// ChatAPI handles live chat functionality
type ChatAPI struct {
	config      *config.Config
	nostrClient nostr.Client
	monitor     StreamMonitor
	wsManager   *WebSocketManager
}

// StreamMonitor interface for getting current stream metadata
type StreamMonitor interface {
	GetCurrentMetadata() *config.StreamMetadata
}

// NewChatAPI creates a new chat API handler
func NewChatAPI(cfg *config.Config, client nostr.Client, monitor StreamMonitor, wsManager *WebSocketManager) *ChatAPI {
	return &ChatAPI{
		config:      cfg,
		nostrClient: client,
		monitor:     monitor,
		wsManager:   wsManager,
	}
}

// ChatMessage represents a live chat message with user profile
type ChatMessage struct {
	ID        string             `json:"id"`
	PubKey    string             `json:"pubkey"`
	CreatedAt int64              `json:"created_at"`
	Content   string             `json:"content"`
	Tags      [][]string         `json:"tags"`
	Sig       string             `json:"sig"`
	Profile   *UserProfile       `json:"profile,omitempty"`
	ReplyTo   string             `json:"reply_to,omitempty"`
}

// ChatMessagesResponse represents the response for chat messages
type ChatMessagesResponse struct {
	Success  bool          `json:"success"`
	Messages []ChatMessage `json:"messages"`
	Error    string        `json:"error,omitempty"`
}

// SendMessageRequest represents a request to send a chat message
type SendMessageRequest struct {
	Content string `json:"content"`
	ReplyTo string `json:"reply_to,omitempty"`
}

// SendMessageResponse represents the response for sending a message
type SendMessageResponse struct {
	Success bool   `json:"success"`
	EventID string `json:"event_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleGetMessages retrieves live chat messages for the current stream
func (api *ChatAPI) HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current stream metadata to find the 'a' tag
	streamMetadata, err := api.getCurrentStreamMetadata()
	if err != nil {
		api.sendErrorResponse(w, "Failed to get stream metadata", http.StatusInternalServerError)
		return
	}

	// Only skip chat if there's no metadata file (dtag == "offline" means no file)
	if streamMetadata.Dtag == "offline" {
		log.Printf("📝 No stream metadata available, returning empty chat")
		response := ChatMessagesResponse{
			Success:  true,
			Messages: []ChatMessage{},
		}
		api.sendJSONResponse(w, response, http.StatusOK)
		return
	}

	log.Printf("📝 Returning cached chat messages for stream: %s (status: %s)", streamMetadata.Dtag, streamMetadata.Status)

	// Get cached messages from WebSocket manager (no subscriptions here!)
	var messages []ChatMessage
	if api.wsManager != nil {
		messages = api.wsManager.GetCachedMessages()
	} else {
		messages = []ChatMessage{}
	}

	log.Printf("📝 Returning %d cached chat messages", len(messages))

	response := ChatMessagesResponse{
		Success:  true,
		Messages: messages,
	}

	api.sendJSONResponse(w, response, http.StatusOK)
}

// HandleSendMessage sends a new live chat message
func (api *ChatAPI) HandleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if user is authenticated
	if !session.IsSessionManagerInitialized() {
		api.sendErrorResponse(w, "Session manager not initialized", http.StatusInternalServerError)
		return
	}

	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil || userSession.Mode != session.WriteMode {
		api.sendErrorResponse(w, "Authentication required for sending messages", http.StatusUnauthorized)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		api.sendErrorResponse(w, "Message content cannot be empty", http.StatusBadRequest)
		return
	}

	// Get current stream metadata
	streamMetadata, err := api.getCurrentStreamMetadata()
	if err != nil {
		api.sendErrorResponse(w, "Failed to get stream metadata", http.StatusInternalServerError)
		return
	}

	// Create the live chat event (kind 1311)
	eventID, err := api.createChatEvent(userSession, streamMetadata, req.Content, req.ReplyTo)
	if err != nil {
		log.Printf("❌ Failed to create chat event: %v", err)
		api.sendErrorResponse(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	response := SendMessageResponse{
		Success: true,
		EventID: eventID,
	}

	api.sendJSONResponse(w, response, http.StatusOK)
}

// getCurrentStreamMetadata gets the current stream metadata
func (api *ChatAPI) getCurrentStreamMetadata() (*config.StreamMetadata, error) {
	// Use the monitor to get current metadata, but only if it has valid data
	if api.monitor != nil {
		metadata := api.monitor.GetCurrentMetadata()
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
			Pubkey: api.config.Nostr.PublicKey,
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

	// Extract dtag and pubkey from the last nostr event (which is the actual live event)
	dtag := metadata.Dtag
	pubkey := api.config.Nostr.PublicKey

	if metadata.LastNostrEvent != "" {
		var event struct {
			PubKey string     `json:"pubkey"`
			Tags   [][]string `json:"tags"`
		}
		if err := json.Unmarshal([]byte(metadata.LastNostrEvent), &event); err != nil {
			log.Printf("❌ Failed to parse last_nostr_event: %v", err)
		} else {
			log.Printf("🔍 Parsed event: pubkey=%s, tags_count=%d", event.PubKey[:16]+"...", len(event.Tags))

			// Get pubkey from event
			if event.PubKey != "" {
				pubkey = event.PubKey
			}

			// Get dtag from the 'd' tag in the event (this is the authoritative source)
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "d" {
					log.Printf("🔍 Found d tag: %s", tag[1])
					dtag = tag[1]
					break
				}
			}
		}
	} else {
		log.Printf("⚠️ No last_nostr_event found in metadata")
	}

	log.Printf("📝 Final metadata: dtag=%s, pubkey=%s", dtag, pubkey[:16]+"...")

	return &config.StreamMetadata{
		Dtag:   dtag,
		Pubkey: pubkey,
		Title:  metadata.Title,
		Status: metadata.Status,
	}, nil
}

// getChatMessages retrieves live chat messages for a stream
func (api *ChatAPI) getChatMessages(dtag, hostPubkey string) ([]ChatMessage, error) {
	if api.nostrClient == nil || !api.nostrClient.IsEnabled() {
		return nil, fmt.Errorf("nostr client not available or disabled")
	}

	// Create the 'a' tag for the live stream event
	aTag := fmt.Sprintf("30311:%s:%s", hostPubkey, dtag)

	// Create filter for kind 1311 (live chat) events with specific 'a' tag
	limit := 100
	filters := []nostrTypes.Filter{
		{
			Kinds: []int{1311}, // Kind 1311 = live chat message
			Tags: map[string][]string{
				"a": {aTag}, // Filter by the specific stream 'a' tag
			},
			Limit: &limit,
		},
	}

	log.Printf("🔍 Looking for messages with 'a' tag: %s", aTag)

	log.Printf("🔍 Fetching chat messages for stream: %s", aTag)

	// Subscribe using the injected nostr client (grain automatically starts it)
	subscription, err := api.nostrClient.Subscribe(filters, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe for chat messages: %w", err)
	}
	defer subscription.Close()

	var chatMessages []ChatMessage
	seenEventIDs := make(map[string]bool) // Deduplication map
	timeout := time.After(5 * time.Second) // 5 second timeout

	// Collect all available messages and filter by 'a' tag
	for {
		select {
		case event := <-subscription.Events:
			if event != nil {
				// Skip if we've already seen this event ID (deduplication)
				if seenEventIDs[event.ID] {
					continue
				}
				seenEventIDs[event.ID] = true

				// Check if this message is for our stream by looking for the 'a' tag
				isForOurStream := false
				var foundATags []string

				for _, tag := range event.Tags {
					if len(tag) >= 2 && tag[0] == "a" {
						foundATags = append(foundATags, tag[1])
						if tag[1] == aTag {
							isForOurStream = true
						}
					}
				}

				if isForOurStream {
					chatMsg := api.eventToChatMessage(event)
					if chatMsg != nil {
						chatMessages = append(chatMessages, *chatMsg)
						log.Printf("💬 Found message for our stream: %s", event.ID[:8])
					}
				}
				// Remove verbose logging of skipped messages
			}
		case <-timeout:
			log.Printf("📝 Chat message fetch timeout, collected %d unique messages for our stream (dtag: %s)", len(chatMessages), dtag)
			goto fetchProfiles
		}
	}

fetchProfiles:
	// Sort messages by created_at
	sort.Slice(chatMessages, func(i, j int) bool {
		return chatMessages[i].CreatedAt < chatMessages[j].CreatedAt
	})

	// Fetch user profiles for all unique pubkeys
	api.enrichWithProfiles(chatMessages)

	log.Printf("📝 Returning %d chat messages with profiles", len(chatMessages))
	return chatMessages, nil
}

// eventToChatMessage converts a nostr event to a ChatMessage
func (api *ChatAPI) eventToChatMessage(event *nostrTypes.Event) *ChatMessage {
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

	// Check for reply (e tag)
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			chatMsg.ReplyTo = tag[1]
			break
		}
	}

	return chatMsg
}

// enrichWithProfiles fetches and adds user profiles to chat messages
func (api *ChatAPI) enrichWithProfiles(messages []ChatMessage) {
	if len(messages) == 0 {
		return
	}

	// Collect unique pubkeys
	pubkeySet := make(map[string]bool)
	for _, msg := range messages {
		pubkeySet[msg.PubKey] = true
	}

	// Fetch profiles for all unique pubkeys
	profiles := api.fetchMultipleProfiles(pubkeySet)

	// Assign profiles to messages
	for i := range messages {
		if profile, exists := profiles[messages[i].PubKey]; exists {
			messages[i].Profile = profile
		}
	}
}

// fetchMultipleProfiles fetches profiles for multiple pubkeys
func (api *ChatAPI) fetchMultipleProfiles(pubkeys map[string]bool) map[string]*UserProfile {
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		log.Printf("Core client not available for profile fetch")
		return make(map[string]*UserProfile)
	}

	// Convert pubkeys map to slice
	var pubkeyList []string
	for pubkey := range pubkeys {
		pubkeyList = append(pubkeyList, pubkey)
	}

	if len(pubkeyList) == 0 {
		return make(map[string]*UserProfile)
	}

	// Create filter for kind 0 (metadata) events
	limit := len(pubkeyList)
	filters := []nostrTypes.Filter{
		{
			Authors: pubkeyList,
			Kinds:   []int{0}, // Kind 0 = user metadata
			Limit:   &limit,
		},
	}

	log.Printf("🔍 Fetching profiles for %d users", len(pubkeyList))

	// Subscribe and get profile events
	subscription, err := coreClient.Subscribe(filters, nil)
	if err != nil {
		log.Printf("Failed to subscribe for profiles: %v", err)
		return make(map[string]*UserProfile)
	}
	defer subscription.Close()

	profiles := make(map[string]*UserProfile)
	timeout := time.After(3 * time.Second) // 3 second timeout for profiles

	// Collect profile events
	for {
		select {
		case event := <-subscription.Events:
			if event != nil && event.Kind == 0 {
				profile := api.parseProfileFromEvent(event)
				if profile != nil {
					profiles[event.PubKey] = profile
				}
			}
		case <-timeout:
			log.Printf("👤 Profile fetch timeout, collected %d profiles", len(profiles))
			return profiles
		}
	}
}

// parseProfileFromEvent parses a kind 0 event into UserProfile (reused from auth.go)
func (api *ChatAPI) parseProfileFromEvent(event *nostrTypes.Event) *UserProfile {
	if event.Kind != 0 {
		return nil
	}

	profile := &UserProfile{}

	// Parse JSON content
	var profileData map[string]interface{}
	if err := json.Unmarshal([]byte(event.Content), &profileData); err != nil {
		log.Printf("Failed to parse profile JSON: %v", err)
		return nil
	}

	// Extract common profile fields
	if name, ok := profileData["name"].(string); ok {
		profile.Name = name
	}
	if displayName, ok := profileData["display_name"].(string); ok {
		profile.DisplayName = displayName
	}
	if about, ok := profileData["about"].(string); ok {
		profile.About = about
	}
	if picture, ok := profileData["picture"].(string); ok {
		profile.Picture = picture
	}
	if banner, ok := profileData["banner"].(string); ok {
		profile.Banner = banner
	}
	if website, ok := profileData["website"].(string); ok {
		profile.Website = website
	}
	if nip05, ok := profileData["nip05"].(string); ok {
		profile.Nip05 = nip05
	}
	if lud16, ok := profileData["lud16"].(string); ok {
		profile.Lud16 = lud16
	}

	return profile
}

// createChatEvent creates and broadcasts a live chat event
func (api *ChatAPI) createChatEvent(userSession *session.UserSession, streamMetadata *config.StreamMetadata, content, replyTo string) (string, error) {
	if !api.nostrClient.IsEnabled() {
		return "", fmt.Errorf("nostr client not enabled")
	}

	// Get the Grain client for event building
	grainClient, ok := api.nostrClient.(*nostr.GrainClient)
	if !ok {
		return "", fmt.Errorf("failed to get grain client")
	}

	client := grainClient.GetClient()
	if client == nil {
		return "", fmt.Errorf("grain core client not available")
	}

	// Create the 'a' tag for the live stream event
	aTag := fmt.Sprintf("30311:%s:%s", streamMetadata.Pubkey, streamMetadata.Dtag)

	// Build the live chat event (kind 1311)
	eventBuilder := core.NewEventBuilder(1311).
		Content(content).
		Tag("a", aTag, "", "root") // Reference to the live stream event

	// Add reply tag if replying to another message
	if replyTo != "" {
		eventBuilder = eventBuilder.ETag(replyTo, "", "reply")
	}

	event := eventBuilder.Build()

	// Sign the event using the session's signing method
	var signedEvent *nostrTypes.Event
	var err error

	switch userSession.SigningMethod {
	case "browser_extension":
		// For browser extension, we'll need to return unsigned event
		// and let the frontend handle signing via extension
		return "", fmt.Errorf("browser extension signing not implemented for chat yet")

	case "private_key":
		// Get the user's signer (this would need to be implemented)
		return "", fmt.Errorf("private key signing not implemented for chat yet")

	default:
		return "", fmt.Errorf("unsupported signing method: %s", userSession.SigningMethod)
	}

	// For now, let's create a mock signed event for testing
	// In production, proper signing would be implemented
	signedEvent = event
	signedEvent.PubKey = userSession.PublicKey
	signedEvent.ID = fmt.Sprintf("mock_id_%d", time.Now().UnixNano())
	signedEvent.Sig = "mock_signature"

	// Broadcast the event
	results, err := client.PublishEvent(signedEvent, nil)
	if err != nil {
		return "", fmt.Errorf("failed to publish chat event: %w", err)
	}

	summary := core.SummarizeBroadcast(results)
	log.Printf("💬 Chat message published to %d/%d relays (%.1f%% success)",
		summary.Successful, summary.TotalRelays, summary.SuccessRate)

	return signedEvent.ID, nil
}

// Helper methods

func (api *ChatAPI) sendJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (api *ChatAPI) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	api.sendJSONResponse(w, response, statusCode)
}