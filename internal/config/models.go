package config

import "sync"

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
	mu          sync.RWMutex
	overrideMap map[string]ServiceOverride
	compatStatus ConfigStatus

	Version     string                   `yaml:"version"`
	Environment EnvironmentConfiguration `yaml:"environment"`
	Services    ServiceConfiguration     `yaml:"services"`
}

// ConfigStatus represents the configuration compatibility status.
// It indicates whether the loaded configuration is compatible with the current version.
type ConfigStatus struct {
	ConfigVersion          string `json:"configVersion"`
	MinimumRequiredVersion string `json:"minimumRequiredVersion"`
	IsCompatible           bool   `json:"isCompatible"`
	WarningMessage         string `json:"warningMessage,omitempty"`
}

// GetExcludeRouters returns a copy of the list of router exclusion patterns.
func (c *TralaConfiguration) GetExcludeRouters() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, len(c.Services.Exclude.Routers))
	copy(result, c.Services.Exclude.Routers)
	return result
}

// GetExcludeEntrypoints returns a copy of the list of entrypoint exclusion patterns.
func (c *TralaConfiguration) GetExcludeEntrypoints() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, len(c.Services.Exclude.Entrypoints))
	copy(result, c.Services.Exclude.Entrypoints)
	return result
}

// GetManualServices returns a copy of the list of manually configured services.
func (c *TralaConfiguration) GetManualServices() []ManualService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]ManualService, len(c.Services.Manual))
	copy(result, c.Services.Manual)
	return result
}

// GetConfigCompatibilityStatus returns the configuration compatibility status.
func (c *TralaConfiguration) GetConfigCompatibilityStatus() ConfigStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compatStatus
}

// GetConfiguration returns a copy of the exported configuration fields.
// This should be used sparingly as it returns the entire config.
func (c *TralaConfiguration) GetConfiguration() TralaConfiguration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return TralaConfiguration{
		Version:     c.Version,
		Environment: c.Environment,
		Services:    c.Services,
	}
}

// GetTraefikAPIHost returns the Traefik API host URL.
func (c *TralaConfiguration) GetTraefikAPIHost() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Traefik.APIHost
}

// GetSelfhstIconURL returns the base URL for selfh.st icons.
func (c *TralaConfiguration) GetSelfhstIconURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.SelfhstIconURL
}

// GetLogLevel returns the configured log level.
func (c *TralaConfiguration) GetLogLevel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.LogLevel
}

// GetLanguage returns the configured language code.
func (c *TralaConfiguration) GetLanguage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Language
}

// GetSearchEngineURL returns the search engine URL template.
func (c *TralaConfiguration) GetSearchEngineURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.SearchEngineURL
}

// GetRefreshIntervalSeconds returns the refresh interval in seconds.
func (c *TralaConfiguration) GetRefreshIntervalSeconds() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.RefreshIntervalSeconds
}

// GetGroupingEnabled returns whether grouping is enabled.
func (c *TralaConfiguration) GetGroupingEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Grouping.Enabled
}

// GetGroupingColumns returns the number of columns for grouped display.
func (c *TralaConfiguration) GetGroupingColumns() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Grouping.Columns
}

// GetTagFrequencyThreshold returns the tag frequency threshold for grouping.
func (c *TralaConfiguration) GetTagFrequencyThreshold() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Grouping.TagFrequencyThreshold
}

// GetMinServicesPerGroup returns the minimum services required per group.
func (c *TralaConfiguration) GetMinServicesPerGroup() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Grouping.MinServicesPerGroup
}

// GetTraefikConfig returns the complete Traefik configuration.
func (c *TralaConfiguration) GetTraefikConfig() TraefikConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Traefik
}

// GetEnableBasicAuth returns whether basic auth is enabled for Traefik API.
func (c *TralaConfiguration) GetEnableBasicAuth() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Traefik.EnableBasicAuth
}

// GetBasicAuthUsername returns the basic auth username.
func (c *TralaConfiguration) GetBasicAuthUsername() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Traefik.BasicAuth.Username
}

// GetBasicAuthPassword returns the basic auth password.
func (c *TralaConfiguration) GetBasicAuthPassword() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Traefik.BasicAuth.Password
}

// GetInsecureSkipVerify returns whether SSL verification is skipped.
func (c *TralaConfiguration) GetInsecureSkipVerify() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Environment.Traefik.InsecureSkipVerify
}

// GetServiceOverrideMap returns a copy of the map of service overrides by router name.
func (c *TralaConfiguration) GetServiceOverrideMap() map[string]ServiceOverride {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]ServiceOverride, len(c.overrideMap))
	for k, v := range c.overrideMap {
		result[k] = v
	}
	return result
}

// GetServiceOverride looks up a service override by router name.
// Returns the override and true if found, or empty override and false if not.
func (c *TralaConfiguration) GetServiceOverride(routerName string) (ServiceOverride, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	override, ok := c.overrideMap[routerName]
	return override, ok
}

// GetIconOverride returns the icon override for a router name, or empty string if none.
func (c *TralaConfiguration) GetIconOverride(routerName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if override, ok := c.overrideMap[routerName]; ok {
		return override.Icon
	}
	return ""
}

// GetDisplayNameOverride returns the display name override for a router name, or empty string if none.
func (c *TralaConfiguration) GetDisplayNameOverride(routerName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if override, ok := c.overrideMap[routerName]; ok {
		return override.DisplayName
	}
	return ""
}

// GetGroupOverride returns the group override for a router name, or empty string if none.
func (c *TralaConfiguration) GetGroupOverride(routerName string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if override, ok := c.overrideMap[routerName]; ok {
		return override.Group
	}
	return ""
}
