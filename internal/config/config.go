// Package config handles loading, validation, and access to the Trala dashboard configuration.
// It provides thread-safe access to configuration values and validates configuration compatibility.
package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"server/internal/models"

	"go.yaml.in/yaml/v4"
)

// Minimum supported configuration version
const MinimumConfigVersion = "3.0"

// Configuration file path
const ConfigurationFilePath = "/config/configuration.yml"

// Global configuration instance and mutex for thread-safe access
var (
	configuration             models.TralaConfiguration
	configurationMux          sync.RWMutex
	configCompatibilityStatus models.ConfigStatus
	serviceOverrideMap        map[string]models.ServiceOverride
)

// Load loads the configuration from file and environment variables.
// It applies defaults, loads from file, overrides from environment, and validates.
func Load() {
	// Step 1: defaults
	config := models.TralaConfiguration{
		Version: "",
		Environment: models.EnvironmentConfiguration{
			SelfhstIconURL:         "https://cdn.jsdelivr.net/gh/selfhst/icons/",
			SearchEngineURL:        "https://www.google.com/search?q=",
			RefreshIntervalSeconds: 30,
			LogLevel:               "info",
			Traefik: models.TraefikConfig{
				APIHost:            "",
				EnableBasicAuth:    false,
				InsecureSkipVerify: false,
				BasicAuth: models.TraefikBasicAuth{
					Username:     "",
					Password:     "",
					PasswordFile: "",
				},
			},
			Grouping: models.GroupingConfig{
				Enabled:               true,
				Columns:               3,
				TagFrequencyThreshold: 0.9,
				MinServicesPerGroup:   2,
			},
		},
		Services: models.ServiceConfiguration{
			Exclude: models.ExcludeConfig{
				Routers:     []string{},
				Entrypoints: []string{},
			},
			Overrides: make([]models.ServiceOverride, 0),
			Manual:    make([]models.ManualService, 0),
		},
	}

	// Step 2: configuration file
	data, err := os.ReadFile(ConfigurationFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Info: No configuration file found at %s. Using defaults + env vars.", ConfigurationFilePath)
			config.Version = MinimumConfigVersion // Set to minimum required if no config file
		} else {
			log.Printf("Warning: Could not read configuration file at %s: %v", ConfigurationFilePath, err)
		}
	} else {
		if err := yaml.Unmarshal(data, &config); err != nil {
			log.Printf("Warning: Could not parse configuration file %s: %v", ConfigurationFilePath, err)
		}
	}

	// Step 3: validate basic auth password configuration before environment overrides
	// This ensures we check both the original config values and environment variables
	basicAuthWarning := ValidateBasicAuthPassword(config.Environment.Traefik)
	if basicAuthWarning != "" {
		log.Printf("WARNING: %s", basicAuthWarning)
	}

	// Step 4: environment overrides
	if v := os.Getenv("SELFHST_ICON_URL"); v != "" {
		config.Environment.SelfhstIconURL = v
	}
	if v := os.Getenv("SEARCH_ENGINE_URL"); v != "" {
		config.Environment.SearchEngineURL = v
	}
	if v := os.Getenv("REFRESH_INTERVAL_SECONDS"); v != "" {
		if num, err := strconv.Atoi(v); err == nil && num > 0 {
			config.Environment.RefreshIntervalSeconds = num
		} else {
			log.Printf("Warning: Invalid REFRESH_INTERVAL_SECONDS '%s', using %d", v, config.Environment.RefreshIntervalSeconds)
		}
	}
	if v := os.Getenv("TRAEFIK_API_HOST"); v != "" {
		config.Environment.Traefik.APIHost = v
	}
	if v := os.Getenv("TRAEFIK_BASIC_AUTH_USERNAME"); v != "" {
		config.Environment.Traefik.BasicAuth.Username = v
	}
	if v := os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD"); v != "" {
		config.Environment.Traefik.BasicAuth.Password = v
	}
	if v := os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD_FILE"); v != "" {
		config.Environment.Traefik.BasicAuth.PasswordFile = v
	}
	if v := os.Getenv("TRAEFIK_INSECURE_SKIP_VERIFY"); v != "" {
		if skipVerify, err := strconv.ParseBool(v); err == nil {
			config.Environment.Traefik.InsecureSkipVerify = skipVerify
		} else {
			log.Printf("Warning: Invalid TRAEFIK_INSECURE_SKIP_VERIFY '%s', using %t", v, config.Environment.Traefik.InsecureSkipVerify)
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		config.Environment.LogLevel = v
	}
	if v := os.Getenv("LANGUAGE"); v != "" {
		config.Environment.Language = v
	}
	if v := os.Getenv("GROUPING_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			config.Environment.Grouping.Enabled = enabled
		} else {
			log.Printf("Warning: Invalid GROUPING_ENABLED '%s', using %t", v, config.Environment.Grouping.Enabled)
		}
	}
	if v := os.Getenv("GROUPING_TAG_FREQUENCY_THRESHOLD"); v != "" {
		if num, err := strconv.ParseFloat(v, 64); err == nil && num > 0 && num <= 1 {
			config.Environment.Grouping.TagFrequencyThreshold = num
		} else {
			log.Printf("Warning: Invalid GROUPING_TAG_FREQUENCY_THRESHOLD '%s', using %f", v, config.Environment.Grouping.TagFrequencyThreshold)
		}
	}
	if v := os.Getenv("GROUPING_MIN_SERVICES_PER_GROUP"); v != "" {
		if num, err := strconv.Atoi(v); err == nil && num >= 1 {
			config.Environment.Grouping.MinServicesPerGroup = num
		} else {
			log.Printf("Warning: Invalid GROUPING_MIN_SERVICES_PER_GROUP '%s', must be >= 1, using %d", v, config.Environment.Grouping.MinServicesPerGroup)
		}
	}
	if v := os.Getenv("GROUPED_COLUMNS"); v != "" {
		if num, err := strconv.Atoi(v); err == nil && num >= 1 && num <= 6 {
			config.Environment.Grouping.Columns = num
		} else {
			log.Printf("Warning: Invalid GROUPED_COLUMNS '%s', must be between 1 and 6, using %d", v, config.Environment.Grouping.Columns)
		}
	}

	// Step 5: post-processing / validation
	if config.Environment.Traefik.APIHost == "" {
		log.Printf("ERROR: Traefik API host is not set. Provide via env var or config file.")
		os.Exit(1)
	}
	if !strings.HasPrefix(config.Environment.Traefik.APIHost, "http://") && !strings.HasPrefix(config.Environment.Traefik.APIHost, "https://") {
		config.Environment.Traefik.APIHost = "http://" + config.Environment.Traefik.APIHost
	}
	if !strings.HasSuffix(config.Environment.SelfhstIconURL, "/") {
		config.Environment.SelfhstIconURL += "/"
	}

	if config.Environment.Traefik.EnableBasicAuth {
		if config.Environment.Traefik.BasicAuth.Username == "" || (config.Environment.Traefik.BasicAuth.Password == "" && config.Environment.Traefik.BasicAuth.PasswordFile == "") {
			log.Printf("ERROR: Basic auth is enabled, but basic auth username, password or password file is not set!")
			os.Exit(1)
		}
		if config.Environment.Traefik.BasicAuth.Password != "" && config.Environment.Traefik.BasicAuth.PasswordFile != "" {
			log.Printf("WARNING: Basic auth password and password file is set, content of file will take precedence over password!")
		}
	}

	passwordFilePath := config.Environment.Traefik.BasicAuth.PasswordFile
	if config.Environment.Traefik.EnableBasicAuth && passwordFilePath != "" {
		data, err := os.ReadFile(passwordFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("ERROR: No password file found at %s for basic auth.", passwordFilePath)
				os.Exit(1)
			} else {
				log.Printf("ERROR: Could not read password file at %s: %v", passwordFilePath, err)
				os.Exit(1)
			}
		} else {
			config.Environment.Traefik.BasicAuth.Password = string(data)
		}
	}

	// Build map that maps a router name to a ServiceOverride for fast lookups
	serviceOverrideMap = make(map[string]models.ServiceOverride, len(config.Services.Overrides))
	for _, o := range config.Services.Overrides {
		serviceOverrideMap[o.Service] = o
	}

	log.Printf("Loaded %d router excludes from %s", len(config.Services.Exclude.Routers), ConfigurationFilePath)
	log.Printf("Loaded %d entrypoint excludes from %s", len(config.Services.Exclude.Entrypoints), ConfigurationFilePath)
	log.Printf("Loaded %d service overrides from %s", len(config.Services.Overrides), ConfigurationFilePath)

	// Validate configuration version (without basic auth validation since we already did it above)
	configCompatibilityStatus = ValidateConfigVersion(config.Version, basicAuthWarning)
	if !configCompatibilityStatus.IsCompatible {
		log.Printf("WARNING: %s", configCompatibilityStatus.WarningMessage)
	}

	// Now that all validation is complete, lock the mutex and update the global configuration
	configurationMux.Lock()
	defer configurationMux.Unlock()

	configuration = config

	if config.Environment.LogLevel == "debug" {
		log.Printf("Using effective configuration:")
		out, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Failed to marshal configuration: %v\n", err)
			return
		}
		fmt.Println(string(out))
	}
}

