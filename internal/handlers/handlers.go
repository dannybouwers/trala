// Package handlers provides HTTP handlers for the Trala dashboard.
// It contains all HTTP endpoint handlers, template rendering, and version information.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"server/internal/config"
	"server/internal/debug"
	appi18n "server/internal/i18n"
	"server/internal/icons"
	"server/internal/models"
	"server/internal/providers"
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

// --- Security Middleware ---

// SecurityHeaders wraps an http.Handler to add security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' https://fonts.googleapis.com 'unsafe-inline'; font-src 'self' https://fonts.gstatic.com; img-src 'self' https: data:; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// --- HTTP Handlers ---

// ServeHTMLTemplate renders the HTML template with i18n support using go-i18n.
func ServeHTMLTemplate(c *config.TralaConfiguration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := c.GetLanguage()

		// Create a localizer for the selected language
		localizer := appi18n.GetLocalizer(lang)

		// Set the response content type and execute the pre-parsed template
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
}

// ServicesHandler is the main API endpoint. It fetches, processes, and returns all service data.
func ServicesHandler(c *config.TralaConfiguration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		instances := c.GetTraefikInstances()
		var allServices []models.Service

		for _, instance := range instances {
			provider := providers.NewTraefikProvider(instance)
			services, err := provider.FetchServices(r.Context())
			if err != nil {
				log.Printf("WARNING: Failed to fetch services from instance %s: %v", instance.Name, err)
				continue
			}
			for _, svc := range services {
				allServices = append(allServices, models.Service{
					Name:     svc.Name,
					URL:      svc.URL,
					Priority: svc.Priority,
					Icon:     svc.Icon,
					Tags:     svc.Tags,
					Host:     instance.Name,
				})
			}
		}

		manualServices := services.GetManualServices()
		finalServices := make([]models.Service, 0, len(allServices)+len(manualServices))
		finalServices = append(finalServices, allServices...)
		finalServices = append(finalServices, manualServices...)

		finalServices = services.CalculateGroups(finalServices)

		sort.Slice(finalServices, func(i, j int) bool {
			return finalServices[i].Priority > finalServices[j].Priority
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(finalServices)
	}
}

// HealthHandler performs health checks and returns the status.
func HealthHandler(c *config.TralaConfiguration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		instances := c.GetTraefikInstances()

		if len(instances) == 0 {
			http.Error(w, "No Traefik instances configured", http.StatusInternalServerError)
			return
		}

		searchEngineURL := c.GetSearchEngineURL()
		selfhstIconURL := c.GetSelfhstIconURL()

		if !config.IsValidUrl(searchEngineURL) {
			http.Error(w, "Search Engine URL is invalid", http.StatusInternalServerError)
			return
		}

		if !config.IsValidUrl(selfhstIconURL) {
			http.Error(w, "Selfhst Icon URL is invalid", http.StatusInternalServerError)
			return
		}

		// One shared client per insecure-skip-verify setting, reused across instances.
		clients := map[bool]*http.Client{}
		getClient := func(skip bool) *http.Client {
			if clients[skip] == nil {
				clients[skip] = traefik.CreateHTTPClientForInstance(skip)
			}
			return clients[skip]
		}

		var failedInstances []string
		for _, instance := range instances {
			entryPointsURL := fmt.Sprintf("%s/api/entrypoints", instance.APIHost)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := traefik.CreateAndExecuteHTTPRequestWithInstance(ctx, getClient(instance.InsecureSkipVerify), "GET", entryPointsURL, instance)
			cancel()
			if err != nil {
				failedInstances = append(failedInstances, instance.Name)
				log.Printf("WARNING: Health check failed for Traefik instance %s: %v", instance.Name, err)
			}
		}

		if len(failedInstances) > 0 {
			http.Error(w, fmt.Sprintf("Traefik instances unreachable: %s", strings.Join(failedInstances, ", ")), http.StatusServiceUnavailable)
			return
		}

		fmt.Fprint(w, "OK")
	}
}

// StatusHandler returns combined application status information.
func StatusHandler(c *config.TralaConfiguration) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		versionInfo := GetVersionInfo()
		configStatus := c.GetConfigCompatibilityStatus()
		searchEngineURL := c.GetSearchEngineURL()
		refreshIntervalSeconds := c.GetRefreshIntervalSeconds()

		searchEngineIconURL := ""
		if searchEngineURL != "" {
			serviceName := services.ExtractServiceNameFromURL(searchEngineURL)
			if serviceName != "" {
				displayNameReplaced := strings.ReplaceAll(serviceName, " ", "-")
				reference := icons.ResolveSelfHstReference(displayNameReplaced)
				searchEngineIconURL = icons.FindIcon(serviceName, searchEngineURL, serviceName, reference)
			}
		}

		instances := c.GetTraefikInstances()
		multiHost := len(instances) > 1

		frontendConfig := models.FrontendConfig{
			SearchEngineURL:        searchEngineURL,
			SearchEngineIconURL:    searchEngineIconURL,
			RefreshIntervalSeconds: refreshIntervalSeconds,
			GroupingEnabled:        c.GetGroupingEnabled(),
			GroupingColumns:        c.GetGroupingColumns(),
			MultiHost:              multiHost,
			MixServices:            false,
		}

		status := models.ApplicationStatus{
			Version:  versionInfo,
			Config:   configStatus,
			Frontend: frontendConfig,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

// --- Helper Functions ---

// debugf is a wrapper for the shared debug utility
var debugf = debug.Debugf
