// Package icons provides icon discovery and caching functionality for the Trala dashboard.
// This file contains caching logic for SelfHst icons, apps, and user icons.
package icons

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

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
)

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

	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var icons []models.SelfHstIcon
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

// GetSelfHstAppTags fetches the integration data from the selfhst CDN and caches it.
// Returns cached data if still valid, otherwise fetches fresh data from the API.
func GetSelfHstAppTags() ([]models.SelfHstApp, error) {
	selfhstAppsCacheMux.RLock()
	if time.Since(selfhstAppsCacheTime) < selfhstAppsCacheTTL && len(selfhstApps) > 0 {
		selfhstAppsCacheMux.RUnlock()
		return selfhstApps, nil
	}
	selfhstAppsCacheMux.RUnlock()

	selfhstAppsCacheMux.Lock()
	defer selfhstAppsCacheMux.Unlock()
	// Double-check after acquiring the lock
	if time.Since(selfhstAppsCacheTime) < selfhstAppsCacheTTL && len(selfhstApps) > 0 {
		return selfhstApps, nil
	}

	log.Println("Refreshing Selfh.st apps cache from trala.json...")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", selfhstAppsURL, nil)
	req.Header.Set("User-Agent", "TraLa-Dashboard-App")

	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data []models.SelfHstApp
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Sort the apps using a multi-level approach for the best fuzzy search results.
	// 1. Primary sort: by length (shortest first). This prioritizes base names over variants
	//    (e.g., "proxmox" over "proxmox-helper-scripts").
	// 2. Secondary sort: alphabetically. This provides a stable order for names of the same length.
	sort.Slice(data, func(i, j int) bool {
		lenI := len(data[i].Reference)
		lenJ := len(data[j].Reference)
		if lenI != lenJ {
			return lenI < lenJ
		}
		return data[i].Reference < data[j].Reference
	})

	selfhstApps = data
	selfhstAppsCacheTime = time.Now()
	log.Printf("Successfully cached %d apps and tags", len(selfhstApps))
	return selfhstApps, nil
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

// debugf logs a message only if LOG_LEVEL is set to "debug".
func debugf(format string, v ...interface{}) {
	// Import config to check log level
	if isDebugLogLevel() {
		log.Printf("DEBUG: "+format, v...)
	}
}

// isDebugLogLevel checks if the log level is set to debug
func isDebugLogLevel() bool {
	// This will be implemented by checking the config package
	// We avoid importing config directly to prevent circular dependencies
	// The log level check is done via a callback set during initialization
	return debugLogEnabled
}

// debugLogEnabled is set by SetDebugMode
var debugLogEnabled = false

// SetDebugMode enables or disables debug logging for the icons package.
func SetDebugMode(enabled bool) {
	debugLogEnabled = enabled
}
