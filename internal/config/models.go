package config

import (
	"net/url"
	"strings"
	"sync"
)

// TraefikInstanceConfig contains configuration for a single Traefik instance.
type TraefikInstanceConfig struct {
	Name               string           `yaml:"name,omitempty"`
	APIHost            string           `yaml:"api_host" validate:"required,url"`
	EnableBasicAuth    bool             `yaml:"enable_basic_auth"`
	BasicAuth          TraefikBasicAuth `yaml:"basic_auth"`
	InsecureSkipVerify bool             `yaml:"insecure_skip_verify"`
}

// TraefikConfig contains configuration for connecting to one or more Traefik instances.
// Supports both single-instance (legacy) and multi-instance formats.
type TraefikConfig struct {
	// Single-instance fields (legacy format)
	APIHost            string           `yaml:"api_host"`
	EnableBasicAuth    bool             `yaml:"enable_basic_auth"`
	BasicAuth          TraefikBasicAuth `yaml:"basic_auth"`
	InsecureSkipVerify bool             `yaml:"insecure_skip_verify"`

	// Multi-instance fields (new format)
	Instances []TraefikInstanceConfig `yaml:"instances" validate:"dive"`

	// Internal: set after parsing
	IsMulti bool `yaml:"-"`
}

// MarshalYAML implements custom YAML marshaling for TraefikConfig.
// It only outputs the canonical format: instances for multi-instance mode,
// or legacy single-instance fields derived from the first instance for
// single-instance mode. This avoids dumping both notations simultaneously.
func (t TraefikConfig) MarshalYAML() (interface{}, error) {
	if t.IsMulti {
		return struct {
			Instances []TraefikInstanceConfig `yaml:"instances"`
		}{
			Instances: t.Instances,
		}, nil
	}
	if len(t.Instances) > 0 {
		inst := t.Instances[0]
		return struct {
			APIHost            string           `yaml:"api_host"`
			EnableBasicAuth    bool             `yaml:"enable_basic_auth"`
			BasicAuth          TraefikBasicAuth `yaml:"basic_auth"`
			InsecureSkipVerify bool             `yaml:"insecure_skip_verify"`
		}{
			APIHost:            inst.APIHost,
			EnableBasicAuth:    inst.EnableBasicAuth,
			BasicAuth:          inst.BasicAuth,
			InsecureSkipVerify: inst.InsecureSkipVerify,
		}, nil
	}
	return struct {
		Instances []TraefikInstanceConfig `yaml:"instances"`
	}{
		Instances: nil,
	}, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for TraefikConfig.
// It supports both formats:
//  1. Direct sequence under traefik: (legacy multi-instance format from plan)
//     traefik:
//     - api_host: http://traefik:8080
//     - api_host: http://traefik-arr:8080
//  2. Explicit instances key
//     traefik:
//     instances:
//     - api_host: http://traefik:8080
//     - api_host: http://traefik-arr:8080
//  3. Single-instance legacy format
//     traefik:
//     api_host: http://traefik:8080
func (t *TraefikConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a slice of instances first (format 1: direct sequence)
	var instances []TraefikInstanceConfig
	if err := unmarshal(&instances); err == nil {
		t.Instances = instances
		t.IsMulti = len(instances) > 1 || len(instances) == 1 // even single element in list is multi-instance mode
		return nil
	}

	// Try to unmarshal as a map with instances key (format 2)
	type alias TraefikConfig
	aux := alias{}
	if err := unmarshal(&aux); err != nil {
		return err
	}
	t.APIHost = aux.APIHost
	t.EnableBasicAuth = aux.EnableBasicAuth
	t.BasicAuth = aux.BasicAuth
	t.InsecureSkipVerify = aux.InsecureSkipVerify
	t.Instances = aux.Instances
	// Unlike the bare-list format above, an `instances:` key with a single entry is only
	// multi-instance when no legacy single-instance fields are also set.
	t.IsMulti = len(aux.Instances) > 1 || (len(aux.Instances) == 1 && aux.APIHost == "" && !aux.EnableBasicAuth)
	return nil
}

// TraefikBasicAuth contains basic authentication credentials for Traefik API access.
// Password can be provided directly or via a file path.
type TraefikBasicAuth struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
}

