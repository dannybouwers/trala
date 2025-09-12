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
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"gopkg.in/yaml.v3"
)

// --- Structs ---

// TraefikRouter represents the essential fields from the Traefik API response.
type TraefikRouter struct {
	Name     string           `json:"name"`
	Rule     string           `json:"rule"`
	Service  string           `json:"service"`
	Priority int              `json:"priority"`
	Using    []string         `json:"using"`         // Added to determine the entrypoint
	TLS      *json.RawMessage `json:"tls,omitempty"` // Added to capture TLS configuration
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
	RouterName string `json:"routerName"`
	URL        string `json:"url"`
	Priority   int    `json:"priority"`
	Icon       string `json:"icon"`
}

type TraefikConfig struct {
	APIHost string `yaml:"api_host"`
}

type IconOverride struct {
	Service string `yaml:"service"`
	Icon    string `yaml:"icon"`
}

type IconConfiguration struct {
	Overrides []IconOverride `yaml:"overrides"`
}

type ServiceConfiguration struct {
	Exclude []string `yaml:"exclude"`
}

type EnvironmentConfiguration struct {
	SelfhstIconURL         string        `yaml:"selfhst_icon_url"`
	SearchEngineURL        string        `yaml:"search_engine_url"`
	RefreshIntervalSeconds int           `yaml:"refresh_interval_seconds"`
	LogLevel               string        `yaml:"log_level"`
	Traefik                TraefikConfig `yaml:"traefik"`
}

type TralaConfiguration struct {
	Version     string                   `yaml:"version"`
	Environment EnvironmentConfiguration `yaml:"environment"`
	Icons       IconConfiguration        `yaml:"icons"`
	Services    ServiceConfiguration     `yaml:"services"`
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
	selfhstIcons     []SelfHstIcon
	selfhstCacheTime time.Time
	selfhstCacheMux  sync.RWMutex
	configuration    TralaConfiguration
	// Map used to quickly map a router name to a given icon override
	iconOverrideMap  map[string]IconOverride
	configurationMux sync.RWMutex
	httpClient       = &http.Client{Timeout: 5 * time.Second}
	// Regex to reliably find Host and PathPrefix.
	hostRegex = regexp.MustCompile(`Host\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
	pathRegex = regexp.MustCompile(`PathPrefix\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
)

const selfhstCacheTTL = 1 * time.Hour
const selfhstAPIURL = "https://raw.githubusercontent.com/selfhst/icons/refs/heads/main/index.json"
const configurationFilePath = "/config/configuration.yml"
const defaultIcon = "" // Frontend will use a fallback if icon is empty.

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
	})
}

// --- Main HTTP Handlers ---

