package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test helpers ---

// writeConfigFile stages a YAML config in a temp dir and returns its path.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "configuration.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// nonExistentPath returns a path inside a temp dir that does not exist.
func nonExistentPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "does-not-exist.yml")
}

// clearConfigEnv unsets every env var the loader reads, so a test starts from a
// clean slate regardless of the invoking shell. Uses t.Setenv so cleanup is
// automatic; also disables t.Parallel.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	vars := []string{
		"SELFHST_ICON_URL",
		"SEARCH_ENGINE_URL",
		"REFRESH_INTERVAL_SECONDS",
		"TRAEFIK_API_HOST",
		"TRAEFIK_BASIC_AUTH_USERNAME",
		"TRAEFIK_BASIC_AUTH_PASSWORD",
		"TRAEFIK_BASIC_AUTH_PASSWORD_FILE",
		"TRAEFIK_INSECURE_SKIP_VERIFY",
		"LOG_LEVEL",
		"LANGUAGE",
		"GROUPING_ENABLED",
		"GROUPING_TAG_FREQUENCY_THRESHOLD",
		"GROUPING_MIN_SERVICES_PER_GROUP",
		"GROUPED_COLUMNS",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}
}

// newPopulatedConfig returns a TralaConfiguration with every field set to known,
// distinct values plus a populated overrideMap and compatStatus — used by the
// getter-contract tests so we test the accessor API without going through the
// loader.
func newPopulatedConfig() *TralaConfiguration {
	overrides := []ServiceOverride{
		{Service: "svc-a", DisplayName: "Service A", Icon: "icon-a", Group: "group-a"},
		{Service: "svc-b", DisplayName: "Service B", Icon: "icon-b", Group: "group-b"},
	}
	c := &TralaConfiguration{
		Version: "3.1",
		Environment: EnvironmentConfiguration{
			SelfhstIconURL:         "https://icons.example/",
			SearchEngineURL:        "https://search.example/?q=",
			RefreshIntervalSeconds: 42,
			LogLevel:               "debug",
			Language:               "nl",
			Traefik: TraefikConfig{
				APIHost:            "https://traefik.example",
				EnableBasicAuth:    true,
				InsecureSkipVerify: true,
				BasicAuth: TraefikBasicAuth{
					Username:     "alice",
					Password:     "s3cret",
					PasswordFile: "/etc/secrets/pw",
				},
			},
			Grouping: GroupingConfig{
				Enabled:               true,
				Columns:               4,
				TagFrequencyThreshold: 0.75,
				MinServicesPerGroup:   3,
			},
		},
		Services: ServiceConfiguration{
			Exclude: ExcludeConfig{
				Routers:     []string{"r1", "r2"},
				Entrypoints: []string{"e1"},
			},
			Overrides: overrides,
			Manual: []ManualService{
				{Name: "m1", URL: "https://m1.example", Icon: "mi", Priority: 1, Group: "mg"},
			},
		},
	}
	c.compatStatus = ConfigStatus{
		ConfigVersion:          "3.1",
		MinimumRequiredVersion: MinimumConfigVersion,
		IsCompatible:           true,
	}
	c.overrideMap = make(map[string]ServiceOverride, len(overrides))
	for _, o := range overrides {
		c.overrideMap[o.Service] = o
	}
	return c
}

// --- Pure function tests (parallel) ---

func TestCompareVersions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{"equal three parts", "3.0.0", "3.0.0", 0},
		{"equal two parts", "3.0", "3.0", 0},
		{"normalized equal", "3", "3.0.0", 0},
		{"major less", "2.0.0", "3.0.0", -1},
		{"major greater", "4.0.0", "3.0.0", 1},
		{"minor less", "3.0.0", "3.1.0", -1},
		{"minor greater", "3.2.0", "3.1.0", 1},
		{"patch less", "3.0.0", "3.0.1", -1},
		{"patch greater", "3.0.2", "3.0.1", 1},
		{"non-numeric treated as zero", "abc", "0.0.0", 0},
		{"mixed non-numeric", "3.x.0", "3.0.0", 0},
		{"empty vs zero", "", "0.0.0", 0},
		{"empty vs real", "", "3.0.0", -1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CompareVersions(tc.v1, tc.v2)
			assert.Equalf(t, tc.want, got, "CompareVersions(%q, %q)", tc.v1, tc.v2)
		})
	}
}

