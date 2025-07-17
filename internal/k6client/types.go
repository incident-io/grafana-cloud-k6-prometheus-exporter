package k6client

import (
	"time"
)

// Test represents a k6 load test
type Test struct {
	ID                int       `json:"id"`
	ProjectID         int       `json:"project_id"`
	Name              string    `json:"name"`
	BaselineTestRunID *int      `json:"baseline_test_run_id"`
	Created           time.Time `json:"created"`
	Updated           time.Time `json:"updated"`
}

// TestRun represents a k6 test run
type TestRun struct {
	ID            int                    `json:"id"`
	TestID        int                    `json:"test_id"`
	ProjectID     int                    `json:"project_id"`
	StartedBy     string                 `json:"started_by"`
	Created       time.Time              `json:"created"`
	Ended         *time.Time             `json:"ended"`
	Status        string                 `json:"status"`
	StatusDetails map[string]interface{} `json:"status_details"`
	StatusHistory []StatusHistoryEntry   `json:"status_history"`
	Result        *string                `json:"result"`
	ResultDetails map[string]interface{} `json:"result_details"`
	Cost          *Cost                  `json:"cost"`
}

// StatusHistoryEntry represents a status change in a test run
type StatusHistoryEntry struct {
	Type    string    `json:"type"`
	Entered time.Time `json:"entered"`
}

// Cost represents the cost information for a test run
type Cost struct {
	VUH           float64            `json:"vuh"`
	VUHBreakdown  map[string]float64 `json:"vuh_breakdown"`
	BilledVUH     float64            `json:"billed_vuh"`
	BilledDollars float64            `json:"billed_dollars"`
}

// Project represents a k6 project
type Project struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
	Organization int       `json:"organization"`
}

// ListResponse represents a paginated list response
type ListResponse struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Value    any     `json:"value"`
}

// TestListResponse represents a list of tests
type TestListResponse struct {
	Count    int     `json:"count"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
	Value    []Test  `json:"value"`
}

// TestRunListResponse represents a list of test runs
type TestRunListResponse struct {
	Count    int       `json:"count"`
	Next     *string   `json:"next"`
	Previous *string   `json:"previous"`
	Value    []TestRun `json:"value"`
}

// ProjectListResponse represents a list of projects
type ProjectListResponse struct {
	Count    int       `json:"count"`
	Next     *string   `json:"next"`
	Previous *string   `json:"previous"`
	Value    []Project `json:"value"`
}

// Known test run status values
const (
	StatusCreated           = "created"
	StatusInitializing      = "initializing"
	StatusRunning           = "running"
	StatusProcessingMetrics = "processing_metrics"
	StatusCompleted         = "completed"
	StatusAborted           = "aborted"
)

// Known test run result values
const (
	ResultPassed = "passed"
	ResultFailed = "failed"
)

// IsTerminalStatus returns true if the status represents a terminal state
func IsTerminalStatus(status string) bool {
	return status == StatusCompleted || status == StatusAborted
}

// GetResult returns a normalized result string or "unknown" if not set
func (tr *TestRun) GetResult() string {
	if tr.Result == nil {
		if tr.Status == StatusAborted {
			return "aborted"
		}
		return "unknown"
	}
	return *tr.Result
}

// GetDuration returns the duration of the test run in seconds
func (tr *TestRun) GetDuration() float64 {
	if tr.Ended == nil {
		// Still running, calculate from start time
		return time.Since(tr.Created).Seconds()
	}
	return tr.Ended.Sub(tr.Created).Seconds()
}

// GetVUH returns the Virtual User Hours consumed, or 0 if not available
func (tr *TestRun) GetVUH() float64 {
	if tr.Cost == nil {
		return 0
	}
	return tr.Cost.VUH
}
