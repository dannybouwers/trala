// Package models contains all data structures and type definitions used throughout
// the Trala dashboard application. This includes configuration types, API response
// types, and internal data structures.
package models

import "encoding/json"

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

// ConfigStatus represents the configuration compatibility status.
// It indicates whether the loaded configuration is compatible with the current version.
type ConfigStatus struct {
	ConfigVersion          string `json:"configVersion"`
	MinimumRequiredVersion string `json:"minimumRequiredVersion"`
	IsCompatible           bool   `json:"isCompatible"`
	WarningMessage         string `json:"warningMessage,omitempty"`
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
	Config   ConfigStatus   `json:"config"`
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

// --- Configuration Types ---

// TraefikBasicAuth contains basic authentication credentials for Traefik API access.
// Password can be provided directly or via a file path.
type TraefikBasicAuth struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

// TraefikConfig contains configuration for connecting to the Traefik API.
// It includes the API host and optional authentication settings.
type TraefikConfig struct {
	APIHost            string           `yaml:"api_host"`
	EnableBasicAuth    bool             `yaml:"enable_basic_auth"`
	BasicAuth          TraefikBasicAuth `yaml:"basic_auth"`
	InsecureSkipVerify bool             `yaml:"insecure_skip_verify"`
}

// ServiceOverride defines overrides for a specific service/router.
// It allows customizing the display name, icon, and group for a service.
type ServiceOverride struct {
	Service     string `yaml:"service"`
	DisplayName string `yaml:"display_name,omitempty"`
	Icon        string `yaml:"icon,omitempty"`
	Group       string `yaml:"group,omitempty"`
}

// ManualService defines a manually configured service.
// This is used for services not discovered via Traefik.
type ManualService struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Icon     string `yaml:"icon,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
	Group    string `yaml:"group,omitempty"`
}

// ExcludeConfig defines patterns for excluding routers and entrypoints.
// Supports wildcard patterns for flexible matching.
type ExcludeConfig struct {
	Routers     []string `yaml:"routers"`
	Entrypoints []string `yaml:"entrypoints"`
}

// ServiceConfiguration contains service-related configuration options.
// It includes exclusions, overrides, and manual service definitions.
type ServiceConfiguration struct {
	Exclude   ExcludeConfig     `yaml:"exclude"`
	Overrides []ServiceOverride `yaml:"overrides"`
	Manual    []ManualService   `yaml:"manual"`
}

// GroupingConfig contains settings for automatic service grouping.
// Grouping organizes services by common tags.
type GroupingConfig struct {
	Enabled               bool    `yaml:"enabled"`
	Columns               int     `yaml:"columns"`
	TagFrequencyThreshold float64 `yaml:"tag_frequency_threshold"`
	MinServicesPerGroup   int     `yaml:"min_services_per_group"`
}

// EnvironmentConfiguration contains environment-level configuration options.
// These settings control the overall behavior of the application.
type EnvironmentConfiguration struct {
	SelfhstIconURL         string         `yaml:"selfhst_icon_url"`
	SearchEngineURL        string         `yaml:"search_engine_url"`
	RefreshIntervalSeconds int            `yaml:"refresh_interval_seconds"`
	LogLevel               string         `yaml:"log_level"`
	Traefik                TraefikConfig  `yaml:"traefik"`
	Language               string         `yaml:"language"`
	Grouping               GroupingConfig `yaml:"grouping"`
}

// TralaConfiguration is the root configuration structure.
// It represents the complete configuration file format.
type TralaConfiguration struct {
	Version     string                   `yaml:"version"`
	Environment EnvironmentConfiguration `yaml:"environment"`
	Services    ServiceConfiguration     `yaml:"services"`
}