func TestIsValidUrl(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"https with host", "https://example.com", true},
		{"http with host and path", "http://example.com/foo", true},
		{"no scheme", "example.com", false},
		{"no host", "http://", false},
		{"empty", "", false},
		{"garbage", "::::", false},
		{"scheme only", "http:", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equalf(t, tc.want, IsValidUrl(tc.in), "IsValidUrl(%q)", tc.in)
		})
	}
}

func TestValidateConfigVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		version          string
		basicAuthWarning string
		wantCompat       bool
		wantMsgContains  string
	}{
		{
			name:            "empty version",
			version:         "",
			wantCompat:      false,
			wantMsgContains: "No configuration version specified",
		},
		{
			name:            "below minimum",
			version:         "2.9",
			wantCompat:      false,
			wantMsgContains: "below the minimum required version",
		},
		{
			name:       "exactly minimum",
			version:    MinimumConfigVersion,
			wantCompat: true,
		},
		{
			name:       "above minimum",
			version:    "4.2.1",
			wantCompat: true,
		},
		{
			name:             "compatible with basic auth warning",
			version:          "3.5",
			basicAuthWarning: "multi-source warning",
			wantCompat:       true,
			wantMsgContains:  "multi-source warning",
		},
		{
			name:             "incompatible merges basic auth warning",
			version:          "2.0",
			basicAuthWarning: "multi-source warning",
			wantCompat:       false,
			wantMsgContains:  "multi-source warning",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ValidateConfigVersion(tc.version, tc.basicAuthWarning)
			assert.Equal(t, tc.version, got.ConfigVersion)
			assert.Equal(t, MinimumConfigVersion, got.MinimumRequiredVersion)
			assert.Equal(t, tc.wantCompat, got.IsCompatible)
			if tc.wantMsgContains != "" {
				assert.Contains(t, got.WarningMessage, tc.wantMsgContains)
			}
		})
	}

	t.Run("incompatible version merges with basic auth warning", func(t *testing.T) {
		t.Parallel()
		got := ValidateConfigVersion("2.0", "auth warn")
		assert.False(t, got.IsCompatible)
		assert.Contains(t, got.WarningMessage, "below the minimum")
		assert.Contains(t, got.WarningMessage, "auth warn")
	})
}

func TestValidateBasicAuthPassword(t *testing.T) {
	// Uses t.Setenv: cannot run t.Parallel.
	cases := []struct {
		name        string
		cfg         TraefikConfig
		envPassword string
		envPwFile   string
		wantWarning bool
	}{
		{
			name: "disabled returns empty regardless of env",
			cfg: TraefikConfig{
				EnableBasicAuth: false,
				BasicAuth:       TraefikBasicAuth{Password: "x", PasswordFile: "/y"},
			},
			envPassword: "env-pass",
			envPwFile:   "/env-pw-file",
			wantWarning: false,
		},
		{
			name: "only config password",
			cfg: TraefikConfig{
				EnableBasicAuth: true,
				BasicAuth:       TraefikBasicAuth{Password: "x"},
			},
			wantWarning: false,
		},
		{
			name: "only config password file",
			cfg: TraefikConfig{
				EnableBasicAuth: true,
				BasicAuth:       TraefikBasicAuth{PasswordFile: "/y"},
			},
			wantWarning: false,
		},
		{
			name: "only env password",
			cfg: TraefikConfig{
				EnableBasicAuth: true,
			},
			envPassword: "env-pass",
			wantWarning: false,
		},
		{
			name: "only env password file",
			cfg: TraefikConfig{
				EnableBasicAuth: true,
			},
			envPwFile:   "/env-pw-file",
			wantWarning: false,
		},
		{
			name: "config password + env password",
			cfg: TraefikConfig{
				EnableBasicAuth: true,
				BasicAuth:       TraefikBasicAuth{Password: "x"},
			},
			envPassword: "env-pass",
			wantWarning: true,
		},
		{
			name: "config password + config password file",
			cfg: TraefikConfig{
				EnableBasicAuth: true,
				BasicAuth:       TraefikBasicAuth{Password: "x", PasswordFile: "/y"},
			},
			wantWarning: true,
		},
		{
			name: "all four sources",
			cfg: TraefikConfig{
				EnableBasicAuth: true,
				BasicAuth:       TraefikBasicAuth{Password: "x", PasswordFile: "/y"},
			},
			envPassword: "env-pass",
			envPwFile:   "/env-pw-file",
			wantWarning: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TRAEFIK_BASIC_AUTH_PASSWORD", tc.envPassword)
			t.Setenv("TRAEFIK_BASIC_AUTH_PASSWORD_FILE", tc.envPwFile)
			got := ValidateBasicAuthPassword(tc.cfg)
			if tc.wantWarning {
				assert.NotEmpty(t, got, "expected a multi-source warning")
				assert.Contains(t, got, "multiple methods")
			} else {
				assert.Empty(t, got, "expected no warning")
			}
		})
	}
}

