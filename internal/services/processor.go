// Package services provides service processing and grouping functionality for the Trala dashboard.
// This file contains the service processing logic that transforms Traefik routers into Service objects.
package services

import (
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"server/internal/config"
	"server/internal/debug"
	"server/internal/icons"
	"server/internal/models"
	"server/internal/traefik"
)

var conf *config.TralaConfiguration

// Init stores the configuration instance for use by service functions.
func Init(c *config.TralaConfiguration) {
	conf = c
}

// ProcessRouter takes a raw Traefik router, finds its best icon, and returns the final Service object.
// It handles router name extraction, URL reconstruction, exclusion checks, and icon/tag discovery.
// Returns the processed Service and a boolean indicating if the router should be included.
func ProcessRouter(router models.TraefikRouter, entryPoints map[string]models.TraefikEntryPoint, instanceName string) (models.Service, bool) {
	routerName := strings.Split(router.Name, "@")[0]

	// Remove entrypoint name from the beginning of router name (case-insensitive)
	if len(router.EntryPoints) > 0 {
		entryPointName := router.EntryPoints[0]
		prefix := entryPointName + "-"
		if len(routerName) > len(prefix) && strings.HasPrefix(strings.ToLower(routerName), strings.ToLower(prefix)) {
			routerName = routerName[len(prefix):]
			debugf("Removed entrypoint prefix '%s' from router name, new name: '%s'", prefix, routerName)
		}
	}

	serviceURL := traefik.ReconstructURL(router, entryPoints)

	if serviceURL == "" {
		debugf("Could not reconstruct URL for router %s from rule: %s", routerName, router.Rule)
		return models.Service{}, false
	}

	if IsExcluded(routerName) {
		debugf("Excluding router: %s", routerName)
		return models.Service{}, false
	}

	if IsEntrypointExcluded(router.EntryPoints) {
		debugf("Excluding router %s due to entrypoint exclusion", routerName)
		return models.Service{}, false
	}

	instances := conf.GetTraefikInstances()
	for _, inst := range instances {
		traefikAPIHost := inst.APIHost
		if traefikAPIHost != "" {
			if !strings.HasPrefix(traefikAPIHost, "http") {
				traefikAPIHost = "http://" + traefikAPIHost
			}
			apiURL := traefikAPIHost + "/api"
			if serviceURL == apiURL {
				debugf("Excluding router %s because it's the Traefik API service for instance %s", routerName, inst.Name)
				return models.Service{}, false
			}
		}
	}

	displayName := conf.GetDisplayNameOverride(routerName)
	if displayName == "" {
		routerNameReplaced := strings.ReplaceAll(routerName, "-", " ")
		displayName = routerNameReplaced
	}

	debugf("Processing router: %s (display: %s), URL: %s", routerName, displayName, serviceURL)
	displayNameReplaced := strings.ReplaceAll(displayName, " ", "-")
	reference := icons.ResolveSelfHstReference(displayNameReplaced)
	iconURL := icons.FindIcon(routerName, serviceURL, displayNameReplaced, reference)
	tags := icons.FindTags(routerName, reference)

	group := conf.GetGroupOverride(routerName)

	return models.Service{
		Name:     displayName,
		URL:      serviceURL,
		Priority: router.Priority,
		Icon:     iconURL,
		Tags:     tags,
		Group:    group,
		Host:     instanceName,
	}, true
}

// GetManualServices processes manually configured services and returns them as Service objects.
// It validates URLs, resolves icons, and applies default values where needed.
func GetManualServices() []models.Service {
	manualServices := conf.GetManualServices()
	result := make([]models.Service, 0, len(manualServices))

	instances := conf.GetTraefikInstances()
	defaultHost := ""
	if len(instances) > 0 {
		defaultHost = instances[0].Name
	}

	for _, manualService := range manualServices {
		if !config.IsValidUrl(manualService.URL) {
			log.Printf("Warning: Invalid URL for manual service '%s': %s", manualService.Name, manualService.URL)
			continue
		}

		displayNameReplaced := strings.ReplaceAll(manualService.Name, " ", "-")
		reference := icons.ResolveSelfHstReference(displayNameReplaced)

		iconURL := manualService.Icon
		if iconURL == "" {
			iconURL = icons.FindIcon(manualService.Name, manualService.URL, displayNameReplaced, reference)
		} else if !strings.HasPrefix(iconURL, "http://") && !strings.HasPrefix(iconURL, "https://") {
			ext := filepath.Ext(iconURL)
			if ext == ".png" || ext == ".svg" || ext == ".webp" {
				iconURL = conf.GetSelfhstIconURL() + strings.TrimPrefix(ext, ".") + "/" + strings.ToLower(iconURL)
			} else {
				iconURL = conf.GetSelfhstIconURL() + "png/" + strings.ToLower(iconURL) + ".png"
			}
		}

		tags := icons.FindTags(manualService.Name, reference)

		priority := manualService.Priority
		if priority == 0 {
			priority = 50
		}

		host := manualService.Host
		if host == "" {
			host = defaultHost
		}

		service := models.Service{
			Name:     manualService.Name,
			URL:      manualService.URL,
			Priority: priority,
			Icon:     iconURL,
			Tags:     tags,
			Group:    manualService.Group,
			Host:     host,
		}

		result = append(result, service)
		debugf("Added manual service: %s (URL: %s, Icon: %s, Priority: %d, Group: %s, Host: %s)",
			manualService.Name, manualService.URL, iconURL, priority, manualService.Group, host)
	}

	return result
}

// IsExcluded checks if a router name is in the exclude list.
// Supports wildcard patterns (*, ?) and logs invalid patterns.
func IsExcluded(routerName string) bool {
	excludePatterns := conf.GetExcludeRouters()

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
	excludePatterns := conf.GetExcludeEntrypoints()

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

// debugf is a wrapper for the shared debug utility
var debugf = debug.Debugf
