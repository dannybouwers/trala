// Package handlers provides HTTP handlers for the Trala dashboard.
// It contains all HTTP endpoint handlers, template rendering, and version information.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"server/internal/config"
	appi18n "server/internal/i18n"
	"server/internal/icons"
	"server/internal/models"
	"server/internal/services"
	"server/internal/traefik"
)

// --- Version Information ---

// Version information set at build time
var (
	version   string
	commit    string
	buildTime string
)

// SetVersionInfo sets the version information from build-time flags.
// This should be called during application initialization.
func SetVersionInfo(v, c, bt string) {
	version = v
	commit = c
	buildTime = bt
}

// GetVersionInfo returns the current version information.
func GetVersionInfo() models.VersionInfo {
	return models.VersionInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
	}
}

// --- Template Handling ---

var (
	htmlTemplate   []byte
	htmlOnce       sync.Once
	parsedTemplate *template.Template
)

// LoadHTMLTemplate reads the index.html file into memory once and parses it.
// The template is parsed with i18n support via a "T" function that accepts a localizer.
func LoadHTMLTemplate(templatePath string) {
	htmlOnce.Do(func() {
		var err error
		templatePath := filepath.Join(templatePath, "index.html")
		htmlTemplate, err = os.ReadFile(templatePath)
		if err != nil {
			log.Fatalf("FATAL: Could not read index.html template at %s: %v", templatePath, err)
		}
		// Parse template once and register a T function that expects a *i18n.Localizer
		// as first argument. The handler will pass the request-local Localizer via
		// the template data as "Localizer".
		tmpl, err := template.New("index").Funcs(template.FuncMap{
			"T": func(localizer *i18n.Localizer, id string) string {
				if localizer == nil {
					return id
				}
				msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id})
				if err != nil {
					return id
				}
				return msg
			},
		}).Parse(string(htmlTemplate))

		if err != nil {
			log.Fatalf("FATAL: Could not parse index.html: %v", err)
		}
		parsedTemplate = tmpl
	})
}

// --- HTTP Handlers ---

// ServeHTMLTemplate renders the HTML template with i18n support using go-i18n.
func ServeHTMLTemplate(w http.ResponseWriter, r *http.Request) {
	lang := config.GetLanguage()

	// Create a localizer for the selected language
	localizer := appi18n.GetLocalizer(lang)

	// Set the response content type and execute the pre-parsed template
	// Set the response content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute the pre-parsed template and pass the request-local Localizer in data.
	// Templates must call the function like: {{ T .Localizer "message.id" }}
	data := map[string]interface{}{
		"Localizer": localizer,
	}
	if err := parsedTemplate.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
	}
}

// ServicesHandler is the main API endpoint. It fetches, processes, and returns all service data.
func ServicesHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch entrypoints from the Traefik API with pagination support.
	entryPointsURL := fmt.Sprintf("%s/api/entrypoints", config.GetTraefikAPIHost())
	entryPoints, err := traefik.FetchAllPages[models.TraefikEntryPoint](w, entryPointsURL)
	if err != nil {
		return // Error already handled by FetchAllPages
	}
	debugf("Successfully fetched %d entrypoints from Traefik.", len(entryPoints))

	// Create a map for faster lookups.
	entryPointsMap := make(map[string]models.TraefikEntryPoint, len(entryPoints))
	for _, ep := range entryPoints {
		entryPointsMap[ep.Name] = ep
	}

	// Fetch routers from the Traefik API with pagination support.
	routersURL := fmt.Sprintf("%s/api/http/routers", config.GetTraefikAPIHost())
	routers, err := traefik.FetchAllPages[models.TraefikRouter](w, routersURL)
	if err != nil {
		return // Error already handled by FetchAllPages
	}
	debugf("Successfully fetched %d routers from Traefik.", len(routers))

	// Process all routers concurrently to find their icons.
	var wg sync.WaitGroup
	serviceChan := make(chan models.Service, len(routers))

	for _, router := range routers {
		wg.Add(1)
		go func(r models.TraefikRouter) {
			defer wg.Done()
			service, ok := services.ProcessRouter(r, entryPointsMap)
			if ok {
				serviceChan <- service
			}
		}(router)
	}

	wg.Wait()
	close(serviceChan)

	// Collect results from Traefik services.
	traefikServices := make([]models.Service, 0, len(routers))
	for service := range serviceChan {
		traefikServices = append(traefikServices, service)
	}

	// Add manual services
	manualServices := services.GetManualServices()

	// Merge all services
	finalServices := make([]models.Service, 0, len(traefikServices)+len(manualServices))
	finalServices = append(finalServices, traefikServices...)
	finalServices = append(finalServices, manualServices...)

	// Calculate groups
	finalServices = services.CalculateGroups(finalServices)

	// Sort by priority (higher priority first)
	sort.Slice(finalServices, func(i, j int) bool {
		return finalServices[i].Priority > finalServices[j].Priority
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalServices)
}

