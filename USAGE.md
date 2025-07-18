# K6 Prometheus Exporter Usage Guide

## Quick Start

### 1. Get your Grafana Cloud k6 API Token

1. Log in to your Grafana Cloud k6 account
2. Navigate to Account Settings → API Tokens
3. Create a new API token with read permissions

### 2. Run with Docker

```bash
# Set your API token
export K6_API_TOKEN=your-api-token-here

# Run the exporter
docker run -p 9090:9090 -e K6_API_TOKEN=$K6_API_TOKEN k6-prometheus-exporter:latest
```

### 3. Run with Docker Compose (includes Prometheus and Grafana)

```bash
# Clone the repository
git clone https://github.com/your-org/grafana-cloud-k6-prometheus-exporter.git
cd grafana-cloud-k6-prometheus-exporter/examples

# Set your API token
export K6_API_TOKEN=your-api-token-here

# Start the stack
docker-compose up -d

# Access services:
# - Exporter metrics: http://localhost:9090/metrics
# - Prometheus: http://localhost:9091
# - Grafana: http://localhost:3000 (admin/admin)
```

## Configuration Options

| Environment Variable | Description | Default | Required |
|---------------------|-------------|---------|----------|
| `K6_API_TOKEN` | Grafana Cloud k6 API token | - | Yes |
| `K6_API_URL` | k6 API base URL | `https://api.k6.io` | No |
| `PORT` | Exporter listen port | `9090` | No |
| `TEST_CACHE_TTL` | How long to cache test list | `60s` | No |
| `STATE_CLEANUP_INTERVAL` | How often to clean old state | `5m` | No |
| `PROJECTS` | Comma-separated project IDs to monitor | All projects | No |
| `MAX_CONCURRENT_REQUESTS` | Max concurrent API requests | `10` | No |
| `API_TIMEOUT` | API request timeout | `30s` | No |

## Available Metrics

### Test Run Metrics

- **`k6_test_run_total`** - Counter of test runs by status
  - Labels: `test_name`, `test_id`, `project_id`, `status`
  - Use case: Track how many times tests have entered each status

- **`k6_test_run_status`** - Current test runs in each status (gauge)
  - Labels: `test_name`, `test_id`, `project_id`, `status`
  - Use case: Monitor currently running/queued tests

- **`k6_test_run_result_total`** - Counter of completed test runs by result
  - Labels: `test_name`, `test_id`, `project_id`, `result`
  - Use case: Track success/failure rates

- **`k6_test_run_duration_seconds`** - Test run duration
  - Labels: `test_name`, `test_id`, `project_id`, `status`
  - Use case: Monitor test execution times

- **`k6_test_run_vuh_consumed`** - Virtual User Hours consumed
  - Labels: `test_name`, `test_id`, `project_id`
  - Use case: Track resource consumption

### Operational Metrics

- **`k6_exporter_api_requests_total`** - API request counter
- **`k6_exporter_api_request_duration_seconds`** - API request latency
- **`k6_exporter_test_runs_tracked`** - Number of test runs in state
- **`k6_exporter_scrape_errors_total`** - Scrape error counter

## Example Prometheus Queries

### Basic Queries

```promql
# Currently running tests
sum(k6_test_run_status{status="running"})

# Test failure rate (last hour)
sum(rate(k6_test_run_result_total{result="failed"}[1h])) 
/ 
sum(rate(k6_test_run_result_total[1h]))

# Average test duration by test name
avg by (test_name) (k6_test_run_duration_seconds{status="completed"})

# VUH consumption by project (last 24h)
sum by (project_id) (increase(k6_test_run_vuh_consumed[24h]))
```

### Alert Queries

```promql
# Alert on any test failure
increase(k6_test_run_result_total{result="failed"}[5m]) > 0

# Alert on tests running too long
k6_test_run_duration_seconds{status="running"} > 3600

# Alert on high failure rate
(
  sum by (project_id) (increase(k6_test_run_result_total{result="failed"}[1h]))
  /
  sum by (project_id) (increase(k6_test_run_result_total[1h]))
) > 0.1
```

## Kubernetes Deployment

### 1. Create namespace and secret

```bash
kubectl create namespace monitoring
kubectl create secret generic k6-exporter-secret \
  --from-literal=api-token=$K6_API_TOKEN \
  -n monitoring
```

### 2. Deploy the exporter

```bash
kubectl apply -f deployments/kubernetes/
```

### 3. Verify deployment

```bash
kubectl get pods -n monitoring -l app=k6-exporter
kubectl logs -n monitoring -l app=k6-exporter
```

### 4. Configure Prometheus scraping

If using Prometheus Operator:
```bash
kubectl apply -f deployments/kubernetes/servicemonitor.yaml
```

For standard Prometheus, add to your scrape config:
```yaml
- job_name: 'k6-exporter'
  kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names:
          - monitoring
  relabel_configs:
    - source_labels: [__meta_kubernetes_service_name]
      action: keep
      regex: k6-exporter
```

## Monitoring Specific Projects

To monitor only specific projects, set the `PROJECTS` environment variable:

```bash
# Docker
docker run -e K6_API_TOKEN=$K6_API_TOKEN -e PROJECTS="12345,67890" k6-prometheus-exporter

# Kubernetes
env:
  - name: PROJECTS
    value: "12345,67890"
```

## Troubleshooting

### Check exporter health

```bash
curl http://localhost:9090/health
```

### View exporter logs

```bash
# Docker
docker logs <container-id>

# Kubernetes
kubectl logs -n monitoring -l app=k6-exporter
```

### Common issues

1. **No metrics appearing**
   - Check API token is valid
   - Verify network connectivity to api.k6.io
   - Check logs for API errors

2. **High memory usage**
   - Reduce `STATE_CLEANUP_INTERVAL` to clean state more frequently
   - Monitor with `k6_exporter_test_runs_tracked` metric

3. **API rate limiting**
   - Increase `TEST_CACHE_TTL` to reduce API calls
   - Check `k6_exporter_api_requests_total{status_code="429"}`

## Production Best Practices

1. **High Availability**: Run multiple replicas behind a load balancer
2. **Resource Limits**: Set appropriate memory/CPU limits in Kubernetes
3. **Monitoring**: Set up alerts for exporter health and API errors
4. **Security**: Store API tokens in secrets management system
5. **Persistence**: Consider Redis backend for state in multi-instance setups