package main

import (
	"log"
	"net/http"
	"strings"
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

// noDirListingFileServer wraps http.FileServer to disable directory listing.
func noDirListingFileServer(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func main() {
	// Load configuration
	config.Load()

	// Initialize HTTP clients
	traefik.InitializeHTTPClient()

	// Create external HTTP client for icon discovery (always has SSL verification enabled)
	externalHTTPClient := &http.Client{Timeout: 5 * time.Second}
	icons.InitHTTPClient(externalHTTPClient)

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
	mux.Handle("/static/", http.StripPrefix("/static/", noDirListingFileServer("/app/static")))
	mux.Handle("/icons/", http.StripPrefix("/icons/", noDirListingFileServer("/icons")))
	mux.HandleFunc("/", handlers.ServeHTMLTemplate)

	// Start server
	log.Println("WARNING: TraLa does not provide authentication. Ensure it is placed behind an authenticating reverse proxy.")
	log.Println("Starting server on :8080...")
	server := &http.Server{
		Addr:              ":8080",
		Handler:           handlers.SecurityHeaders(mux),
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