// HealthHandler performs health checks and returns the status.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the most important configuration (Traefik API host) is valid
	traefikAPIHost := config.GetTraefikAPIHost()
	searchEngineURL := config.GetSearchEngineURL()
	selfhstIconURL := config.GetSelfhstIconURL()

	if traefikAPIHost == "" {
		http.Error(w, "Traefik API host is not set", http.StatusInternalServerError)
		return
	}

	// Validate SearchEngineURL
	if !config.IsValidUrl(searchEngineURL) {
		http.Error(w, "Search Engine URL is invalid", http.StatusInternalServerError)
		return
	}

	// Validate SelfhstIconURL
	if !config.IsValidUrl(selfhstIconURL) {
		http.Error(w, "Selfhst Icon URL is invalid", http.StatusInternalServerError)
		return
	}

	// Check if Traefik is reachable
	entryPointsURL := fmt.Sprintf("%s/api/entrypoints", traefikAPIHost)

	// Create a context with timeout for the health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create and execute the request with context and auth
	resp, err := traefik.CreateAndExecuteHTTPRequestWithContext(w, ctx, "GET", entryPointsURL)
	if err != nil {
		return // Error already handled by CreateAndExecuteHTTPRequestWithContext
	}
	defer resp.Body.Close()

	// If we reach here, all checks passed
	fmt.Fprint(w, "OK")
}

// StatusHandler returns combined application status information.
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	// Get version information
	versionInfo := GetVersionInfo()

	// Get configuration status (already stored in global variable)
	configStatus := config.GetConfigCompatibilityStatus()

	// Get frontend configuration
	searchEngineURL := config.GetSearchEngineURL()
	refreshIntervalSeconds := config.GetRefreshIntervalSeconds()

	// Extract service name from search engine URL and find its icon
	searchEngineIconURL := ""
	if searchEngineURL != "" {
		serviceName := services.ExtractServiceNameFromURL(searchEngineURL)
		if serviceName != "" {
			displayNameReplaced := strings.ReplaceAll(serviceName, " ", "-")
			reference := icons.ResolveSelfHstReference(displayNameReplaced)
			searchEngineIconURL = icons.FindIcon(serviceName, searchEngineURL, serviceName, reference)
		}
	}

	frontendConfig := models.FrontendConfig{
		SearchEngineURL:        searchEngineURL,
		SearchEngineIconURL:    searchEngineIconURL,
		RefreshIntervalSeconds: refreshIntervalSeconds,
		GroupingEnabled:        config.GetGroupingEnabled(),
		GroupingColumns:        config.GetGroupingColumns(),
	}

	// Combine all status information
	status := models.ApplicationStatus{
		Version:  versionInfo,
		Config:   configStatus,
		Frontend: frontendConfig,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// --- Helper Functions ---

// debugf logs a message only if LOG_LEVEL is set to "debug".
func debugf(format string, v ...interface{}) {
	if config.GetLogLevel() == "debug" {
		log.Printf("DEBUG: "+format, v...)
	}
}
