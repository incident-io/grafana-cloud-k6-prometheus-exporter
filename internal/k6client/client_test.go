package k6client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewClient(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewClient("https://api.k6.io", "test-stack-id", "test-token", logger)

	assert.NotNil(t, client)
	assert.Equal(t, "https://api.k6.io", client.baseURL)
	assert.Equal(t, "test-stack-id", client.stackID)
	assert.Equal(t, "test-token", client.apiToken)
	assert.NotNil(t, client.httpClient)
	assert.NotNil(t, client.logger)
}

func TestListProjects(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantProjects   int
		wantErr        bool
	}{
		{
			name: "single_page_response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/cloud/v6/projects", r.URL.Path)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				assert.Equal(t, "1000", r.URL.Query().Get("$top"))

				// Send response
				resp := ProjectListResponse{
					Count: 2,
					Value: []Project{
						{ID: 1, Name: "Project 1"},
						{ID: 2, Name: "Project 2"},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantProjects: 2,
			wantErr:      false,
		},
		{
			name: "paginated_response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") == "2" {
					// Second page - no next URL
					resp := ProjectListResponse{
						Count: 1,
						Value: []Project{
							{ID: 3, Name: "Project 3"},
						},
					}
					json.NewEncoder(w).Encode(resp)
				} else {
					// First page
					next := fmt.Sprintf("http://%s/cloud/v6/projects?page=2&$top=1000", r.Host)
					resp := ProjectListResponse{
						Count: 2,
						Next:  &next,
						Value: []Project{
							{ID: 1, Name: "Project 1"},
							{ID: 2, Name: "Project 2"},
						},
					}
					json.NewEncoder(w).Encode(resp)
				}
			},
			wantProjects: 3,
			wantErr:      false,
		},
		{
			name: "api_error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "Invalid API token"}`))
			},
			wantProjects: 0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			// Create client
			logger := zaptest.NewLogger(t)
			client := NewClient(server.URL, "test-stack-id", "test-token", logger)

			// Make request
			projects, err := client.ListProjects(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, projects, tt.wantProjects)
			}
		})
	}
}

func TestListTests(t *testing.T) {
	tests := []struct {
		name           string
		projectID      *int
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantTests      int
		wantErr        bool
	}{
		{
			name:      "list_all_tests",
			projectID: nil,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/cloud/v6/load_tests", r.URL.Path)

				resp := TestListResponse{
					Count: 2,
					Value: []Test{
						{ID: 1, Name: "Test 1", ProjectID: 1},
						{ID: 2, Name: "Test 2", ProjectID: 1},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantTests: 2,
			wantErr:   false,
		},
		{
			name:      "list_project_tests",
			projectID: intPtr(123),
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/cloud/v6/projects/123/load_tests", r.URL.Path)

				resp := TestListResponse{
					Count: 1,
					Value: []Test{
						{ID: 1, Name: "Test 1", ProjectID: 123},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantTests: 1,
			wantErr:   false,
		},
		{
			name:      "empty_response",
			projectID: nil,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := TestListResponse{
					Count: 0,
					Value: []Test{},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantTests: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			logger := zaptest.NewLogger(t)
			client := NewClient(server.URL, "test-stack-id", "test-token", logger)

			tests, err := client.ListTests(context.Background(), tt.projectID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, tests, tt.wantTests)
			}
		})
	}
}

func TestListTestRuns(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	tests := []struct {
		name           string
		testID         int
		since          *time.Time
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantRuns       int
		wantErr        bool
	}{
		{
			name:   "list_all_runs",
			testID: 1,
			since:  nil,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/cloud/v6/load_tests/1/test_runs", r.URL.Path)
				assert.Equal(t, "created desc", r.URL.Query().Get("$orderby"))

				resp := TestRunListResponse{
					Count: 3,
					Value: []TestRun{
						{ID: 1, TestID: 1, Status: "completed", Created: now},
						{ID: 2, TestID: 1, Status: "running", Created: now.Add(-1 * time.Hour)},
						{ID: 3, TestID: 1, Status: "created", Created: now.Add(-2 * time.Hour)},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantRuns: 3,
			wantErr:  false,
		},
		{
			name:   "filter_by_time",
			testID: 1,
			since:  &yesterday,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := TestRunListResponse{
					Count: 5,
					Value: []TestRun{
						{ID: 1, TestID: 1, Status: "completed", Created: now},
						{ID: 2, TestID: 1, Status: "running", Created: now.Add(-1 * time.Hour)},
						{ID: 3, TestID: 1, Status: "created", Created: now.Add(-12 * time.Hour)},
						{ID: 4, TestID: 1, Status: "completed", Created: now.Add(-20 * time.Hour)},
						{ID: 5, TestID: 1, Status: "completed", Created: now.Add(-30 * time.Hour)}, // Before cutoff
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantRuns: 4, // Only runs after yesterday
			wantErr:  false,
		},
		{
			name:   "status_history_populated",
			testID: 1,
			since:  nil,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := TestRunListResponse{
					Count: 1,
					Value: []TestRun{
						{
							ID:     1,
							TestID: 1,
							Status: "running",
							StatusHistory: []StatusHistoryEntry{
								{Type: "created", Entered: now.Add(-10 * time.Minute)},
								{Type: "initializing", Entered: now.Add(-5 * time.Minute)},
								{Type: "running", Entered: now},
							},
						},
					},
				}
				json.NewEncoder(w).Encode(resp)
			},
			wantRuns: 1,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			logger := zaptest.NewLogger(t)
			client := NewClient(server.URL, "test-stack-id", "test-token", logger)

			runs, err := client.ListTestRuns(context.Background(), tt.testID, tt.since)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, runs, tt.wantRuns)

				// Additional validation for status history test
				if tt.name == "status_history_populated" && len(runs) > 0 {
					assert.Len(t, runs[0].StatusHistory, 3)
				}
			}
		})
	}
}

func TestGetTestRun(t *testing.T) {
	tests := []struct {
		name           string
		testID         int
		runID          int
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		validate       func(t *testing.T, run *TestRun)
	}{
		{
			name:   "successful_get",
			testID: 1,
			runID:  100,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/cloud/v6/load_tests/1/test_runs/100", r.URL.Path)

				run := TestRun{
					ID:        100,
					TestID:    1,
					ProjectID: 10,
					Status:    "completed",
					StartedBy: "user@example.com",
					Created:   time.Now(),
				}
				json.NewEncoder(w).Encode(run)
			},
			wantErr: false,
			validate: func(t *testing.T, run *TestRun) {
				assert.Equal(t, 100, run.ID)
				assert.Equal(t, 1, run.TestID)
				assert.Equal(t, "completed", run.Status)
			},
		},
		{
			name:   "not_found",
			testID: 1,
			runID:  999,
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error": "Test run not found"}`))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			logger := zaptest.NewLogger(t)
			client := NewClient(server.URL, "test-stack-id", "test-token", logger)

			run, err := client.GetTestRun(context.Background(), tt.testID, tt.runID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, run)
			} else {
				require.NoError(t, err)
				require.NotNil(t, run)
				if tt.validate != nil {
					tt.validate(t, run)
				}
			}
		})
	}
}

