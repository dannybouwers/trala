package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Pure function tests (parallel) ---

func TestEnvVarNameForPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want string
	}{
		{"selfhst_icon_url", "SELFHST_ICON_URL"},
		{"search_engine_url", "SEARCH_ENGINE_URL"},
		{"refresh_interval_seconds", "REFRESH_INTERVAL_SECONDS"},
		{"log_level", "LOG_LEVEL"},
		{"language", "LANGUAGE"},
		{"grouping.enabled", "GROUPING_ENABLED"},
		{"grouping.columns", "GROUPING_COLUMNS"},
		{"grouping.tag_frequency_threshold", "GROUPING_TAG_FREQUENCY_THRESHOLD"},
		{"grouping.min_services_per_group", "GROUPING_MIN_SERVICES_PER_GROUP"},
		{"traefik.instances[0].api_host", "TRAEFIK_INSTANCES_1_API_HOST"},
		{"traefik.instances[0].name", "TRAEFIK_INSTANCES_1_NAME"},
		{"traefik.instances[0].enable_basic_auth", "TRAEFIK_INSTANCES_1_ENABLE_BASIC_AUTH"},
		{"traefik.instances[0].insecure_skip_verify", "TRAEFIK_INSTANCES_1_INSECURE_SKIP_VERIFY"},
		{"traefik.instances[0].basic_auth.username", "TRAEFIK_INSTANCES_1_BASIC_AUTH_USERNAME"},
		{"traefik.instances[0].basic_auth.password", "TRAEFIK_INSTANCES_1_BASIC_AUTH_PASSWORD"},
		{"traefik.instances[0].basic_auth.password_file", "TRAEFIK_INSTANCES_1_BASIC_AUTH_PASSWORD_FILE"},
		{"traefik.instances[2].api_host", "TRAEFIK_INSTANCES_3_API_HOST"},
		{"traefik.instances[2].basic_auth.username", "TRAEFIK_INSTANCES_3_BASIC_AUTH_USERNAME"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, envVarNameForPath(tc.path))
		})
	}
}

func TestEnvironmentEnvVarCoversEveryEnvironmentLeaf(t *testing.T) {
	t.Parallel()
	// Every scalar leaf under environment.* must derive a non-empty env var
	// name, so adding a new config option automatically gains env support.
	leaves := []string{
		"selfhst_icon_url",
		"search_engine_url",
		"refresh_interval_seconds",
		"log_level",
		"language",
		"grouping.enabled",
		"grouping.columns",
		"grouping.tag_frequency_threshold",
		"grouping.min_services_per_group",
	}
	for _, leaf := range leaves {
		got := envVarNameForPath(leaf)
		assert.NotEmpty(t, got, "leaf %s must derive an env var name", leaf)
		assert.Equal(t, EnvironmentEnvVar("environment."+leaf), got,
			"non-instance paths must delegate to EnvironmentEnvVar")
	}
}

// --- LoadConfiguration behavioral tests (serial — uses t.Setenv and files) ---

func TestLoadConfiguration_EnvScalarOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SELFHST_ICON_URL", "https://scalar-icons.example")
	t.Setenv("SEARCH_ENGINE_URL", "https://scalar-search.example/?q=")
	t.Setenv("REFRESH_INTERVAL_SECONDS", "11")
	t.Setenv("LOG_LEVEL", "warn")
	t.Setenv("LANGUAGE", "es")
	t.Setenv("GROUPING_ENABLED", "false")
	t.Setenv("GROUPING_COLUMNS", "5")
	t.Setenv("GROUPING_TAG_FREQUENCY_THRESHOLD", "0.4")
	t.Setenv("GROUPING_MIN_SERVICES_PER_GROUP", "7")
	t.Setenv("TRAEFIK_API_HOST", "http://scalar.local")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)

	assert.Equal(t, "https://scalar-icons.example/", conf.GetSelfhstIconURL())
	assert.Equal(t, "https://scalar-search.example/?q=", conf.GetSearchEngineURL())
	assert.Equal(t, 11, conf.GetRefreshIntervalSeconds())
	assert.Equal(t, "warn", conf.GetLogLevel())
	assert.Equal(t, "es", conf.GetLanguage())
	assert.False(t, conf.GetGroupingEnabled())
	assert.Equal(t, 5, conf.GetGroupingColumns())
	assert.InDelta(t, 0.4, conf.GetTagFrequencyThreshold(), 1e-9)
	assert.Equal(t, 7, conf.GetMinServicesPerGroup())
}

