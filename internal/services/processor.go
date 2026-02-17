// Package services provides service processing and grouping functionality for the Trala dashboard.
// This file contains the service processing logic that transforms Traefik routers into Service objects.
package services

import (
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"server/internal/config"
	"server/internal/icons"
	"server/internal/models"
	"server/internal/traefik"
)

// ProcessRouter takes a raw Traefik router, finds its best icon, and returns the final Service object.
// It handles router name extraction, URL reconstruction, exclusion checks, and icon/tag discovery.
// Returns the processed Service and a boolean indicating if the router should be included.
func ProcessRouter(router models.TraefikRouter, entryPoints map[string]models.TraefikEntryPoint) (models.Service, bool) {
	routerName := strings.Split(router.Name, "@")[0]

	// Remove entrypoint name from the beginning of router name (case-insensitive)
	if len(router.EntryPoints) > 0 {
		entryPointName := router.EntryPoints[0]
		// Create the pattern to match: entrypoint name followed by a dash
		prefix := entryPointName + "-"
		// Check if router name starts with the entrypoint name (case-insensitive)
		if len(routerName) > len(prefix) && strings.HasPrefix(strings.ToLower(routerName), strings.ToLower(prefix)) {
			// Remove the entrypoint prefix
			routerName = routerName[len(prefix):]
			debugf("Removed entrypoint prefix '%s' from router name, new name: '%s'", prefix, routerName)
		}
	}

	serviceURL := traefik.ReconstructURL(router, entryPoints)

	if serviceURL == "" {
		debugf("Could not reconstruct URL for router %s from rule: %s", routerName, router.Rule)
		return models.Service{}, false
	}

	// Check if this router should be excluded
	if IsExcluded(routerName) {
		debugf("Excluding router: %s", routerName)
		return models.Service{}, false
	}

	// Check if this router should be excluded based on entrypoints
	if IsEntrypointExcluded(router.EntryPoints) {
		debugf("Excluding router %s due to entrypoint exclusion", routerName)
		return models.Service{}, false
	}

	// Check if this is the Traefik API service and exclude it
	traefikAPIHost := config.GetTraefikAPIHost()
	if traefikAPIHost != "" {
		if !strings.HasPrefix(traefikAPIHost, "http") {
			traefikAPIHost = "http://" + traefikAPIHost
		}
		apiURL := traefikAPIHost + "/api"
		if serviceURL == apiURL {
			debugf("Excluding router %s because it's the Traefik API service", routerName)
			return models.Service{}, false
		}
	}

	// Get display name override if available
	displayName := config.GetDisplayNameOverride(routerName)
	if displayName == "" {
		routerNameReplaced := strings.ReplaceAll(routerName, "-", " ")
		displayName = routerNameReplaced
	}

	debugf("Processing router: %s (display: %s), URL: %s", routerName, displayName, serviceURL)
	displayNameReplaced := strings.ReplaceAll(displayName, " ", "-")
	reference := icons.ResolveSelfHstReference(displayNameReplaced)
	iconURL := icons.FindIcon(routerName, serviceURL, displayNameReplaced, reference)
	tags := icons.FindTags(routerName, reference)

	// get group override if available
	group := config.GetGroupOverride(routerName)

	return models.Service{
		Name:     displayName,
		URL:      serviceURL,
		Priority: router.Priority,
		Icon:     iconURL,
		Tags:     tags,
		Group:    group,
	}, true
}

// GetManualServices processes manually configured services and returns them as Service objects.
// It validates URLs, resolves icons, and applies default values where needed.
func GetManualServices() []models.Service {
	manualServices := config.GetManualServices()
	result := make([]models.Service, 0, len(manualServices))

	for _, manualService := range manualServices {
		// Validate URL
		if !config.IsValidUrl(manualService.URL) {
			log.Printf("Warning: Invalid URL for manual service '%s': %s", manualService.Name, manualService.URL)
			continue
		}

		displayNameReplaced := strings.ReplaceAll(manualService.Name, " ", "-")
		reference := icons.ResolveSelfHstReference(displayNameReplaced)

		// Find icon using the same logic as for Traefik services
		iconURL := manualService.Icon
		if iconURL == "" {
			// If no icon is specified, try to find one automatically
			iconURL = icons.FindIcon(manualService.Name, manualService.URL, displayNameReplaced, reference)
		} else if !strings.HasPrefix(iconURL, "http://") && !strings.HasPrefix(iconURL, "https://") {
			// If icon is specified, check if it's a full URL or just a filename
			// Check if it's a filename with valid extension
			ext := filepath.Ext(iconURL)
			if ext == ".png" || ext == ".svg" || ext == ".webp" {
				iconURL = config.GetSelfhstIconURL() + strings.TrimPrefix(ext, ".") + "/" + strings.ToLower(iconURL)
			} else {
				// Fallback to default behavior if extension is not valid
				iconURL = config.GetSelfhstIconURL() + "png/" + iconURL
			}
		}

		// get tags from manual service
		tags := icons.FindTags(manualService.Name, reference)

		// Default priority if not specified
		priority := manualService.Priority
		if priority == 0 {
			priority = 50 // Default priority for manual services
		}

		service := models.Service{
			Name:     manualService.Name,
			URL:      manualService.URL,
			Priority: priority,
			Icon:     iconURL,
			Tags:     tags,
			Group:    manualService.Group,
		}

		result = append(result, service)
		debugf("Added manual service: %s (URL: %s, Icon: %s, Priority: %d, Group: %s)",
			manualService.Name, manualService.URL, iconURL, priority, manualService.Group)
	}

	return result
}

// IsExcluded checks if a router name is in the exclude list.
// Supports wildcard patterns (*, ?) and logs invalid patterns.
func IsExcluded(routerName string) bool {
	excludePatterns := config.GetExcludeRouters()

	for _, exclude := range excludePatterns {
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

// IsEntrypointExcluded checks if an entrypoint name is in the exclude list.
// Supports wildcard patterns (*, ?) and logs invalid patterns.
func IsEntrypointExcluded(entryPoints []string) bool {
	excludePatterns := config.GetExcludeEntrypoints()

	for _, ep := range entryPoints {
		for _, exclude := range excludePatterns {
			match, err := filepath.Match(exclude, ep)
			if err != nil {
				log.Printf("WARNING: invalid exclude.entrypoints pattern %q: %v", exclude, err)
				continue
			}
			if match {
				debugf("Excluding entrypoint: %s matched pattern %s", ep, exclude)
				return true
			}
		}
	}
	return false
}

// ExtractServiceNameFromURL extracts the service name from a search engine URL.
// It parses the hostname and extracts the second-level domain name.
func ExtractServiceNameFromURL(searchURL string) string {
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

// debugf logs a message only if LOG_LEVEL is set to "debug".
func debugf(format string, v ...interface{}) {
	if config.GetLogLevel() == "debug" {
		log.Printf("DEBUG: "+format, v...)
	}
}
