package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/client/session"

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
	Error       string              `json:"error,omitempty"`
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
			// Decode nsec to get public key
			privateKeyHex, err := tools.DecodeNsec(req.PrivateKey)
			if err != nil {
				api.sendErrorResponse(w, fmt.Sprintf("Invalid nsec format: %v", err), http.StatusBadRequest)
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

	log.Printf("🔑 User logged in: %s (%s mode)", userSession.PublicKey[:16]+"...", userSession.Mode)

	response := LoginResponse{
		Success:   true,
		Message:   "Login successful",
		Session:   userSession,
		PublicKey: userSession.PublicKey,
		NPub:      npub,
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
		Name:     "session_token",
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

	// Try to get session from cookie or header
	// This is a simplified version - in production you'd implement proper session storage
	sessionCookie, err := r.Cookie("session_token")
	if err != nil || sessionCookie.Value == "" {
		response := SessionResponse{
			Success:  true,
			IsActive: false,
		}
		api.sendJSONResponse(w, response, http.StatusOK)
		return
	}

	// In a real implementation, you'd validate the session token and retrieve session data
	// For now, we'll just return a placeholder response
	response := SessionResponse{
		Success:  true,
		IsActive: true,
		Session: &session.UserSession{
			PublicKey:       "placeholder",
			LastActive:      time.Now(),
			Mode:           "read_only", 
			SigningMethod:  "browser_extension",
			ConnectedRelays: api.config.Nostr.Relays,
		},
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

// Helper methods

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