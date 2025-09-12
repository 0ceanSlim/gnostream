package web

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"gnostream/src/analytics"
	"gnostream/src/config"
	"gnostream/src/stream"
	"gnostream/src/web/api"
)

// Server represents the web server
type Server struct {
	config        *config.Config
	monitor       *stream.Monitor
	templates     *template.Template
	viewerTracker *analytics.ViewerTracker
	authAPI       *api.AuthAPI
}

// NewServer creates a new web server instance
func NewServer(cfg *config.Config, monitor *stream.Monitor) *Server {
	server := &Server{
		config:        cfg,
		monitor:       monitor,
		viewerTracker: analytics.NewViewerTracker(),
		authAPI:       api.NewAuthAPI(cfg),
	}

	// Load templates
	server.loadTemplates()

	return server
}

// Router sets up HTTP routes
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Static files - using /res/ prefix to match your structure (with CORS)
	mux.Handle("/res/", http.StripPrefix("/res/", s.corsHandler(http.FileServer(http.Dir("www/res/")))))
	// CSS styles - for compiled Tailwind CSS with proper MIME type
	mux.Handle("/style/", http.StripPrefix("/style/", s.cssHandler(http.FileServer(http.Dir("www/style/")))))

	// Get stream defaults
	streamDefaults := s.config.GetStreamDefaults()

	// HLS streaming files (with CORS and viewer tracking)
	mux.Handle("/live/", http.StripPrefix("/live/", s.hlsTrackingHandler(http.FileServer(http.Dir(streamDefaults.OutputDir)))))
	mux.Handle("/archive/", http.StripPrefix("/archive/", s.hlsTrackingHandler(http.FileServer(http.Dir(streamDefaults.ArchiveDir)))))

	// API endpoints (with CORS)
	mux.HandleFunc("/api/stream-data", s.corsWrapper(s.handleStreamData))
	mux.HandleFunc("/api/health", s.corsWrapper(s.handleHealth))
	mux.HandleFunc("/api/viewers", s.corsWrapper(s.handleViewerMetrics))
	
	// Authentication API endpoints
	mux.HandleFunc("/api/auth/login", s.corsWrapper(s.authAPI.HandleLogin))
	mux.HandleFunc("/api/auth/logout", s.corsWrapper(s.authAPI.HandleLogout))
	mux.HandleFunc("/api/auth/session", s.corsWrapper(s.authAPI.HandleSession))
	mux.HandleFunc("/api/auth/generate-keys", s.corsWrapper(s.authAPI.HandleGenerateKeys))
	mux.HandleFunc("/api/auth/connect-relay", s.corsWrapper(s.authAPI.HandleConnectRelay))


	// Web pages with HTMX routing (with CORS)
	mux.HandleFunc("/", s.corsWrapper(s.handleLive))
	mux.HandleFunc("/archive", s.corsWrapper(s.handleArchive))
	

	return mux
}

// cssHandler ensures CSS files are served with correct MIME type
func (s *Server) cssHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set correct MIME type for CSS files
		if strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		}
		next.ServeHTTP(w, r)
	})
}

// corsHandler adds CORS headers for streaming files
func (s *Server) corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only set CORS for HLS streaming files, not all static resources
		if strings.HasSuffix(r.URL.Path, ".m3u8") || strings.HasSuffix(r.URL.Path, ".ts") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// corsWrapper adds CORS headers for API routes only
func (s *Server) corsWrapper(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only set CORS for API endpoints
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// hlsTrackingHandler wraps file serving with HLS viewer tracking
func (s *Server) hlsTrackingHandler(next http.Handler) http.Handler {
	return s.corsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track HLS requests
		if analytics.IsHLSRequest(r) {
			s.viewerTracker.TrackRequest(r)
			// Only log playlist requests (.m3u8), not individual segments (.ts)
			if strings.HasSuffix(r.URL.Path, ".m3u8") {
				log.Printf("📊 HLS Request: %s from %s (Active viewers: %d)", 
					r.URL.Path, 
					s.getClientIP(r),
					s.viewerTracker.GetActiveViewerCount())
			}
		}
		
		next.ServeHTTP(w, r)
	}))
}

// getClientIP extracts the real client IP (duplicate from analytics, but needed here)
func (s *Server) getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

// loadTemplates loads HTML templates with your structure
func (s *Server) loadTemplates() {
	// Define template directories
	templatePaths := []string{
		"www/views/templates/*.html",  // layout, header, footer
		"www/views/*.html",            // main view pages
		"www/views/components/*.html", // reusable components
	}

	var allFiles []string
	for _, pattern := range templatePaths {
		files, err := filepath.Glob(pattern)
		if err != nil {
			log.Printf("Error globbing pattern %s: %v", pattern, err)
			continue
		}
		allFiles = append(allFiles, files...)
	}

	if len(allFiles) == 0 {
		log.Fatal("No template files found. Please create templates in www/views/")
		return
	}

	// Parse all template files
	templates, err := template.New("").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).ParseFiles(allFiles...)

	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
		return
	}

	s.templates = templates
	log.Printf("Loaded %d template files", len(allFiles))
}

// handleLive serves the live streaming page
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	metadata := s.monitor.GetCurrentMetadata()

	data := struct {
		Title   string
		Summary string
		Tags    []string
		Status  string
		View    string
	}{
		Title:   metadata.Title,
		Summary: metadata.Summary,
		Tags:    metadata.Tags,
		Status:  metadata.Status,
		View:    "live-view",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Check if this is an HTMX request for partial content
	if r.Header.Get("HX-Request") == "true" {
		// Return only the content part
		if err := s.templates.ExecuteTemplate(w, "live-view", data); err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
	} else {
		// Return full layout
		if err := s.templates.ExecuteTemplate(w, "layout", data); err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
	}
}

// handleArchive serves the archive page
func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title   string
		Summary string
		Tags    []string
		Status  string
		View    string
	}{
		Title:   "Stream Archive",
		Summary: "Browse through previous streams",
		Tags:    []string{},
		Status:  "archive",
		View:    "archive-view",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Check if this is an HTMX request for partial content
	if r.Header.Get("HX-Request") == "true" {
		// Return only the content part
		if err := s.templates.ExecuteTemplate(w, "archive-view", data); err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
	} else {
		// Return full layout
		if err := s.templates.ExecuteTemplate(w, "layout", data); err != nil {
			log.Printf("Template error: %v", err)
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}
	}
}

// handleStreamData serves stream metadata as JSON
func (s *Server) handleStreamData(w http.ResponseWriter, r *http.Request) {
	metadata := s.monitor.GetCurrentMetadata()
	viewerCount := s.viewerTracker.GetActiveViewerCount()

	// Add viewer count to response
	response := map[string]interface{}{
		"metadata":       metadata,
		"active_viewers": viewerCount,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON: %v", err)
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}
}

// handleHealth serves health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "healthy"
	if !s.monitor.IsActive() {
		status = "offline"
	}

	response := map[string]interface{}{
		"status": status,
		"active": s.monitor.IsActive(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding health JSON: %v", err)
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}
}

// handleViewerMetrics serves viewer analytics data
func (s *Server) handleViewerMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.viewerTracker.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Printf("Error encoding viewer metrics JSON: %v", err)
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}
}


