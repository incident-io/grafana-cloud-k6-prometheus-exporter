package k6client

import (
	"context"
	"time"
)

// Client defines the interface for k6 API operations
type ClientInterface interface {
	ListProjects(ctx context.Context) ([]Project, error)
	ListTests(ctx context.Context, projectID *int) ([]Test, error)
	ListTestRuns(ctx context.Context, testID int, since *time.Time) ([]TestRun, error)
	GetTestRun(ctx context.Context, testID, runID int) (*TestRun, error)
	GetAllTestRuns(ctx context.Context, projectIDs []string, since *time.Time) ([]TestRun, error)
}