func TestLoadConfiguration_GroupingColumnsCanonicalAndDeprecatedAlias(t *testing.T) {
	baseYAML := `
version: "3.0"
environment:
  traefik:
    api_host: "http://t.local"
  grouping:
    columns: 2
`

	t.Run("canonical GROUPING_COLUMNS applies", func(t *testing.T) {
		clearConfigEnv(t)
		path := writeConfigFile(t, baseYAML)
		t.Setenv("GROUPING_COLUMNS", "5")
		conf, err := LoadConfiguration(path)
		require.NoError(t, err)
		assert.Equal(t, 5, conf.GetGroupingColumns())
	})

	t.Run("deprecated GROUPED_COLUMNS still applies", func(t *testing.T) {
		clearConfigEnv(t)
		path := writeConfigFile(t, baseYAML)
		t.Setenv("GROUPED_COLUMNS", "4")
		conf, err := LoadConfiguration(path)
		require.NoError(t, err)
		assert.Equal(t, 4, conf.GetGroupingColumns())
	})

	t.Run("canonical wins when both are set", func(t *testing.T) {
		clearConfigEnv(t)
		path := writeConfigFile(t, baseYAML)
		t.Setenv("GROUPING_COLUMNS", "5")
		t.Setenv("GROUPED_COLUMNS", "4")
		conf, err := LoadConfiguration(path)
		require.NoError(t, err)
		assert.Equal(t, 5, conf.GetGroupingColumns())
	})

	t.Run("invalid canonical keeps file value", func(t *testing.T) {
		clearConfigEnv(t)
		path := writeConfigFile(t, baseYAML)
		t.Setenv("GROUPING_COLUMNS", "99")
		conf, err := LoadConfiguration(path)
		require.NoError(t, err)
		assert.Equal(t, 2, conf.GetGroupingColumns())
	})
}

func TestLoadConfiguration_EnvInvalidValuesKeepPreviousValue(t *testing.T) {
	clearConfigEnv(t)
	yaml := `
version: "3.0"
environment:
  selfhst_icon_url: "https://icons.example"
  search_engine_url: "https://search.example/?q="
  refresh_interval_seconds: 15
  log_level: warn
  language: fr
  traefik:
    api_host: "http://t.local"
  grouping:
    enabled: false
    columns: 2
    tag_frequency_threshold: 0.5
    min_services_per_group: 4
`
	path := writeConfigFile(t, yaml)

	t.Setenv("REFRESH_INTERVAL_SECONDS", "not-a-number")
	t.Setenv("LOG_LEVEL", "chatter")
	t.Setenv("GROUPING_ENABLED", "nope")
	t.Setenv("GROUPING_COLUMNS", "0")
	t.Setenv("GROUPING_TAG_FREQUENCY_THRESHOLD", "2.0")
	t.Setenv("GROUPING_MIN_SERVICES_PER_GROUP", "-1")

	conf, err := LoadConfiguration(path)
	require.NoError(t, err)

	assert.Equal(t, 15, conf.GetRefreshIntervalSeconds())
	assert.Equal(t, "warn", conf.GetLogLevel())
	assert.False(t, conf.GetGroupingEnabled())
	assert.Equal(t, 2, conf.GetGroupingColumns())
	assert.InDelta(t, 0.5, conf.GetTagFrequencyThreshold(), 1e-9)
	assert.Equal(t, 4, conf.GetMinServicesPerGroup())
}

func TestLoadConfiguration_IndexedInstancesCreateMultiMode(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_INSTANCES_1_API_HOST", "http://inst1.local")
	t.Setenv("TRAEFIK_INSTANCES_2_API_HOST", "http://inst2.local")
	// Legacy single-instance vars must be ignored once indexed vars switch
	// the config to instance-format.
	t.Setenv("TRAEFIK_API_HOST", "http://legacy.local")
	t.Setenv("TRAEFIK_BASIC_AUTH_USERNAME", "legacy-user")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)

	instances := conf.GetTraefikInstances()
	require.Len(t, instances, 2)
	assert.Equal(t, "http://inst1.local", instances[0].APIHost)
	assert.Equal(t, "http://inst2.local", instances[1].APIHost)
	assert.Empty(t, instances[0].BasicAuth.Username, "legacy env vars must not leak into instance-format mode")
	assert.Empty(t, instances[1].BasicAuth.Username, "legacy env vars must be ignored in multi-instance mode")
}