// --- Getter contract tests (parallel) ---

func TestTralaConfiguration_Getters(t *testing.T) {
	t.Parallel()
	c := newPopulatedConfig()

	assert.Equal(t, "https://traefik.example", c.GetTraefikAPIHost())
	assert.Equal(t, "https://icons.example/", c.GetSelfhstIconURL())
	assert.Equal(t, "debug", c.GetLogLevel())
	assert.Equal(t, "nl", c.GetLanguage())
	assert.Equal(t, "https://search.example/?q=", c.GetSearchEngineURL())
	assert.Equal(t, 42, c.GetRefreshIntervalSeconds())
	assert.True(t, c.GetGroupingEnabled())
	assert.Equal(t, 4, c.GetGroupingColumns())
	assert.InDelta(t, 0.75, c.GetTagFrequencyThreshold(), 1e-9)
	assert.Equal(t, 3, c.GetMinServicesPerGroup())
	assert.True(t, c.GetEnableBasicAuth())
	assert.Equal(t, "alice", c.GetBasicAuthUsername())
	assert.Equal(t, "s3cret", c.GetBasicAuthPassword())
	assert.True(t, c.GetInsecureSkipVerify())

	tr := c.GetTraefikConfig()
	assert.Equal(t, "https://traefik.example", tr.APIHost)
	assert.Equal(t, "alice", tr.BasicAuth.Username)

	status := c.GetConfigCompatibilityStatus()
	assert.Equal(t, "3.1", status.ConfigVersion)
	assert.Equal(t, MinimumConfigVersion, status.MinimumRequiredVersion)
	assert.True(t, status.IsCompatible)

	cfg := c.GetConfiguration()
	assert.Equal(t, "3.1", cfg.Version)
	assert.Equal(t, "debug", cfg.Environment.LogLevel)
	// GetConfiguration copies only exported fields; internal map/status not included.
	assert.Nil(t, cfg.overrideMap)
}

func TestTralaConfiguration_SliceGettersReturnCopies(t *testing.T) {
	t.Parallel()

	t.Run("GetExcludeRouters returns a copy", func(t *testing.T) {
		t.Parallel()
		c := newPopulatedConfig()
		got := c.GetExcludeRouters()
		require.Equal(t, []string{"r1", "r2"}, got)
		got[0] = "MUTATED"
		assert.Equal(t, []string{"r1", "r2"}, c.Services.Exclude.Routers,
			"mutating the returned slice must not affect internal state")
	})

	t.Run("GetExcludeEntrypoints returns a copy", func(t *testing.T) {
		t.Parallel()
		c := newPopulatedConfig()
		got := c.GetExcludeEntrypoints()
		require.Equal(t, []string{"e1"}, got)
		got[0] = "MUTATED"
		assert.Equal(t, []string{"e1"}, c.Services.Exclude.Entrypoints)
	})

	t.Run("GetManualServices returns a copy", func(t *testing.T) {
		t.Parallel()
		c := newPopulatedConfig()
		got := c.GetManualServices()
		require.Len(t, got, 1)
		got[0].Name = "MUTATED"
		assert.Equal(t, "m1", c.Services.Manual[0].Name)
	})

	t.Run("GetServiceOverrideMap returns a copy", func(t *testing.T) {
		t.Parallel()
		c := newPopulatedConfig()
		got := c.GetServiceOverrideMap()
		require.Len(t, got, 2)
		delete(got, "svc-a")
		got["svc-new"] = ServiceOverride{Service: "svc-new"}
		_, stillThere := c.overrideMap["svc-a"]
		_, leakedIn := c.overrideMap["svc-new"]
		assert.True(t, stillThere, "original map entry must survive")
		assert.False(t, leakedIn, "new entry must not appear in internal map")
	})
}

