// Package models contains all data structures and type definitions used throughout
// the Trala dashboard application. This includes configuration types, API response
// types, and internal data structures.
package models

import (
	"encoding/json"

	"server/internal/config"
)

// --- Traefik API Types ---

// TraefikRouter represents the essential fields from the Traefik API response.
// It contains routing information including the rule, service, and TLS configuration.
type TraefikRouter struct {
	Name        string           `json:"name"`
	Rule        string           `json:"rule"`
	Service     string           `json:"service"`
	Priority    int              `json:"priority"`
	EntryPoints []string         `json:"entryPoints"`   // Added to determine the entrypoint
	TLS         *json.RawMessage `json:"tls,omitempty"` // Added to capture TLS configuration
}

// TraefikEntryPoint represents the essential fields from the Traefik Entrypoints API.
// It defines how Traefik listens for incoming connections.
type TraefikEntryPoint struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	HTTP    struct {
		TLS json.RawMessage `json:"tls"` // Use RawMessage to check for the presence of TLS configuration
	} `json:"http"`
}

// --- Service Types ---

// Service represents the final, processed data sent to the frontend.
// It contains all the information needed to display a service in the dashboard.
type Service struct {
	Name     string   `json:"Name"`
	URL      string   `json:"url"`
	Priority int      `json:"priority"`
	Icon     string   `json:"icon"`
	Tags     []string `json:"tags"`
	Group    string   `json:"group"`
}

// IconAndTags represents the icon URL and associated tags for a service.
// This is used internally for icon and tag lookups.
type IconAndTags struct {
	Icon string
	Tags []string
}

// --- Status Types ---

// VersionInfo represents the application version information.
// It contains build-time metadata about the application.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}



// FrontendConfig represents the configuration data sent to the frontend.
// It contains settings that the frontend needs for proper operation.
type FrontendConfig struct {
	SearchEngineURL        string `json:"searchEngineURL"`
	SearchEngineIconURL    string `json:"searchEngineIconURL"`
	RefreshIntervalSeconds int    `json:"refreshIntervalSeconds"`
	GroupingEnabled        bool   `json:"groupingEnabled"`
	GroupingColumns        int    `json:"groupingColumns"`
}

// ApplicationStatus represents the combined status information for the application.
// It aggregates version, configuration, and frontend status into a single response.
type ApplicationStatus struct {
	Version  VersionInfo    `json:"version"`
	Config   config.ConfigStatus   `json:"config"`
	Frontend FrontendConfig `json:"frontend"`
}

// --- SelfHst Types ---

// SelfHstIcon represents an entry in the selfh.st icons index.json.
// It contains metadata about available icons from the selfh.st icon library.
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

// SelfHstApp represents an entry in the selfh.st apps CDN integrations/trala.json.
// It contains app metadata including tags for service grouping.
type SelfHstApp struct {
	Reference string   `json:"reference"`
	Name      string   `json:"name"`
	Tags      []string `json:"tags"`
}