// ValidateConfigVersion checks if the configuration version is compatible.
// It returns a ConfigStatus indicating compatibility and any warning messages.
func ValidateConfigVersion(configVersion string, basicAuthWarning string) models.ConfigStatus {
	status := models.ConfigStatus{
		ConfigVersion:          configVersion,
		MinimumRequiredVersion: MinimumConfigVersion,
		IsCompatible:           true,
	}

	// Check if configuration version is specified
	if configVersion == "" {
		status.IsCompatible = false
		status.WarningMessage = "No configuration version specified. Please add 'version: X.Y' to your configuration file."
		return status
	}

	// Compare versions
	if CompareVersions(configVersion, MinimumConfigVersion) < 0 {
		status.IsCompatible = false
		status.WarningMessage = fmt.Sprintf("Configuration version %s is below the minimum required version %s. Some configuration options may be ignored.", configVersion, MinimumConfigVersion)
	}

	// Merge with basic auth warning if present
	if basicAuthWarning != "" {
		// If there's already a warning message, append to it
		if status.WarningMessage != "" {
			status.WarningMessage += " " + basicAuthWarning
		} else {
			status.WarningMessage = basicAuthWarning
		}
	}

	return status
}

// ValidateBasicAuthPassword checks if the basic auth password is configured using only one method.
// Returns a warning message if multiple password sources are configured.
func ValidateBasicAuthPassword(config models.TraefikConfig) string {
	// If basic auth is not enabled, no validation needed
	if !config.EnableBasicAuth {
		return ""
	}

	// Count the number of password sources that are set
	passwordSources := 0

	// Check config file password
	if config.BasicAuth.Password != "" {
		passwordSources++
	}

	// Check config file password file
	if config.BasicAuth.PasswordFile != "" {
		passwordSources++
	}

	// Check environment variable password
	if os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD") != "" {
		passwordSources++
	}

	// Check environment variable password file
	if os.Getenv("TRAEFIK_BASIC_AUTH_PASSWORD_FILE") != "" {
		passwordSources++
	}

	// If more than one password source is configured, it's a warning
	if passwordSources > 1 {
		return "Basic auth password is configured using multiple methods. Please use only one method: either password in config file, password file, or environment variable."
	}

	return ""
}

