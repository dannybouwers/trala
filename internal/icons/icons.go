// Package icons provides icon discovery and caching functionality for the Trala dashboard.
// This file contains the main icon finding logic and helper functions.
package icons

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"server/internal/config"

	"github.com/PuerkitoBio/goquery"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

// DefaultIcon is the default icon returned when no icon is found.
// The frontend will use a fallback if icon is empty.
const DefaultIcon = ""

// FindIcon tries all icon-finding methods in order of priority and returns the icon URL.
// The priority order is:
// 1. User-defined overrides (from configuration)
// 2. User icons (fuzzy matched from /icons directory)
// 3. SelfHst icons (fuzzy matched from selfh.st icon library)
// 4. /favicon.ico from the service URL
// 5. HTML parsing for <link> tags
func FindIcon(routerName, serviceURL string, displayNameReplaced string, reference string) string {
	// Priority 1: Check user-defined overrides.
	if iconValue := config.GetIconOverride(routerName); iconValue != "" {
		// Check if it's a full URL
		if strings.HasPrefix(iconValue, "http://") || strings.HasPrefix(iconValue, "https://") {
			debugf("[%s] Found icon via override (full URL): %s", routerName, iconValue)
			return iconValue
		}

		// Check if it's a filename with valid extension
		ext := filepath.Ext(iconValue)
		if ext == ".png" || ext == ".svg" || ext == ".webp" {
			iconURL := config.GetSelfhstIconURL() + strings.TrimPrefix(ext, ".") + "/" + strings.ToLower(iconValue)
			debugf("[%s] Found icon via override (filename): %s", routerName, iconURL)
			return iconURL
		}

		// Fallback to default behavior if extension is not valid
	iconURL := config.GetSelfhstIconURL() + "png/" + strings.ToLower(iconValue) + ".png"
		debugf("[%s] Found icon via override (fallback): %s", routerName, iconURL)
		return iconURL
	}

	// Priority 2: Check user icons
	if iconPath := FindUserIcon(displayNameReplaced); iconPath != "" {
		// For user icons, we return the URL that can be served by the application
		debugf("[%s] Found icon via user icons (fuzzy search): %s", displayNameReplaced, iconPath)
		return iconPath
	}

	// Priority 3: Fuzzy search against selfh.st icons
	if reference != "" {
		iconURL := GetSelfHstIconURL(reference)
		debugf("[%s] Found icon via fuzzy search: %s", displayNameReplaced, iconURL)
		return iconURL
	}

	// Priority 4: Check for /favicon.ico.
	if iconURL := FindFavicon(serviceURL); iconURL != "" {
		debugf("[%s] Found icon via /favicon.ico: %s", routerName, iconURL)
		return iconURL
	}

	// Priority 5: Parse service's HTML for a <link> tag.
	if iconURL := FindHTMLIcon(serviceURL); iconURL != "" {
		debugf("[%s] Found icon via HTML parsing: %s", routerName, iconURL)
		return iconURL
	}

	debugf("[%s] No icon found, will use fallback.", routerName)
	return DefaultIcon
}

// FindTags finds tags for a service using the provided selfh.st reference.
// Returns an empty slice if no tags are found or if reference is empty.
func FindTags(routerName string, reference string) []string {
	if reference != "" {
		tags := GetServiceTags(reference)
		debugf("[%s] Found tags via fuzzy search: %v", routerName, tags)
		return tags
	}

	debugf("[%s] No tags found.", routerName)
	return []string{}
}

// ResolveSelfHstReference performs fuzzy search to find the matching selfh.st reference for a service name.
// Returns the best matching reference string, or empty string if no match found.
func ResolveSelfHstReference(serviceName string) string {
	icons, err := GetSelfHstIconNames()
	if err != nil {
		log.Printf("ERROR: Could not get selfh.st icon list for reference resolution: %v", err)
		return ""
	}

	references := make([]string, len(icons))
	for i, icon := range icons {
		references[i] = icon.Reference
	}

	matches := fuzzy.FindFold(serviceName, references)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// GetSelfHstIconURL generates the icon URL for a given selfh.st reference.
// Prefers SVG format if available, otherwise falls back to PNG.
func GetSelfHstIconURL(reference string) string {
	if reference == "" {
		return ""
	}

	icons, err := GetSelfHstIconNames()
	if err != nil {
		log.Printf("ERROR: Could not get selfh.st icon list for URL generation: %v", err)
		return ""
	}

	for _, icon := range icons {
		if icon.Reference == reference {
			// Prefer SVG if available
			if icon.SVG == "Yes" {
				return fmt.Sprintf(config.GetSelfhstIconURL()+"svg/%s.svg", icon.Reference)
			}
			// Fallback to PNG
			return fmt.Sprintf(config.GetSelfhstIconURL()+"png/%s.png", icon.Reference)
		}
	}
	return ""
}

// GetServiceTags retrieves the tags for a given selfh.st reference.
// Returns an empty slice if no tags are found or if reference is empty.
func GetServiceTags(reference string) []string {
	if reference == "" {
		return []string{}
	}

	data, err := GetSelfHstAppTags()
	if err != nil {
		log.Printf("ERROR: Could not get integration data for tags: %v", err)
		return []string{}
	}

	for _, entry := range data {
		if entry.Reference == reference {
			return entry.Tags
		}
	}

	return []string{}
}

// FindFavicon checks for the existence of /favicon.ico at the service URL.
// Returns the favicon URL if it exists and is a valid image, otherwise empty string.
func FindFavicon(serviceURL string) string {
	u, err := url.Parse(serviceURL)
	if err != nil {
		return ""
	}
	faviconURL := fmt.Sprintf("%s://%s/favicon.ico", u.Scheme, u.Host)
	if IsValidImageURL(faviconURL) {
		return faviconURL
	}
	return ""
}

// FindHTMLIcon fetches and parses the service's HTML to find icon links.
// It looks for apple-touch-icon and icon link rels in order.
func FindHTMLIcon(serviceURL string) string {
	if externalHTTPClient == nil {
		return ""
	}

	resp, err := externalHTTPClient.Get(serviceURL)
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
			// Use the final URL after redirects as the base for resolving relative URLs
			finalURL := resp.Request.URL.String()
			absoluteIconURL, err := resolveURL(finalURL, iconPath)
			if err == nil && IsValidImageURL(absoluteIconURL) {
				return absoluteIconURL
			}
		}
	}
	return ""
}

// IsValidImageURL performs a HEAD request to check if a URL points to a valid image.
// Returns true if the URL returns a 200 OK status with an image content type.
func IsValidImageURL(iconURL string) bool {
	if externalHTTPClient == nil {
		return false
	}

	resp, err := externalHTTPClient.Head(iconURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	return resp.StatusCode == http.StatusOK && strings.HasPrefix(contentType, "image/")
}

// resolveURL resolves a path against a base URL, returning the absolute URL.
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
