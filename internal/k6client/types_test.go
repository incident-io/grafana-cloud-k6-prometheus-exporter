package k6client

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsTerminalStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{StatusCreated, false},
		{StatusInitializing, false},
		{StatusRunning, false},
		{StatusProcessingMetrics, false},
		{StatusCompleted, true},
		{StatusAborted, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsTerminalStatus(tt.status))
		})
	}
}

func TestGetResult(t *testing.T) {
	tests := []struct {
		name     string
		testRun  TestRun
		expected string
	}{
		{
			name: "result_passed",
			testRun: TestRun{
				Result: stringPtr(ResultPassed),
				Status: StatusCompleted,
			},
			expected: ResultPassed,
		},
		{
			name: "result_failed",
			testRun: TestRun{
				Result: stringPtr(ResultFailed),
				Status: StatusCompleted,
			},
			expected: ResultFailed,
		},
		{
			name: "result_nil_aborted",
			testRun: TestRun{
				Result: nil,
				Status: StatusAborted,
			},
			expected: "aborted",
		},
		{
			name: "result_nil_running",
			testRun: TestRun{
				Result: nil,
				Status: StatusRunning,
			},
			expected: "unknown",
		},
		{
			name: "result_nil_completed",
			testRun: TestRun{
				Result: nil,
				Status: StatusCompleted,
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.testRun.GetResult())
		})
	}
}

func TestGetDuration(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	
	tests := []struct {
		name     string
		testRun  TestRun
		minDuration float64
		maxDuration float64
	}{
		{
			name: "completed_test_run",
			testRun: TestRun{
				Created: oneHourAgo,
				Ended:   &now,
			},
			minDuration: 3599, // At least 3599 seconds
			maxDuration: 3601, // At most 3601 seconds (accounting for test execution time)
		},
		{
			name: "still_running",
			testRun: TestRun{
				Created: oneHourAgo,
				Ended:   nil,
			},
			minDuration: 3600, // At least 1 hour
			maxDuration: 3610, // Allow up to 10 seconds for test execution
		},
		{
			name: "just_started",
			testRun: TestRun{
				Created: now,
				Ended:   nil,
			},
			minDuration: 0,
			maxDuration: 10, // Should be very small
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := tt.testRun.GetDuration()
			assert.GreaterOrEqual(t, duration, tt.minDuration)
			assert.LessOrEqual(t, duration, tt.maxDuration)
		})
	}
}

func TestGetVUH(t *testing.T) {
	tests := []struct {
		name     string
		testRun  TestRun
		expected float64
	}{
		{
			name: "with_vuh",
			testRun: TestRun{
				Cost: &Cost{
					VUH: 123.45,
				},
			},
			expected: 123.45,
		},
		{
			name: "nil_cost",
			testRun: TestRun{
				Cost: nil,
			},
			expected: 0,
		},
		{
			name: "zero_vuh",
			testRun: TestRun{
				Cost: &Cost{
					VUH: 0,
				},
			},
			expected: 0,
		},
		{
			name: "with_full_cost_details",
			testRun: TestRun{
				Cost: &Cost{
					VUH: 100.5,
					VUHBreakdown: map[string]float64{
						"cloud": 80.5,
						"local": 20.0,
					},
					BilledVUH:     100.5,
					BilledDollars: 10.05,
				},
			},
			expected: 100.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.testRun.GetVUH())
		})
	}
}

func TestStatusHistoryEntry(t *testing.T) {
	now := time.Now()
	
	entry := StatusHistoryEntry{
		Type:    StatusRunning,
		Entered: now,
	}
	
	assert.Equal(t, StatusRunning, entry.Type)
	assert.Equal(t, now, entry.Entered)
}

func TestProjectFields(t *testing.T) {
	now := time.Now()
	
	project := Project{
		ID:           123,
		Name:         "Test Project",
		Description:  "A test project",
		Created:      now,
		Updated:      now.Add(1 * time.Hour),
		Organization: 456,
	}
	
	assert.Equal(t, 123, project.ID)
	assert.Equal(t, "Test Project", project.Name)
	assert.Equal(t, "A test project", project.Description)
	assert.Equal(t, now, project.Created)
	assert.Equal(t, now.Add(1*time.Hour), project.Updated)
	assert.Equal(t, 456, project.Organization)
}

func TestTestFields(t *testing.T) {
	now := time.Now()
	baselineID := 999
	
	test := Test{
		ID:                123,
		ProjectID:         456,
		Name:              "Performance Test",
		BaselineTestRunID: &baselineID,
		Created:           now,
		Updated:           now.Add(1 * time.Hour),
	}
	
	assert.Equal(t, 123, test.ID)
	assert.Equal(t, 456, test.ProjectID)
	assert.Equal(t, "Performance Test", test.Name)
	assert.NotNil(t, test.BaselineTestRunID)
	assert.Equal(t, 999, *test.BaselineTestRunID)
	assert.Equal(t, now, test.Created)
	assert.Equal(t, now.Add(1*time.Hour), test.Updated)
}

func TestCostBreakdown(t *testing.T) {
	cost := Cost{
		VUH: 150.0,
		VUHBreakdown: map[string]float64{
			"cloud":     100.0,
			"dedicated": 50.0,
		},
		BilledVUH:     150.0,
		BilledDollars: 15.0,
	}
	
	assert.Equal(t, 150.0, cost.VUH)
	assert.Equal(t, 100.0, cost.VUHBreakdown["cloud"])
	assert.Equal(t, 50.0, cost.VUHBreakdown["dedicated"])
	assert.Equal(t, 150.0, cost.BilledVUH)
	assert.Equal(t, 15.0, cost.BilledDollars)
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}