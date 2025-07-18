package collector

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/grafana-cloud-k6-prometheus-exporter/internal/config"
	"github.com/grafana-cloud-k6-prometheus-exporter/internal/k6client"
	"github.com/grafana-cloud-k6-prometheus-exporter/internal/state"
)

// Collector implements the prometheus.Collector interface
type Collector struct {
	client       k6client.ClientInterface
	stateManager *state.Manager
	config       *config.Config
	logger       *zap.Logger
	metrics      *OperationalMetrics

	// Cache for test data
	testCache      map[int]*k6client.Test
	testCacheMutex sync.RWMutex
	lastTestFetch  time.Time
}

// NewCollector creates a new k6 metrics collector
func NewCollector(client k6client.ClientInterface, stateManager *state.Manager, cfg *config.Config, logger *zap.Logger) *Collector {
	return &Collector{
		client:       client,
		stateManager: stateManager,
		config:       cfg,
		logger:       logger,
		metrics:      NewOperationalMetrics(),
		testCache:    make(map[int]*k6client.Test),
	}
}

// NewCollectorWithRegistry creates a new k6 metrics collector with a custom registry (for testing)
func NewCollectorWithRegistry(client k6client.ClientInterface, stateManager *state.Manager, cfg *config.Config, logger *zap.Logger, reg prometheus.Registerer) *Collector {
	return &Collector{
		client:       client,
		stateManager: stateManager,
		config:       cfg,
		logger:       logger,
		metrics:      NewOperationalMetricsWithRegistry(reg),
		testCache:    make(map[int]*k6client.Test),
	}
}

// Describe implements prometheus.Collector
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	// Send all metric descriptors
	ch <- testRunTotalDesc
	ch <- testRunStatusDesc
	ch <- testRunResultTotalDesc
	ch <- testRunDurationSecondsDesc
	ch <- testRunVUHConsumedDesc
	ch <- testRunInfoDesc
	// Note: operational metrics (like scrape duration, test runs tracked) are handled separately
}

// Collect implements prometheus.Collector
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	// Update operational metrics
	c.metrics.TestRunsTracked.Set(float64(c.stateManager.GetStateCount()))

	// Collect metrics
	if err := c.collectMetrics(ch); err != nil {
		c.logger.Error("failed to collect metrics", zap.Error(err))
		c.metrics.ScrapeErrorsTotal.WithLabelValues("collect").Inc()
	}

	// Record scrape duration
	duration := time.Since(start).Seconds()
	c.metrics.ScrapeDuration.Observe(duration)
}