// serveHTMLTemplate serves the static index.html file, injecting environment variables.
func serveHTMLTemplate(w http.ResponseWriter, r *http.Request) {
	replacer := strings.NewReplacer(
		"%%SEARCH_ENGINE_URL%%", configuration.Environment.SearchEngineURL,
		"%%REFRESH_INTERVAL_SECONDS%%", strconv.Itoa(configuration.Environment.RefreshIntervalSeconds),
	)
	replacedHTML := replacer.Replace(string(htmlTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(replacedHTML))
}

// servicesHandler is the main API endpoint. It fetches, processes, and returns all service data.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch entrypoints from the Traefik API.
	entryPointsURL := fmt.Sprintf("%s/api/entrypoints", configuration.Environment.Traefik.APIHost)
	debugf("Fetching entrypoints from Traefik API: %s", entryPointsURL)
	resp, err := httpClient.Get(entryPointsURL)
	if err != nil {
		log.Printf("ERROR: Could not fetch entrypoints from Traefik API: %v", err)
		http.Error(w, "Could not connect to Traefik API to get entrypoints", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Traefik Entrypoints API returned non-200 status: %s", resp.Status)
		http.Error(w, "Received non-200 status from Traefik Entrypoints API", http.StatusBadGateway)
		return
	}

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

	resp, err = httpClient.Get(routersURL)
	if err != nil {
		log.Printf("ERROR: Could not fetch routers from Traefik API: %v", err)
		http.Error(w, "Could not connect to Traefik API to get routers", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Traefik Routers API returned non-200 status: %s", resp.Status)
		http.Error(w, "Received non-200 status from Traefik Routers API", http.StatusBadGateway)
		return
	}

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

	// 5. Collect results and send as JSON.
	finalServices := make([]Service, 0, len(routers))
	for service := range serviceChan {
		finalServices = append(finalServices, service)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalServices)
}

// --- Data Processing & Icon Finding ---

// processRouter takes a raw Traefik router, finds its best icon, and sends the final Service object to a channel.
func processRouter(router TraefikRouter, entryPoints map[string]TraefikEntryPoint, ch chan<- Service) {
	routerName := strings.Split(router.Name, "@")[0]

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

	debugf("Processing router: %s, URL: %s", routerName, serviceURL)
	iconURL := findBestIconURL(routerName, serviceURL)

	ch <- Service{
		RouterName: routerName,
		URL:        serviceURL,
		Priority:   router.Priority,
		Icon:       iconURL,
	}
}

// findBestIconURL tries all icon-finding methods in order of priority.
func findBestIconURL(routerName, serviceURL string) string {
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

	// Priority 2: Fuzzy search against selfh.st icons
	routerNameReplaced := strings.ReplaceAll(routerName, " ", "-")
	if iconURL := findSelfHstIcon(routerNameReplaced); iconURL != "" {
		debugf("[%s] Found icon via fuzzy search: %s", routerNameReplaced, iconURL)
		return iconURL
	}

	// Priority 3: Check for /favicon.ico.
	if iconURL := findFavicon(serviceURL); iconURL != "" {
		debugf("[%s] Found icon via /favicon.ico: %s", routerName, iconURL)
		return iconURL
	}

	// Priority 4: Parse service's HTML for a <link> tag.
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

	if override, ok := iconOverrideMap[routerName]; ok {
		return override.Icon
	}
	return ""
}

// isExcluded checks if a router name is in the exclude list.
func isExcluded(routerName string) bool {
	configurationMux.RLock()
	defer configurationMux.RUnlock()

	for _, exclude := range configuration.Services.Exclude {
		if exclude == routerName {
			return true
		}
	}
	return false
}

// findSelfHstIcon performs a fuzzy search.
func findSelfHstIcon(routerName string) string {
	icons, err := getSelfHstIconNames()
	if err != nil {
		log.Printf("ERROR: Could not get selfh.st icon list for fuzzy search: %v", err)
		return ""
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
					return fmt.Sprintf(configuration.Environment.SelfhstIconURL+"svg/%s.svg", icon.Reference)
				}
				// Fallback to PNG
				return fmt.Sprintf(configuration.Environment.SelfhstIconURL+"png/%s.png", icon.Reference)
			}
		}
	}
	return ""
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
	if len(router.Using) == 0 {
		debugf("[%s] Router has no 'using' entrypoints defined. Cannot determine URL.", router.Name)
		return ""
	}
	entryPointName := router.Using[0] // Use the first specified entrypoint
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

func loadConfiguration() {
	configurationMux.Lock()
	defer configurationMux.Unlock()

	// Step 1: defaults
	config := TralaConfiguration{
		Version: "1.0",
		Environment: EnvironmentConfiguration{
			SelfhstIconURL:         "https://cdn.jsdelivr.net/gh/selfhst/icons/",
			SearchEngineURL:        "https://www.google.com/search?q=",
			RefreshIntervalSeconds: 30,
			LogLevel:               "info",
			Traefik: TraefikConfig{
				APIHost: "",
			},
		},
		Icons: IconConfiguration{
			Overrides: make([]IconOverride, 0),
		},
		Services: ServiceConfiguration{
			Exclude: make([]string, 0),
		},
	}

	// Step 2: configuration file
	data, err := os.ReadFile(configurationFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Info: No configuration file found at %s. Using defaults + env vars.", configurationFilePath)
		} else {
			log.Printf("Warning: Could not read configuration file at %s: %v", configurationFilePath, err)
		}
	} else {
		if err := yaml.Unmarshal(data, &config); err != nil {
			log.Printf("Warning: Could not parse configuration file %s: %v", configurationFilePath, err)
		}
	}

	// Step 3: environment overrides
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
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		config.Environment.LogLevel = v
	}

	// Step 4: post-processing / validation
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

	// Build map that maps a router name to a IconOverride for fast lookups
	iconOverrideMap = make(map[string]IconOverride, len(config.Icons.Overrides))
	for _, o := range config.Icons.Overrides {
		iconOverrideMap[o.Service] = o
	}

	log.Printf("Loaded %d service excludes from %s", len(config.Services.Exclude), configurationFilePath)
	log.Printf("Loaded %d icon overrides from %s", len(config.Icons.Overrides), configurationFilePath)

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
	go getSelfHstIconNames() // Pre-warm the cache in the background.

	const templatePath = "template"
	loadHTMLTemplate(templatePath)

	const staticPath = "static"

	mux := http.NewServeMux()
	mux.HandleFunc("/api/services", servicesHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticPath))))
	mux.HandleFunc("/", serveHTMLTemplate)

	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
