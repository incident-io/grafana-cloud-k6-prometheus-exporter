package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.states)
	assert.Equal(t, 0, manager.GetStateCount())
}

func TestRecordTestRunStatus(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	// First, we need to create test run states before we can record status
	// RecordTestRunStatus only tracks status changes for existing runs
	
	// Create initial states
	manager.UpdateTestRun(&TestRunState{
		TestRunID:     1,
		TestID:        100,
		ProjectID:     1000,
		CurrentStatus: "created",
		Created:       time.Now(),
	})

	tests := []struct {
		name      string
		runID     int
		status    string
		wantNew   bool
		desc      string
	}{
		{
			name:    "existing_run_same_status",
			runID:   1,
			status:  "created",
			wantNew: false,
			desc:    "same status for existing test run",
		},
		{
			name:    "existing_run_new_status",
			runID:   1,
			status:  "running",
			wantNew: true,
			desc:    "new status for existing test run",
		},
		{
			name:    "existing_run_duplicate_new_status",
			runID:   1,
			status:  "running",
			wantNew: false,
			desc:    "duplicate of the new status",
		},
		{
			name:    "new_test_run",
			runID:   2,
			status:  "created",
			wantNew: true,
			desc:    "new test run ID (not in state)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNew := manager.RecordTestRunStatus(tt.runID, tt.status)
			assert.Equal(t, tt.wantNew, gotNew, tt.desc)
		})
	}

	// Verify state was properly recorded for run 1
	state1 := manager.GetTestRunState(1)
	require.NotNil(t, state1)
	assert.Contains(t, state1.StatusHistory, "created")
	assert.Contains(t, state1.StatusHistory, "running")
	assert.Equal(t, 2, len(state1.StatusHistory))
}

func TestUpdateTestRun(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	now := time.Now()
	endTime := now.Add(1 * time.Hour)
	result := "passed"

	// Create initial state
	state1 := &TestRunState{
		TestRunID:     1,
		TestID:        100,
		ProjectID:     1000,
		TestName:      "my-test",
		CurrentStatus: "created",
		Created:       now,
		StartedBy:     "user@example.com",
		VUH:           0,
	}
	manager.UpdateTestRun(state1)

	// Verify initial state
	retrieved := manager.GetTestRunState(1)
	require.NotNil(t, retrieved)
	assert.Equal(t, "created", retrieved.CurrentStatus)
	assert.Contains(t, retrieved.StatusHistory, "created")
	assert.Equal(t, 1, len(retrieved.StatusHistory))

	// Update with new status
	state2 := &TestRunState{
		TestRunID:     1,
		TestID:        100,
		ProjectID:     1000,
		TestName:      "my-test",
		CurrentStatus: "running",
		Created:       now,
		StartedBy:     "user@example.com",
		VUH:           0.5,
	}
	manager.UpdateTestRun(state2)

	// Verify update
	retrieved = manager.GetTestRunState(1)
	require.NotNil(t, retrieved)
	assert.Equal(t, "running", retrieved.CurrentStatus)
	assert.Contains(t, retrieved.StatusHistory, "created")
	assert.Contains(t, retrieved.StatusHistory, "running")
	assert.Equal(t, 2, len(retrieved.StatusHistory))
	assert.Equal(t, 0.5, retrieved.VUH)

	// Final update with completion - should remove the test run from state
	state3 := &TestRunState{
		TestRunID:     1,
		TestID:        100,
		ProjectID:     1000,
		TestName:      "my-test",
		CurrentStatus: "completed",
		Created:       now,
		Ended:         &endTime,
		Result:        &result,
		StartedBy:     "user@example.com",
		VUH:           1.5,
	}
	manager.UpdateTestRun(state3)

	// Verify the test run was removed from state (completed runs are not tracked)
	retrieved = manager.GetTestRunState(1)
	assert.Nil(t, retrieved, "Completed test runs should be removed from state")
	assert.Equal(t, 0, manager.GetStateCount(), "Manager should have no states after completing the only test run")
}

func TestGetAllStates(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	// Add multiple test runs
	for i := 1; i <= 3; i++ {
		state := &TestRunState{
			TestRunID:     i,
			TestID:        100 + i,
			ProjectID:     1000,
			TestName:      "test-" + string(rune('0'+i)),
			CurrentStatus: "running",
			Created:       time.Now(),
			StartedBy:     "user@example.com",
		}
		manager.UpdateTestRun(state)
	}

	// Get all states
	states := manager.GetAllStates()
	assert.Len(t, states, 3)

	// Verify each state is a copy (not same reference)
	for _, state := range states {
		original := manager.states[state.TestRunID]
		assert.NotSame(t, state, original, "returned state should be a copy")
		// Verify the map is also a copy by checking the addresses are different
		assert.NotEqual(t, fmt.Sprintf("%p", state.StatusHistory), fmt.Sprintf("%p", original.StatusHistory), "status history should be a copy")
	}
}

