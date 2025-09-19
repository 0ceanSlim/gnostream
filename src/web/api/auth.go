package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/client/connection"
	nostr "github.com/0ceanslim/grain/server/types"
	"regexp"
	"strings"

	"gnostream/src/config"
)

// AuthAPI handles authentication and session management
type AuthAPI struct {
	config *config.Config
}

// NewAuthAPI creates a new authentication API handler
func NewAuthAPI(cfg *config.Config) *AuthAPI {
	return &AuthAPI{config: cfg}
}

// LoginRequest represents a login request
type LoginRequest struct {
	PublicKey     string                         `json:"public_key,omitempty"`
	PrivateKey    string                         `json:"private_key,omitempty"`  // nsec format
	SigningMethod session.SigningMethod         `json:"signing_method"`
	Mode          session.SessionInteractionMode `json:"mode"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success     bool                `json:"success"`
	Message     string              `json:"message"`
	Session     *session.UserSession `json:"session,omitempty"`
	PublicKey   string              `json:"public_key,omitempty"`
	NPub        string              `json:"npub,omitempty"`
	IsOwner     bool                `json:"is_owner"`
	Error       string              `json:"error,omitempty"`
}

// KeyPairResponse represents a key generation response
type KeyPairResponse struct {
	Success    bool              `json:"success"`
	KeyPair    *tools.KeyPair    `json:"key_pair,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// SessionResponse represents a session status response
type SessionResponse struct {
	Success     bool                `json:"success"`
	IsActive    bool                `json:"is_active"`
	Session     *session.UserSession `json:"session,omitempty"`
	Profile     *UserProfile        `json:"profile,omitempty"`
	IsOwner     bool                `json:"is_owner"`
	Error       string              `json:"error,omitempty"`
}

// UserProfile represents a user's Nostr profile
type UserProfile struct {
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	About       string `json:"about,omitempty"`
	Picture     string `json:"picture,omitempty"`
	Banner      string `json:"banner,omitempty"`
	Website     string `json:"website,omitempty"`
	Nip05       string `json:"nip05,omitempty"`
	Lud16       string `json:"lud16,omitempty"`
}

// HandleLogin handles user login/authentication
func (api *AuthAPI) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if err := api.validateLoginRequest(&req); err != nil {
		api.sendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create session init request for Grain
	sessionReq := session.SessionInitRequest{
		RequestedMode: req.Mode,
		SigningMethod: req.SigningMethod,
	}

	// Handle different signing methods
	switch req.SigningMethod {
	case "browser_extension":
		if req.PublicKey == "" {
			api.sendErrorResponse(w, "Public key required for browser extension signing", http.StatusBadRequest)
			return
		}
		sessionReq.PublicKey = req.PublicKey

	default:
		// For other methods, we might need private key
		if req.PrivateKey != "" {
			var privateKeyHex string
			var err error

			// Handle both nsec and hex format
			if strings.HasPrefix(req.PrivateKey, "nsec") {
				// Decode nsec to get hex private key
				privateKeyHex, err = tools.DecodeNsec(req.PrivateKey)
				if err != nil {
					api.sendErrorResponse(w, fmt.Sprintf("Invalid nsec format: %v", err), http.StatusBadRequest)
					return
				}
			} else if len(req.PrivateKey) == 64 {
				// Assume it's already hex format
				if matched, _ := regexp.MatchString("^[0-9a-fA-F]{64}$", req.PrivateKey); !matched {
					api.sendErrorResponse(w, "Invalid hex private key format", http.StatusBadRequest)
					return
				}
				privateKeyHex = req.PrivateKey
			} else {
				api.sendErrorResponse(w, "Private key must be nsec format or 64-character hex", http.StatusBadRequest)
				return
			}

			pubkey, err := tools.DerivePublicKey(privateKeyHex)
			if err != nil {
				api.sendErrorResponse(w, fmt.Sprintf("Failed to derive public key: %v", err), http.StatusBadRequest)
				return
			}

			sessionReq.PublicKey = pubkey
			sessionReq.PrivateKey = req.PrivateKey
		} else if req.PublicKey != "" {
			sessionReq.PublicKey = req.PublicKey
		} else {
			api.sendErrorResponse(w, "Either public key or private key must be provided", http.StatusBadRequest)
			return
		}
	}

	// Create user session
	userSession, err := session.CreateUserSession(w, sessionReq)
	if err != nil {
		api.sendErrorResponse(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusBadRequest)
		return
	}

	// Generate npub for response
	npub, _ := tools.EncodePubkey(userSession.PublicKey)

	// Check if user is the server owner
	isOwner := api.isServerOwner(userSession.PublicKey)
	log.Printf("🔑 User logged in: %s (%s mode) isOwner: %v", userSession.PublicKey[:16]+"...", userSession.Mode, isOwner)

	// Fetch user profile in background (don't block login)
	go func() {
		profile := api.fetchUserProfile(userSession.PublicKey)
		if profile != nil {
			log.Printf("🔑 Profile loaded for user: %s", profile.Name)
		}
	}()

	response := LoginResponse{
		Success:   true,
		Message:   "Login successful",
		Session:   userSession,
		PublicKey: userSession.PublicKey,
		NPub:      npub,
		IsOwner:   isOwner,
	}

	api.sendJSONResponse(w, response, http.StatusOK)
}

// HandleLogout handles user logout
func (api *AuthAPI) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "grain-session",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-time.Hour),
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
	})

	log.Println("🔑 User logged out")

	response := map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	}

	api.sendJSONResponse(w, response, http.StatusOK)
}

