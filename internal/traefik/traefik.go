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
	"server/internal/debug"
	"server/internal/models"
)

// --- Global Variables ---

// HTTPClient is the HTTP client for Traefik API calls (may have SSL verification disabled)
var HTTPClient *http.Client

var conf *config.TralaConfiguration

// Init stores the configuration instance for use by traefik functions.
func Init(c *config.TralaConfiguration) {
	conf = c
}

// Regex patterns to reliably find Host and PathPrefix in Traefik rules
var (
	hostRegex = regexp.MustCompile(`Host\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
	pathRegex = regexp.MustCompile(`PathPrefix\(\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\)`)
)

// --- HTTP Client Initialization ---

// InitializeHTTPClient initializes the HTTP client for Traefik API calls.
// It configures TLS settings based on the single-instance configuration (may disable SSL verification).
func InitializeHTTPClient() {
	insecureSkipVerify := false
	if conf != nil {
		instances := conf.GetTraefikInstances()
		if len(instances) > 0 && instances[0].InsecureSkipVerify {
			insecureSkipVerify = true
		}
	}

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

	if insecureSkipVerify {
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

// CreateHTTPClientForInstance creates an HTTP client for a specific Traefik instance.
func CreateHTTPClientForInstance(insecureSkipVerify bool) *http.Client {
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

	if insecureSkipVerify {
		traefikTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		traefikTransport.TLSClientConfig = &tls.Config{}
	}

	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: traefikTransport,
	}
}

// CreateHTTPRequestWithInstanceAuthAndContext creates an HTTP request with context and basic auth for a specific instance.
func CreateHTTPRequestWithInstanceAuthAndContext(ctx context.Context, method, url string, instance config.TraefikInstanceConfig) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	if instance.EnableBasicAuth {
		debugf("Setting basic auth for instance %s", instance.Name)
		req.SetBasicAuth(instance.BasicAuth.Username, instance.BasicAuth.Password)
	}

	return req, nil
}

// CreateAndExecuteHTTPRequestWithInstance creates an authenticated HTTP request for a specific
// instance and executes it using the provided client. The caller should pass a shared
// *http.Client (e.g. from CreateHTTPClientForInstance) rather than creating a new one per call.
func CreateAndExecuteHTTPRequestWithInstance(ctx context.Context, client *http.Client, method, url string, instance config.TraefikInstanceConfig) (*http.Response, error) {
	req, err := CreateHTTPRequestWithInstanceAuthAndContext(ctx, method, url, instance)
	if err != nil {
		log.Printf("ERROR: Could not create request: %v", err)
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Could not fetch from %s: %v", url, err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: API returned non-200 status: %s", resp.Status)
		resp.Body.Close()
		return nil, fmt.Errorf("non-200 status: %s", resp.Status)
	}

	return resp, nil
}

// --- Pagination ---

// FetchAllPagesWithInstanceAuth fetches all pages using per-instance authentication and the
// provided shared client.
func FetchAllPagesWithInstanceAuth[T any](ctx context.Context, client *http.Client, baseURL string, instance config.TraefikInstanceConfig) ([]T, error) {
	var allItems []T
	currentURL := baseURL

	for {
		req, err := CreateHTTPRequestWithInstanceAuthAndContext(ctx, "GET", currentURL, instance)
		if err != nil {
			log.Printf("ERROR: Could not create request for %s: %v", currentURL, err)
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("ERROR: Could not fetch from %s: %v", currentURL, err)
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("ERROR: API returned non-200 status: %s", resp.Status)
			resp.Body.Close()
			return nil, fmt.Errorf("non-200 status: %s", resp.Status)
		}

		var items []T
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			log.Printf("ERROR: Could not decode API response from %s: %v", currentURL, err)
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		allItems = append(allItems, items...)

		nextPage := resp.Header.Get("X-Next-Page")
		if nextPage == "" || nextPage == "1" {
			break
		}

		parsedURL, err := url.Parse(currentURL)
		if err != nil {
			log.Printf("ERROR: Could not parse URL %s: %v", currentURL, err)
			break
		}

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
	if router.TLS != nil {
		tlsStr := string(*router.TLS)
		if tlsStr != "null" && tlsStr != "{}" && tlsStr != "" {
			return "https"
		}
	}

	if entryPoint.HTTP.TLS != nil {
		tlsStr := string(entryPoint.HTTP.TLS)
		if tlsStr != "null" && tlsStr != "{}" && tlsStr != "" {
			return "https"
		}
	}

	return "http"
}

// ReconstructURL extracts the base URL from a Traefik rule and determines the protocol and port
// based on the router's entrypoint.
func ReconstructURL(router models.TraefikRouter, entryPoints map[string]models.TraefikEntryPoint) string {
	hostMatches := hostRegex.FindStringSubmatch(router.Rule)
	if len(hostMatches) < 2 {
		return ""
	}
	hostname := hostMatches[1]

	path := ""
	pathMatches := pathRegex.FindStringSubmatch(router.Rule)
	if len(pathMatches) >= 2 {
		path = pathMatches[1]
	}

	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimSuffix(path, "/")

	if len(router.EntryPoints) == 0 {
		debugf("[%s] Router has no entrypoints defined. Cannot determine URL.", router.Name)
		return ""
	}
	entryPointName := router.EntryPoints[0]
	entryPoint, ok := entryPoints[entryPointName]
	if !ok {
		debugf("[%s] Entrypoint '%s' not found in Traefik configuration.", router.Name, entryPointName)
		return ""
	}

	protocol := DetermineProtocol(router, entryPoint)
	port := strings.TrimPrefix(entryPoint.Address, ":")

	if (protocol == "http" && port == "80") || (protocol == "https" && port == "443") {
		return fmt.Sprintf("%s://%s%s", protocol, hostname, path)
	}

	return fmt.Sprintf("%s://%s:%s%s", protocol, hostname, port, path)
}

// debugf is a wrapper for the shared debug utility
var debugf = debug.Debugf