func TestTralaConfiguration_OverrideLookups(t *testing.T) {
	t.Parallel()

	t.Run("GetServiceOverride", func(t *testing.T) {
		t.Parallel()
		c := newPopulatedConfig()
		got, ok := c.GetServiceOverride("svc-a")
		require.True(t, ok)
		assert.Equal(t, "Service A", got.DisplayName)

		_, ok = c.GetServiceOverride("unknown")
		assert.False(t, ok)
	})

	cases := []struct {
		name   string
		lookup func(*TralaConfiguration, string) string
		router string
		want   string
	}{
		{"icon hit", (*TralaConfiguration).GetIconOverride, "svc-a", "icon-a"},
		{"icon miss", (*TralaConfiguration).GetIconOverride, "nope", ""},
		{"display name hit", (*TralaConfiguration).GetDisplayNameOverride, "svc-b", "Service B"},
		{"display name miss", (*TralaConfiguration).GetDisplayNameOverride, "nope", ""},
		{"group hit", (*TralaConfiguration).GetGroupOverride, "svc-a", "group-a"},
		{"group miss", (*TralaConfiguration).GetGroupOverride, "nope", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newPopulatedConfig()
			assert.Equal(t, tc.want, tc.lookup(c, tc.router))
		})
	}
}

func TestTralaConfiguration_ConcurrentReads(t *testing.T) {
	t.Parallel()
	c := newPopulatedConfig()

	const goroutines = 32
	const iters = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				_ = c.GetTraefikAPIHost()
				_ = c.GetSelfhstIconURL()
				_ = c.GetLogLevel()
				_ = c.GetLanguage()
				_ = c.GetExcludeRouters()
				_ = c.GetExcludeEntrypoints()
				_ = c.GetManualServices()
				_ = c.GetServiceOverrideMap()
				_, _ = c.GetServiceOverride("svc-a")
				_ = c.GetIconOverride("svc-a")
				_ = c.GetConfigCompatibilityStatus()
			}
		}()
	}
	wg.Wait()
}

// --- LoadConfiguration behavioral tests (serial — uses t.Setenv and files) ---

func TestLoadConfiguration_DefaultsWhenFileMissing(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "traefik.local")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)
	require.NotNil(t, conf)

	assert.Equal(t, MinimumConfigVersion, conf.Version)
	assert.Equal(t, 30, conf.GetRefreshIntervalSeconds())
	assert.Equal(t, "info", conf.GetLogLevel())
	assert.Equal(t, "https://cdn.jsdelivr.net/gh/selfhst/icons/", conf.GetSelfhstIconURL())
	assert.Equal(t, "https://www.google.com/search?q=", conf.GetSearchEngineURL())
	assert.True(t, conf.GetGroupingEnabled())
	assert.Equal(t, 3, conf.GetGroupingColumns())
	assert.InDelta(t, 0.9, conf.GetTagFrequencyThreshold(), 1e-9)
	assert.Equal(t, 2, conf.GetMinServicesPerGroup())
	assert.Equal(t, "http://traefik.local", conf.GetTraefikAPIHost(),
		"bare host should be prefixed with http://")
	assert.False(t, conf.GetEnableBasicAuth())
	assert.False(t, conf.GetInsecureSkipVerify())
	assert.Empty(t, conf.GetExcludeRouters())
	assert.Empty(t, conf.GetExcludeEntrypoints())
	assert.Empty(t, conf.GetManualServices())
}

