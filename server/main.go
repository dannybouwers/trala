package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Version information set at build time
var (
	version   string
	commit    string
	buildTime string
	bundle    *i18n.Bundle
	localizer *i18n.Localizer
)

// Minimum supported configuration version
const minimumConfigVersion = "2.0"

// --- Structs ---

// SelfHstSoftware represents an entry in the selfh.st software database
type SelfHstSoftware struct {
	ID        string `json:"0"` // Index 0
	Name      string `json:"1"` // Index 1
	Reference string `json:"2"` // Index 2
	// ... other fields
	TagIndices string `json:"18"` // Index 18 - comma-separated tag indices
}

// SelfHstTag represents a tag in the selfh.st tags database
type SelfHstTag struct {
	Tag        string   `json:"Tag"`
	ValidTypes []string `json:"Valid Types"`
}

// ServiceCategory represents category information for a service
type ServiceCategory struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	IsManual    bool   `json:"isManual"`
}

// CategorizationConfig represents categorization settings
type CategorizationConfig struct {
	Enabled            bool    `yaml:"enabled"`
	ExcludeCommonTags  bool    `yaml:"exclude_common_tags"`
	CommonTagThreshold float64 `yaml:"common_tag_threshold"`
	DefaultViewMode    string  `yaml:"default_view_mode"`
}

// TraefikRouter represents the essential fields from the Traefik API response.
type TraefikRouter struct {
	Name        string           `json:"name"`
	Rule        string           `json:"rule"`
	Service     string           `json:"service"`
	Priority    int              `json:"priority"`
	EntryPoints []string         `json:"entryPoints"`   // Added to determine the entrypoint
	TLS         *json.RawMessage `json:"tls,omitempty"` // Added to capture TLS configuration
}

// TraefikEntryPoint represents the essential fields from the Traefik Entrypoints API.
type TraefikEntryPoint struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	HTTP    struct {
		TLS json.RawMessage `json:"tls"` // Use RawMessage to check for the presence of TLS configuration
	} `json:"http"`
}

// Service represents the final, processed data sent to the frontend.
type Service struct {
	Name     string           `json:"Name"`
	URL      string           `json:"url"`
	Priority int              `json:"priority"`
	Icon     string           `json:"icon"`
	Category *ServiceCategory `json:"category,omitempty"`
}

// VersionInfo represents the application version information
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

// ConfigStatus represents the configuration compatibility status
type ConfigStatus struct {
	ConfigVersion          string `json:"configVersion"`
	MinimumRequiredVersion string `json:"minimumRequiredVersion"`
	IsCompatible           bool   `json:"isCompatible"`
	WarningMessage         string `json:"warningMessage,omitempty"`
}

type TraefikBasicAuth struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

type TraefikConfig struct {
	APIHost         string           `yaml:"api_host"`
	EnableBasicAuth bool             `yaml:"enable_basic_auth"`
	BasicAuth       TraefikBasicAuth `yaml:"basic_auth"`
}

type ServiceOverride struct {
	Service     string `yaml:"service"`
	DisplayName string `yaml:"display_name,omitempty"`
	Icon        string `yaml:"icon,omitempty"`
	Category    string `yaml:"category,omitempty"`
}

