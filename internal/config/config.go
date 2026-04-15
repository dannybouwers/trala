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

	"go.yaml.in/yaml/v4"
)

// Minimum supported configuration version
const MinimumConfigVersion = "3.0"

// Configuration file path
const ConfigurationFilePath = "/config/configuration.yml"

// Load loads the configuration from file and environment variables.
// It applies defaults, loads from file, overrides from environment, and validates.
func NewTralaConfiguration() *TralaConfiguration {
	// Step 1: defaults
	config := TralaConfiguration{
		Version: "",
		Environment: EnvironmentConfiguration{
			SelfhstIconURL:         "https://cdn.jsdelivr.net/gh/selfhst/icons/",
			SearchEngineURL:        "https://www.google.com/search?q=",
			RefreshIntervalSeconds: 30,
			LogLevel:               "info",
			Traefik: TraefikConfig{
				APIHost:            "",
				EnableBasicAuth:    false,
				InsecureSkipVerify: false,
				BasicAuth: TraefikBasicAuth{
					Username:     "",
					Password:     "",
					PasswordFile: "",
				},
			},
			Grouping: GroupingConfig{
				Enabled:               true,
				Columns:               3,
				TagFrequencyThreshold: 0.9,
				MinServicesPerGroup:   2,
			},
		},
		Services: ServiceConfiguration{
			Exclude: ExcludeConfig{
				Routers:     []string{},
				Entrypoints: []string{},
			},
			Overrides: make([]ServiceOverride, 0),
			Manual:    make([]ManualService, 0),
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
			log.Printf("ERROR: Failed to parse configuration file: %v", err)
			log.Printf("FATAL: The configuration file contains invalid YAML. Please check the syntax.")
			log.Printf("HINT: Common issues include:")
			log.Printf("  - Incorrect indentation (use spaces, not tabs)")
			log.Printf("  - Missing colons after field names")
			log.Printf("  - Unquoted strings with special characters")
			os.Exit(1)
		}

		// After successful YAML unmarshal, add debug logging
		// Use the log level that was potentially set in config file (if any)
		debugLog := func(format string, v ...interface{}) {
			// Use the log level set in the config file (defaults to "info" if not set)
			if config.Environment.LogLevel == "debug" {
				log.Printf("DEBUG: "+format, v...)
			}
		}

		debugLog("Successfully parsed configuration file:")
		debugLog("  - Version: %s", config.Version)
		debugLog("  - Exclude routers: %v", config.Services.Exclude.Routers)
		debugLog("  - Exclude entrypoints: %v", config.Services.Exclude.Entrypoints)
		debugLog("  - Service overrides: %d items", len(config.Services.Overrides))
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

	// Validate LOG_LEVEL
	validLogLevels := map[string]bool{"info": true, "debug": true, "warn": true, "error": true}
	if config.Environment.LogLevel != "" && !validLogLevels[config.Environment.LogLevel] {
		log.Printf("Warning: Unknown LOG_LEVEL '%s', defaulting to 'info'", config.Environment.LogLevel)
		config.Environment.LogLevel = "info"
	}

	// After environment overrides, log effective configuration
	debugLogEffectiveConfig := func(format string, v ...interface{}) {
		if config.Environment.LogLevel == "debug" {
			log.Printf("DEBUG: "+format, v...)
		}
	}

	debugLogEffectiveConfig("=== Effective Configuration ===")
	debugLogEffectiveConfig("Traefik API: %s", config.Environment.Traefik.APIHost)
	debugLogEffectiveConfig("Log Level: %s", config.Environment.LogLevel)
	debugLogEffectiveConfig("Language: %s", config.Environment.Language)
	debugLogEffectiveConfig("Refresh Interval: %d seconds", config.Environment.RefreshIntervalSeconds)
	debugLogEffectiveConfig("Grouping Enabled: %t", config.Environment.Grouping.Enabled)
	debugLogEffectiveConfig("Grouping Columns: %d", config.Environment.Grouping.Columns)
	debugLogEffectiveConfig("Excluded routers: %v", config.Services.Exclude.Routers)
	debugLogEffectiveConfig("Excluded entrypoints: %v", config.Services.Exclude.Entrypoints)
	debugLogEffectiveConfig("Service overrides: %d", len(config.Services.Overrides))

	// Log each service override individually
	for _, o := range config.Services.Overrides {
		debugLogEffectiveConfig("Override: %s -> name=%s, icon=%s, group=%s",
			o.Service, o.DisplayName, o.Icon, o.Group)
	}

	// Log manual services
	debugLogEffectiveConfig("Manual services: %d", len(config.Services.Manual))
	for _, m := range config.Services.Manual {
		debugLogEffectiveConfig("Manual: %s -> name=%s, url=%s, icon=%s, group=%s",
			m.Name, m.Name, m.URL, m.Icon, m.Group)
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
			config.Environment.Traefik.BasicAuth.Password = strings.TrimSpace(string(data))
		}
	}

	log.Printf("Loaded %d router excludes from %s", len(config.Services.Exclude.Routers), ConfigurationFilePath)
	log.Printf("Loaded %d entrypoint excludes from %s", len(config.Services.Exclude.Entrypoints), ConfigurationFilePath)
	log.Printf("Loaded %d service overrides from %s", len(config.Services.Overrides), ConfigurationFilePath)

	// Validate configuration version (without basic auth validation since we already did it above)
	status := ValidateConfigVersion(config.Version, basicAuthWarning)
	if !status.IsCompatible {
		log.Printf("WARNING: %s", status.WarningMessage)
	}

	// Now that all validation is complete, lock the mutex and store state on the instance
	config.mu.Lock()
	defer config.mu.Unlock()

	config.compatStatus = status

	// Build map that maps a router name to a ServiceOverride for fast lookups (inside lock)
	config.overrideMap = make(map[string]ServiceOverride, len(config.Services.Overrides))
	for _, o := range config.Services.Overrides {
		config.overrideMap[o.Service] = o
	}

	if config.Environment.LogLevel == "debug" {
		log.Printf("Using effective configuration:")
		out, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Failed to marshal configuration: %v\n", err)
			return nil
		}
		output := string(out)
		if config.Environment.Traefik.BasicAuth.Password != "" {
			output = strings.ReplaceAll(output, config.Environment.Traefik.BasicAuth.Password, "***REDACTED***")
		}
		if config.Environment.Traefik.BasicAuth.PasswordFile != "" {
			output = strings.ReplaceAll(output, config.Environment.Traefik.BasicAuth.PasswordFile, "***REDACTED***")
		}
		fmt.Println(output)
	}

	return &config
}

// ValidateConfigVersion checks if the configuration version is compatible.
// It returns a ConfigStatus indicating compatibility and any warning messages.
func ValidateConfigVersion(configVersion string, basicAuthWarning string) ConfigStatus {
	status := ConfigStatus{
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
func ValidateBasicAuthPassword(config TraefikConfig) string {
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

