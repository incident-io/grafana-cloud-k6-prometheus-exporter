package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds the application configuration
type Config struct {
	// K6 API configuration
	K6APIToken      string `envconfig:"K6_API_TOKEN" required:"true"`
	K6APIURL        string `envconfig:"K6_API_URL" default:"https://api.k6.io"`
	GrafanaStackID  string `envconfig:"GRAFANA_STACK_ID" required:"true"`

	// Server configuration
	Port int `envconfig:"PORT" default:"9090"`

	// Operational configuration
	TestCacheTTL         time.Duration `envconfig:"TEST_CACHE_TTL" default:"60s"`
	StateCleanupInterval time.Duration `envconfig:"STATE_CLEANUP_INTERVAL" default:"5m"`
	ScrapeInterval       time.Duration `envconfig:"SCRAPE_INTERVAL" default:"15s"`

	// Filtering
	Projects []string `envconfig:"PROJECTS"` // Comma-separated list of project IDs to monitor

	// Advanced configuration
	MaxConcurrentRequests int           `envconfig:"MAX_CONCURRENT_REQUESTS" default:"10"`
	APITimeout            time.Duration `envconfig:"API_TIMEOUT" default:"30s"`
	RetryAttempts         int           `envconfig:"RETRY_ATTEMPTS" default:"3"`
	RetryDelay            time.Duration `envconfig:"RETRY_DELAY" default:"1s"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.K6APIToken == "" {
		return fmt.Errorf("K6_API_TOKEN is required")
	}

	if c.GrafanaStackID == "" {
		return fmt.Errorf("GRAFANA_STACK_ID is required")
	}

	if !strings.HasPrefix(c.K6APIURL, "http://") && !strings.HasPrefix(c.K6APIURL, "https://") {
		return fmt.Errorf("K6_API_URL must start with http:// or https://")
	}

	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535")
	}

	if c.TestCacheTTL < time.Second {
		return fmt.Errorf("TEST_CACHE_TTL must be at least 1 second")
	}

	if c.StateCleanupInterval < time.Minute {
		return fmt.Errorf("STATE_CLEANUP_INTERVAL must be at least 1 minute")
	}

	if c.MaxConcurrentRequests < 1 {
		return fmt.Errorf("MAX_CONCURRENT_REQUESTS must be at least 1")
	}

	return nil
}

// GetAPIBaseURL returns the base URL for the k6 API with proper formatting
func (c *Config) GetAPIBaseURL() string {
	return strings.TrimRight(c.K6APIURL, "/")
}

// ShouldMonitorProject returns true if the project should be monitored
func (c *Config) ShouldMonitorProject(projectID string) bool {
	if len(c.Projects) == 0 {
		return true // Monitor all projects if none specified
	}

	for _, p := range c.Projects {
		if p == projectID {
			return true
		}
	}
	return false
}