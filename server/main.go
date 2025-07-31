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
	Name     string `json:"name"`
	Rule     string `json:"rule"`
	Service  string `json:"service"`
	Priority int    `json:"priority"`
}

// Service represents the final, processed data sent to the frontend.
type Service struct {
	RouterName string `json:"routerName"`
	URL        string `json:"url"`
	Priority   int    `json:"priority"`
	Icon       string `json:"icon"`
}

// GithubTreeResponse and GithubTreeEntry are for parsing the GitHub API response.
type GithubTreeResponse struct {
	Tree []GithubTreeEntry `json:"tree"`
}
type GithubTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// --- Global Variables & Constants ---

var (
	htmlTemplate      []byte
	htmlOnce          sync.Once
	selfhstIconNames  []string
	selfhstCacheTime  time.Time
	selfhstCacheMux   sync.RWMutex
	overrideConfig    map[string]string
	overrideConfigMux sync.RWMutex
	httpClient        = &http.Client{Timeout: 5 * time.Second}
	logLevel          string
	// Regex to reliably find Host and PathPrefix.
	hostRegex = regexp.MustCompile(`Host\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
	pathRegex = regexp.MustCompile(`PathPrefix\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
)

const selfhstCacheTTL = 1 * time.Hour
const selfhstAPIURL = "https://api.github.com/repos/selfhst/icons/git/trees/caffa4e885cb560daf8299889e8092b2c464edec"
const overrideConfigPath = "/config/icon_overrides.yml"
const defaultIcon = "" // Frontend will use a fallback if icon is empty.

// --- Logging ---

// debugf logs a message only if LOG_LEVEL is set to "debug".
func debugf(format string, v ...interface{}) {
	if logLevel == "debug" {
		log.Printf("DEBUG: "+format, v...)
	}
}

// --- Config & Template Loading ---

// loadHTMLTemplate reads the index.html file into memory once.
func loadHTMLTemplate(staticPath string) {
	htmlOnce.Do(func() {
		var err error
		templatePath := filepath.Join(staticPath, "index.html")
		htmlTemplate, err = os.ReadFile(templatePath)
		if err != nil {
			log.Fatalf("FATAL: Could not read index.html template at %s: %v", templatePath, err)
		}
	})
}

// loadOverrides reads and parses the optional icon_overrides.yml file.
func loadOverrides() {
	overrideConfigMux.Lock()
	defer overrideConfigMux.Unlock()

	data, err := os.ReadFile(overrideConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Info: No override config file found at %s. Continuing without overrides.", overrideConfigPath)
			overrideConfig = make(map[string]string) // Initialize empty map
		} else {
			log.Printf("Warning: Could not read override config file at %s: %v", overrideConfigPath, err)
		}
		return
	}

	var config map[string]string
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("Warning: Could not parse override config file %s: %v", overrideConfigPath, err)
		return
	}

	overrideConfig = config
	log.Printf("Successfully loaded %d icon overrides from %s", len(overrideConfig), overrideConfigPath)
}

// --- Main HTTP Handlers ---

