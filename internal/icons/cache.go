// Package icons provides icon discovery and caching functionality for the Trala dashboard.
// This file contains caching logic for SelfHst icons, apps, and user icons.
package icons

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"server/internal/debug"
	"server/internal/models"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// Cache constants
const (
	selfhstCacheTTL     = 1 * time.Hour
	selfhstAppsCacheTTL = 24 * time.Hour
	selfhstAPIURL       = "https://raw.githubusercontent.com/selfhst/icons/refs/heads/main/index.json"
	selfhstAppsURL      = "https://raw.githubusercontent.com/selfhst/cdn/refs/heads/main/directory/integrations/trala.json"
	userIconsDir        = "/icons"
	bundledDataDir      = "/app/data"
)

// loadBundledData reads a bundled JSON file and returns its contents.
func loadBundledData(filename string) ([]byte, error) {
	path := filepath.Join(bundledDataDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundled data %s: %w", path, err)
	}
	return data, nil
}

// sortByReference sorts items by the length of their reference string (shortest first),
// then alphabetically by reference.
func sortByReference[T any](items []T, getRef func(T) string) {
	sort.Slice(items, func(i, j int) bool {
		a, b := getRef(items[i]), getRef(items[j])
		if len(a) != len(b) {
			return len(a) < len(b)
		}
		return a < b
	})
}

// refreshConfig holds the configuration for refreshData.
type refreshConfig[T any] struct {
	Name        string // e.g. "icons" or "apps" for log identification
	URL         string
	BundledFile string
	LogMsg      string
	Data        *[]T
	CacheTime   *time.Time
	CacheMux    *sync.RWMutex
	TTL         time.Duration
	GetRef      func(T) string
	Decode      func(io.Reader, *[]T) error
	SuccessMsg  string
}

// refreshData fetches data from a URL, caches it, and falls back to bundled data on failure.
func refreshData[T any](client *http.Client, cfg refreshConfig[T]) ([]T, error) {
	cfg.CacheMux.RLock()
	if time.Since(*cfg.CacheTime) < cfg.TTL && len(*cfg.Data) > 0 {
		cfg.CacheMux.RUnlock()
		return *cfg.Data, nil
	}
	cfg.CacheMux.RUnlock()

	cfg.CacheMux.Lock()
	defer cfg.CacheMux.Unlock()
	if time.Since(*cfg.CacheTime) < cfg.TTL && len(*cfg.Data) > 0 {
		return *cfg.Data, nil
	}

	log.Println(cfg.LogMsg)
	req, err := http.NewRequestWithContext(context.Background(), "GET", cfg.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "TraLa-Dashboard-App")

	tryBundled := func(msg string) ([]T, error) {
		data, err := loadBundledData(cfg.BundledFile)
		if err != nil {
			return nil, fmt.Errorf("[%s] %s, bundled data also unavailable: %w", cfg.Name, msg, err)
		}
		if err := cfg.Decode(bytes.NewReader(data), cfg.Data); err != nil {
			return nil, fmt.Errorf("[%s] %s, bundled data also unavailable: %w", cfg.Name, msg, err)
		}
		sortByReference(*cfg.Data, cfg.GetRef)
		*cfg.CacheTime = time.Now()
		return *cfg.Data, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] Fetch failed, falling back to bundled data: %v", cfg.Name, err)
		return tryBundled(fmt.Sprintf("live fetch failed (%v)", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] Fetch failed, falling back to bundled data: status %d", cfg.Name, resp.StatusCode)
		return tryBundled(fmt.Sprintf("API returned status %d", resp.StatusCode))
	}

	if err := cfg.Decode(resp.Body, cfg.Data); err != nil {
		return nil, err
	}

	sortByReference(*cfg.Data, cfg.GetRef)
	*cfg.CacheTime = time.Now()
	log.Printf(cfg.SuccessMsg, len(*cfg.Data))
	return *cfg.Data, nil
}

// Cache variables for SelfHst icons
var (
	selfhstIcons     []models.SelfHstIcon
	selfhstCacheTime time.Time
	selfhstCacheMux  sync.RWMutex
)