func TestLoadConfiguration_FromYAMLFile(t *testing.T) {
	clearConfigEnv(t)
	yaml := `
version: "3.2"
environment:
  selfhst_icon_url: "https://icons.example"
  search_engine_url: "https://ddg.example/?q="
  refresh_interval_seconds: 15
  log_level: warn
  language: fr
  traefik:
    api_host: "https://traefik.example"
    insecure_skip_verify: true
  grouping:
    enabled: false
    columns: 5
    tag_frequency_threshold: 0.5
    min_services_per_group: 4
services:
  exclude:
    routers:
      - foo@docker
      - bar@docker
    entrypoints:
      - web-secure
  overrides:
    - service: svc-a
      display_name: "Service A"
      icon: icon-a
      group: group-a
    - service: svc-b
      display_name: "Service B"
  manual:
    - name: m1
      url: "https://m1.example"
      icon: mi
      priority: 2
      group: mg
`
	path := writeConfigFile(t, yaml)

	conf, err := LoadConfiguration(path)
	require.NoError(t, err)
	require.NotNil(t, conf)

	assert.Equal(t, "3.2", conf.Version)
	assert.Equal(t, "https://icons.example/", conf.GetSelfhstIconURL(),
		"trailing slash should be appended")
	assert.Equal(t, "https://ddg.example/?q=", conf.GetSearchEngineURL())
	assert.Equal(t, 15, conf.GetRefreshIntervalSeconds())
	assert.Equal(t, "warn", conf.GetLogLevel())
	assert.Equal(t, "fr", conf.GetLanguage())
	assert.Equal(t, "https://traefik.example", conf.GetTraefikAPIHost())
	assert.True(t, conf.GetInsecureSkipVerify())
	assert.False(t, conf.GetGroupingEnabled())
	assert.Equal(t, 5, conf.GetGroupingColumns())
	assert.InDelta(t, 0.5, conf.GetTagFrequencyThreshold(), 1e-9)
	assert.Equal(t, 4, conf.GetMinServicesPerGroup())
	assert.Equal(t, []string{"foo@docker", "bar@docker"}, conf.GetExcludeRouters())
	assert.Equal(t, []string{"web-secure"}, conf.GetExcludeEntrypoints())

	manual := conf.GetManualServices()
	require.Len(t, manual, 1)
	assert.Equal(t, "m1", manual[0].Name)
	assert.Equal(t, "https://m1.example", manual[0].URL)
	assert.Equal(t, 2, manual[0].Priority)

	// Override map is populated and queryable.
	override, ok := conf.GetServiceOverride("svc-a")
	require.True(t, ok)
	assert.Equal(t, "Service A", override.DisplayName)
	assert.Equal(t, "icon-a", override.Icon)
	assert.Equal(t, "group-a", override.Group)

	_, ok = conf.GetServiceOverride("svc-b")
	assert.True(t, ok)
	_, ok = conf.GetServiceOverride("does-not-exist")
	assert.False(t, ok)

	// Version at/above minimum → compatible.
	status := conf.GetConfigCompatibilityStatus()
	assert.True(t, status.IsCompatible)
}

func TestLoadConfiguration_InvalidYAML(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")

	path := writeConfigFile(t, "::: this is not yaml :::\n  bad: [unclosed")
	conf, err := LoadConfiguration(path)
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse configuration file")
}

func TestLoadConfiguration_MissingTraefikHost(t *testing.T) {
	clearConfigEnv(t)

	conf, err := LoadConfiguration(nonExistentPath(t))
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_host")
	assert.Contains(t, err.Error(), "is required")
}

func TestLoadConfiguration_EnvOverrides(t *testing.T) {
	// Base YAML has a traefik host so the loader doesn't fail; env vars should
	// replace each setting. This is a single big test because env vars are
	// process-global via t.Setenv.
	clearConfigEnv(t)
	baseYAML := `
version: "3.0"
environment:
  traefik:
    api_host: "from-file"
`
	path := writeConfigFile(t, baseYAML)

	t.Setenv("SELFHST_ICON_URL", "https://env-icons.example/")
	t.Setenv("SEARCH_ENGINE_URL", "https://env-search.example/?q=")
	t.Setenv("REFRESH_INTERVAL_SECONDS", "77")
	t.Setenv("TRAEFIK_API_HOST", "https://env-traefik.example")
	t.Setenv("TRAEFIK_BASIC_AUTH_USERNAME", "bob")
	t.Setenv("TRAEFIK_BASIC_AUTH_PASSWORD", "envpass")
	t.Setenv("TRAEFIK_INSECURE_SKIP_VERIFY", "true")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LANGUAGE", "de")
	t.Setenv("GROUPING_ENABLED", "false")
	t.Setenv("GROUPING_TAG_FREQUENCY_THRESHOLD", "0.25")
	t.Setenv("GROUPING_MIN_SERVICES_PER_GROUP", "5")
	t.Setenv("GROUPED_COLUMNS", "6")

	conf, err := LoadConfiguration(path)
	require.NoError(t, err)
	require.NotNil(t, conf)

	assert.Equal(t, "https://env-icons.example/", conf.GetSelfhstIconURL())
	assert.Equal(t, "https://env-search.example/?q=", conf.GetSearchEngineURL())
	assert.Equal(t, 77, conf.GetRefreshIntervalSeconds())
	assert.Equal(t, "https://env-traefik.example", conf.GetTraefikAPIHost())
	assert.Equal(t, "bob", conf.GetBasicAuthUsername())
	assert.Equal(t, "envpass", conf.GetBasicAuthPassword())
	assert.True(t, conf.GetInsecureSkipVerify())
	assert.Equal(t, "debug", conf.GetLogLevel())
	assert.Equal(t, "de", conf.GetLanguage())
	assert.False(t, conf.GetGroupingEnabled())
	assert.InDelta(t, 0.25, conf.GetTagFrequencyThreshold(), 1e-9)
	assert.Equal(t, 5, conf.GetMinServicesPerGroup())
	assert.Equal(t, 6, conf.GetGroupingColumns())
}