// serveHTMLTemplate serves the static index.html file, injecting environment variables.
func serveHTMLTemplate(w http.ResponseWriter, r *http.Request) {
	searchURL := os.Getenv("SEARCH_ENGINE_URL")
	if searchURL == "" {
		searchURL = "https://www.google.com/search?q="
	}
	refreshInterval := os.Getenv("REFRESH_INTERVAL_SECONDS")
	if refreshInterval == "" {
		refreshInterval = "30"
	}
	replacer := strings.NewReplacer(
		"%%SEARCH_ENGINE_URL%%", searchURL,
		"%%REFRESH_INTERVAL_SECONDS%%", refreshInterval,
	)
	replacedHTML := replacer.Replace(string(htmlTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(replacedHTML))
}

// servicesHandler is the main API endpoint. It fetches, processes, and returns all service data.
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Fetch routers from Traefik API.
	traefikAPIHost := os.Getenv("TRAEFIK_API_HOST")
	if traefikAPIHost == "" {
		http.Error(w, "TRAEFIK_API_HOST environment variable not set", http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(traefikAPIHost, "http") {
		traefikAPIHost = "http://" + traefikAPIHost
	}

	traefikURL := fmt.Sprintf("%s/api/http/routers", traefikAPIHost)
	debugf("Fetching routers from Traefik API: %s", traefikURL)

	resp, err := httpClient.Get(traefikURL)
	if err != nil {
		log.Printf("ERROR: Could not fetch from Traefik API: %v", err)
		http.Error(w, "Could not connect to Traefik API", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Traefik API returned non-200 status: %s", resp.Status)
		http.Error(w, "Received non-200 status from Traefik API", http.StatusBadGateway)
		return
	}

	var routers []TraefikRouter
	if err := json.NewDecoder(resp.Body).Decode(&routers); err != nil {
		log.Printf("ERROR: Could not decode Traefik API response: %v", err)
		http.Error(w, "Invalid JSON from Traefik API", http.StatusInternalServerError)
		return
	}
	debugf("Successfully fetched %d routers from Traefik.", len(routers))

	// 2. Process all routers concurrently to find their icons.
	var wg sync.WaitGroup
	serviceChan := make(chan Service, len(routers))

	for _, router := range routers {
		wg.Add(1)
		go func(r TraefikRouter) {
			defer wg.Done()
			processRouter(r, serviceChan)
		}(router)
	}

	wg.Wait()
	close(serviceChan)

	// 3. Collect results and send as JSON.
	finalServices := make([]Service, 0, len(routers))
	for service := range serviceChan {
		finalServices = append(finalServices, service)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalServices)
}

// --- Data Processing & Icon Finding ---

// processRouter takes a raw Traefik router, finds its best icon, and sends the final Service object to a channel.
func processRouter(router TraefikRouter, ch chan<- Service) {
	routerName := strings.Split(router.Name, "@")[0]
	serviceURL := reconstructURL(router.Rule)

	if serviceURL == "" {
		debugf("Could not reconstruct URL for router %s from rule: %s", routerName, router.Rule)
		return
	}

	debugf("Processing router: %s", routerName)
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
	if iconName := checkOverrides(routerName); iconName != "" {
		url := "https://cdn.jsdelivr.net/gh/selfhst/icons/png/" + iconName
		debugf("[%s] Found icon via override: %s", routerName, url)
		return url
	}

	// Priority 2: Fuzzy search against selfh.st icons.
	if iconName := findSelfHstIcon(routerName); iconName != "" {
		url := "https://cdn.jsdelivr.net/gh/selfhst/icons/png/" + iconName
		debugf("[%s] Found icon via fuzzy search: %s", routerName, url)
		return url
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
	overrideConfigMux.RLock()
	defer overrideConfigMux.RUnlock()
	if iconName, ok := overrideConfig[routerName]; ok {
		return iconName
	}
	return ""
}

// findSelfHstIcon performs a fuzzy search.
func findSelfHstIcon(routerName string) string {
	icons, err := getSelfHstIconNames()
	if err != nil {
		log.Printf("ERROR: Could not get selfh.st icon list for fuzzy search: %v", err)
		return ""
	}
	matches := fuzzy.Find(routerName, icons)
	if len(matches) > 0 {
		return matches[0] + ".png"
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

// getSelfHstIconNames fetches the list of icons from the GitHub API and caches it.
func getSelfHstIconNames() ([]string, error) {
	selfhstCacheMux.RLock()
	if time.Since(selfhstCacheTime) < selfhstCacheTTL && len(selfhstIconNames) > 0 {
		selfhstCacheMux.RUnlock()
		return selfhstIconNames, nil
	}
	selfhstCacheMux.RUnlock()

	selfhstCacheMux.Lock()
	defer selfhstCacheMux.Unlock()
	// Double-check after acquiring the lock
	if time.Since(selfhstCacheTime) < selfhstCacheTTL && len(selfhstIconNames) > 0 {
		return selfhstIconNames, nil
	}

	log.Println("Refreshing selfh.st icon cache from GitHub API...")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", selfhstAPIURL, nil)
	req.Header.Set("User-Agent", "TraLa-Dashboard-App")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var treeResponse GithubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&treeResponse); err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range treeResponse.Tree {
		if entry.Type == "blob" && strings.HasSuffix(entry.Path, ".png") && !strings.HasSuffix(entry.Path, "-dark.png") && !strings.HasSuffix(entry.Path, "-light.png") {
			names = append(names, strings.TrimSuffix(entry.Path, ".png"))
		}
	}

	// Sort the icon names using a multi-level approach for the best fuzzy search results.
	// 1. Primary sort: by length (shortest first). This prioritizes base names over variants
	//    (e.g., "proxmox" over "proxmox-helper-scripts").
	// 2. Secondary sort: alphabetically. This provides a stable order for names of the same length.
	sort.Slice(names, func(i, j int) bool {
		lenI := len(names[i])
		lenJ := len(names[j])
		if lenI != lenJ {
			return lenI < lenJ
		}
		return names[i] < names[j]
	})

	selfhstIconNames = names
	selfhstCacheTime = time.Now()
	log.Printf("Successfully cached %d icon names.", len(selfhstIconNames))
	return selfhstIconNames, nil
}

// reconstructURL extracts the base URL from a Traefik rule using regular expressions.
func reconstructURL(rule string) string {
	// Find the hostname using regex. This is more reliable than splitting.
	hostMatches := hostRegex.FindStringSubmatch(rule)
	if len(hostMatches) < 2 {
		return "" // No Host(`...`) found, cannot proceed.
	}
	hostname := hostMatches[1]

	// Find an optional PathPrefix.
	path := ""
	pathMatches := pathRegex.FindStringSubmatch(rule)
	if len(pathMatches) >= 2 {
		path = pathMatches[1]
	}

	// Clean up the path to ensure it's well-formed.
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimSuffix(path, "/")

	return fmt.Sprintf("https://%s%s", hostname, path)
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

// --- Main Application Setup ---
func main() {
	logLevel = os.Getenv("LOG_LEVEL")

	loadOverrides()
	go getSelfHstIconNames() // Pre-warm the cache in the background.

	const staticPath = "static"
	loadHTMLTemplate(staticPath)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/services", servicesHandler)
	mux.HandleFunc("/", serveHTMLTemplate)

	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