func (c *Collector) collectMetrics(ch chan<- prometheus.Metric) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.APITimeout)
	defer cancel()

	// Update test cache if needed
	if time.Since(c.lastTestFetch) > c.config.TestCacheTTL {
		if err := c.updateTestCache(ctx); err != nil {
			c.logger.Error("failed to update test cache", zap.Error(err))
			c.metrics.ScrapeErrorsTotal.WithLabelValues("test_cache").Inc()
		}
	}

	// Fetch test runs from the last 24 hours
	since := time.Now().Add(-24 * time.Hour)
	testRuns, err := c.client.GetAllTestRuns(ctx, c.config.Projects, &since)
	if err != nil {
		return fmt.Errorf("fetch test runs: %w", err)
	}

	// Update last scrape timestamp
	c.metrics.LastScrapeTimestamp.WithLabelValues("test_runs").SetToCurrentTime()

	// Process test runs
	statusCounts := make(map[string]map[string]int)    // status -> labels -> count
	resultCounts := make(map[string]map[string]int)    // result -> labels -> count
	activeRuns := make(map[string][]*k6client.TestRun) // status -> runs

	for _, run := range testRuns {
		// Get test name from cache or status details
		testName := c.getTestName(run.TestID)
		if testName == "" {
			if name, ok := run.StatusDetails["test_name"].(string); ok {
				testName = name
			} else {
				testName = fmt.Sprintf("test_%d", run.TestID)
			}
		}

		// Create state for this test run
		runState := &state.TestRunState{
			TestRunID:     run.ID,
			TestID:        run.TestID,
			ProjectID:     run.ProjectID,
			TestName:      testName,
			CurrentStatus: run.Status,
			Created:       run.Created,
			Ended:         run.Ended,
			Result:        run.Result,
			StartedBy:     run.StartedBy,
			VUH:           run.GetVUH(),
		}

		// Update state manager
		c.stateManager.UpdateTestRun(runState)

		// Create label key for deduplication
		labelKey := fmt.Sprintf("%s|%d|%d", testName, run.TestID, run.ProjectID)

		// Count current status (for gauge)
		if statusCounts[run.Status] == nil {
			statusCounts[run.Status] = make(map[string]int)
		}
		statusCounts[run.Status][labelKey]++

		// Track active runs by status
		if activeRuns[run.Status] == nil {
			activeRuns[run.Status] = make([]*k6client.TestRun, 0)
		}
		activeRuns[run.Status] = append(activeRuns[run.Status], &run)

		// Count results for completed runs
		if k6client.IsTerminalStatus(run.Status) {
			result := run.GetResult()
			if resultCounts[result] == nil {
				resultCounts[result] = make(map[string]int)
			}
			resultCounts[result][labelKey]++
		}

		// Send info metric
		ch <- prometheus.MustNewConstMetric(
			testRunInfoDesc,
			prometheus.GaugeValue,
			1,
			testName,
			strconv.Itoa(run.TestID),
			strconv.Itoa(run.ProjectID),
			strconv.Itoa(run.ID),
		)

		// Send duration metric
		ch <- prometheus.MustNewConstMetric(
			testRunDurationSecondsDesc,
			prometheus.GaugeValue,
			run.GetDuration(),
			testName,
			strconv.Itoa(run.TestID),
			strconv.Itoa(run.ProjectID),
			run.Status,
		)

		// Send VUH metric if available
		if run.Cost != nil && run.Cost.VUH > 0 {
			ch <- prometheus.MustNewConstMetric(
				testRunVUHConsumedDesc,
				prometheus.GaugeValue,
				run.Cost.VUH,
				testName,
				strconv.Itoa(run.TestID),
				strconv.Itoa(run.ProjectID),
				strconv.Itoa(run.ID),
			)
		}
	}

	// Send status gauges
	allStatuses := []string{
		k6client.StatusCreated,
		k6client.StatusInitializing,
		k6client.StatusRunning,
		k6client.StatusProcessingMetrics,
		k6client.StatusCompleted,
		k6client.StatusAborted,
	}

	// For each possible status, send gauge metrics
	for _, status := range allStatuses {
		if statusCounts[status] == nil {
			// No runs in this status, but we still need to send 0 values
			// for previously seen label combinations
			continue
		}

		for labelKey, count := range statusCounts[status] {
			parts := splitLabelKey(labelKey)
			if len(parts) == 3 {
				ch <- prometheus.MustNewConstMetric(
					testRunStatusDesc,
					prometheus.GaugeValue,
					float64(count),
					parts[0], // test_name
					parts[1], // test_id
					parts[2], // project_id
					status,
				)
			}
		}
	}

	// Send total counters based on state history
	for _, runState := range c.stateManager.GetAllStates() {
		// For each status this run has been in, send a counter
		for status := range runState.StatusHistory {
			ch <- prometheus.MustNewConstMetric(
				testRunTotalDesc,
				prometheus.CounterValue,
				1,
				runState.TestName,
				strconv.Itoa(runState.TestID),
				strconv.Itoa(runState.ProjectID),
				status,
			)
		}

		// Send result counter if completed
		if runState.Result != nil {
			ch <- prometheus.MustNewConstMetric(
				testRunResultTotalDesc,
				prometheus.CounterValue,
				1,
				runState.TestName,
				strconv.Itoa(runState.TestID),
				strconv.Itoa(runState.ProjectID),
				*runState.Result,
			)
		}
	}

	return nil
}

// updateTestCache updates the cached test information
func (c *Collector) updateTestCache(ctx context.Context) error {
	tests, err := c.client.ListTests(ctx, nil)
	if err != nil {
		return fmt.Errorf("list tests: %w", err)
	}

	c.testCacheMutex.Lock()
	defer c.testCacheMutex.Unlock()

	// Clear and rebuild cache
	c.testCache = make(map[int]*k6client.Test)
	for i := range tests {
		c.testCache[tests[i].ID] = &tests[i]
	}
	c.lastTestFetch = time.Now()

	c.logger.Info("updated test cache", zap.Int("test_count", len(tests)))
	return nil
}

// getTestName returns the test name from cache
func (c *Collector) getTestName(testID int) string {
	c.testCacheMutex.RLock()
	defer c.testCacheMutex.RUnlock()

	if test, exists := c.testCache[testID]; exists {
		return test.Name
	}
	return ""
}

// splitLabelKey splits a label key back into its components
func splitLabelKey(key string) []string {
	// Simple split - in production you might want more robust parsing
	parts := make([]string, 0, 3)
	lastIdx := 0
	for i := 0; i < 2; i++ {
		idx := indexOf(key, "|", lastIdx)
		if idx == -1 {
			break
		}
		parts = append(parts, key[lastIdx:idx])
		lastIdx = idx + 1
	}
	if lastIdx < len(key) {
		parts = append(parts, key[lastIdx:])
	}
	return parts
}

// indexOf finds the index of substr in s starting from start
func indexOf(s, substr string, start int) int {
	idx := start
	for idx < len(s) {
		if idx+len(substr) <= len(s) && s[idx:idx+len(substr)] == substr {
			return idx
		}
		idx++
	}
	return -1
}

// StartBackgroundTasks starts background tasks like state cleanup
func (c *Collector) StartBackgroundTasks(ctx context.Context) {
	// State cleanup task
	go func() {
		ticker := time.NewTicker(c.config.StateCleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				removed := c.stateManager.Cleanup(24 * time.Hour)
				if removed > 0 {
					c.logger.Info("cleaned up old test run states", zap.Int("removed", removed))
				}
			}
		}
	}()
}