type ManualService struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Icon     string `yaml:"icon,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
	Category string `yaml:"category,omitempty"`
}

type ServiceConfiguration struct {
	Exclude   []string          `yaml:"exclude"`
	Overrides []ServiceOverride `yaml:"overrides"`
	Manual    []ManualService   `yaml:"manual"`
}

type EnvironmentConfiguration struct {
	SelfhstIconURL         string        `yaml:"selfhst_icon_url"`
	SearchEngineURL        string        `yaml:"search_engine_url"`
	RefreshIntervalSeconds int           `yaml:"refresh_interval_seconds"`
	LogLevel               string        `yaml:"log_level"`
	Traefik                TraefikConfig `yaml:"traefik"`
	Language               string        `yaml:"language"`
}

type TralaConfiguration struct {
	Version        string                   `yaml:"version"`
	Environment    EnvironmentConfiguration `yaml:"environment"`
	Services       ServiceConfiguration     `yaml:"services"`
	Categorization CategorizationConfig     `yaml:"categorization"`
}

// FrontendConfig represents the configuration data sent to the frontend
type FrontendConfig struct {
	SearchEngineURL        string               `json:"searchEngineURL"`
	SearchEngineIconURL    string               `json:"searchEngineIconURL"`
	RefreshIntervalSeconds int                  `json:"refreshIntervalSeconds"`
	Categorization         CategorizationConfig `json:"categorization"`
}

// ApplicationStatus represents the combined status information for the application
type ApplicationStatus struct {
	Version  VersionInfo    `json:"version"`
	Config   ConfigStatus   `json:"config"`
	Frontend FrontendConfig `json:"frontend"`
}

// SelfHstIcon represents an entry in the selfh.st icons index.json.
type SelfHstIcon struct {
	Name      string `json:"Name"`
	Reference string `json:"Reference"`
	SVG       string `json:"SVG"`
	PNG       string `json:"PNG"`
	WebP      string `json:"WebP"`
	Light     string `json:"Light"`
	Dark      string `json:"Dark"`
	Category  string `json:"Category"`
	Tags      string `json:"Tags"`
	CreatedAt string `json:"CreatedAt"`
}

// --- Global Variables & Constants ---

var (
	htmlTemplate     []byte
	htmlOnce         sync.Once
	parsedTemplate   *template.Template
	selfhstIcons     []SelfHstIcon
	selfhstCacheTime time.Time
	selfhstCacheMux  sync.RWMutex
	configuration    TralaConfiguration
	// Map used to quickly map a router name to a given service override
	serviceOverrideMap map[string]ServiceOverride
	configurationMux   sync.RWMutex
	httpClient         = &http.Client{Timeout: 5 * time.Second}
	// Regex to reliably find Host and PathPrefix.
	hostRegex = regexp.MustCompile(`Host\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
	pathRegex = regexp.MustCompile(`PathPrefix\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
	// User icons
	userIcons    map[string]string // Map of icon names to file paths
	userIconsMux sync.RWMutex
	// Sorted user icon names for fuzzy matching
	sortedUserIconNames    []string
	sortedUserIconNamesMux sync.RWMutex

	// Categorization system variables
	selfhstSoftware      []SelfHstSoftware
	selfhstTags          []SelfHstTag
	selfhstDataCacheTime time.Time
	selfhstDataCacheMux  sync.RWMutex
	tagFrequencyMap      map[string]int
	commonTags           map[string]bool
	processedTags        map[string]string // Tag index to name mapping
	serviceCategoryMap   map[string]string
	categoryMutex        sync.RWMutex
)

const selfhstCacheTTL = 1 * time.Hour
const selfhstAPIURL = "https://raw.githubusercontent.com/selfhst/icons/refs/heads/main/index.json"
const selfhstSoftwareAPIURL = "https://raw.githubusercontent.com/selfhst/cdn/refs/heads/main/directory/software.json"
const selfhstTagsAPIURL = "https://raw.githubusercontent.com/selfhst/cdn/refs/heads/main/directory/tags.json"
const selfhstDataCacheTTL = 6 * time.Hour // Longer cache for static data
const commonTagThreshold = 0.8            // 80% threshold for excluding common tags
const configurationFilePath = "/config/configuration.yml"
const defaultIcon = "" // Frontend will use a fallback if icon is empty.
const translationDir = "/app/translations"

// Global variable to track configuration compatibility status
var configCompatibilityStatus ConfigStatus

// --- Logging ---

// debugf logs a message only if LOG_LEVEL is set to "debug".
func debugf(format string, v ...interface{}) {
	if configuration.Environment.LogLevel == "debug" {
		log.Printf("DEBUG: "+format, v...)
	}
}

// --- Config & Template Loading ---

// loadHTMLTemplate reads the index.html file into memory once.
func loadHTMLTemplate(templatePath string) {
	htmlOnce.Do(func() {
		var err error
		templatePath := filepath.Join(templatePath, "index.html")
		htmlTemplate, err = os.ReadFile(templatePath)
		if err != nil {
			log.Fatalf("FATAL: Could not read index.html template at %s: %v", templatePath, err)
		}
		// Parse Template once and register a T function that expects a *i18n.Localizer
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

// --- HTTP Helper Functions ---

// createHTTPRequestWithAuth creates an HTTP request with basic auth if enabled in configuration
func createHTTPRequestWithAuth(method, url string) (*http.Request, error) {
	return createHTTPRequestWithAuthAndContext(context.Background(), method, url)
}

// createHTTPRequestWithAuthAndContext creates an HTTP request with context and basic auth if enabled in configuration
func createHTTPRequestWithAuthAndContext(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	// Set basic auth option if enabled
	if configuration.Environment.Traefik.EnableBasicAuth {
		debugf("Setting basic auth")
		req.SetBasicAuth(configuration.Environment.Traefik.BasicAuth.Username, configuration.Environment.Traefik.BasicAuth.Password)
	}

	return req, nil
}

// createAndExecuteHTTPRequest creates an authenticated HTTP request, executes it, and handles common errors
// Returns the response and error, or writes an HTTP error response and returns nil
func createAndExecuteHTTPRequest(w http.ResponseWriter, method, url string) (*http.Response, error) {
	req, err := createHTTPRequestWithAuth(method, url)
	if err != nil {
		log.Printf("ERROR: Could not create request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("ERROR: Could not fetch from %s: %v", url, err)
		http.Error(w, "Could not connect to API", http.StatusBadGateway)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: API returned non-200 status: %s", resp.Status)
		http.Error(w, "Received non-200 status from API", http.StatusBadGateway)
		resp.Body.Close()
		return nil, fmt.Errorf("non-200 status: %s", resp.Status)
	}

	return resp, nil
}

// createAndExecuteHTTPRequestWithContext creates an authenticated HTTP request with context, executes it, and handles common errors
// Returns the response and error, or writes an HTTP error response and returns nil
func createAndExecuteHTTPRequestWithContext(w http.ResponseWriter, ctx context.Context, method, url string) (*http.Response, error) {
	req, err := createHTTPRequestWithAuthAndContext(ctx, method, url)
	if err != nil {
		log.Printf("ERROR: Could not create request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("ERROR: Could not fetch from %s: %v", url, err)
		http.Error(w, "Could not connect to API", http.StatusBadGateway)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: API returned non-200 status: %s", resp.Status)
		http.Error(w, "Received non-200 status from API", http.StatusBadGateway)
		resp.Body.Close()
		return nil, fmt.Errorf("non-200 status: %s", resp.Status)
	}

	return resp, nil
}

// --- Main HTTP Handlers ---

// serveHTMLTemplate renders the HTML template with i18n support using go-i18n
func serveHTMLTemplate(w http.ResponseWriter, r *http.Request) {
	configurationMux.RLock()
	lang := configuration.Environment.Language
	configurationMux.RUnlock()

	// Create a localizer for the selected language
	localizer := i18n.NewLocalizer(bundle, lang)

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

// initI18n initializes the i18n bundle and loads the appropriate translation file.
// It falls back to English if the desired language file is missing.
func initI18n() {
	const fallbackLang = "en"

	// Get the language from environment configuration
	lang := configuration.Environment.Language
	if lang == "" {
		log.Printf("Language not set - using fallback language: %s", fallbackLang)
		lang = fallbackLang
	}

	// Build the path to the translation file for the selected language
	translationFile := filepath.Join(translationDir, lang+".yaml")
	log.Printf("Attempting to load translation file: %s", translationFile)

	// Check if the translation file exists
	if _, err := os.Stat(translationFile); os.IsNotExist(err) {
		log.Printf("Translation file not found for language '%s': %s", lang, translationFile)

		// Fallback to default language if the desired file is missing
		lang = fallbackLang
		translationFile = filepath.Join(translationDir, lang+".yaml")
		log.Printf("Falling back to default translation file: %s", translationFile)

		// If fallback file is also missing, terminate the application
		if _, err := os.Stat(translationFile); os.IsNotExist(err) {
			log.Fatalf("FATAL: Fallback translation file also not found: %s", translationFile)
			return
		}
	}

	log.Printf("Language set to: %s", lang)

	// Create a new i18n bundle with the selected language
	bundle = i18n.NewBundle(language.Make(lang))

	// Register the YAML unmarshal function to read translation files
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	// Load the translation file into the bundle
	if _, err := bundle.LoadMessageFile(translationFile); err != nil {
		log.Fatalf("Failed to load translation file '%s': %v", translationFile, err)

		// Create a localizer for the current language
		localizer = i18n.NewLocalizer(bundle, lang)
	}
}

// T is a helper function for localization. It takes a message ID and returns the localized string.
// If the localization fails, it returns the message ID as a fallback.
func T(id string) string {
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id})
	if err != nil {
		// If localization fails, return the message ID as a fallback.
		return id
	}
	return msg
}

// servicesHandler is the main API endpoint. It fetches, processes, and returns all service data.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch entrypoints from the Traefik API.
	entryPointsURL := fmt.Sprintf("%s/api/entrypoints", configuration.Environment.Traefik.APIHost)
	debugf("Fetching entrypoints from Traefik API: %s", entryPointsURL)
	resp, err := createAndExecuteHTTPRequest(w, "GET", entryPointsURL)
	if err != nil {
		return // Error already handled by createAndExecuteHTTPRequest
	}
	defer resp.Body.Close()

	var entryPoints []TraefikEntryPoint
	if err := json.NewDecoder(resp.Body).Decode(&entryPoints); err != nil {
		log.Printf("ERROR: Could not decode Traefik Entrypoints API response: %v", err)
		http.Error(w, "Invalid JSON from Traefik Entrypoints API", http.StatusInternalServerError)
		return
	}
	debugf("Successfully fetched %d entrypoints from Traefik.", len(entryPoints))

	// Create a map for faster lookups.
	entryPointsMap := make(map[string]TraefikEntryPoint, len(entryPoints))
	for _, ep := range entryPoints {
		entryPointsMap[ep.Name] = ep
	}

	// 3. Fetch routers from the Traefik API.
	routersURL := fmt.Sprintf("%s/api/http/routers", configuration.Environment.Traefik.APIHost)
	debugf("Fetching routers from Traefik API: %s", routersURL)

	resp, err = createAndExecuteHTTPRequest(w, "GET", routersURL)
	if err != nil {
		return // Error already handled by createAndExecuteHTTPRequest
	}
	defer resp.Body.Close()

	var routers []TraefikRouter
	if err := json.NewDecoder(resp.Body).Decode(&routers); err != nil {
		log.Printf("ERROR: Could not decode Traefik Routers API response: %v", err)
		http.Error(w, "Invalid JSON from Traefik Routers API", http.StatusInternalServerError)
		return
	}
	debugf("Successfully fetched %d routers from Traefik.", len(routers))

	// 4. Process all routers concurrently to find their icons.
	var wg sync.WaitGroup
	serviceChan := make(chan Service, len(routers))

	for _, router := range routers {
		wg.Add(1)
		go func(r TraefikRouter) {
			defer wg.Done()
			processRouter(r, entryPointsMap, serviceChan)
		}(router)
	}

	wg.Wait()
	close(serviceChan)

	// 5. Collect results from Traefik services.
	traefikServices := make([]Service, 0, len(routers))
	for service := range serviceChan {
		traefikServices = append(traefikServices, service)
	}

	// 6. Add manual services
	manualServices := getManualServices()

	// 7. Merge and sort all services by priority
	finalServices := make([]Service, 0, len(traefikServices)+len(manualServices))
	finalServices = append(finalServices, traefikServices...)
	finalServices = append(finalServices, manualServices...)

	// 8. Apply categorization based on user's actual services
	if configuration.Categorization.Enabled {
		finalServices = applyCategorizationToServices(finalServices)
	}

	// Sort by priority (higher priority first)
	sort.Slice(finalServices, func(i, j int) bool {
		return finalServices[i].Priority > finalServices[j].Priority
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalServices)
}

func IsValidUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// healthHandler performs health checks and returns the status
func healthHandler(w http.ResponseWriter, r *http.Request) {

	// Check if the most important configuration (Traefik API host) is valid
	configurationMux.RLock()
	traefikAPIHost := configuration.Environment.Traefik.APIHost
	searchEngineURL := configuration.Environment.SearchEngineURL
	selfhstIconURL := configuration.Environment.SelfhstIconURL
	configurationMux.RUnlock()

	if traefikAPIHost == "" {
		http.Error(w, "Traefik API host is not set", http.StatusInternalServerError)
		return
	}

	// Validate SearchEngineURL
	if !IsValidUrl(searchEngineURL) {
		http.Error(w, "Search Engine URL is invalid", http.StatusInternalServerError)
		return
	}

	// Validate SelfhstIconURL
	if !IsValidUrl(selfhstIconURL) {
		http.Error(w, "Selfhst Icon URL is invalid", http.StatusInternalServerError)
		return
	}

	// Check if Traefik is reachable
	entryPointsURL := fmt.Sprintf("%s/api/entrypoints", traefikAPIHost)

	// Create a context with timeout for the health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create and execute the request with context and auth
	resp, err := createAndExecuteHTTPRequestWithContext(w, ctx, "GET", entryPointsURL)
	if err != nil {
		return // Error already handled by createAndExecuteHTTPRequestWithContext
	}
	defer resp.Body.Close()

	// If we reach here, all checks passed
	fmt.Fprint(w, "OK")
}

// statusHandler returns combined application status information
func statusHandler(w http.ResponseWriter, r *http.Request) {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	// Get version information
	versionInfo := VersionInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
	}

	// Get configuration status (already stored in global variable)
	configStatus := configCompatibilityStatus

	// Get frontend configuration
	searchEngineURL := configuration.Environment.SearchEngineURL
	refreshIntervalSeconds := configuration.Environment.RefreshIntervalSeconds

	// Extract service name from search engine URL and find its icon
	searchEngineIconURL := ""
	if searchEngineURL != "" {
		serviceName := extractServiceNameFromURL(searchEngineURL)
		if serviceName != "" {
			searchEngineIconURL = findBestIconURL(serviceName, searchEngineURL, serviceName)
		}
	}

	frontendConfig := FrontendConfig{
		SearchEngineURL:        searchEngineURL,
		SearchEngineIconURL:    searchEngineIconURL,
		RefreshIntervalSeconds: refreshIntervalSeconds,
		Categorization:         configuration.Categorization,
	}

	// Combine all status information
	status := ApplicationStatus{
		Version:  versionInfo,
		Config:   configStatus,
		Frontend: frontendConfig,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// --- Data Processing & Icon Finding ---

// processRouter takes a raw Traefik router, finds its best icon, and sends the final Service object to a channel.
func processRouter(router TraefikRouter, entryPoints map[string]TraefikEntryPoint, ch chan<- Service) {
	routerName := strings.Split(router.Name, "@")[0]

	// Remove entrypoint name from the beginning of router name (case-insensitive)
	if len(router.EntryPoints) > 0 {
		entryPointName := router.EntryPoints[0]
		// Create the pattern to match: entrypoint name followed by a dash
		prefix := entryPointName + "-"
		// Check if router name starts with the entrypoint name (case-insensitive)
		if strings.HasPrefix(strings.ToLower(routerName), strings.ToLower(prefix)) {
			// Remove the entrypoint prefix
			routerName = routerName[len(prefix):]
			debugf("Removed entrypoint prefix '%s' from router name, new name: '%s'", prefix, routerName)
		}
	}

	serviceURL := reconstructURL(router, entryPoints)

	if serviceURL == "" {
		debugf("Could not reconstruct URL for router %s from rule: %s", routerName, router.Rule)
		return
	}

	// Check if this router should be excluded
	if isExcluded(routerName) {
		debugf("Excluding router: %s", routerName)
		return
	}

	// Check if this is the Traefik API service and exclude it
	traefikAPIHost := configuration.Environment.Traefik.APIHost
	if traefikAPIHost != "" {
		if !strings.HasPrefix(traefikAPIHost, "http") {
			traefikAPIHost = "http://" + traefikAPIHost
		}
		apiURL := traefikAPIHost + "/api"
		if serviceURL == apiURL {
			debugf("Excluding router %s because it's the Traefik API service", routerName)
			return
		}
	}

	// Get display name override if available
	displayName := getDisplayNameOverride(routerName)
	if displayName == "" {
		displayName = routerName
	}

	debugf("Processing router: %s (display: %s), URL: %s", routerName, displayName, serviceURL)
	iconURL := findBestIconURL(routerName, serviceURL, displayName)

	// Get icon reference for categorization
	iconReference := ""
	if iconURL != "" {
		// Extract reference from icon URL if it's a selfh.st icon
		if strings.Contains(iconURL, configuration.Environment.SelfhstIconURL) {
			// Extract reference from URL like ".../svg/radarr.svg" -> "radarr"
			parts := strings.Split(iconURL, "/")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				iconReference = strings.TrimSuffix(lastPart, filepath.Ext(lastPart))
			}
		}
	}

	// Assign category if categorization is enabled
	category := assignServiceCategory(displayName, iconReference)

	ch <- Service{
		Name:     displayName,
		URL:      serviceURL,
		Priority: router.Priority,
		Icon:     iconURL,
		Category: category,
	}
}

// findBestIconURL tries all icon-finding methods in order of priority.
func findBestIconURL(routerName, serviceURL string, displayName string) string {
	displayNameReplaced := strings.ReplaceAll(displayName, " ", "-")

	// Priority 1: Check user-defined overrides.
	if iconValue := checkOverrides(routerName); iconValue != "" {
		// Check if it's a full URL
		if strings.HasPrefix(iconValue, "http://") || strings.HasPrefix(iconValue, "https://") {
			debugf("[%s] Found icon via override (full URL): %s", routerName, iconValue)
			return iconValue
		}

		// Check if it's a filename with valid extension
		ext := filepath.Ext(iconValue)
		if ext == ".png" || ext == ".svg" || ext == ".webp" {
			url := configuration.Environment.SelfhstIconURL + strings.TrimPrefix(ext, ".") + "/" + strings.ToLower(iconValue)
			debugf("[%s] Found icon via override (filename): %s", routerName, url)
			return url
		}

		// Fallback to default behavior if extension is not valid
		url := configuration.Environment.SelfhstIconURL + "png/" + iconValue
		debugf("[%s] Found icon via override (fallback): %s", routerName, url)
		return url
	}

	// Priority 2: Check user icons
	if iconPath := findUserIcon(displayNameReplaced); iconPath != "" {
		// For user icons, we return the URL that can be served by the application
		debugf("[%s] Found icon via user icons (fuzzy search): %s", displayNameReplaced, iconPath)
		return iconPath
	}

	// Priority 3: Fuzzy search against selfh.st icons
	if iconURL, iconReference := findSelfHstIcon(displayNameReplaced); iconURL != "" {
		debugf("[%s] Found icon via fuzzy search: %s (reference: %s)", displayNameReplaced, iconURL, iconReference)
		return iconURL
	}

	// Priority 4: Check for /favicon.ico.
	if iconURL := findFavicon(serviceURL); iconURL != "" {
		debugf("[%s] Found icon via /favicon.ico: %s", routerName, iconURL)
		return iconURL
	}

	// Priority 5: Parse service's HTML for a <link> tag.
	if iconURL := findHTMLIcon(serviceURL); iconURL != "" {
		debugf("[%s] Found icon via HTML parsing: %s", routerName, iconURL)
		return iconURL
	}

	debugf("[%s] No icon found, will use fallback.", routerName)
	return defaultIcon
}

// --- Icon Finding Helper Methods ---

// checkOverrides looks for a router name in the loaded config file.
func checkOverrides(routerName string) string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	if override, ok := serviceOverrideMap[routerName]; ok {
		return override.Icon
	}
	return ""
}

// getDisplayNameOverride looks for a router name in the loaded config file.
func getDisplayNameOverride(routerName string) string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	if override, ok := serviceOverrideMap[routerName]; ok {
		return override.DisplayName
	}
	return ""
}

// isExcluded checks if a router name is in the exclude list.
// Supports wildcard patterns (*, ?) and logs invalid patterns.
func isExcluded(routerName string) bool {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	for _, exclude := range configuration.Services.Exclude {
		match, err := filepath.Match(exclude, routerName)
		if err != nil {
			// Log invalid pattern so it is visible in docker logs
			log.Printf("WARNING: invalid exclude pattern %q: %v", exclude, err)
			continue
		}
		if match {
			return true
		}
	}
	return false
}

// findSelfHstIcon performs a fuzzy search and returns both icon URL and reference
func findSelfHstIcon(routerName string) (string, string) {
	icons, err := getSelfHstIconNames()
	if err != nil {
		log.Printf("ERROR: Could not get selfh.st icon list for fuzzy search: %v", err)
		return "", ""
	}

	// Extract reference names for fuzzy matching
	references := make([]string, len(icons))
	for i, icon := range icons {
		references[i] = icon.Reference
	}

	matches := fuzzy.FindFold(routerName, references)
	if len(matches) > 0 {
		// Find the matching icon to determine the best extension
		for _, icon := range icons {
			if icon.Reference == matches[0] {
				// Prefer SVG if available
				if icon.SVG == "Yes" {
					return fmt.Sprintf(configuration.Environment.SelfhstIconURL+"svg/%s.svg", icon.Reference), icon.Reference
				}
				// Fallback to PNG
				return fmt.Sprintf(configuration.Environment.SelfhstIconURL+"png/%s.png", icon.Reference), icon.Reference
			}
		}
	}
	return "", ""
}

// findFavicon checks for the existence of /favicon.ico.
func findFavicon(serviceURL string) string {
	u, err := url.Parse(serviceURL)
	if err != nil {
		return ""
	}
	faviconURL := fmt.Sprintf("%s://%s/favicon.ico", u.Scheme, u.Host)
	if isValidImageURL(faviconURL) {
		return faviconURL
	}
	return ""
}

// findHTMLIcon fetches and parses the service's HTML.
func findHTMLIcon(serviceURL string) string {
	resp, err := httpClient.Get(serviceURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ""
	}
	selectors := []string{"link[rel='apple-touch-icon']", "link[rel='icon']"}
	for _, selector := range selectors {
		if iconPath, exists := doc.Find(selector).Attr("href"); exists {
			absoluteIconURL, err := resolveURL(serviceURL, iconPath)
			if err == nil && isValidImageURL(absoluteIconURL) {
				return absoluteIconURL
			}
		}
	}
	return ""
}

// isValidImageURL performs a HEAD request to check if a URL points to a valid image.
func isValidImageURL(url string) bool {
	resp, err := httpClient.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	return resp.StatusCode == http.StatusOK && strings.HasPrefix(contentType, "image/")
}

// scanUserIcons scans the user icon directory and builds a map of icon names to file paths
func scanUserIcons() error {
	userIconsMux.Lock()
	defer userIconsMux.Unlock()

	// Initialize the map
	userIcons = make(map[string]string)

	// Check if the directory exists
	if _, err := os.Stat("/icons"); os.IsNotExist(err) {
		debugf("User icons directory does not exist: %s", "/icons")
		return nil
	}

	log.Println("Scanning user icons directory...")

	// Walk the directory to find all image files
	err := filepath.Walk("/icons", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's an image file
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".svg" || ext == ".webp" || ext == ".gif" {
			// Get the base name without extension as the icon name
			iconName := strings.ToLower(strings.TrimSuffix(info.Name(), ext))
			userIcons[iconName] = path
			debugf("Found user icon: %s -> %s", iconName, path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Sort the icons using a multi-level approach for the best fuzzy search results.
	// 1. Primary sort: by length (shortest first). This prioritizes base names over variants
	//    (e.g., "proxmox" over "proxmox-helper-scripts").
	// 2. Secondary sort: alphabetically. This provides a stable order for names of the same length.
	iconNames := make([]string, 0, len(userIcons))
	for name := range userIcons {
		iconNames = append(iconNames, name)
	}
	sort.Slice(iconNames, func(i, j int) bool {
		lenI := len(iconNames[i])
		lenJ := len(iconNames[j])
		if lenI != lenJ {
			return lenI < lenJ
		}
		return iconNames[i] < iconNames[j]
	})

	// Store the sorted icon names in our global variable for use in fuzzy matching
	sortedUserIconNamesMux.Lock()
	sortedUserIconNames = iconNames
	sortedUserIconNamesMux.Unlock()

	log.Printf("Successfully scanned user icons directory. Found %d icons.", len(userIcons))
	return nil
}

// findUserIcon performs a fuzzy search against user icons
func findUserIcon(routerName string) string {
	userIconsMux.RLock()
	defer userIconsMux.RUnlock()

	// If no user icons are loaded, return empty
	if len(userIcons) == 0 {
		return ""
	}

	// Use precomputed sorted icon names for fuzzy matching
	sortedUserIconNamesMux.RLock()
	iconNames := sortedUserIconNames
	sortedUserIconNamesMux.RUnlock()

	// Perform fuzzy search
	matches := fuzzy.FindFold(routerName, iconNames)
	if len(matches) > 0 {
		// Return the path of the best match
		if path, ok := userIcons[matches[0]]; ok {
			// Convert file path to URL that can be served by the application
			// The path will be something like "/icons/myicon.png"
			// We want to serve it from "/icons/myicon.png"
			debugf("[%s] Found user icon via fuzzy search: %s -> %s", routerName, matches[0], path)
			return path
		}
	}

	return ""
}

// --- Categorization Functions ---

// getSelfhstData fetches and caches software and tags data from selfh.st
func getSelfhstData() ([]SelfHstSoftware, []SelfHstTag, error) {
	selfhstDataCacheMux.RLock()
	if time.Since(selfhstDataCacheTime) < selfhstDataCacheTTL && len(selfhstSoftware) > 0 && len(selfhstTags) > 0 {
		selfhstDataCacheMux.RUnlock()
		return selfhstSoftware, selfhstTags, nil
	}
	selfhstDataCacheMux.RUnlock()

	selfhstDataCacheMux.Lock()
	defer selfhstDataCacheMux.Unlock()
	// Double-check after acquiring the lock
	if time.Since(selfhstDataCacheTime) < selfhstDataCacheTTL && len(selfhstSoftware) > 0 && len(selfhstTags) > 0 {
		return selfhstSoftware, selfhstTags, nil
	}

	log.Println("Refreshing selfhst data cache...")

	// Fetch software data
	software, err := fetchSelfhstSoftware()
	if err != nil {
		log.Printf("Warning: Could not fetch selfhst software data: %v", err)
		// Return cached data if available, even if expired
		if len(selfhstSoftware) > 0 && len(selfhstTags) > 0 {
			return selfhstSoftware, selfhstTags, nil
		}
		return nil, nil, err
	}

	// Fetch tags data
	tags, err := fetchSelfhstTags()
	if err != nil {
		log.Printf("Warning: Could not fetch selfhst tags data: %v", err)
		// Return cached data if available, even if expired
		if len(selfhstSoftware) > 0 && len(selfhstTags) > 0 {
			return selfhstSoftware, selfhstTags, nil
		}
		return nil, nil, err
	}

	// Process the data
	if err := processSelfhstData(software, tags); err != nil {
		log.Printf("Warning: Could not process selfhst data: %v", err)
		return nil, nil, err
	}

	selfhstSoftware = software
	selfhstTags = tags
	selfhstDataCacheTime = time.Now()
	log.Printf("Successfully cached %d software entries and %d tags.", len(software), len(tags))

	return selfhstSoftware, selfhstTags, nil
}

// fetchSelfhstSoftware fetches software data from the selfh.st API
func fetchSelfhstSoftware() ([]SelfHstSoftware, error) {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", selfhstSoftwareAPIURL, nil)
	req.Header.Set("User-Agent", "TraLa-Dashboard-App")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status: %s", resp.Status)
	}

	// The software.json is an array of arrays, not objects
	var rawSoftware [][]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawSoftware); err != nil {
		return nil, err
	}

	software := make([]SelfHstSoftware, 0, len(rawSoftware))
	for _, entry := range rawSoftware {
		if len(entry) > 17 { // Ensure we have at least 18 fields (0-17)
			var softwareEntry SelfHstSoftware
			if err := json.Unmarshal(entry[0], &softwareEntry.ID); err == nil {
				if err := json.Unmarshal(entry[1], &softwareEntry.Name); err == nil {
					if err := json.Unmarshal(entry[2], &softwareEntry.Reference); err == nil {
						if err := json.Unmarshal(entry[17], &softwareEntry.TagIndices); err == nil {
							software = append(software, softwareEntry)
						}
					}
				}
			}
		}
	}

	return software, nil
}

// fetchSelfhstTags fetches tags data from the selfh.st API
func fetchSelfhstTags() ([]SelfHstTag, error) {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", selfhstTagsAPIURL, nil)
	req.Header.Set("User-Agent", "TraLa-Dashboard-App")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status: %s", resp.Status)
	}

	var tags []SelfHstTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	return tags, nil
}

// processSelfhstData analyzes tag frequencies and builds mappings
func processSelfhstData(software []SelfHstSoftware, tags []SelfHstTag) error {
	categoryMutex.Lock()
	defer categoryMutex.Unlock()

	debugf("Processing %d software entries and %d tags", len(software), len(tags))

	// Build tag index to name mapping
	processedTags = make(map[string]string)
	for i, tag := range tags {
		// Map numeric index to tag name (since software.json uses numeric indices)
		processedTags[strconv.Itoa(i)] = tag.Tag
		debugf("Tag mapping: %s -> %s", strconv.Itoa(i), tag.Tag)
	}

	// Build service to category mapping (without filtering common tags yet)
	serviceCategoryMap = buildServiceCategoryMap(software, processedTags, make(map[string]bool))

	debugf("Processed %d tags", len(processedTags))
	debugf("Built category mapping for %d services", len(serviceCategoryMap))

	// Log some sample mappings for debugging
	count := 0
	for ref, category := range serviceCategoryMap {
		if count < 5 {
			debugf("Sample mapping: %s -> %s", ref, category)
			count++
		}
	}

	return nil
}

// analyzeTagFrequency counts how many times each tag appears
func analyzeTagFrequency(software []SelfHstSoftware) map[string]int {
	frequency := make(map[string]int)

	for _, sw := range software {
		if sw.TagIndices != "" {
			indices := strings.Split(sw.TagIndices, ",")
			for _, index := range indices {
				index = strings.TrimSpace(index)
				if index != "" {
					frequency[index]++
				}
			}
		}
	}

	return frequency
}

// identifyCommonTags marks tags that appear on more than the threshold percentage of services
func identifyCommonTags(frequency map[string]int, totalServices int) map[string]bool {
	common := make(map[string]bool)

	for tagIndex, count := range frequency {
		percentage := float64(count) / float64(totalServices)
		if percentage > commonTagThreshold {
			common[tagIndex] = true
			debugf("Marked tag %s as common (%.2f%% of services)", tagIndex, percentage*100)
		}
	}

	return common
}

// buildServiceCategoryMap creates a mapping from service reference to category
func buildServiceCategoryMap(software []SelfHstSoftware, tagMap map[string]string, commonTags map[string]bool) map[string]string {
	serviceMap := make(map[string]string)

	for _, sw := range software {
		if sw.TagIndices != "" && sw.Reference != "" {
			indices := strings.Split(sw.TagIndices, ",")
			debugf("Processing software %s with tag indices: %s", sw.Reference, sw.TagIndices)

			// Find the first non-common tag
			for _, index := range indices {
				index = strings.TrimSpace(index)
				if index != "" && !commonTags[index] {
					if tagName, exists := tagMap[index]; exists {
						serviceMap[strings.ToLower(sw.Reference)] = tagName
						debugf("Mapped %s -> %s (tag index: %s)", sw.Reference, tagName, index)
						break
					} else {
						debugf("Tag index %s not found in tag map", index)
					}
				} else if index != "" {
					debugf("Skipping common tag index: %s", index)
				}
			}

			if _, exists := serviceMap[strings.ToLower(sw.Reference)]; !exists {
				debugf("No category mapped for software: %s", sw.Reference)
			}
		}
	}

	return serviceMap
}

// assignServiceCategory determines the category for a service using icon reference
func assignServiceCategory(serviceName string, iconReference string) *ServiceCategory {
	// Check for manual override first
	if category := getManualCategoryOverride(serviceName); category != nil {
		debugf("Using manual category override for %s: %s", serviceName, category.Name)
		return category
	}

	// Check for automatic categorization
	if !configuration.Categorization.Enabled {
		return nil
	}

	categoryMutex.RLock()
	defer categoryMutex.RUnlock()

	debugf("Assigning category for %s (icon reference: %s)", serviceName, iconReference)
	debugf("Available service category map entries: %d", len(serviceCategoryMap))

	// Try exact match using icon reference
	if iconReference != "" {
		if category, exists := serviceCategoryMap[strings.ToLower(iconReference)]; exists {
			debugf("Found exact category match for %s: %s", iconReference, category)
			return &ServiceCategory{
				Name:        category,
				DisplayName: toDisplayName(category),
				IsManual:    false,
			}
		} else {
			debugf("No exact match found for icon reference: %s", iconReference)
		}
	}

	// Try fuzzy match against selfhst software database
	if category := getAutomaticCategory(serviceName); category != nil {
		debugf("Found fuzzy category match for %s: %s", serviceName, category.Name)
		return category
	}

	debugf("No category found for %s", serviceName)
	return nil
}

// getManualCategoryOverride checks for manual category override in configuration
func getManualCategoryOverride(serviceName string) *ServiceCategory {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	if override, ok := serviceOverrideMap[serviceName]; ok && override.Category != "" {
		return &ServiceCategory{
			Name:        override.Category,
			DisplayName: toDisplayName(override.Category),
			IsManual:    true,
		}
	}

	return nil
}

// getManualServiceCategory checks for manual category in manual service configuration
func getManualServiceCategory(serviceName string) *ServiceCategory {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	for _, manualService := range configuration.Services.Manual {
		if manualService.Name == serviceName && manualService.Category != "" {
			return &ServiceCategory{
				Name:        manualService.Category,
				DisplayName: toDisplayName(manualService.Category),
				IsManual:    true,
			}
		}
	}

	return nil
}

// getAutomaticCategory performs fuzzy matching against selfhst database
func getAutomaticCategory(serviceName string) *ServiceCategory {
	software, _, err := getSelfhstData()
	if err != nil {
		debugf("Error getting selfhst data for automatic categorization: %v", err)
		return nil
	}

	debugf("Trying automatic categorization for %s with %d software entries", serviceName, len(software))

	// Create a list of software names for fuzzy matching
	names := make([]string, 0, len(software))
	for _, sw := range software {
		if sw.Reference != "" {
			names = append(names, sw.Reference)
		}
	}

	// Perform fuzzy search
	matches := fuzzy.FindFold(strings.ToLower(serviceName), names)
	debugf("Fuzzy matches for %s: %v", serviceName, matches)

	if len(matches) > 0 {
		// Find the matching software entry
		for _, sw := range software {
			if sw.Reference == matches[0] {
				debugf("Found software match: %s -> %s", matches[0], sw.Reference)
				// Get the category from our pre-built mapping
				if category, exists := serviceCategoryMap[strings.ToLower(sw.Reference)]; exists {
					debugf("Found category for %s: %s", sw.Reference, category)
					return &ServiceCategory{
						Name:        category,
						DisplayName: toDisplayName(category),
						IsManual:    false,
					}
				} else {
					debugf("No category found in mapping for %s", sw.Reference)
				}
				break
			}
		}
	}

	return nil
}

// toDisplayName converts a tag name to a human-readable display name
func toDisplayName(tagName string) string {
	// Convert underscores and hyphens to spaces
	displayName := strings.ReplaceAll(tagName, "_", " ")
	displayName = strings.ReplaceAll(displayName, "-", " ")

	// Capitalize each word
	words := strings.Fields(displayName)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// applyCategorizationToServices analyzes the user's services and applies categorization
func applyCategorizationToServices(services []Service) []Service {
	debugf("Applying categorization to %d user services", len(services))

	// Collect all icon references from user services
	iconReferences := make([]string, 0, len(services))
	for _, service := range services {
		if service.Icon != "" && strings.Contains(service.Icon, configuration.Environment.SelfhstIconURL) {
			// Extract reference from icon URL
			parts := strings.Split(service.Icon, "/")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				reference := strings.TrimSuffix(lastPart, filepath.Ext(lastPart))
				iconReferences = append(iconReferences, reference)
			}
		}
	}

	debugf("Found %d icon references from user services", len(iconReferences))

	// Get selfhst data to analyze tag frequencies for these specific services
	software, tags, err := getSelfhstData()
	if err != nil {
		debugf("Error getting selfhst data for categorization: %v", err)
		return services
	}

	// Build tag index to name mapping
	processedTags := make(map[string]string)
	for i, tag := range tags {
		processedTags[strconv.Itoa(i)] = tag.Tag
	}

	// Analyze tag frequency for user's services only
	userTagFrequency := make(map[string]int)
	totalUserServices := 0

	for _, ref := range iconReferences {
		// Find matching software entry
		for _, sw := range software {
			if strings.EqualFold(sw.Reference, ref) {
				if sw.TagIndices != "" {
					indices := strings.Split(sw.TagIndices, ",")
					for _, index := range indices {
						index = strings.TrimSpace(index)
						if index != "" {
							userTagFrequency[index]++
						}
					}
					totalUserServices++
				}
				break
			}
		}
	}

	debugf("User tag frequency analysis: %v (total services: %d)", userTagFrequency, totalUserServices)

	// Identify common tags based on user's services
	userCommonTags := make(map[string]bool)
	for tagIndex, count := range userTagFrequency {
		percentage := float64(count) / float64(totalUserServices)
		if percentage > configuration.Categorization.CommonTagThreshold {
			userCommonTags[tagIndex] = true
			debugf("Marked tag %s as common for user services (%.2f%% of %d services)",
				tagIndex, percentage*100, totalUserServices)
		}
	}

	// Build service category map for user services
	userServiceCategoryMap := make(map[string]string)
	for _, ref := range iconReferences {
		for _, sw := range software {
			if strings.EqualFold(sw.Reference, ref) && sw.TagIndices != "" {
				indices := strings.Split(sw.TagIndices, ",")

				// Find first non-common tag
				for _, index := range indices {
					index = strings.TrimSpace(index)
					if index != "" && !userCommonTags[index] {
						if tagName, exists := processedTags[index]; exists {
							userServiceCategoryMap[strings.ToLower(ref)] = tagName
							debugf("Mapped user service %s -> %s", ref, tagName)
							break
						}
					}
				}

				// If no non-common tags found, don't assign a category
				if _, exists := userServiceCategoryMap[strings.ToLower(ref)]; !exists {
					debugf("No non-common tags found for %s, no category assigned", ref)
				}
				break
			}
		}
	}

	debugf("Built category mapping for %d user services", len(userServiceCategoryMap))

	// Apply categories to services
	categorizedServices := make([]Service, len(services))
	for i, service := range services {
		categorizedServices[i] = service

		// Get icon reference for this service
		iconReference := ""
		if service.Icon != "" && strings.Contains(service.Icon, configuration.Environment.SelfhstIconURL) {
			parts := strings.Split(service.Icon, "/")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				iconReference = strings.TrimSuffix(lastPart, filepath.Ext(lastPart))
			}
		}

		// Check for manual override first (service overrides take precedence)
		if category := getManualCategoryOverride(service.Name); category != nil {
			categorizedServices[i].Category = category
			continue
		}

		// Check for manual service category (for manual services)
		if category := getManualServiceCategory(service.Name); category != nil {
			categorizedServices[i].Category = category
			continue
		}

		// Apply automatic categorization only if category exists
		if iconReference != "" {
			if category, exists := userServiceCategoryMap[strings.ToLower(iconReference)]; exists {
				categorizedServices[i].Category = &ServiceCategory{
					Name:        category,
					DisplayName: toDisplayName(category),
					IsManual:    false,
				}
				debugf("Applied category %s to service %s", category, service.Name)
			} else {
				debugf("No category found for service %s (icon reference: %s)", service.Name, iconReference)
			}
		}
	}

	return categorizedServices
}

// --- Caching & Utility ---

// getSelfHstIconNames fetches the list of icons from the selfh.st index.json and caches it.
func getSelfHstIconNames() ([]SelfHstIcon, error) {
	selfhstCacheMux.RLock()
	if time.Since(selfhstCacheTime) < selfhstCacheTTL && len(selfhstIcons) > 0 {
		selfhstCacheMux.RUnlock()
		return selfhstIcons, nil
	}
	selfhstCacheMux.RUnlock()

	selfhstCacheMux.Lock()
	defer selfhstCacheMux.Unlock()
	// Double-check after acquiring the lock
	if time.Since(selfhstCacheTime) < selfhstCacheTTL && len(selfhstIcons) > 0 {
		return selfhstIcons, nil
	}

	log.Println("Refreshing selfh.st icon cache from index.json...")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", selfhstAPIURL, nil)
	req.Header.Set("User-Agent", "TraLa-Dashboard-App")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var icons []SelfHstIcon
	if err := json.NewDecoder(resp.Body).Decode(&icons); err != nil {
		return nil, err
	}

	// Sort the icons using a multi-level approach for the best fuzzy search results.
	// 1. Primary sort: by length (shortest first). This prioritizes base names over variants
	//    (e.g., "proxmox" over "proxmox-helper-scripts").
	// 2. Secondary sort: alphabetically. This provides a stable order for names of the same length.
	sort.Slice(icons, func(i, j int) bool {
		lenI := len(icons[i].Reference)
		lenJ := len(icons[j].Reference)
		if lenI != lenJ {
			return lenI < lenJ
		}
		return icons[i].Reference < icons[j].Reference
	})

	selfhstIcons = icons
	selfhstCacheTime = time.Now()
	log.Printf("Successfully cached %d icons.", len(selfhstIcons))
	return selfhstIcons, nil
}

// determineProtocol determines the correct protocol (http/https) for a service
// based on TLS configuration in both router and entrypoint.
func determineProtocol(router TraefikRouter, entryPoint TraefikEntryPoint) string {
	// Primary method: Check router TLS configuration (highest priority)
	// This is the most reliable indicator of whether a service should use HTTPS
	if router.TLS != nil {
		tlsStr := string(*router.TLS)
		// Check for non-empty, non-null TLS configuration
		if tlsStr != "null" && tlsStr != "{}" && tlsStr != "" {
			return "https"
		}
	}

	// Secondary method: Check entrypoint TLS configuration
	// The TLS field is a json.RawMessage, so we need to check various possible values
	if entryPoint.HTTP.TLS != nil {
		tlsStr := string(entryPoint.HTTP.TLS)
		// Check for non-empty, non-null TLS configuration
		if tlsStr != "null" && tlsStr != "{}" && tlsStr != "" {
			return "https"
		}
	}

	// Default to HTTP
	return "http"
}

// reconstructURL extracts the base URL from a Traefik rule and determines the protocol and port
// based on the router's entrypoint.
func reconstructURL(router TraefikRouter, entryPoints map[string]TraefikEntryPoint) string {
	// Find the hostname using regex. This is more reliable than splitting.
	hostMatches := hostRegex.FindStringSubmatch(router.Rule)
	if len(hostMatches) < 2 {
		return "" // No Host(`...`) found, cannot proceed.
	}
	hostname := hostMatches[1]

	// Find an optional PathPrefix.
	path := ""
	pathMatches := pathRegex.FindStringSubmatch(router.Rule)
	if len(pathMatches) >= 2 {
		path = pathMatches[1]
	}

	// Clean up the path.
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimSuffix(path, "/")

	// Determine protocol and port via the entrypoint.
	if len(router.EntryPoints) == 0 {
		debugf("[%s] Router has no entrypoints defined. Cannot determine URL.", router.Name)
		return ""
	}
	entryPointName := router.EntryPoints[0] // Use the first specified entrypoint
	entryPoint, ok := entryPoints[entryPointName]
	if !ok {
		debugf("[%s] Entrypoint '%s' not found in Traefik configuration.", router.Name, entryPointName)
		return ""
	}

	// Use the enhanced protocol detection logic
	protocol := determineProtocol(router, entryPoint)

	// Address is in the format ":port"
	port := strings.TrimPrefix(entryPoint.Address, ":")

	// Omit the port if it's the default for the protocol.
	if (protocol == "http" && port == "80") || (protocol == "https" && port == "443") {
		return fmt.Sprintf("%s://%s%s", protocol, hostname, path)
	}

	return fmt.Sprintf("%s://%s%s:%s", protocol, hostname, path, port)
}

func resolveURL(baseURL string, path string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

// extractServiceNameFromURL extracts the service name from a search engine URL
func extractServiceNameFromURL(searchURL string) string {
	parsedURL, err := url.Parse(searchURL)
	if err != nil {
		return ""
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return ""
	}

	// Remove common TLDs and extract the main domain name
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return hostname
	}

	// Use the second-level domain (e.g., "example" from "www.example.com")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}

	return hostname
}

// getManualServices processes manually configured services and returns them as Service objects
func getManualServices() []Service {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	manualServices := make([]Service, 0, len(configuration.Services.Manual))

	for _, manualService := range configuration.Services.Manual {
		// Validate URL
		if !IsValidUrl(manualService.URL) {
			log.Printf("Warning: Invalid URL for manual service '%s': %s", manualService.Name, manualService.URL)
			continue
		}

		// Find icon using the same logic as for Traefik services
		iconURL := manualService.Icon
		if iconURL == "" {
			// If no icon is specified, try to find one automatically
			iconURL = findBestIconURL(manualService.Name, manualService.URL, manualService.Name)
		} else if !strings.HasPrefix(iconURL, "http://") && !strings.HasPrefix(iconURL, "https://") {
			// If icon is specified, check if it's a full URL or just a filename
			// Check if it's a filename with valid extension
			ext := filepath.Ext(iconURL)
			if ext == ".png" || ext == ".svg" || ext == ".webp" {
				iconURL = configuration.Environment.SelfhstIconURL + strings.TrimPrefix(ext, ".") + "/" + strings.ToLower(iconURL)
			} else {
				// Fallback to default behavior if extension is not valid
				iconURL = configuration.Environment.SelfhstIconURL + "png/" + iconURL
			}
		}

		// Default priority if not specified
		priority := manualService.Priority
		if priority == 0 {
			priority = 50 // Default priority for manual services
		}

		service := Service{
			Name:     manualService.Name,
			URL:      manualService.URL,
			Priority: priority,
			Icon:     iconURL,
		}

		manualServices = append(manualServices, service)
		debugf("Added manual service: %s (URL: %s, Icon: %s, Priority: %d)",
			manualService.Name, manualService.URL, iconURL, priority)
	}

	return manualServices
}

// compareVersions compares two version strings using semantic versioning
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Normalize versions by ensuring they have 3 components (major.minor.patch)
	normalizeVersion := func(v string) []int {
		parts := strings.Split(v, ".")
		result := make([]int, 3)
		for i := 0; i < 3; i++ {
			if i < len(parts) {
				if num, err := strconv.Atoi(parts[i]); err == nil {
					result[i] = num
				}
			}
			// Missing parts default to 0
		}
		return result
	}

	v1Parts := normalizeVersion(v1)
	v2Parts := normalizeVersion(v2)

	for i := 0; i < 3; i++ {
		if v1Parts[i] < v2Parts[i] {
			return -1
		} else if v1Parts[i] > v2Parts[i] {
			return 1
		}
	}
	return 0
}

// validateBasicAuthPassword checks if the basic auth password is configured using only one method
func validateBasicAuthPassword(config TraefikConfig) string {
	// If basic auth is not enabled, no validation needed
	if !config.EnableBasicAuth {
		return ""
	}

	// Count the number of password sources that are set
	passwordSources := 0

	// Check config file password
	if config.BasicAuth.Password != "" {
		passwordSources++
	}

	// Check config file password file
	if config.BasicAuth.PasswordFile != "" {
		passwordSources++
	}

	// Check environment variable password
	if os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD") != "" {
		passwordSources++
	}

	// Check environment variable password file
	if os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD_FILE") != "" {
		passwordSources++
	}

	// If more than one password source is configured, it's a warning
	if passwordSources > 1 {
		return "Basic auth password is configured using multiple methods. Please use only one method: either password in config file, password file, or environment variable."
	}

	return ""
}

// validateConfigVersion checks if the configuration version is compatible
func validateConfigVersion(configVersion string, basicAuthWarning string) ConfigStatus {
	status := ConfigStatus{
		ConfigVersion:          configVersion,
		MinimumRequiredVersion: minimumConfigVersion,
		IsCompatible:           true,
	}

	// Check if configuration version is specified
	if configVersion == "" {
		status.IsCompatible = false
		status.WarningMessage = "No configuration version specified. Please add 'version: X.Y' to your configuration file."
		return status
	}

	// Compare versions
	if compareVersions(configVersion, minimumConfigVersion) < 0 {
		status.IsCompatible = false
		status.WarningMessage = fmt.Sprintf("Configuration version %s is below the minimum required version %s. Some configuration options may be ignored.", configVersion, minimumConfigVersion)
	}

	// Merge with basic auth warning if present
	if basicAuthWarning != "" {
		// If there's already a warning message, append to it
		if status.WarningMessage != "" {
			status.WarningMessage += " " + basicAuthWarning
		} else {
			status.WarningMessage = basicAuthWarning
		}
	}

	return status
}

func loadConfiguration() {
	// Step 1: defaults
	config := TralaConfiguration{
		Version: "",
		Environment: EnvironmentConfiguration{
			SelfhstIconURL:         "https://cdn.jsdelivr.net/gh/selfhst/icons/",
			SearchEngineURL:        "https://www.google.com/search?q=",
			RefreshIntervalSeconds: 30,
			LogLevel:               "info",
			Traefik: TraefikConfig{
				APIHost:         "",
				EnableBasicAuth: false,
				BasicAuth: TraefikBasicAuth{
					Username:     "",
					Password:     "",
					PasswordFile: "",
				},
			},
		},
		Services: ServiceConfiguration{
			Exclude:   make([]string, 0),
			Overrides: make([]ServiceOverride, 0),
			Manual:    make([]ManualService, 0),
		},
		Categorization: CategorizationConfig{
			Enabled:            true, // Enabled by default
			ExcludeCommonTags:  true,
			CommonTagThreshold: 0.9, // Increased to 90% as requested
			DefaultViewMode:    "grouped",
		},
	}

	// Step 2: configuration file
	data, err := os.ReadFile(configurationFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Info: No configuration file found at %s. Using defaults + env vars.", configurationFilePath)
			config.Version = minimumConfigVersion // Set to minimum required if no config file
		} else {
			log.Printf("Warning: Could not read configuration file at %s: %v", configurationFilePath, err)
		}
	} else {
		if err := yaml.Unmarshal(data, &config); err != nil {
			log.Printf("Warning: Could not parse configuration file %s: %v", configurationFilePath, err)
		}
	}

	// Step 3: validate basic auth password configuration before environment overrides
	// This ensures we check both the original config values and environment variables
	basicAuthWarning := validateBasicAuthPassword(config.Environment.Traefik)
	if basicAuthWarning != "" {
		log.Printf("WARNING: %s", basicAuthWarning)
	}

	// Step 4: environment overrides
	if v := os.Getenv("SELFHST_ICON_URL"); v != "" {
		config.Environment.SelfhstIconURL = v
	}
	if v := os.Getenv("SEARCH_ENGINE_URL"); v != "" {
		config.Environment.SearchEngineURL = v
	}
	if v := os.Getenv("REFRESH_INTERVAL_SECONDS"); v != "" {
		if num, err := strconv.Atoi(v); err == nil && num > 0 {
			config.Environment.RefreshIntervalSeconds = num
		} else {
			log.Printf("Warning: Invalid REFRESH_INTERVAL_SECONDS '%s', using %d", v, config.Environment.RefreshIntervalSeconds)
		}
	}
	if v := os.Getenv("TRAEFIK_API_HOST"); v != "" {
		config.Environment.Traefik.APIHost = v
	}
	if v := os.Getenv("TRAEFIK_BASIC_AUTH_USERNAME"); v != "" {
		config.Environment.Traefik.BasicAuth.Username = v
	}
	if v := os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD"); v != "" {
		config.Environment.Traefik.BasicAuth.Password = v
	}
	if v := os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD_FILE"); v != "" {
		config.Environment.Traefik.BasicAuth.PasswordFile = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		config.Environment.LogLevel = v
	}
	if v := os.Getenv("LANGUAGE"); v != "" {
		config.Environment.Language = v
	}

	// Step 5: post-processing / validation
	if config.Environment.Traefik.APIHost == "" {
		log.Printf("ERROR: Traefik API host is not set. Provide via env var or config file.")
		os.Exit(1)
	}
	if !strings.HasPrefix(config.Environment.Traefik.APIHost, "http://") && !strings.HasPrefix(config.Environment.Traefik.APIHost, "https://") {
		config.Environment.Traefik.APIHost = "http://" + config.Environment.Traefik.APIHost
	}
	if !strings.HasSuffix(config.Environment.SelfhstIconURL, "/") {
		config.Environment.SelfhstIconURL += "/"
	}

	if config.Environment.Traefik.EnableBasicAuth {
		if config.Environment.Traefik.BasicAuth.Username == "" || (config.Environment.Traefik.BasicAuth.Password == "" && config.Environment.Traefik.BasicAuth.PasswordFile == "") {
			log.Printf("ERROR: Basic auth is enabled, but basic auth username, password or password file is not set!")
			os.Exit(1)
		}
		if config.Environment.Traefik.BasicAuth.Password != "" && config.Environment.Traefik.BasicAuth.PasswordFile != "" {
			log.Printf("WARNING: Basic auth password and password file is set, content of file will take precedence over password!")
		}
	}

	passwordFilePath := config.Environment.Traefik.BasicAuth.PasswordFile
	if config.Environment.Traefik.EnableBasicAuth && passwordFilePath != "" {
		data, err := os.ReadFile(passwordFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("ERROR: No password file found at %s for basic auth.", passwordFilePath)
				os.Exit(1)
			} else {
				log.Printf("ERROR: Could not read password file at %s: %v", passwordFilePath, err)
				os.Exit(1)
			}
		} else {
			config.Environment.Traefik.BasicAuth.Password = string(data)
		}
	}

	// Build map that maps a router name to a ServiceOverride for fast lookups
	serviceOverrideMap = make(map[string]ServiceOverride, len(config.Services.Overrides))
	for _, o := range config.Services.Overrides {
		serviceOverrideMap[o.Service] = o
	}

	log.Printf("Loaded %d service excludes from %s", len(config.Services.Exclude), configurationFilePath)
	log.Printf("Loaded %d service overrides from %s", len(config.Services.Overrides), configurationFilePath)

	// Validate configuration version (without basic auth validation since we already did it above)
	configCompatibilityStatus = validateConfigVersion(config.Version, basicAuthWarning)
	if !configCompatibilityStatus.IsCompatible {
		log.Printf("WARNING: %s", configCompatibilityStatus.WarningMessage)
	}

	// Now that all validation is complete, lock the mutex and update the global configuration
	configurationMux.Lock()
	defer configurationMux.Unlock()

	configuration = config

	if config.Environment.LogLevel == "debug" {
		debugf("Using effective configuration:")
		out, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Failed to marshal configuration: %v\n", err)
			return
		}
		fmt.Println(string(out))
	}
}

// --- Main Application Setup ---
func main() {
	loadConfiguration()
	initI18n()
	const templatePath = "template"
	loadHTMLTemplate(templatePath)

	const staticPath = "static"

	// Pre-warm the caches in the background
	go getSelfHstIconNames() // Pre-warm the selfh.st icon cache
	go func() {
		if err := scanUserIcons(); err != nil {
			log.Printf("Warning: Could not scan user icons directory: %v", err)
		}
	}() // Pre-warm the user icons cache

	// Initialize categorization system if enabled
	if configuration.Categorization.Enabled {
		go func() {
			if _, _, err := getSelfhstData(); err != nil {
				log.Printf("Warning: Could not initialize categorization system: %v", err)
			} else {
				log.Println("Categorization system initialized successfully")
			}
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/services", servicesHandler)
	mux.HandleFunc("/api/status", statusHandler)
	mux.HandleFunc("/api/health", healthHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))
	mux.Handle("/icons/", http.StripPrefix("/icons/", http.FileServer(http.Dir("/icons"))))
	mux.HandleFunc("/", serveHTMLTemplate)

	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