func TestLoadConfiguration_EnvInvalidValuesKeepDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")
	t.Setenv("REFRESH_INTERVAL_SECONDS", "not-a-number")
	t.Setenv("TRAEFIK_INSECURE_SKIP_VERIFY", "maybe")
	t.Setenv("GROUPING_ENABLED", "nope")
	t.Setenv("GROUPING_TAG_FREQUENCY_THRESHOLD", "2.0") // >1 is invalid
	t.Setenv("GROUPING_MIN_SERVICES_PER_GROUP", "0")    // <1 is invalid
	t.Setenv("GROUPED_COLUMNS", "99")                   // >6 is invalid

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)
	require.NotNil(t, conf)

	// Defaults preserved when env values are invalid.
	assert.Equal(t, 30, conf.GetRefreshIntervalSeconds())
	assert.False(t, conf.GetInsecureSkipVerify())
	assert.True(t, conf.GetGroupingEnabled())
	assert.InDelta(t, 0.9, conf.GetTagFrequencyThreshold(), 1e-9)
	assert.Equal(t, 2, conf.GetMinServicesPerGroup())
	assert.Equal(t, 3, conf.GetGroupingColumns())
}

func TestLoadConfiguration_InvalidLogLevelFallsBackToInfo(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")
	t.Setenv("LOG_LEVEL", "chatter")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)
	assert.Equal(t, "info", conf.GetLogLevel())
}

func TestLoadConfiguration_APIHostSchemePrefix(t *testing.T) {
	clearConfigEnv(t)

	t.Run("bare host gets http prefix", func(t *testing.T) {
		t.Setenv("TRAEFIK_API_HOST", "traefik.local")
		conf, err := LoadConfiguration(nonExistentPath(t))
		require.NoError(t, err)
		assert.Equal(t, "http://traefik.local", conf.GetTraefikAPIHost())
	})

	t.Run("https host is preserved", func(t *testing.T) {
		t.Setenv("TRAEFIK_API_HOST", "https://traefik.local")
		conf, err := LoadConfiguration(nonExistentPath(t))
		require.NoError(t, err)
		assert.Equal(t, "https://traefik.local", conf.GetTraefikAPIHost())
	})

	t.Run("http host is preserved", func(t *testing.T) {
		t.Setenv("TRAEFIK_API_HOST", "http://traefik.local")
		conf, err := LoadConfiguration(nonExistentPath(t))
		require.NoError(t, err)
		assert.Equal(t, "http://traefik.local", conf.GetTraefikAPIHost())
	})
}

func TestLoadConfiguration_SelfhstIconURLTrailingSlash(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")
	t.Setenv("SELFHST_ICON_URL", "https://icons.example")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)
	assert.Equal(t, "https://icons.example/", conf.GetSelfhstIconURL())
}

func TestLoadConfiguration_BasicAuthEnabledNoCredentials(t *testing.T) {
	clearConfigEnv(t)
	yaml := `
version: "3.0"
environment:
  traefik:
    api_host: "http://t.local"
    enable_basic_auth: true
`
	path := writeConfigFile(t, yaml)
	conf, err := LoadConfiguration(path)
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "basic auth is enabled")
}

