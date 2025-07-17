package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Save current env vars
	originalToken := os.Getenv("K6_API_TOKEN")
	originalStackID := os.Getenv("GRAFANA_STACK_ID")
	defer os.Setenv("K6_API_TOKEN", originalToken)
	defer os.Setenv("GRAFANA_STACK_ID", originalStackID)

	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
		errMsg  string
		verify  func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid_minimal_config",
			envVars: map[string]string{
				"K6_API_TOKEN":      "test-token",
				"GRAFANA_STACK_ID":  "test-stack-id",
			},
			wantErr: false,
			verify: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test-token", cfg.K6APIToken)
				assert.Equal(t, "test-stack-id", cfg.GrafanaStackID)
				assert.Equal(t, "https://api.k6.io", cfg.K6APIURL)
				assert.Equal(t, 9090, cfg.Port)
				assert.Equal(t, 60*time.Second, cfg.TestCacheTTL)
			},
		},
		{
			name: "missing_api_token",
			envVars: map[string]string{
				"K6_API_TOKEN":      "",
				"GRAFANA_STACK_ID":  "test-stack-id",
			},
			wantErr: true,
			errMsg:  "K6_API_TOKEN is required",
		},
		{
			name: "missing_stack_id",
			envVars: map[string]string{
				"K6_API_TOKEN":      "test-token",
				"GRAFANA_STACK_ID":  "",
			},
			wantErr: true,
			errMsg:  "GRAFANA_STACK_ID is required",
		},
		{
			name: "custom_values",
			envVars: map[string]string{
				"K6_API_TOKEN":               "custom-token",
				"GRAFANA_STACK_ID":           "custom-stack-id",
				"K6_API_URL":                 "https://custom.api.com",
				"PORT":                       "8080",
				"TEST_CACHE_TTL":             "120s",
				"STATE_CLEANUP_INTERVAL":     "10m",
				"PROJECTS":                   "proj1,proj2,proj3",
				"MAX_CONCURRENT_REQUESTS":    "20",
				"API_TIMEOUT":                "60s",
			},
			wantErr: false,
			verify: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "custom-token", cfg.K6APIToken)
				assert.Equal(t, "custom-stack-id", cfg.GrafanaStackID)
				assert.Equal(t, "https://custom.api.com", cfg.K6APIURL)
				assert.Equal(t, 8080, cfg.Port)
				assert.Equal(t, 120*time.Second, cfg.TestCacheTTL)
				assert.Equal(t, 10*time.Minute, cfg.StateCleanupInterval)
				assert.Equal(t, []string{"proj1", "proj2", "proj3"}, cfg.Projects)
				assert.Equal(t, 20, cfg.MaxConcurrentRequests)
				assert.Equal(t, 60*time.Second, cfg.APITimeout)
			},
		},
		{
			name: "invalid_port_high",
			envVars: map[string]string{
				"K6_API_TOKEN":      "test-token",
				"GRAFANA_STACK_ID":  "test-stack-id",
				"PORT":         "70000",
			},
			wantErr: true,
			errMsg:  "PORT must be between 1 and 65535",
		},
		{
			name: "invalid_port_low",
			envVars: map[string]string{
				"K6_API_TOKEN":      "test-token",
				"GRAFANA_STACK_ID":  "test-stack-id",
				"PORT":         "0",
			},
			wantErr: true,
			errMsg:  "PORT must be between 1 and 65535",
		},
		{
			name: "invalid_url_format",
			envVars: map[string]string{
				"K6_API_TOKEN":      "test-token",
				"GRAFANA_STACK_ID":  "test-stack-id",
				"K6_API_URL":   "not-a-url",
			},
			wantErr: true,
			errMsg:  "K6_API_URL must start with http:// or https://",
		},
		{
			name: "invalid_test_cache_ttl",
			envVars: map[string]string{
				"K6_API_TOKEN":   "test-token",
				"TEST_CACHE_TTL": "500ms",
			},
			wantErr: true,
			errMsg:  "TEST_CACHE_TTL must be at least 1 second",
		},
		{
			name: "invalid_state_cleanup_interval",
			envVars: map[string]string{
				"K6_API_TOKEN":            "test-token",
				"STATE_CLEANUP_INTERVAL":  "30s",
			},
			wantErr: true,
			errMsg:  "STATE_CLEANUP_INTERVAL must be at least 1 minute",
		},
		{
			name: "invalid_max_concurrent_requests",
			envVars: map[string]string{
				"K6_API_TOKEN":            "test-token",
				"MAX_CONCURRENT_REQUESTS": "0",
			},
			wantErr: true,
			errMsg:  "MAX_CONCURRENT_REQUESTS must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant env vars
			envVars := []string{
				"K6_API_TOKEN", "K6_API_URL", "PORT", "TEST_CACHE_TTL",
				"STATE_CLEANUP_INTERVAL", "PROJECTS", "MAX_CONCURRENT_REQUESTS",
				"API_TIMEOUT", "RETRY_ATTEMPTS", "RETRY_DELAY", "SCRAPE_INTERVAL",
			}
			for _, v := range envVars {
				os.Unsetenv(v)
			}

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Load config
			cfg, err := Load()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				if tt.verify != nil {
					tt.verify(t, cfg)
				}
			}
		})
	}
}

func TestGetAPIBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		apiURL   string
		expected string
	}{
		{
			name:     "url_without_trailing_slash",
			apiURL:   "https://api.k6.io",
			expected: "https://api.k6.io",
		},
		{
			name:     "url_with_trailing_slash",
			apiURL:   "https://api.k6.io/",
			expected: "https://api.k6.io",
		},
		{
			name:     "url_with_multiple_trailing_slashes",
			apiURL:   "https://api.k6.io///",
			expected: "https://api.k6.io",
		},
		{
			name:     "url_with_path",
			apiURL:   "https://api.k6.io/v1/",
			expected: "https://api.k6.io/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{K6APIURL: tt.apiURL}
			assert.Equal(t, tt.expected, cfg.GetAPIBaseURL())
		})
	}
}

func TestShouldMonitorProject(t *testing.T) {
	tests := []struct {
		name      string
		projects  []string
		projectID string
		expected  bool
	}{
		{
			name:      "no_filter_monitors_all",
			projects:  []string{},
			projectID: "12345",
			expected:  true,
		},
		{
			name:      "project_in_list",
			projects:  []string{"12345", "67890"},
			projectID: "12345",
			expected:  true,
		},
		{
			name:      "project_not_in_list",
			projects:  []string{"12345", "67890"},
			projectID: "99999",
			expected:  false,
		},
		{
			name:      "single_project_filter",
			projects:  []string{"12345"},
			projectID: "12345",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Projects: tt.projects}
			assert.Equal(t, tt.expected, cfg.ShouldMonitorProject(tt.projectID))
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_config",
			config: Config{
				K6APIToken:            "token",
				GrafanaStackID:        "stack-id",
				K6APIURL:              "https://api.k6.io",
				Port:                  9090,
				TestCacheTTL:          60 * time.Second,
				StateCleanupInterval:  5 * time.Minute,
				MaxConcurrentRequests: 10,
			},
			wantErr: false,
		},
		{
			name: "empty_token",
			config: Config{
				K6APIToken:            "",
				GrafanaStackID:        "stack-id",
				K6APIURL:              "https://api.k6.io",
				Port:                  9090,
				TestCacheTTL:          60 * time.Second,
				StateCleanupInterval:  5 * time.Minute,
				MaxConcurrentRequests: 10,
			},
			wantErr: true,
			errMsg:  "K6_API_TOKEN is required",
		},
		{
			name: "empty_stack_id",
			config: Config{
				K6APIToken:            "token",
				GrafanaStackID:        "",
				K6APIURL:              "https://api.k6.io",
				Port:                  9090,
				TestCacheTTL:          60 * time.Second,
				StateCleanupInterval:  5 * time.Minute,
				MaxConcurrentRequests: 10,
			},
			wantErr: true,
			errMsg:  "GRAFANA_STACK_ID is required",
		},
		{
			name: "invalid_url_no_scheme",
			config: Config{
				K6APIToken:            "token",
				GrafanaStackID:        "stack-id",
				K6APIURL:              "api.k6.io",
				Port:                  9090,
				TestCacheTTL:          60 * time.Second,
				StateCleanupInterval:  5 * time.Minute,
				MaxConcurrentRequests: 10,
			},
			wantErr: true,
			errMsg:  "K6_API_URL must start with http:// or https://",
		},
		{
			name: "http_url_allowed",
			config: Config{
				K6APIToken:            "token",
				GrafanaStackID:        "stack-id",
				K6APIURL:              "http://localhost:8080",
				Port:                  9090,
				TestCacheTTL:          60 * time.Second,
				StateCleanupInterval:  5 * time.Minute,
				MaxConcurrentRequests: 10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}