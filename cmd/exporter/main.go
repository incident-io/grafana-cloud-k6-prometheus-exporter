package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/grafana-cloud-k6-prometheus-exporter/internal/collector"
	"github.com/grafana-cloud-k6-prometheus-exporter/internal/config"
	"github.com/grafana-cloud-k6-prometheus-exporter/internal/k6client"
	"github.com/grafana-cloud-k6-prometheus-exporter/internal/state"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Initialize logger
	logger := initLogger()
	defer logger.Sync()

	logger.Info("starting k6 prometheus exporter",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("build_date", date),
	)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	// Validate API token is not empty
	if cfg.K6APIToken == "" {
		logger.Fatal("K6_API_TOKEN environment variable is required")
	}

	logger.Info("configuration loaded",
		zap.String("api_url", cfg.K6APIURL),
		zap.String("stack_id", cfg.GrafanaStackID),
		zap.Int("port", cfg.Port),
		zap.Duration("test_cache_ttl", cfg.TestCacheTTL),
		zap.Duration("state_cleanup_interval", cfg.StateCleanupInterval),
		zap.Strings("projects", cfg.Projects),
	)

	// Create k6 API client
	apiClient := k6client.NewClient(cfg.GetAPIBaseURL(), cfg.GrafanaStackID, cfg.K6APIToken, logger)

	// Create state manager
	stateManager := state.NewManager(logger)

	// Create collector
	k6Collector := collector.NewCollector(apiClient, stateManager, cfg, logger)

	// Register collector with Prometheus
	prometheus.MustRegister(k6Collector)

	// Create context for background tasks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background tasks
	k6Collector.StartBackgroundTasks(ctx)

	// Setup HTTP server
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})

	// Version endpoint
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version":"%s","commit":"%s","build_date":"%s"}`+"\n", version, commit, date)
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting HTTP server", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutting down...")

	// Cancel background tasks
	cancel()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("failed to shutdown HTTP server", zap.Error(err))
	}

	logger.Info("exporter stopped")
}

// initLogger initializes the zap logger
func initLogger() *zap.Logger {
	// Check if we're in production mode
	if os.Getenv("ENV") == "production" {
		config := zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logger, err := config.Build()
		if err != nil {
			panic(fmt.Sprintf("failed to initialize production logger: %v", err))
		}
		return logger
	}

	// Development mode
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize development logger: %v", err))
	}
	return logger
}
