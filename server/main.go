package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"gopkg.in/yaml.v3"
)

// --- Global variables & Structs ---
var (
	htmlTemplate    []byte
	htmlOnce        sync.Once
	iconCache       []string
	iconCacheTime   time.Time
	iconCacheMux    sync.RWMutex
	overrideConfig  map[string]string
	overrideConfigMux sync.RWMutex
)

const iconCacheTTL = 1 * time.Hour
const selfhstAPIURL = "https://api.github.com/repos/selfhst/icons/git/trees/caffa4e885cb560daf8299889e8092b2c464edec"
const overrideConfigPath = "/config/icon_overrides.yml"

type GithubTreeResponse struct {
	Tree []GithubTreeEntry `json:"tree"`
}
type GithubTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// --- Config & Template Loading ---

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

// --- Icon Proxy & Fetching Logic ---

func getSelfHstIcons() ([]string, error) {
	iconCacheMux.RLock()
	if time.Since(iconCacheTime) < iconCacheTTL && len(iconCache) > 0 {
		iconCacheMux.RUnlock()
		return iconCache, nil
	}
	iconCacheMux.RUnlock()

	iconCacheMux.Lock()
	defer iconCacheMux.Unlock()
	if time.Since(iconCacheTime) < iconCacheTTL && len(iconCache) > 0 {
		return iconCache, nil
	}

	log.Println("Refreshing selfh.st icon cache from GitHub API...")
	req, _ := http.NewRequest("GET", selfhstAPIURL, nil)
	req.Header.Set("User-Agent", "TraLa-Dashboard-App")
	
	client := &http.Client{}
	resp, err := client.Do(req)
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

	iconCache = names
	iconCacheTime = time.Now()
	log.Printf("Successfully cached %d icon names.", len(iconCache))
	return iconCache, nil
}

func overrideHandler(w http.ResponseWriter, r *http.Request) {
	routerName := r.URL.Query().Get("routerName")
	if routerName == "" {
		http.Error(w, "Missing 'routerName' query parameter", http.StatusBadRequest)
		return
	}

	overrideConfigMux.RLock()
	defer overrideConfigMux.RUnlock()

	if iconName, ok := overrideConfig[routerName]; ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"iconName": iconName})
		return
	}

	http.NotFound(w, r)
}

func selfhstHandler(w http.ResponseWriter, r *http.Request) {
	routerName := r.URL.Query().Get("routerName")
	if routerName == "" {
		http.Error(w, "Missing 'routerName' query parameter", http.StatusBadRequest)
		return
	}

	icons, err := getSelfHstIcons()
	if err != nil {
		http.Error(w, "Could not fetch icon list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	match := fuzzy.Find(routerName, icons)
	if len(match) == 0 {
		http.NotFound(w, r)
		return
	}

	bestMatch := match[0] 
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"iconName": bestMatch + ".png"})
}

func faviconProxyHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}
	faviconURL, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	faviconURL.Path = "/favicon.ico"
	resp, err := http.Head(faviconURL.String())
	if err != nil || resp.StatusCode != http.StatusOK {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"iconUrl": faviconURL.String()})
}


type IconResponse struct {
	IconURL string `json:"iconUrl"`
}

func findIconHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}
	res, err := http.Get(targetURL)
	if err != nil {
		http.Error(w, "Failed to fetch target URL: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		http.Error(w, "Target URL returned non-200 status: "+res.Status, http.StatusBadGateway)
		return
	}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		http.Error(w, "Failed to parse HTML: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var iconPath string
	selectors := []string{"link[rel='apple-touch-icon']", "link[rel='icon']"}
	for _, selector := range selectors {
		doc.Find(selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
			if href, exists := s.Attr("href"); exists {
				iconPath = href
				return false
			}
			return true
		})
		if iconPath != "" {
			break
		}
	}
	if iconPath == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(IconResponse{IconURL: ""})
		return
	}
	absoluteIconURL, err := resolveURL(targetURL, iconPath)
	if err != nil {
		http.Error(w, "Failed to resolve icon URL: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(IconResponse{IconURL: absoluteIconURL})
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

// --- Traefik API Proxy Logic ---
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func newTraefikProxy(targetHost string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetHost)
	if err != nil {
		return nil, err
	}
	proxy := &httputil.ReverseProxy{}
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		req.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
	}
	return proxy, nil
}

// --- Main Application ---
func main() {
	traefikAPIHost := os.Getenv("TRAEFIK_API_HOST")
	if traefikAPIHost == "" {
		log.Fatal("TRAEFIK_API_HOST environment variable not set.")
	}
	traefikProxy, err := newTraefikProxy(traefikAPIHost)
	if err != nil {
		log.Fatalf("Failed to create Traefik proxy: %v", err)
	}
	
	loadOverrides()

	const staticPath = "static"
	loadHTMLTemplate(staticPath)
	
	mux := http.NewServeMux()
	apiProxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming API request, proxying to Traefik: %s %s", r.Method, r.URL.Path)
		traefikProxy.ServeHTTP(w, r)
	})

	mux.Handle("/api/", apiProxyHandler)
	mux.HandleFunc("/proxy/icon-override", overrideHandler)
	mux.HandleFunc("/proxy/selfhst-icons", selfhstHandler)
	mux.HandleFunc("/proxy/favicon", faviconProxyHandler)
	mux.HandleFunc("/proxy/icon", findIconHandler)
	mux.HandleFunc("/", serveHTMLTemplate)
	
	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
