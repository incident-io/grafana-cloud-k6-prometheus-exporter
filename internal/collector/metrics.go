package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metric descriptors
var (
	// Test run metrics
	testRunTotalDesc = prometheus.NewDesc(
		"k6_test_run_total",
		"Total number of test runs by status",
		[]string{"test_name", "test_id", "project_id", "status"},
		nil,
	)

	testRunStatusDesc = prometheus.NewDesc(
		"k6_test_run_status",
		"Current test runs in each status (gauge)",
		[]string{"test_name", "test_id", "project_id", "status"},
		nil,
	)

	testRunResultTotalDesc = prometheus.NewDesc(
		"k6_test_run_result_total",
		"Total number of completed test runs by result",
		[]string{"test_name", "test_id", "project_id", "result"},
		nil,
	)

	testRunDurationSecondsDesc = prometheus.NewDesc(
		"k6_test_run_duration_seconds",
		"Duration of test runs in seconds",
		[]string{"test_name", "test_id", "project_id", "status"},
		nil,
	)

	testRunVUHConsumedDesc = prometheus.NewDesc(
		"k6_test_run_vuh_consumed",
		"Virtual User Hours consumed by test runs",
		[]string{"test_name", "test_id", "project_id", "run_id"},
		nil,
	)

	testRunInfoDesc = prometheus.NewDesc(
		"k6_test_run_info",
		"Information about test runs",
		[]string{"test_name", "test_id", "project_id", "run_id"},
		nil,
	)

	// Operational metrics
	exporterAPIRequestsTotalDesc = prometheus.NewDesc(
		"k6_exporter_api_requests_total",
		"Total number of API requests made by the exporter",
		[]string{"endpoint", "method", "status_code"},
		nil,
	)

	exporterAPIRequestDurationSecondsDesc = prometheus.NewDesc(
		"k6_exporter_api_request_duration_seconds",
		"Duration of API requests in seconds",
		[]string{"endpoint"},
		nil,
	)

	exporterLastScrapeTimestampDesc = prometheus.NewDesc(
		"k6_exporter_last_scrape_timestamp",
		"Unix timestamp of the last successful scrape",
		[]string{"endpoint"},
		nil,
	)

	exporterTestRunsTrackedDesc = prometheus.NewDesc(
		"k6_exporter_test_runs_tracked",
		"Number of test runs currently being tracked in state",
		nil,
		nil,
	)

	exporterScrapeDurationSecondsDesc = prometheus.NewDesc(
		"k6_exporter_scrape_duration_seconds",
		"Duration of the scrape operation in seconds",
		nil,
		nil,
	)

	exporterScrapeErrorsTotalDesc = prometheus.NewDesc(
		"k6_exporter_scrape_errors_total",
		"Total number of scrape errors",
		[]string{"error_type"},
		nil,
	)
)

// MetricValue represents a single metric value to be collected
type MetricValue struct {
	Desc   *prometheus.Desc
	Type   prometheus.ValueType
	Value  float64
	Labels []string
}

// OperationalMetrics holds operational metrics for the exporter
type OperationalMetrics struct {
	APIRequestsTotal    *prometheus.CounterVec
	APIRequestDuration  *prometheus.HistogramVec
	LastScrapeTimestamp *prometheus.GaugeVec
	TestRunsTracked     prometheus.Gauge
	ScrapeDuration      prometheus.Histogram
	ScrapeErrorsTotal   *prometheus.CounterVec
}

// NewOperationalMetrics creates operational metrics that are registered globally
func NewOperationalMetrics() *OperationalMetrics {
	return NewOperationalMetricsWithRegistry(prometheus.DefaultRegisterer)
}

// NewOperationalMetricsWithRegistry creates operational metrics with a specific registry
func NewOperationalMetricsWithRegistry(reg prometheus.Registerer) *OperationalMetrics {
	metrics := &OperationalMetrics{
		APIRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k6_exporter_api_requests_total",
				Help: "Total number of API requests made by the exporter",
			},
			[]string{"endpoint", "method", "status_code"},
		),
		APIRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "k6_exporter_api_request_duration_seconds",
				Help:    "Duration of API requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"endpoint"},
		),
		LastScrapeTimestamp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "k6_exporter_last_scrape_timestamp",
				Help: "Unix timestamp of the last successful scrape",
			},
			[]string{"endpoint"},
		),
		TestRunsTracked: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "k6_exporter_test_runs_tracked",
				Help: "Number of test runs currently being tracked in state",
			},
		),
		ScrapeDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "k6_exporter_scrape_duration_seconds",
				Help:    "Duration of the scrape operation in seconds",
				Buckets: prometheus.DefBuckets,
			},
		),
		ScrapeErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "k6_exporter_scrape_errors_total",
				Help: "Total number of scrape errors",
			},
			[]string{"error_type"},
		),
	}

	// Register all metrics if registerer is provided
	if reg != nil {
		reg.MustRegister(
			metrics.APIRequestsTotal,
			metrics.APIRequestDuration,
			metrics.LastScrapeTimestamp,
			metrics.TestRunsTracked,
			metrics.ScrapeDuration,
			metrics.ScrapeErrorsTotal,
		)
	}

	return metrics
}
