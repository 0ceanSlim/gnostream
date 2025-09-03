package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gnostream/src/config"
	"gnostream/src/rtmp"
	"gnostream/src/stream"
	"gnostream/src/web"
)

func main() {
	log.Println("🎬 Starting Live Streaming Server...")

	// Load configuration
	cfg, err := config.Load("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Server will run on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Ensure required directories exist
	if err := ensureDirectories(cfg); err != nil {
		log.Fatalf("Failed to create required directories: %v", err)
	}

	// Initialize stream monitor
	monitor, err := stream.NewMonitor(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize stream monitor: %v", err)
	}

	// Initialize and start RTMP server if enabled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var rtmpServer *rtmp.Server
	rtmpDefaults := cfg.GetRTMPDefaults()
	if rtmpDefaults.Enabled {
		rtmpServer = rtmp.NewServer(cfg)

		// Set up stream handlers to connect RTMP server with stream monitor
		rtmpServer.SetStreamHandlers(
			monitor.HandleStreamStart, // Called when stream starts
			monitor.HandleStreamStop,  // Called when stream stops
		)

		// Start RTMP server
		go func() {
			log.Printf("🎬 Starting RTMP server on port %d...", rtmpDefaults.Port)
			if err := rtmpServer.Start(ctx); err != nil {
				log.Printf("RTMP server error: %v", err)
			}
		}()
	} else {
		// Start traditional stream monitoring if RTMP server is disabled
		go func() {
			log.Println("📡 Starting stream monitor...")
			if err := monitor.Start(ctx); err != nil {
				log.Printf("Stream monitor error: %v", err)
			}
		}()
	}

	// Initialize web server
	webServer := web.NewServer(cfg, monitor)

	// Setup HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      webServer.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("🚀 Server starting on http://%s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Cancel monitor context
	cancel()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server gracefully stopped")
}

// ensureDirectories creates required directories if they don't exist
func ensureDirectories(cfg *config.Config) error {
	streamDefaults := cfg.GetStreamDefaults()
	directories := []string{
		streamDefaults.OutputDir,
		streamDefaults.ArchiveDir,
		"www/res",
		"www/views",
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	log.Println("✅ Required directories created/verified")
	return nil
}
