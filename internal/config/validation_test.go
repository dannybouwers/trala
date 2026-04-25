package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidConfig(t *testing.T) {
	t.Parallel()
	c := newPopulatedConfig()
	err := Validate(c)
	require.NoError(t, err)
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		mutate    func(*TralaConfiguration)
		wantField string
		wantTag   string
	}{
		{
			name: "missing version",
			mutate: func(c *TralaConfiguration) {
				c.Version = ""
			},
			wantField: "version",
			wantTag:   "required",
		},
		{
			name: "missing api host",
			mutate: func(c *TralaConfiguration) {
				c.Environment.Traefik.APIHost = ""
			},
			wantField: "api_host",
			wantTag:   "required",
		},
		{
			name: "missing selfhst icon url",
			mutate: func(c *TralaConfiguration) {
				c.Environment.SelfhstIconURL = ""
			},
			wantField: "selfhst_icon_url",
			wantTag:   "required",
		},
		{
			name: "missing search engine url",
			mutate: func(c *TralaConfiguration) {
				c.Environment.SearchEngineURL = ""
			},
			wantField: "search_engine_url",
			wantTag:   "required",
		},
		{
			name: "missing manual service name",
			mutate: func(c *TralaConfiguration) {
				c.Services.Manual[0].Name = ""
			},
			wantField: "name",
			wantTag:   "required",
		},
		{
			name: "missing manual service url",
			mutate: func(c *TralaConfiguration) {
				c.Services.Manual[0].URL = ""
			},
			wantField: "url",
			wantTag:   "required",
		},
		{
			name: "missing service override service",
			mutate: func(c *TralaConfiguration) {
				c.Services.Overrides[0].Service = ""
			},
			wantField: "service",
			wantTag:   "required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newPopulatedConfig()
			tc.mutate(c)
			err := Validate(c)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantField)
			assert.Contains(t, err.Error(), tc.wantTag)
		})
	}
}

func TestValidate_InvalidURLs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*TralaConfiguration)
	}{
		{
			name: "invalid api host",
			mutate: func(c *TralaConfiguration) {
				c.Environment.Traefik.APIHost = "not-a-url"
			},
		},
		{
			name: "invalid selfhst icon url",
			mutate: func(c *TralaConfiguration) {
				c.Environment.SelfhstIconURL = "not-a-url"
			},
		},
		{
			name: "invalid search engine url",
			mutate: func(c *TralaConfiguration) {
				c.Environment.SearchEngineURL = "not-a-url"
			},
		},
		{
			name: "invalid manual service url",
			mutate: func(c *TralaConfiguration) {
				c.Services.Manual[0].URL = "not-a-url"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newPopulatedConfig()
			tc.mutate(c)
			err := Validate(c)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "URL")
		})
	}
}

func TestValidate_OutOfRangeValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		mutate    func(*TralaConfiguration)
		wantField string
		wantTag   string
	}{
		{
			name: "refresh interval zero",
			mutate: func(c *TralaConfiguration) {
				c.Environment.RefreshIntervalSeconds = 0
			},
			wantField: "refresh_interval_seconds",
			wantTag:   ">=",
		},
		{
			name: "columns zero",
			mutate: func(c *TralaConfiguration) {
				c.Environment.Grouping.Columns = 0
			},
			wantField: "columns",
			wantTag:   ">=",
		},
		{
			name: "columns too high",
			mutate: func(c *TralaConfiguration) {
				c.Environment.Grouping.Columns = 7
			},
			wantField: "columns",
			wantTag:   "<=",
		},
		{
			name: "tag frequency zero",
			mutate: func(c *TralaConfiguration) {
				c.Environment.Grouping.TagFrequencyThreshold = 0
			},
			wantField: "tag_frequency_threshold",
			wantTag:   ">",
		},
		{
			name: "tag frequency above 1",
			mutate: func(c *TralaConfiguration) {
				c.Environment.Grouping.TagFrequencyThreshold = 1.5
			},
			wantField: "tag_frequency_threshold",
			wantTag:   "<=",
		},
		{
			name: "min services per group zero",
			mutate: func(c *TralaConfiguration) {
				c.Environment.Grouping.MinServicesPerGroup = 0
			},
			wantField: "min_services_per_group",
			wantTag:   ">=",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newPopulatedConfig()
			tc.mutate(c)
			err := Validate(c)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantField)
			assert.Contains(t, err.Error(), tc.wantTag)
		})
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	t.Parallel()
	c := newPopulatedConfig()
	c.Environment.LogLevel = "verbose"
	err := Validate(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_level")
	assert.Contains(t, err.Error(), "one of")
}

func TestValidate_NilConfig(t *testing.T) {
	t.Parallel()
	err := Validate(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil config")
}

func TestValidate_MultipleErrors(t *testing.T) {
	t.Parallel()
	c := newPopulatedConfig()
	c.Version = ""
	c.Environment.Traefik.APIHost = ""
	err := Validate(c)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
	assert.Contains(t, err.Error(), "api_host")
	assert.Contains(t, err.Error(), ";")
}