func TestLoadConfiguration_BasicAuthPasswordFile(t *testing.T) {
	clearConfigEnv(t)

	// Write the password file with surrounding whitespace to verify trimming.
	pwDir := t.TempDir()
	pwFile := filepath.Join(pwDir, "pw")
	require.NoError(t, os.WriteFile(pwFile, []byte("  file-password  \n"), 0o600))

	yaml := `
version: "3.0"
environment:
  traefik:
    api_host: "http://t.local"
    enable_basic_auth: true
    basic_auth:
      username: alice
      password_file: ` + pwFile + `
`
	path := writeConfigFile(t, yaml)
	conf, err := LoadConfiguration(path)
	require.NoError(t, err)
	require.NotNil(t, conf)

	assert.Equal(t, "file-password", conf.GetBasicAuthPassword(),
		"password file contents should be trimmed and used as password")
	assert.True(t, conf.GetEnableBasicAuth())
	assert.Equal(t, "alice", conf.GetBasicAuthUsername())
}

func TestLoadConfiguration_BasicAuthPasswordFileMissing(t *testing.T) {
	clearConfigEnv(t)

	missingFile := filepath.Join(t.TempDir(), "nope")
	yaml := `
version: "3.0"
environment:
  traefik:
    api_host: "http://t.local"
    enable_basic_auth: true
    basic_auth:
      username: alice
      password_file: ` + missingFile + `
`
	path := writeConfigFile(t, yaml)
	conf, err := LoadConfiguration(path)
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password file")
}

func TestLoadConfiguration_VersionBelowMinimum(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")

	yaml := `
version: "2.0"
environment:
  traefik:
    api_host: "http://t.local"
`
	path := writeConfigFile(t, yaml)
	conf, err := LoadConfiguration(path)
	require.NoError(t, err, "incompatible version must not abort the load")
	require.NotNil(t, conf)

	status := conf.GetConfigCompatibilityStatus()
	assert.Equal(t, "2.0", status.ConfigVersion)
	assert.False(t, status.IsCompatible)
	assert.Contains(t, status.WarningMessage, "below the minimum")
}

func TestLoadConfiguration_DebugLogLevelMarshalsEffectiveConfig(t *testing.T) {
	// debug log level triggers a yaml.Marshal + Println branch inside
	// LoadConfiguration. Exercising it for coverage; we only check that
	// the call succeeds.
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TRAEFIK_BASIC_AUTH_PASSWORD", "shown-in-debug")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)
	assert.Equal(t, "debug", conf.GetLogLevel())
}

func TestLoadConfiguration_ValidationFailsOnInvalidURL(t *testing.T) {
	clearConfigEnv(t)
	// Use "http://" which has a scheme but no host, so it fails URL validation
	// and bypasses the "bare host gets http:// prefix" post-processing.
	t.Setenv("TRAEFIK_API_HOST", "http://")

	conf, err := LoadConfiguration(nonExistentPath(t))
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a valid URL")
}

func TestLoadConfiguration_ValidationFailsOnInvalidSelfhstIconURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")
	t.Setenv("SELFHST_ICON_URL", "not-a-url")

	conf, err := LoadConfiguration(nonExistentPath(t))
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a valid URL")
}

func TestLoadConfiguration_ValidationFailsOnInvalidSearchEngineURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://t.local")
	t.Setenv("SEARCH_ENGINE_URL", "not-a-url")

	conf, err := LoadConfiguration(nonExistentPath(t))
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a valid URL")
}

func TestLoadConfiguration_ValidationFailsOnMissingVersion(t *testing.T) {
	clearConfigEnv(t)
	yaml := `
version: ""
environment:
  traefik:
    api_host: "http://t.local"
`
	path := writeConfigFile(t, yaml)

	conf, err := LoadConfiguration(path)
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
	assert.Contains(t, err.Error(), "required")
}

func TestLoadConfiguration_ValidationFailsOnInvalidManualServiceURL(t *testing.T) {
	clearConfigEnv(t)
	yaml := `
version: "3.0"
environment:
  traefik:
    api_host: "http://t.local"
services:
  manual:
    - name: bad-service
      url: "not-a-url"
`
	path := writeConfigFile(t, yaml)

	conf, err := LoadConfiguration(path)
	assert.Nil(t, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a valid URL")
}
