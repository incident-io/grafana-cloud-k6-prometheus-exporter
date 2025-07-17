package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/grafana-cloud-k6-prometheus-exporter/internal/config"
	"github.com/grafana-cloud-k6-prometheus-exporter/internal/k6client"
	"github.com/grafana-cloud-k6-prometheus-exporter/internal/state"
)

// mockK6Client implements a mock k6 API client for testing
type mockK6Client struct {
	tests    []k6client.Test
	testRuns []k6client.TestRun
	err      error
}

func (m *mockK6Client) ListProjects(ctx context.Context) ([]k6client.Project, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []k6client.Project{}, nil
}

func (m *mockK6Client) ListTests(ctx context.Context, projectID *int) ([]k6client.Test, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tests, nil
}

func (m *mockK6Client) ListTestRuns(ctx context.Context, testID int, since *time.Time) ([]k6client.TestRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	var runs []k6client.TestRun
	for _, run := range m.testRuns {
		if run.TestID == testID {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (m *mockK6Client) GetTestRun(ctx context.Context, testID, runID int) (*k6client.TestRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, run := range m.testRuns {
		if run.TestID == testID && run.ID == runID {
			return &run, nil
		}
	}
	return nil, fmt.Errorf("test run not found")
}

func (m *mockK6Client) GetAllTestRuns(ctx context.Context, projectIDs []string, since *time.Time) ([]k6client.TestRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	// Add test names to status details for testing
	for i := range m.testRuns {
		if m.testRuns[i].StatusDetails == nil {
			m.testRuns[i].StatusDetails = make(map[string]interface{})
		}
		// Find the test name
		for _, test := range m.tests {
			if test.ID == m.testRuns[i].TestID {
				m.testRuns[i].StatusDetails["test_name"] = test.Name
				break
			}
		}
	}
	
	return m.testRuns, nil
}

func TestCollectorDescribe(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		TestCacheTTL:         60 * time.Second,
		StateCleanupInterval: 5 * time.Minute,
		APITimeout:           30 * time.Second,
	}
	
	client := &mockK6Client{}
	stateManager := state.NewManager(logger)
	collector := NewCollector(client, stateManager, cfg, logger)

	// Collect descriptions
	ch := make(chan *prometheus.Desc, 20)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	// Verify expected metrics are described
	expectedDescs := []string{
		"k6_test_run_total",
		"k6_test_run_status",
		"k6_test_run_result_total",
		"k6_test_run_duration_seconds",
		"k6_test_run_vuh_consumed",
		"k6_test_run_info",
	}

	descriptions := make([]string, 0)
	for desc := range ch {
		descriptions = append(descriptions, desc.String())
	}

	for _, expected := range expectedDescs {
		found := false
		for _, desc := range descriptions {
			if strings.Contains(desc, expected) {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected metric %s not found in descriptions", expected)
	}
}

func TestCollectorCollect(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		TestCacheTTL:         60 * time.Second,
		StateCleanupInterval: 5 * time.Minute,
		APITimeout:           30 * time.Second,
		Projects:             []string{},
	}

	now := time.Now()
	endTime := now.Add(30 * time.Minute)
	resultPassed := "passed"
	
	// Setup mock data
	mockClient := &mockK6Client{
		tests: []k6client.Test{
			{ID: 1, Name: "Performance Test", ProjectID: 100},
			{ID: 2, Name: "Load Test", ProjectID: 100},
		},
		testRuns: []k6client.TestRun{
			{
				ID:        10,
				TestID:    1,
				ProjectID: 100,
				Status:    k6client.StatusRunning,
				StartedBy: "user1@example.com",
				Created:   now.Add(-10 * time.Minute),
				Cost:      &k6client.Cost{VUH: 5.5},
			},
			{
				ID:        11,
				TestID:    1,
				ProjectID: 100,
				Status:    k6client.StatusCompleted,
				StartedBy: "user2@example.com",
				Created:   now.Add(-40 * time.Minute),
				Ended:     &endTime,
				Result:    &resultPassed,
				Cost:      &k6client.Cost{VUH: 10.0},
			},
			{
				ID:        12,
				TestID:    2,
				ProjectID: 100,
				Status:    k6client.StatusInitializing,
				StartedBy: "user1@example.com",
				Created:   now.Add(-2 * time.Minute),
			},
		},
	}

	stateManager := state.NewManager(logger)
	
	// Create a custom registry for testing
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(mockClient, stateManager, cfg, logger, registry)
	registry.MustRegister(collector)

	// Collect metrics
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	// Verify metrics
	metricMap := make(map[string]*dto.MetricFamily)
	for _, mf := range metricFamilies {
		metricMap[*mf.Name] = mf
	}

	// Check test run info metrics
	infoMetric, exists := metricMap["k6_test_run_info"]
	assert.True(t, exists, "k6_test_run_info metric should exist")
	assert.Len(t, infoMetric.Metric, 3, "Should have 3 test runs")

	// Check test run status metrics
	statusMetric, exists := metricMap["k6_test_run_status"]
	assert.True(t, exists, "k6_test_run_status metric should exist")
	assert.Greater(t, len(statusMetric.Metric), 0, "Should have status metrics")

	// Check duration metrics
	durationMetric, exists := metricMap["k6_test_run_duration_seconds"]
	assert.True(t, exists, "k6_test_run_duration_seconds metric should exist")
	assert.Len(t, durationMetric.Metric, 3, "Should have duration for all test runs")

	// Check VUH metrics
	vuhMetric, exists := metricMap["k6_test_run_vuh_consumed"]
	assert.True(t, exists, "k6_test_run_vuh_consumed metric should exist")
	assert.Len(t, vuhMetric.Metric, 2, "Should have VUH for 2 test runs with cost data")

	// Note: test runs tracked metric is handled by operational metrics registered separately
}

func TestCollectorWithErrors(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		TestCacheTTL:         60 * time.Second,
		StateCleanupInterval: 5 * time.Minute,
		APITimeout:           30 * time.Second,
	}

	// Setup mock client that returns errors
	mockClient := &mockK6Client{
		err: fmt.Errorf("API error"),
	}

	stateManager := state.NewManager(logger)
	
	// Create a custom registry for testing
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(mockClient, stateManager, cfg, logger, registry)

	// Register the collector
	registry.MustRegister(collector)
	
	// Gather metrics to trigger collection
	metricFamilies, err := registry.Gather()
	require.NoError(t, err, "Should be able to gather metrics even with API errors")
	
	// Check that we still get operational metrics
	foundOperationalMetrics := false
	for _, mf := range metricFamilies {
		name := *mf.Name
		if strings.Contains(name, "k6_exporter_") {
			foundOperationalMetrics = true
			break
		}
	}
	
	assert.True(t, foundOperationalMetrics, "Should still have operational metrics even with API errors")
}

func TestGetTestName(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		TestCacheTTL: 60 * time.Second,
	}

	mockClient := &mockK6Client{
		tests: []k6client.Test{
			{ID: 1, Name: "Performance Test"},
			{ID: 2, Name: "Load Test"},
		},
	}

	stateManager := state.NewManager(logger)
	
	// Use custom registry for testing
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(mockClient, stateManager, cfg, logger, registry)

	// Update test cache
	ctx := context.Background()
	err := collector.updateTestCache(ctx)
	require.NoError(t, err)

	// Test getting names
	assert.Equal(t, "Performance Test", collector.getTestName(1))
	assert.Equal(t, "Load Test", collector.getTestName(2))
	assert.Equal(t, "", collector.getTestName(999)) // Non-existent test
}

func TestUpdateTestCache(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		TestCacheTTL: 60 * time.Second,
	}

	tests := []k6client.Test{
		{ID: 1, Name: "Test 1"},
		{ID: 2, Name: "Test 2"},
		{ID: 3, Name: "Test 3"},
	}

	mockClient := &mockK6Client{
		tests: tests,
	}

	stateManager := state.NewManager(logger)
	
	// Use custom registry for testing
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(mockClient, stateManager, cfg, logger, registry)

	// Update cache
	ctx := context.Background()
	err := collector.updateTestCache(ctx)
	require.NoError(t, err)

	// Verify cache contents
	assert.Equal(t, "Test 1", collector.getTestName(1))
	assert.Equal(t, "Test 2", collector.getTestName(2))
	assert.Equal(t, "Test 3", collector.getTestName(3))
}

