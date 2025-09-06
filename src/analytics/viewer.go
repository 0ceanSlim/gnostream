package analytics

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ViewerSession represents a viewer session
type ViewerSession struct {
	ID            string    `json:"id"`
	IPAddress     string    `json:"ip_address"`
	UserAgent     string    `json:"user_agent"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
	RequestCount  int       `json:"request_count"`
	PlaylistReqs  int       `json:"playlist_requests"`
	SegmentReqs   int       `json:"segment_requests"`
	IsActive      bool      `json:"is_active"`
}

// ViewerMetrics represents current viewer statistics
type ViewerMetrics struct {
	TotalViewers     int               `json:"total_viewers"`
	ActiveViewers    int               `json:"active_viewers"`
	PeakViewers      int               `json:"peak_viewers"`
	Sessions         []ViewerSession   `json:"sessions"`
	RequestsPerMin   int               `json:"requests_per_minute"`
	LastUpdated      time.Time         `json:"last_updated"`
}

// ViewerTracker tracks HLS viewer sessions
type ViewerTracker struct {
	sessions       map[string]*ViewerSession
	metrics        ViewerMetrics
	mutex          sync.RWMutex
	sessionTimeout time.Duration
	cleanupTicker  *time.Ticker
}

// NewViewerTracker creates a new viewer tracker
func NewViewerTracker() *ViewerTracker {
	tracker := &ViewerTracker{
		sessions:       make(map[string]*ViewerSession),
		sessionTimeout: 30 * time.Second, // Consider inactive after 30s
		cleanupTicker:  time.NewTicker(10 * time.Second),
	}

	// Start cleanup routine
	go tracker.cleanupRoutine()

	return tracker
}

// generateSessionID creates a unique session ID from IP and User-Agent
func (vt *ViewerTracker) generateSessionID(ip, userAgent string) string {
	hash := sha256.Sum256([]byte(ip + "|" + userAgent + "|" + fmt.Sprint(time.Now().Unix()/300))) // 5-min buckets
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for shorter ID
}

// TrackRequest records an HLS request
func (vt *ViewerTracker) TrackRequest(r *http.Request) {
	vt.mutex.Lock()
	defer vt.mutex.Unlock()

	// Extract client info
	ip := vt.getClientIP(r)
	userAgent := r.UserAgent()
	
	// Generate session ID
	sessionID := vt.generateSessionID(ip, userAgent)
	
	// Get or create session
	session, exists := vt.sessions[sessionID]
	if !exists {
		session = &ViewerSession{
			ID:        sessionID,
			IPAddress: ip,
			UserAgent: userAgent,
			FirstSeen: time.Now(),
			IsActive:  true,
		}
		vt.sessions[sessionID] = session
	}

	// Update session
	session.LastSeen = time.Now()
	session.RequestCount++
	session.IsActive = true

	// Categorize request type
	path := strings.ToLower(r.URL.Path)
	if strings.HasSuffix(path, ".m3u8") {
		session.PlaylistReqs++
	} else if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".mp4") {
		session.SegmentReqs++
	}

	// Update metrics
	vt.updateMetrics()
}

// getClientIP extracts the real client IP
func (vt *ViewerTracker) getClientIP(r *http.Request) string {
	// Check for forwarded IP headers
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to remote address
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

// updateMetrics recalculates current metrics
func (vt *ViewerTracker) updateMetrics() {
	now := time.Now()
	activeCount := 0
	totalCount := len(vt.sessions)

	// Count active sessions
	for _, session := range vt.sessions {
		if now.Sub(session.LastSeen) <= vt.sessionTimeout {
			activeCount++
		} else {
			session.IsActive = false
		}
	}

	vt.metrics.TotalViewers = totalCount
	vt.metrics.ActiveViewers = activeCount
	vt.metrics.LastUpdated = now

	// Update peak viewers
	if activeCount > vt.metrics.PeakViewers {
		vt.metrics.PeakViewers = activeCount
	}

	// Update sessions slice for API
	vt.metrics.Sessions = make([]ViewerSession, 0, len(vt.sessions))
	for _, session := range vt.sessions {
		vt.metrics.Sessions = append(vt.metrics.Sessions, *session)
	}
}

// GetMetrics returns current viewer metrics
func (vt *ViewerTracker) GetMetrics() ViewerMetrics {
	vt.mutex.RLock()
	defer vt.mutex.RUnlock()
	
	// Update active status before returning
	vt.updateMetrics()
	
	return vt.metrics
}

// GetActiveViewerCount returns just the active viewer count
func (vt *ViewerTracker) GetActiveViewerCount() int {
	vt.mutex.RLock()
	defer vt.mutex.RUnlock()
	
	now := time.Now()
	activeCount := 0
	
	for _, session := range vt.sessions {
		if now.Sub(session.LastSeen) <= vt.sessionTimeout {
			activeCount++
		}
	}
	
	return activeCount
}

// cleanupRoutine removes old inactive sessions
func (vt *ViewerTracker) cleanupRoutine() {
	for range vt.cleanupTicker.C {
		vt.cleanupInactiveSessions()
	}
}

// cleanupInactiveSessions removes sessions inactive for more than 5 minutes
func (vt *ViewerTracker) cleanupInactiveSessions() {
	vt.mutex.Lock()
	defer vt.mutex.Unlock()
	
	cutoff := time.Now().Add(-5 * time.Minute)
	
	for id, session := range vt.sessions {
		if session.LastSeen.Before(cutoff) {
			delete(vt.sessions, id)
		}
	}
	
	vt.updateMetrics()
}

// ResetMetrics resets peak viewers and other cumulative stats
func (vt *ViewerTracker) ResetMetrics() {
	vt.mutex.Lock()
	defer vt.mutex.Unlock()
	
	vt.metrics.PeakViewers = vt.metrics.ActiveViewers
}

// IsHLSRequest checks if the request is for HLS content
func IsHLSRequest(r *http.Request) bool {
	path := strings.ToLower(r.URL.Path)
	ext := filepath.Ext(path)
	
	return ext == ".m3u8" || ext == ".ts" || ext == ".mp4"
}

// Stop stops the viewer tracker
func (vt *ViewerTracker) Stop() {
	if vt.cleanupTicker != nil {
		vt.cleanupTicker.Stop()
	}
}