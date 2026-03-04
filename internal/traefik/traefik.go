// Package traefik provides a client for interacting with the Traefik API.
// It handles HTTP client initialization, authentication, pagination, and URL reconstruction.
package traefik

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"server/internal/config"
	"server/internal/models"
)

// --- Global Variables ---

// HTTPClient is the HTTP client for Traefik API calls (may have SSL verification disabled)
var HTTPClient *http.Client

// Regex patterns to reliably find Host and PathPrefix in Traefik rules
var (
	hostRegex = regexp.MustCompile(`Host\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
	pathRegex = regexp.MustCompile(`PathPrefix\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
)

// --- HTTP Client Initialization ---

// InitializeHTTPClient initializes the HTTP client for Traefik API calls.
// It configures TLS settings based on the configuration (may disable SSL verification).
func InitializeHTTPClient() {
	// Create Traefik HTTP client (may have SSL verification disabled)
	traefikTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Configure TLS for Traefik client based on configuration
	if config.GetInsecureSkipVerify() {
		traefikTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		log.Printf("WARNING: SSL certificate verification is disabled for Traefik API connections")
	} else {
		traefikTransport.TLSClientConfig = &tls.Config{}
	}

	HTTPClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: traefikTransport,
	}
}

// --- HTTP Request Helpers ---

// CreateHTTPRequestWithAuth creates an HTTP request with basic auth if enabled in configuration.
func CreateHTTPRequestWithAuth(method, url string) (*http.Request, error) {
	return CreateHTTPRequestWithAuthAndContext(context.Background(), method, url)
}

// CreateHTTPRequestWithAuthAndContext creates an HTTP request with context and basic auth if enabled in configuration.
func CreateHTTPRequestWithAuthAndContext(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	// Set basic auth option if enabled
	if config.GetEnableBasicAuth() {
		debugf("Setting basic auth")
		req.SetBasicAuth(config.GetBasicAuthUsername(), config.GetBasicAuthPassword())
	}

	return req, nil
}

// CreateAndExecuteHTTPRequest creates an authenticated HTTP request, executes it, and handles common errors.
// Returns the response and error, or writes an HTTP error response and returns nil.
func CreateAndExecuteHTTPRequest(w http.ResponseWriter, method, url string) (*http.Response, error) {
	req, err := CreateHTTPRequestWithAuth(method, url)
	if err != nil {
		log.Printf("ERROR: Could not create request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, err
	}

	resp, err := HTTPClient.Do(req)
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

// CreateAndExecuteHTTPRequestWithContext creates an authenticated HTTP request with context, executes it, and handles common errors.
// Returns the response and error, or writes an HTTP error response and returns nil.
func CreateAndExecuteHTTPRequestWithContext(w http.ResponseWriter, ctx context.Context, method, url string) (*http.Response, error) {
	req, err := CreateHTTPRequestWithAuthAndContext(ctx, method, url)
	if err != nil {
		log.Printf("ERROR: Could not create request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return nil, err
	}

	resp, err := HTTPClient.Do(req)
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

// --- Pagination ---

// FetchAllPages fetches all pages of data from a paginated Traefik API endpoint.
// It handles the X-Next-Page header to iterate through all pages.
func FetchAllPages[T any](w http.ResponseWriter, baseURL string) ([]T, error) {
	var allItems []T
	currentURL := baseURL

	for {
		// Create request with context
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req, err := CreateHTTPRequestWithAuthAndContext(ctx, "GET", currentURL)
		if err != nil {
			log.Printf("ERROR: Could not create request for %s: %v", currentURL, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return nil, err
		}

		resp, err := HTTPClient.Do(req)
		if err != nil {
			log.Printf("ERROR: Could not fetch from %s: %v", currentURL, err)
			http.Error(w, "Could not connect to API", http.StatusBadGateway)
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("ERROR: API returned non-200 status: %s", resp.Status)
			http.Error(w, "Received non-200 status from API", http.StatusBadGateway)
			resp.Body.Close()
			return nil, fmt.Errorf("non-200 status: %s", resp.Status)
		}

		// Decode the current page
		var items []T
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			log.Printf("ERROR: Could not decode API response from %s: %v", currentURL, err)
			http.Error(w, "Invalid JSON from API", http.StatusInternalServerError)
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		allItems = append(allItems, items...)

		// Check for next page
		nextPage := resp.Header.Get("X-Next-Page")
		if nextPage == "" || nextPage == "1" {
			// No more pages
			break
		}

		// Construct URL for next page
		parsedURL, err := url.Parse(currentURL)
		if err != nil {
			log.Printf("ERROR: Could not parse URL %s: %v", currentURL, err)
			break
		}

		// Add or update the page query parameter
		query := parsedURL.Query()
		query.Set("page", nextPage)
		parsedURL.RawQuery = query.Encode()
		currentURL = parsedURL.String()
	}

	return allItems, nil
}

// --- URL Reconstruction ---

// DetermineProtocol determines the correct protocol (http/https) for a service
// based on TLS configuration in both router and entrypoint.
func DetermineProtocol(router models.TraefikRouter, entryPoint models.TraefikEntryPoint) string {
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

// ReconstructURL extracts the base URL from a Traefik rule and determines the protocol and port
// based on the router's entrypoint.
func ReconstructURL(router models.TraefikRouter, entryPoints map[string]models.TraefikEntryPoint) string {
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
	protocol := DetermineProtocol(router, entryPoint)

	// Address is in the format ":port"
	port := strings.TrimPrefix(entryPoint.Address, ":")

	// Omit the port if it's the default for the protocol.
	if (protocol == "http" && port == "80") || (protocol == "https" && port == "443") {
		return fmt.Sprintf("%s://%s%s", protocol, hostname, path)
	}

	return fmt.Sprintf("%s://%s%s:%s", protocol, hostname, path, port)
}

// --- Helper Functions ---

// debugf logs a message only if LOG_LEVEL is set to "debug".
func debugf(format string, v ...interface{}) {
	if config.GetLogLevel() == "debug" {
		log.Printf("DEBUG: "+format, v...)
	}
}