func TestSplitLabelKey(t *testing.T) {
	tests := []struct {
		key      string
		expected []string
	}{
		{
			key:      "test_name|123|456",
			expected: []string{"test_name", "123", "456"},
		},
		{
			key:      "test with spaces|1|2",
			expected: []string{"test with spaces", "1", "2"},
		},
		{
			key:      "single",
			expected: []string{"single"},
		},
		{
			key:      "two|parts",
			expected: []string{"two", "parts"},
		},
		{
			key:      "|empty|start",
			expected: []string{"", "empty", "start"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := splitLabelKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBackgroundTasks(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		TestCacheTTL:         60 * time.Second,
		StateCleanupInterval: 100 * time.Millisecond, // Fast cleanup for testing
		APITimeout:           30 * time.Second,
	}

	mockClient := &mockK6Client{}
	stateManager := state.NewManager(logger)
	
	// Use custom registry for testing
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(mockClient, stateManager, cfg, logger, registry)

	// Add some old state
	oldState := &state.TestRunState{
		TestRunID:     1,
		TestID:        100,
		ProjectID:     1000,
		CurrentStatus: "completed",
		Created:       time.Now().Add(-25 * time.Hour),
		LastUpdated:   time.Now().Add(-25 * time.Hour),
		Ended:         timePtr(time.Now().Add(-25 * time.Hour)),
	}
	stateManager.UpdateTestRun(oldState)
	
	// Verify state was added
	assert.Equal(t, 1, stateManager.GetStateCount())

	// Start background tasks
	ctx, cancel := context.WithCancel(context.Background())
	collector.StartBackgroundTasks(ctx)

	// Wait for cleanup to run (cleanup interval is 100ms, so wait a bit longer)
	time.Sleep(250 * time.Millisecond)

	// Old state should be cleaned up
	assert.Equal(t, 0, stateManager.GetStateCount())
	
	// Cancel context
	cancel()
}

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}

// Integration test with real Prometheus metrics gathering
func TestCollectorIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		TestCacheTTL:         60 * time.Second,
		StateCleanupInterval: 5 * time.Minute,
		APITimeout:           30 * time.Second,
	}

	// Create test data
	now := time.Now()
	resultFailed := "failed"
	
	mockClient := &mockK6Client{
		tests: []k6client.Test{
			{ID: 1, Name: "API Test", ProjectID: 100},
		},
		testRuns: []k6client.TestRun{
			{
				ID:        1,
				TestID:    1,
				ProjectID: 100,
				Status:    k6client.StatusCompleted,
				StartedBy: "ci@example.com",
				Created:   now.Add(-1 * time.Hour),
				Ended:     &now,
				Result:    &resultFailed,
				Cost:      &k6client.Cost{VUH: 25.5},
				StatusHistory: []k6client.StatusHistoryEntry{
					{Type: k6client.StatusCreated, Entered: now.Add(-1 * time.Hour)},
					{Type: k6client.StatusInitializing, Entered: now.Add(-55 * time.Minute)},
					{Type: k6client.StatusRunning, Entered: now.Add(-50 * time.Minute)},
					{Type: k6client.StatusCompleted, Entered: now},
				},
			},
		},
	}

	stateManager := state.NewManager(logger)
	
	// Register and gather metrics
	registry := prometheus.NewRegistry()
	collector := NewCollectorWithRegistry(mockClient, stateManager, cfg, logger, registry)
	registry.MustRegister(collector)

	// First collection to populate state
	_, err := registry.Gather()
	require.NoError(t, err)

	// Verify state manager has the test run
	assert.Equal(t, 1, stateManager.GetStateCount())
	runState := stateManager.GetTestRunState(1)
	require.NotNil(t, runState)
	assert.Equal(t, k6client.StatusCompleted, runState.CurrentStatus)
	assert.NotNil(t, runState.Result)
	assert.Equal(t, "failed", *runState.Result)
}