// CompareVersions compares two version strings using semantic versioning.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	// Normalize versions by ensuring they have 3 components (major.minor.patch)
	normalizeVersion := func(v string) []int {
		parts := strings.Split(v, ".")
		result := make([]int, 3)
		for i := 0; i < 3; i++ {
			if i < len(parts) {
				if num, err := strconv.Atoi(parts[i]); err == nil {
					result[i] = num
				}
			}
			// Missing parts default to 0
		}
		return result
	}

	v1Parts := normalizeVersion(v1)
	v2Parts := normalizeVersion(v2)

	for i := 0; i < 3; i++ {
		if v1Parts[i] < v2Parts[i] {
			return -1
		} else if v1Parts[i] > v2Parts[i] {
			return 1
		}
	}
	return 0
}

// IsValidUrl checks if a string is a valid URL with scheme and host.
func IsValidUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// --- Configuration Accessors ---

// GetTraefikAPIHost returns the Traefik API host URL.
func GetTraefikAPIHost() string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Traefik.APIHost
}

// GetSelfhstIconURL returns the base URL for selfh.st icons.
func GetSelfhstIconURL() string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.SelfhstIconURL
}

// GetLogLevel returns the configured log level.
func GetLogLevel() string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.LogLevel
}

// GetLanguage returns the configured language code.
func GetLanguage() string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Language
}