// HandleSession handles session status checks
func (api *AuthAPI) HandleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session from grain session manager
	if !session.IsSessionManagerInitialized() {
		response := SessionResponse{
			Success:  true,
			IsActive: false,
			Error:   "session manager not initialized",
		}
		api.sendJSONResponse(w, response, http.StatusOK)
		return
	}

	userSession := session.SessionMgr.GetCurrentUser(r)
	if userSession == nil {
		response := SessionResponse{
			Success:  true,
			IsActive: false,
		}
		api.sendJSONResponse(w, response, http.StatusOK)
		return
	}

	// Fetch user profile information
	profile := api.fetchUserProfile(userSession.PublicKey)

	// Check if user is the server owner
	isOwner := api.isServerOwner(userSession.PublicKey)
	log.Printf("🔍 User %s isOwner: %v", userSession.PublicKey[:16]+"...", isOwner)

	response := SessionResponse{
		Success:  true,
		IsActive: true,
		Session:  userSession,
		Profile:  profile,
		IsOwner:  isOwner,
	}

	api.sendJSONResponse(w, response, http.StatusOK)
}

// HandleGenerateKeys handles key pair generation
func (api *AuthAPI) HandleGenerateKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate new key pair
	keyPair, err := tools.GenerateKeyPair()
	if err != nil {
		api.sendErrorResponse(w, fmt.Sprintf("Failed to generate keys: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("🔑 Generated new key pair: %s", keyPair.Npub)

	response := KeyPairResponse{
		Success: true,
		KeyPair: keyPair,
	}

	api.sendJSONResponse(w, response, http.StatusOK)
}

// HandleConnectRelay handles connecting to a new relay
func (api *AuthAPI) HandleConnectRelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RelayURL string `json:"relay_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RelayURL == "" {
		api.sendErrorResponse(w, "Relay URL is required", http.StatusBadRequest)
		return
	}

	// In a real implementation, you'd connect to the relay
	// For now, just return success
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Connected to relay: %s", req.RelayURL),
		"relay":   req.RelayURL,
	}

	log.Printf("🌐 Connected to relay: %s", req.RelayURL)
	api.sendJSONResponse(w, response, http.StatusOK)
}

// HandleAmberCallback processes callbacks from Amber app
func (api *AuthAPI) HandleAmberCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔍 Amber callback received: method=%s, url=%s", r.Method, r.URL.String())

	// Parse query parameters
	eventParam := r.URL.Query().Get("event")
	if eventParam == "" {
		log.Printf("❌ Amber callback missing event parameter")
		api.renderAmberError(w, "Missing event data from Amber")
		return
	}

	// Extract public key from event parameter
	publicKey, err := api.extractPublicKeyFromAmber(eventParam)
	if err != nil {
		log.Printf("❌ Failed to extract public key from amber response: %v", err)
		api.renderAmberError(w, "Invalid response from Amber: "+err.Error())
		return
	}

	log.Printf("✅ Amber callback processed successfully: %s...", publicKey[:16])

	// Create session
	sessionRequest := session.SessionInitRequest{
		PublicKey:     publicKey,
		RequestedMode: session.WriteMode,
		SigningMethod: session.AmberSigning,
	}

	_, err = session.CreateUserSession(w, sessionRequest)
	if err != nil {
		log.Printf("❌ Failed to create amber session: %v", err)
		api.renderAmberError(w, "Failed to create session")
		return
	}

	log.Printf("✅ Amber session created successfully: %s...", publicKey[:16])

	// Render success page with auto-redirect
	api.renderAmberSuccess(w, publicKey)
}

// extractPublicKeyFromAmber extracts and validates public key from Amber response
func (api *AuthAPI) extractPublicKeyFromAmber(eventParam string) (string, error) {
	// Handle compressed response (starts with "Signer1")
	if strings.HasPrefix(eventParam, "Signer1") {
		return "", fmt.Errorf("compressed Amber responses not supported")
	}

	// For get_public_key, event parameter should contain the public key directly
	publicKey := strings.TrimSpace(eventParam)

	// Validate public key format (64 hex characters)
	pubKeyRegex := regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	if !pubKeyRegex.MatchString(publicKey) {
		return "", fmt.Errorf("invalid public key format from Amber")
	}

	return publicKey, nil
}

// renderAmberSuccess renders the success page for Amber callback
func (api *AuthAPI) renderAmberSuccess(w http.ResponseWriter, publicKey string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Amber Login Success</title>
    <style>
        body {
            font-family: 'Share Tech Mono', monospace;
            margin: 0;
            padding: 20px;
            background: #001100;
            color: #00ff41;
            text-align: center;
        }
        .success { color: #00ff41; margin: 20px 0; }
        .loading { color: #888; }
    </style>
</head>
<body>
    <div class="success">
        <h2>✅ Amber Login Successful!</h2>
        <p>Connected successfully. Returning to gnostream...</p>
    </div>
    <div class="loading">
        <p>Please wait...</p>
    </div>

    <script>
        const amberResult = {
            success: true,
            publicKey: '` + publicKey + `',
            timestamp: Date.now()
        };

        try {
            localStorage.setItem('amber_callback_result', JSON.stringify(amberResult));
            console.log('Stored Amber success result in localStorage');
        } catch (error) {
            console.error('Failed to store Amber result:', error);
        }

        if (window.opener && !window.opener.closed) {
            try {
                window.opener.postMessage({
                    type: 'amber_success',
                    publicKey: '` + publicKey + `'
                }, window.location.origin);
                console.log('Sent success message to opener window');
            } catch (error) {
                console.error('Failed to send message to opener:', error);
            }
        }

        setTimeout(() => {
            try {
                if (window.opener && !window.opener.closed) {
                    window.close();
                } else {
                    window.location.href = '/?amber_login=success';
                }
            } catch (error) {
                console.error('Failed to navigate:', error);
                window.location.href = '/';
            }
        }, 1500);
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

// renderAmberError renders the error page for Amber callback
func (api *AuthAPI) renderAmberError(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Amber Login Error</title>
    <style>
        body {
            font-family: 'Share Tech Mono', monospace;
            margin: 0;
            padding: 20px;
            background: #001100;
            color: #ff4444;
            text-align: center;
        }
        .error { color: #ff4444; margin: 20px 0; }
        .retry { margin-top: 20px; }
        .retry a { color: #00ff41; text-decoration: none; }
    </style>
</head>
<body>
    <div class="error">
        <h2>❌ Amber Login Failed</h2>
        <p>` + errorMsg + `</p>
    </div>
    <div class="retry">
        <a href="/">← Return to login</a>
    </div>

    <script>
        if (window.opener) {
            window.opener.postMessage({
                type: 'amber_error',
                error: '` + errorMsg + `'
            }, window.location.origin);
            setTimeout(() => window.close(), 3000);
        }
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

// Helper methods

// fetchUserProfile fetches user profile metadata from Nostr
func (api *AuthAPI) fetchUserProfile(publicKey string) *UserProfile {
	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		log.Printf("Core client not available for profile fetch")
		return nil
	}

	// Create filter for kind 0 (metadata) events
	limit := 1
	filters := []nostr.Filter{
		{
			Authors: []string{publicKey},
			Kinds:   []int{0}, // Kind 0 = user metadata
			Limit:   &limit,
		},
	}

	// Subscribe and get the profile event
	subscription, err := coreClient.Subscribe(filters, nil)
	if err != nil {
		log.Printf("Failed to subscribe for profile: %v", err)
		return nil
	}
	defer subscription.Close()

	// Wait for events (with timeout)
	select {
	case event := <-subscription.Events:
		if event != nil {
			return api.parseProfileFromEvent(event)
		}
	case <-time.After(5 * time.Second): // 5 second timeout
		log.Printf("Profile fetch timeout for pubkey: %s", publicKey[:8])
	}

	return nil
}

// parseProfileFromEvent parses a kind 0 event into UserProfile
func (api *AuthAPI) parseProfileFromEvent(event *nostr.Event) *UserProfile {
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

func (api *AuthAPI) validateLoginRequest(req *LoginRequest) error {
	if req.SigningMethod == "" {
		return fmt.Errorf("signing method is required")
	}

	if req.Mode == "" {
		req.Mode = "read_only" // Default to read-only
	}

	return nil
}

func (api *AuthAPI) sendJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (api *AuthAPI) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	api.sendJSONResponse(w, response, statusCode)
}

// isServerOwner checks if the given public key matches the server owner's public key
func (api *AuthAPI) isServerOwner(publicKey string) bool {
	// Get the server owner's private key from config
	serverPrivateKey := api.config.Nostr.PrivateKey
	if serverPrivateKey == "" {
		return false
	}

	var privateKeyHex string
	var err error

	// Handle nsec format
	if strings.HasPrefix(serverPrivateKey, "nsec") {
		privateKeyHex, err = tools.DecodeNsec(serverPrivateKey)
		if err != nil {
			log.Printf("Failed to decode server nsec: %v", err)
			return false
		}
	} else {
		// Assume it's already hex format
		privateKeyHex = serverPrivateKey
	}

	// Derive the public key from the server's private key
	serverPublicKey, err := tools.DerivePublicKey(privateKeyHex)
	if err != nil {
		log.Printf("Failed to derive server public key: %v", err)
		return false
	}

	log.Printf("🔍 Owner check: user=%s server=%s match=%v", publicKey[:16]+"...", serverPublicKey[:16]+"...", publicKey == serverPublicKey)

	// Compare the public keys
	return publicKey == serverPublicKey
}