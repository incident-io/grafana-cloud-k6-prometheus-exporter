package state

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// TestRunState tracks the state of a test run
type TestRunState struct {
	TestRunID     int
	TestID        int
	ProjectID     int
	TestName      string
	CurrentStatus string
	StatusHistory map[string]time.Time // Status -> First seen at
	LastUpdated   time.Time
	Created       time.Time
	Ended         *time.Time
	Result        *string
	StartedBy     string
	VUH           float64
}

// Manager manages test run states to prevent duplicate counting
type Manager struct {
	mu     sync.RWMutex
	states map[int]*TestRunState // Key is TestRunID
	logger *zap.Logger
}

// NewManager creates a new state manager
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		states: make(map[int]*TestRunState),
		logger: logger,
	}
}

// RecordTestRunStatus records a test run status and returns true if this is a new status
func (m *Manager) RecordTestRunStatus(runID int, status string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[runID]
	if !exists {
		// This is a new test run we haven't seen before
		m.logger.Debug("recording new test run",
			zap.Int("run_id", runID),
			zap.String("status", status),
		)
		return true
	}

	// Check if we've already seen this status
	if _, seen := state.StatusHistory[status]; seen {
		return false
	}

	// This is a new status for this test run
	state.StatusHistory[status] = time.Now()
	state.CurrentStatus = status
	state.LastUpdated = time.Now()

	m.logger.Debug("recording new status for test run",
		zap.Int("run_id", runID),
		zap.String("status", status),
		zap.String("previous_status", state.CurrentStatus),
	)

	return true
}

// UpdateTestRun updates or creates a test run state
func (m *Manager) UpdateTestRun(state *TestRunState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Skip storing completed or aborted runs
	if state.CurrentStatus == "completed" || state.CurrentStatus == "aborted" {
		// If we already have this run in state, remove it
		if _, exists := m.states[state.TestRunID]; exists {
			delete(m.states, state.TestRunID)
			m.logger.Debug("removed completed test run from state",
				zap.Int("run_id", state.TestRunID),
				zap.String("status", state.CurrentStatus),
			)
		}
		return
	}

	existing, exists := m.states[state.TestRunID]
	if !exists {
		// Initialize status history
		state.StatusHistory = make(map[string]time.Time)
		state.StatusHistory[state.CurrentStatus] = state.Created
		state.LastUpdated = time.Now()
		m.states[state.TestRunID] = state
		
		m.logger.Debug("created new test run state",
			zap.Int("run_id", state.TestRunID),
			zap.String("status", state.CurrentStatus),
		)
		return
	}

	// Update existing state
	existing.CurrentStatus = state.CurrentStatus
	existing.LastUpdated = time.Now()
	existing.Ended = state.Ended
	existing.Result = state.Result
	existing.VUH = state.VUH

	// Record new status if we haven't seen it
	if _, seen := existing.StatusHistory[state.CurrentStatus]; !seen {
		existing.StatusHistory[state.CurrentStatus] = time.Now()
	}
}

// GetTestRunState returns the state of a test run
func (m *Manager) GetTestRunState(runID int) *TestRunState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[runID]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	stateCopy := *state
	stateCopy.StatusHistory = make(map[string]time.Time, len(state.StatusHistory))
	for k, v := range state.StatusHistory {
		stateCopy.StatusHistory[k] = v
	}

	return &stateCopy
}

// GetAllStates returns all test run states
func (m *Manager) GetAllStates() []*TestRunState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]*TestRunState, 0, len(m.states))
	for _, state := range m.states {
		// Create a copy
		stateCopy := *state
		stateCopy.StatusHistory = make(map[string]time.Time, len(state.StatusHistory))
		for k, v := range state.StatusHistory {
			stateCopy.StatusHistory[k] = v
		}
		states = append(states, &stateCopy)
	}

	return states
}

// Cleanup removes old test run states
func (m *Manager) Cleanup(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for runID, state := range m.states {
		// Remove if:
		// 1. The test run ended and it's older than maxAge
		// 2. The test run hasn't been updated in maxAge (likely stuck/abandoned)
		shouldRemove := false
		
		if state.Ended != nil && state.Ended.Before(cutoff) {
			shouldRemove = true
		} else if state.LastUpdated.Before(cutoff) {
			shouldRemove = true
		}

		if shouldRemove {
			delete(m.states, runID)
			removed++
			m.logger.Debug("removed old test run state",
				zap.Int("run_id", runID),
				zap.Time("last_updated", state.LastUpdated),
			)
		}
	}

	if removed > 0 {
		m.logger.Info("cleaned up old test run states",
			zap.Int("removed", removed),
			zap.Int("remaining", len(m.states)),
		)
	}

	return removed
}

// GetStateCount returns the number of tracked test runs
func (m *Manager) GetStateCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.states)
}

// HasSeenStatus checks if we've already recorded a specific status for a test run
func (m *Manager) HasSeenStatus(runID int, status string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[runID]
	if !exists {
		return false
	}

	_, seen := state.StatusHistory[status]
	return seen
}

// GetStatusCounts returns counts of test runs by current status
func (m *Manager) GetStatusCounts() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int)
	for _, state := range m.states {
		counts[state.CurrentStatus]++
	}

	return counts
}

// CleanupCompletedRuns removes all completed and aborted test runs from state
func (m *Manager) CleanupCompletedRuns() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	for runID, state := range m.states {
		// Remove completed and aborted runs
		if state.CurrentStatus == "completed" || state.CurrentStatus == "aborted" {
			delete(m.states, runID)
			removed++
			m.logger.Debug("removed completed test run state",
				zap.Int("run_id", runID),
				zap.String("status", state.CurrentStatus),
			)
		}
	}

	if removed > 0 {
		m.logger.Info("cleaned up completed test run states",
			zap.Int("removed", removed),
			zap.Int("remaining", len(m.states)),
		)
	}

	return removed
}