func TestCleanup(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	now := time.Now()
	oldTime := now.Add(-25 * time.Hour)
	recentTime := now.Add(-1 * time.Hour)

	// Skip adding completed test run as they are rejected now
	// Completed runs are immediately removed and not stored

	// Add old abandoned test run
	state2 := &TestRunState{
		TestRunID:     2,
		TestID:        102,
		ProjectID:     1000,
		CurrentStatus: "running",
		Created:       oldTime,
		LastUpdated:   oldTime,
	}
	manager.UpdateTestRun(state2)
	// Manually set the LastUpdated to old time after UpdateTestRun
	manager.mu.Lock()
	manager.states[2].LastUpdated = oldTime
	manager.mu.Unlock()

	// Add recent test run
	state3 := &TestRunState{
		TestRunID:     3,
		TestID:        103,
		ProjectID:     1000,
		CurrentStatus: "running",
		Created:       recentTime,
		LastUpdated:   now,
	}
	manager.UpdateTestRun(state3)

	// Before cleanup - only active test runs are stored
	assert.Equal(t, 2, manager.GetStateCount(), "Should have 2 active test runs")

	// Run cleanup
	removed := manager.Cleanup(24 * time.Hour)
	
	// Only the old abandoned test run (2) should be removed
	assert.Equal(t, 1, removed)
	assert.Equal(t, 1, manager.GetStateCount())

	// Verify only recent test run remains
	assert.Nil(t, manager.GetTestRunState(2))
	assert.NotNil(t, manager.GetTestRunState(3))
}

func TestHasSeenStatus(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	// Test non-existent run
	assert.False(t, manager.HasSeenStatus(1, "created"))

	// Add a test run
	state := &TestRunState{
		TestRunID:     1,
		TestID:        100,
		ProjectID:     1000,
		CurrentStatus: "created",
		Created:       time.Now(),
	}
	manager.UpdateTestRun(state)

	// Test existing status
	assert.True(t, manager.HasSeenStatus(1, "created"))
	assert.False(t, manager.HasSeenStatus(1, "running"))

	// Update to new status
	state.CurrentStatus = "running"
	manager.UpdateTestRun(state)

	// Both statuses should be seen
	assert.True(t, manager.HasSeenStatus(1, "created"))
	assert.True(t, manager.HasSeenStatus(1, "running"))
}

func TestCleanupCompletedRuns(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	// Add some active test runs
	for i := 1; i <= 3; i++ {
		state := &TestRunState{
			TestRunID:     i,
			TestID:        100 + i,
			ProjectID:     1000,
			CurrentStatus: "running",
			Created:       time.Now(),
		}
		manager.UpdateTestRun(state)
	}

	// Verify all are tracked
	assert.Equal(t, 3, manager.GetStateCount())

	// Call CleanupCompletedRuns - should not remove any since all are running
	removed := manager.CleanupCompletedRuns()
	assert.Equal(t, 0, removed)
	assert.Equal(t, 3, manager.GetStateCount())

	// Now try to add completed runs - they should be rejected
	state := &TestRunState{
		TestRunID:     4,
		TestID:        104,
		ProjectID:     1000,
		CurrentStatus: "completed",
		Created:       time.Now(),
	}
	manager.UpdateTestRun(state)
	
	// Should still have 3 (completed run was rejected)
	assert.Equal(t, 3, manager.GetStateCount())
}

func TestGetStatusCounts(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	// Add test runs with different active statuses only
	statuses := []string{"created", "running", "running", "initializing", "running"}
	for i, status := range statuses {
		state := &TestRunState{
			TestRunID:     i + 1,
			TestID:        100 + i,
			ProjectID:     1000,
			CurrentStatus: status,
			Created:       time.Now(),
		}
		manager.UpdateTestRun(state)
	}

	// Get status counts - only active states
	counts := manager.GetStatusCounts()
	assert.Equal(t, 1, counts["created"])
	assert.Equal(t, 3, counts["running"])
	assert.Equal(t, 1, counts["initializing"])
	assert.Equal(t, 0, counts["completed"]) // Completed runs are not tracked
	assert.Equal(t, 0, counts["aborted"]) // Aborted runs are not tracked
}

func TestConcurrentAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	manager := NewManager(logger)

	// Run concurrent operations
	done := make(chan bool)
	
	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				state := &TestRunState{
					TestRunID:     id*1000 + j,
					TestID:        100 + id,
					ProjectID:     1000,
					CurrentStatus: "running",
					Created:       time.Now(),
				}
				manager.UpdateTestRun(state)
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 200; j++ {
				manager.GetAllStates()
				manager.GetStatusCounts()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Verify state
	assert.Equal(t, 1000, manager.GetStateCount())
}