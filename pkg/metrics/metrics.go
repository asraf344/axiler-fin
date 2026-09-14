package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the platform
type Metrics struct {
	// HTTP request metrics
	RequestsTotal    prometheus.CounterVec
	RequestLatency   prometheus.HistogramVec
	RequestsInFlight prometheus.GaugeVec

	// Edge-specific metrics
	EdgeAuthFailures  prometheus.CounterVec
	EdgeRateLimitHits prometheus.CounterVec
	EdgeSuspicious    prometheus.CounterVec

	// Business logic metrics
	SearchRequestsTotal prometheus.CounterVec
	TransfersTotal      prometheus.CounterVec
	TransferAmount      prometheus.HistogramVec

	// Service health
	ServiceHealth       prometheus.GaugeVec
	HealthCheckFailures prometheus.CounterVec

	// Dependency metrics
	VaultAuthDuration prometheus.HistogramVec
	DatabaseLatency   prometheus.HistogramVec
}

// New creates and registers all metrics
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		RequestsTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"service", "endpoint", "method", "status", "tenant"},
		),
		RequestLatency: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
			},
			[]string{"service", "endpoint", "tenant"},
		),
		RequestsInFlight: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "HTTP requests currently being processed",
			},
			[]string{"service", "endpoint"},
		),
		EdgeAuthFailures: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "edge_auth_failures_total",
				Help: "Authentication failures at edge",
			},
			[]string{"reason"},
		),
		EdgeRateLimitHits: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "edge_ratelimit_hits_total",
				Help: "Rate limit rejections",
			},
			[]string{"tenant"},
		),
		EdgeSuspicious: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "edge_suspicious_activity_total",
				Help: "Suspicious activity detected at edge",
			},
			[]string{"reason"},
		),
		SearchRequestsTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "search_requests_total",
				Help: "Total search requests",
			},
			[]string{"tenant", "status"},
		),
		TransfersTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "transfers_total",
				Help: "Total transfer operations",
			},
			[]string{"tenant", "outcome"},
		),
		TransferAmount: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "transfer_amount_usd",
				Help:    "Transfer amounts in USD",
				Buckets: []float64{10, 100, 500, 1000, 5000, 10000, 50000},
			},
			[]string{"tenant"},
		),
		ServiceHealth: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "service_health_status",
				Help: "Service health status (1=healthy, 0=unhealthy)",
			},
			[]string{"service"},
		),
		HealthCheckFailures: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "health_check_failures_total",
				Help: "Health check failures",
			},
			[]string{"service", "reason"},
		),
		VaultAuthDuration: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "vault_auth_duration_seconds",
				Help:    "Duration of Vault authentication",
				Buckets: []float64{.01, .05, .1, .5, 1},
			},
			[]string{"service"},
		),
		DatabaseLatency: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "database_query_duration_seconds",
				Help:    "Database query latency",
				Buckets: []float64{.001, .005, .01, .05, .1, .5},
			},
			[]string{"service", "query"},
		),
	}

	// Register all metrics
	reg.MustRegister(
		&m.RequestsTotal,
		&m.RequestLatency,
		&m.RequestsInFlight,
		&m.EdgeAuthFailures,
		&m.EdgeRateLimitHits,
		&m.EdgeSuspicious,
		&m.SearchRequestsTotal,
		&m.TransfersTotal,
		&m.TransferAmount,
		&m.ServiceHealth,
		&m.HealthCheckFailures,
		&m.VaultAuthDuration,
		&m.DatabaseLatency,
	)

	return m
}