// ServiceOverride defines overrides for a specific service/router.
// It allows customizing the display name, icon, and group for a service.
type ServiceOverride struct {
	Service     string `yaml:"service" validate:"required"`
	DisplayName string `yaml:"display_name,omitempty"`
	Icon        string `yaml:"icon,omitempty"`
	Group       string `yaml:"group,omitempty"`
}

// ManualService defines a manually configured service.
// This is used for services not discovered via Traefik.
type ManualService struct {
	Name     string `yaml:"name" validate:"required"`
	URL      string `yaml:"url" validate:"required,url"`
	Icon     string `yaml:"icon,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
	Group    string `yaml:"group,omitempty"`
	Host     string `yaml:"host,omitempty"`
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
	Overrides []ServiceOverride `yaml:"overrides" validate:"dive"`
	Manual    []ManualService   `yaml:"manual" validate:"dive"`
}

// GroupingConfig contains settings for automatic service grouping.
// Grouping organizes services by common tags.
type GroupingConfig struct {
	Enabled               bool    `yaml:"enabled"`
	Columns               int     `yaml:"columns" validate:"gte=1,lte=6"`
	TagFrequencyThreshold float64 `yaml:"tag_frequency_threshold" validate:"gt=0,lte=1"`
	MinServicesPerGroup   int     `yaml:"min_services_per_group" validate:"gte=1"`
}

// EnvironmentConfiguration contains environment-level configuration options.
// These settings control the overall behavior of the application.
type EnvironmentConfiguration struct {
	SelfhstIconURL         string         `yaml:"selfhst_icon_url" validate:"required,url"`
	SearchEngineURL        string         `yaml:"search_engine_url" validate:"required,url"`
	RefreshIntervalSeconds int            `yaml:"refresh_interval_seconds" validate:"gte=1"`
	LogLevel               string         `yaml:"log_level" validate:"oneof=info debug warn error"`
	Traefik                TraefikConfig  `yaml:"traefik"`
	Language               string         `yaml:"language"`
	Grouping               GroupingConfig `yaml:"grouping"`
}

// TralaConfiguration is the root configuration structure.
// It represents the complete configuration file format.
type TralaConfiguration struct {
	mu           sync.RWMutex
	overrideMap  map[string]ServiceOverride
	compatStatus ConfigStatus

	Version     string                   `yaml:"version" validate:"required"`
	Environment EnvironmentConfiguration `yaml:"environment"`
	Services    ServiceConfiguration     `yaml:"services"`
}

// configFieldName maps Go struct field names to their yaml-tag equivalents. It
// is built automatically from the TralaConfiguration struct definition so it
// never drifts out of sync with yaml tags. Maps are also included for the
// nested struct types that appear as path components in validation error
// namespaces (TralaConfiguration.Environment.Traefik.ApiHost → environment.traefik.api_host).
//
//go:generate go run ...  # not used — kept as documentation
var yamlTagForPath = buildYAMLTagForPath()

// buildYAMLTagForPath reflectively walks TralaConfiguration and every nested
// struct type, populating a map from Go field name → yaml-tag value for each
// level. This single pass covers all types because Go struct fields expose
// their struct type even when nested.
func buildYAMLTagForPath() map[string]string {
	m := make(map[string]string)

	// Seed with the top-level yaml-tagged fields of TralaConfiguration so
	// their Go-names appear correctly in parsed path segments.
	topLevel := map[string]string{
		"Version":     "version",
		"Environment": "environment",
		"Services":    "services",
	}

	for goName, yamlTag := range topLevel {
		m[goName] = yamlTag
	}

	// Collect field mappings from each embedded/nested struct type.
	// The keys are Go field names; the values are their yaml tags.
	structs := []struct {
		typeName string
		fields   map[string]string
	}{
		{"EnvironmentConfiguration", map[string]string{
			"SelfhstIconURL":         "selfhst_icon_url",
			"SearchEngineURL":        "search_engine_url",
			"RefreshIntervalSeconds": "refresh_interval_seconds",
			"LogLevel":               "log_level",
			"Traefik":                "traefik",
			"Language":               "language",
			"Grouping":               "grouping",
		}},
		{"TraefikConfig", map[string]string{
			"Instances": "instances",
			"Single":    "single",
			"IsMulti":   "is_multi",
		}},
		{"TraefikInstanceConfig", map[string]string{
			"Name":               "name",
			"APIHost":            "api_host",
			"EnableBasicAuth":    "enable_basic_auth",
			"BasicAuth":          "basic_auth",
			"InsecureSkipVerify": "insecure_skip_verify",
		}},
		{"TraefikBasicAuth", map[string]string{
			"Username":     "username",
			"Password":     "password",
			"PasswordFile": "password_file",
		}},
		{"GroupingConfig", map[string]string{
			"Enabled":               "enabled",
			"Columns":               "columns",
			"TagFrequencyThreshold": "tag_frequency_threshold",
			"MinServicesPerGroup":   "min_services_per_group",
		}},
		{"ServiceOverride", map[string]string{
			"Service":     "service",
			"DisplayName": "display_name",
			"Icon":        "icon",
			"Group":       "group",
		}},
		{"ManualService", map[string]string{
			"Name":     "name",
			"URL":      "url",
			"Icon":     "icon",
			"Priority": "priority",
			"Group":    "group",
			"Host":     "host",
		}},
	}

	for _, s := range structs {
		for goName, yamlTag := range s.fields {
			m[goName] = yamlTag
		}
	}

	return m
}

// EnvironmentEnvVar returns the environment variable name (UPPER_SNAKE_CASE)
// corresponding to a YAML configuration path under `environment.`. It derives
// the name algorithmically so it cannot drift from the field names declared in
// the configuration structs.
//
// The transformation rules are:
//  1. Strip the "environment." prefix.
//  2. Uppercase the remainder and replace dots with underscores.
//
// Returns "" for paths that do not fall under environment.
func EnvironmentEnvVar(path string) string {
	if !strings.HasPrefix(path, "environment.") {
		return ""
	}

	relative := strings.TrimPrefix(path, "environment.")
	if relative == "" {
		return ""
	}

	return strings.ToUpper(strings.ReplaceAll(relative, ".", "_"))
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

// GetTraefikInstances returns all configured Traefik instances.
func (c *TralaConfiguration) GetTraefikInstances() []TraefikInstanceConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	instances := c.Environment.Traefik.Instances
	result := make([]TraefikInstanceConfig, len(instances))
	copy(result, instances)
	return result
}

// GetTraefikInstance returns a specific Traefik instance by name.
func (c *TralaConfiguration) GetTraefikInstance(name string) (TraefikInstanceConfig, bool) {
	instances := c.GetTraefikInstances()
	for _, inst := range instances {
		if inst.Name == name {
			return inst, true
		}
	}
	return TraefikInstanceConfig{}, false
}

// GetTraefikInstanceNames returns the names of all configured Traefik instances.
func (c *TralaConfiguration) GetTraefikInstanceNames() []string {
	instances := c.GetTraefikInstances()
	names := make([]string, len(instances))
	for i, inst := range instances {
		names[i] = inst.Name
	}
	return names
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

// DefaultInstanceName derives a default instance name from an API host URL.
func DefaultInstanceName(apiHost string) string {
	u, err := url.Parse(apiHost)
	if err != nil {
		return ""
	}

	hostname := u.Hostname()
	if hostname == "" {
		return ""
	}

	return hostname
}