// GetSearchEngineURL returns the search engine URL template.
func GetSearchEngineURL() string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.SearchEngineURL
}

// GetRefreshIntervalSeconds returns the refresh interval in seconds.
func GetRefreshIntervalSeconds() int {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.RefreshIntervalSeconds
}

// GetGroupingEnabled returns whether grouping is enabled.
func GetGroupingEnabled() bool {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Grouping.Enabled
}

// GetGroupingColumns returns the number of columns for grouped display.
func GetGroupingColumns() int {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Grouping.Columns
}

// GetTagFrequencyThreshold returns the tag frequency threshold for grouping.
func GetTagFrequencyThreshold() float64 {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Grouping.TagFrequencyThreshold
}

// GetMinServicesPerGroup returns the minimum services required per group.
func GetMinServicesPerGroup() int {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Grouping.MinServicesPerGroup
}

// GetTraefikConfig returns the complete Traefik configuration.
func GetTraefikConfig() models.TraefikConfig {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Traefik
}

// GetEnableBasicAuth returns whether basic auth is enabled for Traefik API.
func GetEnableBasicAuth() bool {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Traefik.EnableBasicAuth
}

// GetBasicAuthUsername returns the basic auth username.
func GetBasicAuthUsername() string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Traefik.BasicAuth.Username
}

// GetBasicAuthPassword returns the basic auth password.
func GetBasicAuthPassword() string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Traefik.BasicAuth.Password
}

// GetInsecureSkipVerify returns whether SSL verification is skipped.
func GetInsecureSkipVerify() bool {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Environment.Traefik.InsecureSkipVerify
}

// GetServiceOverrideMap returns the map of service overrides by router name.
func GetServiceOverrideMap() map[string]models.ServiceOverride {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return serviceOverrideMap
}

// GetExcludeRouters returns the list of router exclusion patterns.
func GetExcludeRouters() []string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Services.Exclude.Routers
}

// GetExcludeEntrypoints returns the list of entrypoint exclusion patterns.
func GetExcludeEntrypoints() []string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Services.Exclude.Entrypoints
}

// GetManualServices returns the list of manually configured services.
func GetManualServices() []models.ManualService {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration.Services.Manual
}

// GetConfigCompatibilityStatus returns the configuration compatibility status.
func GetConfigCompatibilityStatus() models.ConfigStatus {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configCompatibilityStatus
}

// GetConfiguration returns a copy of the complete configuration.
// This should be used sparingly as it returns the entire config.
func GetConfiguration() models.TralaConfiguration {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	return configuration
}

// GetServiceOverride looks up a service override by router name.
// Returns the override and true if found, or empty override and false if not.
func GetServiceOverride(routerName string) (models.ServiceOverride, bool) {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	override, ok := serviceOverrideMap[routerName]
	return override, ok
}

// GetIconOverride returns the icon override for a router name, or empty string if none.
func GetIconOverride(routerName string) string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	if override, ok := serviceOverrideMap[routerName]; ok {
		return override.Icon
	}
	return ""
}

// GetDisplayNameOverride returns the display name override for a router name, or empty string if none.
func GetDisplayNameOverride(routerName string) string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	if override, ok := serviceOverrideMap[routerName]; ok {
		return override.DisplayName
	}
	return ""
}

// GetGroupOverride returns the group override for a router name, or empty string if none.
func GetGroupOverride(routerName string) string {
	configurationMux.RLock()
	defer configurationMux.RUnlock()
	if override, ok := serviceOverrideMap[routerName]; ok {
		return override.Group
	}
	return ""
}
