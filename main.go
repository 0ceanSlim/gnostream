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

	"stream-server/internal/config"
	"stream-server/internal/stream"
	"stream-server/internal/web"
)

func main() {
	log.Println("🎬 Starting Live Streaming Server...")

	// Load configuration
	cfg, err := config.Load("configs/config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Server will run on %s:%d", cfg.Server.Host, cfg.Server.Port)

	// Initialize stream monitor
	monitor, err := stream.NewMonitor(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize stream monitor: %v", err)
	}

	// Start stream monitoring in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Println("📡 Starting stream monitor...")
		if err := monitor.Start(ctx); err != nil {
			log.Printf("Stream monitor error: %v", err)
		}
	}()

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
