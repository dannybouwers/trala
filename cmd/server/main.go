package main

import (
	"log"
	"net/http"
	"time"

	"server/internal/config"
	"server/internal/handlers"
	"server/internal/i18n"
	"server/internal/icons"
	"server/internal/traefik"
)

// Version information set at build time
var (
	version   string
	commit    string
	buildTime string
)

func main() {
	// Load configuration
	config.Load()

	// Initialize HTTP clients
	traefik.InitializeHTTPClient()

	// Create external HTTP client for icon discovery (always has SSL verification enabled)
	externalHTTPClient := &http.Client{Timeout: 5 * time.Second}
	icons.InitHTTPClient(externalHTTPClient)

	// Set debug mode for icons package based on log level
	if config.GetLogLevel() == "debug" {
		icons.SetDebugMode(true)
	}

	// Initialize i18n
	i18n.Init()

	// Set version info in handlers
	handlers.SetVersionInfo(version, commit, buildTime)

	// Load HTML template
	handlers.LoadHTMLTemplate("/app/template")

	// Pre-warm caches
	go icons.GetSelfHstIconNames()
	go icons.GetSelfHstAppTags()
	go icons.ScanUserIcons()

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/services", handlers.ServicesHandler)
	mux.HandleFunc("/api/status", handlers.StatusHandler)
	mux.HandleFunc("/api/health", handlers.HealthHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("/app/static"))))
	mux.Handle("/icons/", http.StripPrefix("/icons/", http.FileServer(http.Dir("/icons"))))
	mux.HandleFunc("/", handlers.ServeHTMLTemplate)

	// Start server
	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
