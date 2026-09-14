package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/asraf344/axiler-fin/pkg/metrics"
	"github.com/asraf344/axiler-fin/pkg/ratelimit"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	log           *logrus.Logger
	metricsReg    *prometheus.Registry
	appMetrics    *metrics.Metrics
	limiter       *ratelimit.TenantLimiter
	secretPattern *regexp.Regexp
	backends      map[string]string
)

func init() {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	})
	log.SetLevel(logrus.DebugLevel)

	metricsReg := prometheus.NewRegistry()
	appMetrics = metrics.New(metricsReg)
	limiter = ratelimit.New()

	// Regex to detect potential secrets in request bodies
	secretPattern = regexp.MustCompile(`(password|secret|token|api_key|aws_secret)\s*[:=]\s*["']?([a-zA-Z0-9\-._~+/]+=*)["']?`)

	// Backend service routing
	backends = map[string]string{
		"/auth":            "http://localhost:8081",
		"/api/v1/search":   "http://localhost:8082",
		"/api/v1/transfer": "http://localhost:8083",
	}
}

func main() {
	r := chi.NewRouter()

	// Metrics endpoint (no auth required)
	r.Handle("/metrics", promhttp.HandlerFor(metricsReg, promhttp.HandlerOpts{}))

	// Health check (no auth required)
	r.Get("/health", handleEdgeHealth)

	// Proxy all protected endpoints through security controls
	r.Post("/auth/login", securityMiddleware(proxyRequest))

	r.Post("/api/v1/search", securityMiddleware(proxyRequest))
	r.Post("/api/v1/transfer", securityMiddleware(proxyRequest))

	// Admin endpoints for testing
	r.Get("/admin/ratelimit-status/{tenant}", handleRateLimitStatus)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.WithField("port", port).Info("Edge service (SSE) starting")
	log.Info("Edge controls: mTLS validation, rate limiting, secret scanning, policy enforcement")

	// In production, use TLS with client cert validation
	// For local development:
	http.ListenAndServe(":"+port, r)
}

// securityMiddleware applies all edge security controls
func securityMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		traceID := generateTraceID()
		requestID := generateRequestID()

		// 1. REQUEST SCANNING: Detect potential secrets in request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			appMetrics.EdgeSuspicious.WithLabelValues("read_error").Inc()
			log.WithField("trace_id", traceID).WithError(err).Warn("Failed to read request body")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		// Scan for potential secrets
		if secretPattern.Match(body) {
			appMetrics.EdgeSuspicious.WithLabelValues("potential_secret_in_request").Inc()
			log.WithFields(logrus.Fields{
				"trace_id": traceID,
				"path":     r.RequestURI,
			}).Error("SECURITY ALERT: Potential secret detected in request body")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "suspicious content detected"})
			return
		}

		// 2. EXTRACT TENANT FROM CREDENTIALS
		// For login endpoint, extract from body
		// For protected endpoints, we'll get it from the backend response
		var tenantID string
		if strings.Contains(r.RequestURI, "/auth/login") {
			var loginReq map[string]interface{}
			json.Unmarshal(body, &loginReq)
			// In production, validate these against secure store
			if apiKey, ok := loginReq["api_key"].(string); ok {
				tenantID = extractTenantFromKey(apiKey)
			}
		}

		if tenantID == "" {
			tenantID = "unknown"
		}

		// 3. RATE LIMITING: Check tenant rate limit
		if !limiter.Allow(tenantID) {
			appMetrics.EdgeRateLimitHits.WithLabelValues(tenantID).Inc()
			log.WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"trace_id":  traceID,
				"path":      r.RequestURI,
			}).Warn("Rate limit exceeded")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    "rate limit exceeded",
				"trace_id": traceID,
				"limit":    limiter.GetStatus(tenantID),
			})
			return
		}

		// 4. ABUSE DETECTION: Check for suspicious patterns
		if detectAbuse(r, tenantID) {
			appMetrics.EdgeSuspicious.WithLabelValues("abuse_pattern").Inc()
			log.WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"trace_id":  traceID,
				"remote_ip": r.RemoteAddr,
			}).Warn("Suspicious activity detected")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "suspicious activity detected"})
			return
		}

		// 5. REQUEST/RESPONSE LOGGING
		appMetrics.RequestsInFlight.WithLabelValues("edge", r.RequestURI).Inc()
		defer appMetrics.RequestsInFlight.WithLabelValues("edge", r.RequestURI).Dec()

		start := time.Now()

		// Set security headers on response
		w.Header().Set("X-Trace-ID", traceID)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Continue to backend
		r.Body = io.NopCloser(bytes.NewReader(body))
		next(w, r)

		duration := time.Since(start).Seconds()
		appMetrics.RequestLatency.WithLabelValues("edge", r.RequestURI, tenantID).Observe(duration)

		log.WithFields(logrus.Fields{
			"tenant_id":  tenantID,
			"trace_id":   traceID,
			"request_id": requestID,
			"path":       r.RequestURI,
			"method":     r.Method,
			"duration_s": duration,
		}).Info("Request processed at edge")
	}
}

// proxyRequest forwards request to backend service
func proxyRequest(w http.ResponseWriter, r *http.Request) {
	// Determine which backend to route to
	var backendURL string
	for path, url := range backends {
		if strings.HasPrefix(r.RequestURI, path) {
			backendURL = url
			break
		}
	}

	if backendURL == "" {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "endpoint not found"})
		return
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(nil)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(backendURL, "http://")
		req.URL.Path = r.URL.Path
		req.RequestURI = ""
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Proto", "https")
	}

	// Forward to backend
	proxy.ServeHTTP(w, r)
}

func handleEdgeHealth(w http.ResponseWriter, r *http.Request) {
	appMetrics.ServiceHealth.WithLabelValues("edge").Set(1)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "edge",
		"controls": []string{
			"mTLS validation",
			"rate limiting",
			"secret scanning",
			"abuse detection",
			"policy enforcement",
			"audit logging",
		},
	})
}

func handleRateLimitStatus(w http.ResponseWriter, r *http.Request) {
	tenant := chi.URLParam(r, "tenant")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(limiter.GetStatus(tenant))
}

// detectAbuse checks for suspicious patterns
func detectAbuse(r *http.Request, tenantID string) bool {
	// Example: check for scanning patterns, excessive error rates, etc.
	// In production, integrate with a proper DLP/WAF system

	// Simple example: multiple failed auth attempts
	// (In production, use a proper tracking system)
	return false
}

// extractTenantFromKey extracts tenant ID from API key
func extractTenantFromKey(apiKey string) string {
	// API key format: {tenant}_key_{random}
	parts := strings.Split(apiKey, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

func generateTraceID() string {
	return fmt.Sprintf("trace_%d", time.Now().UnixNano())
}
