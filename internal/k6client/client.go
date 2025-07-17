package k6client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// Client is the k6 API client
type Client struct {
	baseURL    string
	apiToken   string
	stackID    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new k6 API client
func NewClient(baseURL, stackID, apiToken string, logger *zap.Logger) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		stackID:  stackID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, path string, params url.Values) (*http.Response, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if params != nil {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Stack-Id", c.stackID)

	c.logger.Debug("making API request",
		zap.String("method", method),
		zap.String("url", u.String()),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error: %s (status %d): %s", resp.Status, resp.StatusCode, string(body))
	}

	return resp, nil
}

// ListProjects lists all projects
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var allProjects []Project
	nextURL := "/cloud/v6/projects"
	firstPage := true

	for nextURL != "" {
		var resp *http.Response
		var err error

		// Only add parameters on the first page
		// For subsequent pages, use the full URL from the 'next' field
		if firstPage {
			params := url.Values{}
			params.Set("$top", "1000")
			resp, err = c.doRequest(ctx, http.MethodGet, nextURL, params)
			firstPage = false
		} else {
			// Pass nil params to preserve query parameters in the nextURL
			resp, err = c.doRequest(ctx, http.MethodGet, nextURL, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		defer resp.Body.Close()

		var result ProjectListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		allProjects = append(allProjects, result.Value...)

		// Check if there's a next page
		if result.Next != nil && *result.Next != "" {
			// Extract path from next URL
			u, err := url.Parse(*result.Next)
			if err != nil {
				return nil, fmt.Errorf("parse next URL: %w", err)
			}
			nextURL = u.Path + "?" + u.RawQuery
		} else {
			nextURL = ""
		}
	}

	c.logger.Info("listed projects", zap.Int("count", len(allProjects)))
	return allProjects, nil
}

// ListTests lists all tests, optionally filtered by project
func (c *Client) ListTests(ctx context.Context, projectID *int) ([]Test, error) {
	var allTests []Test
	var nextURL string
	firstPage := true

	if projectID != nil {
		nextURL = fmt.Sprintf("/cloud/v6/projects/%d/load_tests", *projectID)
	} else {
		nextURL = "/cloud/v6/load_tests"
	}

	for nextURL != "" {
		var resp *http.Response
		var err error

		if firstPage {
			params := url.Values{}
			// params.Set("$top", "1000")
			resp, err = c.doRequest(ctx, http.MethodGet, nextURL, params)
			firstPage = false
		} else {
			// Pass nil params to preserve query parameters in the nextURL
			resp, err = c.doRequest(ctx, http.MethodGet, nextURL, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("list tests: %w", err)
		}
		defer resp.Body.Close()

		var result TestListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		allTests = append(allTests, result.Value...)

		// Check if there's a next page
		if result.Next != nil && *result.Next != "" {
			// Extract path from next URL
			u, err := url.Parse(*result.Next)
			if err != nil {
				return nil, fmt.Errorf("parse next URL: %w", err)
			}
			nextURL = u.Path + "?" + u.RawQuery
		} else {
			nextURL = ""
		}
	}

	c.logger.Info("listed tests",
		zap.Int("count", len(allTests)),
		zap.Bool("filtered", projectID != nil),
	)
	return allTests, nil
}

// ListTestRuns lists test runs for a specific test
func (c *Client) ListTestRuns(ctx context.Context, testID int, since *time.Time) ([]TestRun, error) {
	var allRuns []TestRun
	nextURL := fmt.Sprintf("/cloud/v6/load_tests/%d/test_runs", testID)
	firstPage := true

	for nextURL != "" {
		var resp *http.Response
		var err error

		if firstPage {
			params := url.Values{}
			resp, err = c.doRequest(ctx, http.MethodGet, nextURL, params)
			firstPage = false
		} else {
			// Pass nil params to preserve query parameters in the nextURL
			resp, err = c.doRequest(ctx, http.MethodGet, nextURL, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("list test runs for test %d: %w", testID, err)
		}
		defer resp.Body.Close()

		var result TestRunListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		// Filter by since time if provided
		for _, run := range result.Value {
			if since == nil || run.Created.After(*since) {
				allRuns = append(allRuns, run)
			} else {
				// Since results are ordered by created desc, we can stop here
				c.logger.Debug("stopping pagination, reached since time",
					zap.Time("since", *since),
					zap.Time("run_created", run.Created),
				)
				return allRuns, nil
			}
		}

		// Check if there's a next page
		if result.Next != nil && *result.Next != "" {
			// Extract path from next URL
			u, err := url.Parse(*result.Next)
			if err != nil {
				return nil, fmt.Errorf("parse next URL: %w", err)
			}
			nextURL = u.Path + "?" + u.RawQuery
		} else {
			nextURL = ""
		}
	}

	c.logger.Debug("listed test runs",
		zap.Int("test_id", testID),
		zap.Int("count", len(allRuns)),
		zap.Bool("filtered_by_time", since != nil),
	)
	return allRuns, nil
}

// GetTestRun gets a specific test run
func (c *Client) GetTestRun(ctx context.Context, testID, runID int) (*TestRun, error) {
	path := fmt.Sprintf("/cloud/v6/load_tests/%d/test_runs/%d", testID, runID)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("get test run %d for test %d: %w", runID, testID, err)
	}
	defer resp.Body.Close()

	var testRun TestRun
	if err := json.NewDecoder(resp.Body).Decode(&testRun); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &testRun, nil
}

// GetAllTestRuns fetches all test runs for all tests in the specified projects
func (c *Client) GetAllTestRuns(ctx context.Context, projectIDs []string, since *time.Time) ([]TestRun, error) {
	// First, get all tests
	var tests []Test
	var err error

	if len(projectIDs) > 0 {
		// Fetch tests for each specified project
		for _, projectID := range projectIDs {
			pid := 0
			if _, err := fmt.Sscanf(projectID, "%d", &pid); err != nil {
				c.logger.Warn("invalid project ID, skipping", zap.String("project_id", projectID))
				continue
			}
			projectTests, err := c.ListTests(ctx, &pid)
			if err != nil {
				c.logger.Error("failed to list tests for project",
					zap.Int("project_id", pid),
					zap.Error(err),
				)
				continue
			}
			tests = append(tests, projectTests...)
		}
	} else {
		// Fetch all tests
		tests, err = c.ListTests(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("list all tests: %w", err)
		}
	}

	// Now fetch test runs for each test
	var allRuns []TestRun
	for _, test := range tests {
		runs, err := c.ListTestRuns(ctx, test.ID, since)
		if err != nil {
			c.logger.Error("failed to list test runs",
				zap.Int("test_id", test.ID),
				zap.String("test_name", test.Name),
				zap.Error(err),
			)
			continue
		}

		// Add test name to each run for better metrics labeling
		for i := range runs {
			// Store test name in a custom field (we'll handle this in the collector)
			if runs[i].StatusDetails == nil {
				runs[i].StatusDetails = make(map[string]interface{})
			}
			runs[i].StatusDetails["test_name"] = test.Name
		}

		allRuns = append(allRuns, runs...)
	}

	c.logger.Info("fetched all test runs",
		zap.Int("test_count", len(tests)),
		zap.Int("run_count", len(allRuns)),
		zap.Bool("filtered_by_time", since != nil),
	)

	return allRuns, nil
}
