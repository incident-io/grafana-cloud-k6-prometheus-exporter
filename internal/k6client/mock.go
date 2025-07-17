package k6client

import (
	"context"
	"fmt"
	"time"
)

// MockClient is a mock implementation of the k6 API client for testing
type MockClient struct {
	// Test data
	Projects []Project
	Tests    []Test
	TestRuns map[int][]TestRun // Key is test ID

	// Error simulation
	ListProjectsError  error
	ListTestsError     error
	ListTestRunsError  error
	GetTestRunError    error
	GetAllTestRunsError error

	// Call tracking
	ListProjectsCalled    int
	ListTestsCalled       int
	ListTestRunsCalled    int
	GetTestRunCalled      int
	GetAllTestRunsCalled  int
}

// NewMockClient creates a new mock client
func NewMockClient() *MockClient {
	return &MockClient{
		Projects: []Project{},
		Tests:    []Test{},
		TestRuns: make(map[int][]TestRun),
	}
}

// ListProjects mock implementation
func (m *MockClient) ListProjects(ctx context.Context) ([]Project, error) {
	m.ListProjectsCalled++
	if m.ListProjectsError != nil {
		return nil, m.ListProjectsError
	}
	return m.Projects, nil
}

// ListTests mock implementation
func (m *MockClient) ListTests(ctx context.Context, projectID *int) ([]Test, error) {
	m.ListTestsCalled++
	if m.ListTestsError != nil {
		return nil, m.ListTestsError
	}
	
	if projectID == nil {
		return m.Tests, nil
	}
	
	// Filter by project ID
	var filtered []Test
	for _, test := range m.Tests {
		if test.ProjectID == *projectID {
			filtered = append(filtered, test)
		}
	}
	return filtered, nil
}

// ListTestRuns mock implementation
func (m *MockClient) ListTestRuns(ctx context.Context, testID int, since *time.Time) ([]TestRun, error) {
	m.ListTestRunsCalled++
	if m.ListTestRunsError != nil {
		return nil, m.ListTestRunsError
	}
	
	runs, exists := m.TestRuns[testID]
	if !exists {
		return []TestRun{}, nil
	}
	
	if since == nil {
		return runs, nil
	}
	
	// Filter by time
	var filtered []TestRun
	for _, run := range runs {
		if run.Created.After(*since) {
			filtered = append(filtered, run)
		}
	}
	return filtered, nil
}

// GetTestRun mock implementation
func (m *MockClient) GetTestRun(ctx context.Context, testID, runID int) (*TestRun, error) {
	m.GetTestRunCalled++
	if m.GetTestRunError != nil {
		return nil, m.GetTestRunError
	}
	
	runs, exists := m.TestRuns[testID]
	if !exists {
		return nil, nil
	}
	
	for _, run := range runs {
		if run.ID == runID {
			runCopy := run
			return &runCopy, nil
		}
	}
	
	return nil, nil
}

// GetAllTestRuns mock implementation
func (m *MockClient) GetAllTestRuns(ctx context.Context, projectIDs []string, since *time.Time) ([]TestRun, error) {
	m.GetAllTestRunsCalled++
	if m.GetAllTestRunsError != nil {
		return nil, m.GetAllTestRunsError
	}
	
	var allRuns []TestRun
	
	// Get tests for specified projects or all tests
	var tests []Test
	if len(projectIDs) > 0 {
		for _, test := range m.Tests {
			for _, pid := range projectIDs {
				var projectID int
				if _, err := fmt.Sscanf(pid, "%d", &projectID); err == nil && test.ProjectID == projectID {
					tests = append(tests, test)
					break
				}
			}
		}
	} else {
		tests = m.Tests
	}
	
	// Get runs for each test
	for _, test := range tests {
		runs, exists := m.TestRuns[test.ID]
		if !exists {
			continue
		}
		
		for _, run := range runs {
			// Add test name to status details
			if run.StatusDetails == nil {
				run.StatusDetails = make(map[string]interface{})
			}
			run.StatusDetails["test_name"] = test.Name
			
			// Filter by time if specified
			if since == nil || run.Created.After(*since) {
				allRuns = append(allRuns, run)
			}
		}
	}
	
	return allRuns, nil
}

// AddTestData is a helper method to easily add test data
func (m *MockClient) AddTestData(project Project, test Test, runs ...TestRun) {
	// Add project if not exists
	found := false
	for _, p := range m.Projects {
		if p.ID == project.ID {
			found = true
			break
		}
	}
	if !found {
		m.Projects = append(m.Projects, project)
	}
	
	// Add test if not exists
	found = false
	for _, t := range m.Tests {
		if t.ID == test.ID {
			found = true
			break
		}
	}
	if !found {
		m.Tests = append(m.Tests, test)
	}
	
	// Add test runs
	if m.TestRuns[test.ID] == nil {
		m.TestRuns[test.ID] = []TestRun{}
	}
	m.TestRuns[test.ID] = append(m.TestRuns[test.ID], runs...)
}

// Reset resets all call counters and errors
func (m *MockClient) Reset() {
	m.ListProjectsCalled = 0
	m.ListTestsCalled = 0
	m.ListTestRunsCalled = 0
	m.GetTestRunCalled = 0
	m.GetAllTestRunsCalled = 0
	
	m.ListProjectsError = nil
	m.ListTestsError = nil
	m.ListTestRunsError = nil
	m.GetTestRunError = nil
	m.GetAllTestRunsError = nil
}