func TestLoadConfiguration_IndexedInstanceFieldsOnNonFirstIndex(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_INSTANCES_1_API_HOST", "http://inst1.local")
	t.Setenv("TRAEFIK_INSTANCES_2_API_HOST", "http://inst2.local")
	t.Setenv("TRAEFIK_INSTANCES_2_NAME", "arr")
	t.Setenv("TRAEFIK_INSTANCES_2_ENABLE_BASIC_AUTH", "true")
	t.Setenv("TRAEFIK_INSTANCES_2_BASIC_AUTH_USERNAME", "carol")
	t.Setenv("TRAEFIK_INSTANCES_2_BASIC_AUTH_PASSWORD", "carol-pw")
	t.Setenv("TRAEFIK_INSTANCES_2_INSECURE_SKIP_VERIFY", "true")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)

	instances := conf.GetTraefikInstances()
	require.Len(t, instances, 2)
	assert.Equal(t, "http://inst1.local", instances[0].APIHost)
	assert.False(t, instances[0].EnableBasicAuth)
	assert.False(t, instances[0].InsecureSkipVerify)

	assert.Equal(t, "arr", instances[1].Name)
	assert.Equal(t, "http://inst2.local", instances[1].APIHost)
	assert.True(t, instances[1].EnableBasicAuth)
	assert.Equal(t, "carol", instances[1].BasicAuth.Username)
	assert.Equal(t, "carol-pw", instances[1].BasicAuth.Password)
	assert.True(t, instances[1].InsecureSkipVerify)
}

func TestLoadConfiguration_IndexedInstancesMergeWithFile(t *testing.T) {
	clearConfigEnv(t)
	yaml := `
version: "3.0"
environment:
  traefik:
    instances:
      - api_host: "http://file-one.local"
      - api_host: "http://file-two.local"
`
	path := writeConfigFile(t, yaml)

	t.Setenv("TRAEFIK_INSTANCES_1_API_HOST", "http://env-one.local")
	t.Setenv("TRAEFIK_INSTANCES_3_API_HOST", "http://env-three.local")

	conf, err := LoadConfiguration(path)
	require.NoError(t, err)

	instances := conf.GetTraefikInstances()
	require.Len(t, instances, 3)
	assert.Equal(t, "http://env-one.local", instances[0].APIHost, "env index 1 must override file instance 1")
	assert.Equal(t, "http://file-two.local", instances[1].APIHost, "file instance 2 must be untouched")
	assert.Equal(t, "http://env-three.local", instances[2].APIHost, "env index 3 must append a new instance")
}

func TestLoadConfiguration_IndexedInstanceInvalidEntriesIgnored(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://legacy.local")
	t.Setenv("TRAEFIK_INSTANCES_0_API_HOST", "http://junk.local")
	t.Setenv("TRAEFIK_INSTANCES_X_API_HOST", "http://junk.local")
	t.Setenv("TRAEFIK_INSTANCES_1_JUNK", "junk")
	t.Setenv("TRAEFIK_INSTANCES_2_INSECURE_SKIP_VERIFY", "maybe")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)

	instances := conf.GetTraefikInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, "http://legacy.local", instances[0].APIHost)
	assert.False(t, instances[0].InsecureSkipVerify, "invalid bool must keep previous value")
}

func TestLoadConfiguration_IndexedInstanceEmptyValueIsUnset(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_API_HOST", "http://legacy.local")
	t.Setenv("TRAEFIK_INSTANCES_1_API_HOST", "")

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err)

	assert.Equal(t, "http://legacy.local", conf.GetTraefikInstances()[0].APIHost,
		"empty indexed env var must be treated as unset")
}

func TestLoadConfiguration_IndexedInstanceBasicAuthPasswordFileValue(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("TRAEFIK_INSTANCES_1_API_HOST", "http://inst1.local")
	t.Setenv("TRAEFIK_INSTANCES_1_ENABLE_BASIC_AUTH", "true")
	t.Setenv("TRAEFIK_INSTANCES_1_BASIC_AUTH_USERNAME", "alice")
	t.Setenv("TRAEFIK_INSTANCES_1_BASIC_AUTH_PASSWORD_FILE", fmt.Sprintf("%s/nope", t.TempDir()))

	conf, err := LoadConfiguration(nonExistentPath(t))
	require.NoError(t, err, "multi-instance password_file is not read at load time")

	instances := conf.GetTraefikInstances()
	require.Len(t, instances, 1)
	assert.Equal(t, "alice", instances[0].BasicAuth.Username)
	assert.NotEmpty(t, instances[0].BasicAuth.PasswordFile)
}