func TestGetAllTestRuns(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		projectIDs     []string
		since          *time.Time
		serverResponse func(callCount int) http.HandlerFunc
		wantRuns       int
		wantErr        bool
	}{
		{
			name:       "multiple_projects",
			projectIDs: []string{"1", "2"},
			since:      nil,
			serverResponse: func(callCount int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/cloud/v6/projects/1/load_tests":
						resp := TestListResponse{
							Count: 1,
							Value: []Test{
								{ID: 1, Name: "Test 1", ProjectID: 1},
							},
						}
						json.NewEncoder(w).Encode(resp)
					case "/cloud/v6/projects/2/load_tests":
						resp := TestListResponse{
							Count: 1,
							Value: []Test{
								{ID: 2, Name: "Test 2", ProjectID: 2},
							},
						}
						json.NewEncoder(w).Encode(resp)
					case "/cloud/v6/load_tests/1/test_runs":
						resp := TestRunListResponse{
							Count: 2,
							Value: []TestRun{
								{ID: 1, TestID: 1, Status: "running", Created: now},
								{ID: 2, TestID: 1, Status: "completed", Created: now},
							},
						}
						json.NewEncoder(w).Encode(resp)
					case "/cloud/v6/load_tests/2/test_runs":
						resp := TestRunListResponse{
							Count: 1,
							Value: []TestRun{
								{ID: 3, TestID: 2, Status: "created", Created: now},
							},
						}
						json.NewEncoder(w).Encode(resp)
					}
				}
			},
			wantRuns: 3,
			wantErr:  false,
		},
		{
			name:       "invalid_project_id",
			projectIDs: []string{"invalid"},
			since:      nil,
			serverResponse: func(callCount int) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					// Should not make any API calls for invalid project ID
					t.Error("unexpected API call")
				}
			},
			wantRuns: 0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				tt.serverResponse(callCount)(w, r)
			}))
			defer server.Close()

			logger := zaptest.NewLogger(t)
			client := NewClient(server.URL, "test-stack-id", "test-token", logger)

			runs, err := client.GetAllTestRuns(context.Background(), tt.projectIDs, tt.since)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, runs, tt.wantRuns)
			}
		})
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}