// Cache variables for SelfHst apps
var (
	selfhstApps          []models.SelfHstApp
	selfhstAppsCacheTime time.Time
	selfhstAppsCacheMux  sync.RWMutex
)

// Cache variables for user icons
var (
	userIcons    map[string]string // Map of icon names to file paths
	userIconsMux sync.RWMutex
	// Sorted user icon names for fuzzy matching
	sortedUserIconNames    []string
	sortedUserIconNamesMux sync.RWMutex
)

// externalHTTPClient is the HTTP client for external calls
var externalHTTPClient *http.Client

// InitHTTPClient initializes the HTTP client used for external icon requests.
// This must be called before using any icon discovery functions.
func InitHTTPClient(client *http.Client) {
	externalHTTPClient = client
}

// GetSelfHstIconNames fetches the list of icons from the selfh.st index.json and caches it.
// Returns cached data if still valid, otherwise fetches fresh data from the API.
func GetSelfHstIconNames() ([]models.SelfHstIcon, error) {
	return refreshData(externalHTTPClient, refreshConfig[models.SelfHstIcon]{
		Name:        "icons",
		URL:         selfhstAPIURL,
		BundledFile: "selfhst-icons.json",
		LogMsg:      "Refreshing selfh.st icon cache from index.json...",
		Data:        &selfhstIcons,
		CacheTime:   &selfhstCacheTime,
		CacheMux:    &selfhstCacheMux,
		TTL:         selfhstCacheTTL,
		GetRef:      func(i models.SelfHstIcon) string { return i.Reference },
		Decode:      func(r io.Reader, t *[]models.SelfHstIcon) error { return json.NewDecoder(r).Decode(t) },
		SuccessMsg:  "Successfully cached %d icons.",
	})
}

// GetSelfHstAppTags fetches the integration data from the selfh.st CDN and caches it.
// Returns cached data if still valid, otherwise fetches fresh data from the API.
func GetSelfHstAppTags() ([]models.SelfHstApp, error) {
	return refreshData(externalHTTPClient, refreshConfig[models.SelfHstApp]{
		Name:        "apps",
		URL:         selfhstAppsURL,
		BundledFile: "selfhst-apps.json",
		LogMsg:      "Refreshing Selfh.st apps cache from trala.json...",
		Data:        &selfhstApps,
		CacheTime:   &selfhstAppsCacheTime,
		CacheMux:    &selfhstAppsCacheMux,
		TTL:         selfhstAppsCacheTTL,
		GetRef:      func(i models.SelfHstApp) string { return i.Reference },
		Decode:      func(r io.Reader, t *[]models.SelfHstApp) error { return json.NewDecoder(r).Decode(t) },
		SuccessMsg:  "Successfully cached %d apps and tags",
	})
}

// ScanUserIcons scans the user icon directory and builds a map of icon names to file paths.
// This function should be called at startup to populate the user icons cache.
func ScanUserIcons() error {
	userIconsMux.Lock()
	defer userIconsMux.Unlock()

	// Initialize the map
	userIcons = make(map[string]string)

	// Check if the directory exists
	if _, err := os.Stat(userIconsDir); os.IsNotExist(err) {
		debugf("User icons directory does not exist: %s", userIconsDir)
		return nil
	}

	log.Println("Scanning user icons directory...")

	// Walk the directory to find all image files
	err := filepath.Walk(userIconsDir, func(path string, info os.FileInfo, err error) error {
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

// FindUserIcon performs a fuzzy search against user icons.
// Returns the file path of the best matching icon, or empty string if no match found.
func FindUserIcon(routerName string) string {
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

// debugf is a wrapper for the shared debug utility
var debugf = debug.Debugf

// SetDebugMode is deprecated and has no effect.
// Debug logging is now controlled by the debug package via config.GetLogLevel().
// This function is kept for backward compatibility with any external callers.
func SetDebugMode(enabled bool) {
	// No-op: debug level is now determined by the debug package
	// This function is kept for backward compatibility with any external callers
	_ = enabled // Suppress unused parameter warning
}
