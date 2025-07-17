# Grafana Cloud k6 Prometheus Exporter

A Prometheus exporter for Grafana Cloud k6 test runs, enabling monitoring and alerting on k6 test execution metrics.

## Features

- Exports k6 test run metrics to Prometheus format
- Tracks test run status transitions and results
- Prevents duplicate counting with state management
- Provides operational metrics for monitoring the exporter itself
- Supports filtering by project

## Metrics

### Test Run Metrics

- `k6_test_run_total` - Counter of test runs by status
- `k6_test_run_status` - Gauge showing current test runs in each status
- `k6_test_run_result_total` - Counter for completed test runs by result
- `k6_test_run_duration_seconds` - Gauge for test duration
- `k6_test_run_vuh_consumed` - Gauge for Virtual User Hours consumed
- `k6_test_run_info` - Info metric with test metadata

### Operational Metrics

- `k6_exporter_api_requests_total` - Counter for API requests
- `k6_exporter_api_request_duration_seconds` - Histogram for API latency
- `k6_exporter_last_scrape_timestamp` - Gauge with last successful scrape time
- `k6_exporter_test_runs_tracked` - Gauge showing number of test runs in state

## Configuration

Configure via environment variables:

```bash
K6_API_TOKEN=your-api-token          # Required: Grafana Cloud k6 API token
GRAFANA_STACK_ID=your-stack-id       # Required: Grafana Cloud Stack ID
K6_API_URL=https://api.k6.io         # Optional: API base URL (default: https://api.k6.io)
PORT=9090                             # Optional: Exporter port (default: 9090)
TEST_CACHE_TTL=60s                    # Optional: Test list cache TTL (default: 60s)
STATE_CLEANUP_INTERVAL=300s           # Optional: State cleanup interval (default: 300s)
PROJECTS=project1,project2            # Optional: Comma-separated list of projects to monitor
```

## Installation

### Binary

```bash
go install github.com/incident-iografana-cloud-k6-prometheus-exporter@latest
```

### Docker

```bash
docker build -t k6-exporter .
docker run -p 9090:9090 -e K6_API_TOKEN=your-token -e GRAFANA_STACK_ID=your-stack-id k6-exporter
```

### Kubernetes

```bash
kubectl apply -f deployments/kubernetes/
```

## Usage

1. Set your Grafana Cloud k6 API token and stack ID:
   ```bash
   export K6_API_TOKEN=your-api-token
   export GRAFANA_STACK_ID=your-stack-id
   ```

2. Run the exporter:
   ```bash
   ./k6-exporter
   ```

3. Configure Prometheus to scrape the exporter:
   ```yaml
   scrape_configs:
     - job_name: 'k6-exporter'
       static_configs:
         - targets: ['localhost:9090']
   ```

## Example Queries

```promql
# Currently running tests
sum(k6_test_run_status{status="running"})

# Test failure rate by project (last hour)
sum by (project_id) (
  increase(k6_test_run_result_total{result="failed"}[1h])
) / sum by (project_id) (
  increase(k6_test_run_result_total[1h])
)

# Average test duration
avg(k6_test_run_duration_seconds{status="completed"})
```

## Development

```bash
# Run tests
make test

# Build binary
make build

# Run linting
make lint
```
