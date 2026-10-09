package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"whatsapp-games/internal/connmanager"
	"whatsapp-games/internal/db"
	"whatsapp-games/internal/gamelogic"
	"whatsapp-games/internal/tunnel"
	"whatsapp-games/internal/web"
)

func main() {
	port := flag.String("port", "8088", "Port to listen on")
	dbPath := flag.String("db", "games.db", "SQLite database file path")
	useTunnel := flag.Bool("tunnel", false, "Start cloudflared quick tunnel for public HTTPS")
	flag.Parse()

	if envPort := os.Getenv("PORT"); envPort != "" {
		*port = envPort
	}
	if envDB := os.Getenv("DB_PATH"); envDB != "" {
		*dbPath = envDB
	}
	if os.Getenv("TUNNEL") == "true" || os.Getenv("TUNNEL") == "1" {
		*useTunnel = true
	}

	log.Println("==================================================")
	log.Println("🎮 Starting WhatsApp Webview Games Server")
	log.Println("==================================================")

	// 1. Initialize Database Service with buffered channel batch worker
	dbSvc, err := db.NewService(*dbPath, 1024)
	if err != nil {
		log.Fatalf("Failed to initialize Database Service: %v", err)
	}
	defer dbSvc.Close()
	log.Printf("[Database Service] Initialized with SQLite database: %s", *dbPath)

	// 2. Initialize Connection Manager Agent
	connMgr := connmanager.NewManager(nil)
	log.Println("[Connection Manager Agent] Initialized")

	// 3. Initialize Game Logic Agent
	_ = gamelogic.NewAgent(connMgr, dbSvc)
	log.Println("[Game Logic Agent] Initialized and linked with Connection Manager")

	// HTTP Router
	mux := http.NewServeMux()

	// WebSocket Route
	mux.HandleFunc("/ws", connMgr.ServeHTTP)

	// REST API Routes
	mux.HandleFunc("/api/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		entries, err := dbSvc.GetLeaderboard(25)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get leaderboard: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entries)
	})

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":            "ok",
			"active_websockets": connMgr.ConnectedCount(),
		})
	})

	// Static Webview Assets
	mux.Handle("/", http.FileServer(http.FS(web.GetFS())))

	serverAddr := ":" + *port
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Launch HTTP Server
	go func() {
		log.Printf("[Server] HTTP/WebSocket listening on http://localhost:%s", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Cloudflared Quick Tunnel (if requested)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var tunnelCleanup func()
	if *useTunnel {
		localURL := fmt.Sprintf("http://127.0.0.1:%s", *port)
		tunnelURL, cleanup, err := tunnel.StartQuickTunnel(ctx, localURL)
		if err != nil {
			log.Printf("[Tunnel] Warning: failed to start cloudflared tunnel: %v", err)
		} else {
			tunnelCleanup = cleanup
			log.Println("--------------------------------------------------")
			log.Printf("🌐 Public Webview HTTPS URL: %s", tunnelURL)
			log.Printf("👉 Set WHATSAPP_GAMES_URL=%s in whatsapp-go", tunnelURL)
			log.Println("--------------------------------------------------")
		}
	}

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\n[Server] Shutting down gracefully...")
	if tunnelCleanup != nil {
		tunnelCleanup()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Server] HTTP shutdown error: %v", err)
	}
	log.Println("[Server] Goodbye!")
}
