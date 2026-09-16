/*
 * Copyright (c) 2026 peek8.io
 *
 * Created Date: Wednesday, September 16th 2026, 9:37:20 am
 * Author: Md. Asraful Haque
 *
 */

package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AbuseDetector detects abusive HTTP request patterns
type AbuseDetector struct {
	log *logrus.Logger

	// Request tracking
	requestHistory map[string]*RequestHistory // key: tenant_id|ip
	mu             sync.RWMutex

	// Thresholds
	MaxRequestsPerSecond      int
	MaxFailedAuthPerMinute    int
	MaxRequestSize            int64
	SuspiciousHeaderThreshold int
	VelocityThreshold         int // requests per minute from single IP

	// Patterns for detection
	sqlInjectionPattern *regexp.Regexp
	xssPattern          *regexp.Regexp
	commandInjection    *regexp.Regexp
}

// RequestHistory tracks requests from a source
type RequestHistory struct {
	Requests        []time.Time
	FailedAttempts  []time.Time
	SuspiciousCount int
	LastRequest     time.Time
	mu              sync.Mutex
}

// AbuseSignal contains information about detected abuse
type AbuseSignal struct {
	Detected        bool
	Reason          string
	Severity        int // 1-10, 10 = most severe
	Recommendations []string
}

// New creates a new abuse detector
func NewAbuseDetector(log *logrus.Logger) *AbuseDetector {
	return &AbuseDetector{
		log:                       log,
		requestHistory:            make(map[string]*RequestHistory),
		MaxRequestsPerSecond:      100,
		MaxFailedAuthPerMinute:    5,
		MaxRequestSize:            10 * 1024 * 1024, // 10MB
		SuspiciousHeaderThreshold: 3,
		VelocityThreshold:         1000, // 1000 req/min = 16.6 req/sec

		// Compile regex patterns
		sqlInjectionPattern: regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop|create|alter|exec|execute|script|javascript|onerror|onload|onclick|base64_decode|base64_encode)`),
		xssPattern:          regexp.MustCompile(`(?i)(<script|javascript:|onerror=|onload=|onclick=|<iframe|<embed|<object)`),
		commandInjection:    regexp.MustCompile(`(?i)(;|&&|\|\|||\$\(|%0a|%0d)`),
	}
}

// Detect analyzes request for abuse patterns
func (ad *AbuseDetector) Detect(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{
		Detected: false,
		Severity: 0,
	}

	// Chain detection checks
	checks := []func(*http.Request, string) AbuseSignal{
		ad.checkRequestSize,
		ad.checkSuspiciousHeaders,
		ad.checkPayloadInjection,
		ad.checkRateLimit,
		ad.checkVelocity,
		ad.checkUserAgent,
		ad.checkHTTPMethods,
		ad.checkPathTraversal,
		ad.checkBotPatterns,
	}

	for _, check := range checks {
		result := check(r, tenantID)
		if result.Detected {
			signal.Detected = true
			signal.Severity = max(signal.Severity, result.Severity)
			signal.Reason += result.Reason + "; "
			signal.Recommendations = append(signal.Recommendations, result.Recommendations...)
		}
	}

	// Track the request
	ad.trackRequest(r, tenantID, signal.Detected)

	return signal
}

// ==================== Detection Strategies ====================

// checkRequestSize detects unusually large requests
func (ad *AbuseDetector) checkRequestSize(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	if r.ContentLength > ad.MaxRequestSize {
		signal.Detected = true
		signal.Severity = 8
		signal.Reason = fmt.Sprintf("Request size %d exceeds limit %d", r.ContentLength, ad.MaxRequestSize)
		signal.Recommendations = []string{"Reject request", "Log IP", "Alert security team"}
		return signal
	}

	return signal
}

// checkSuspiciousHeaders detects suspicious or missing headers
func (ad *AbuseDetector) checkSuspiciousHeaders(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	suspiciousCount := 0
	reasons := []string{}

	// Check for missing common headers
	if r.Header.Get("User-Agent") == "" {
		suspiciousCount++
		reasons = append(reasons, "missing User-Agent")
	}

	// Check for suspicious User-Agent patterns
	ua := r.Header.Get("User-Agent")
	if isSuspiciousUserAgent(ua) {
		suspiciousCount++
		reasons = append(reasons, "suspicious User-Agent: "+ua)
	}

	// Check for injection in headers
	for name, values := range r.Header {
		for _, value := range values {
			if ad.sqlInjectionPattern.MatchString(value) ||
				ad.xssPattern.MatchString(value) ||
				ad.commandInjection.MatchString(value) {
				suspiciousCount++
				reasons = append(reasons, fmt.Sprintf("injection in header %s", name))
				break
			}
		}
	}

	// Check for suspicious header combinations
	if r.Header.Get("X-Forwarded-For") != "" && r.Header.Get("X-Real-IP") != "" {
		// Could be proxy spoofing attempt
		if !isTrustedProxy(r) {
			suspiciousCount++
			reasons = append(reasons, "multiple proxy headers from untrusted source")
		}
	}

	if suspiciousCount >= ad.SuspiciousHeaderThreshold {
		signal.Detected = true
		signal.Severity = 6
		signal.Reason = "Suspicious headers: " + strings.Join(reasons, ", ")
		signal.Recommendations = []string{"Log request", "Verify headers", "Consider rate limiting"}
	}

	return signal
}

// checkPayloadInjection detects injection attempts in request body
func (ad *AbuseDetector) checkPayloadInjection(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	if r.Body == nil {
		return signal
	}

	// Read body (but preserve it for handler)
	bodyLimit := 10000
	if r.ContentLength >= 0 && r.ContentLength < int64(bodyLimit) {
		bodyLimit = int(r.ContentLength)
	}
	bodyBytes := make([]byte, bodyLimit) // Limit read to first 10KB
	n, _ := io.ReadFull(r.Body, bodyBytes)
	body := string(bodyBytes[:n])

	// Restore body for handler
	r.Body = io.NopCloser(strings.NewReader(body))

	detections := []struct {
		pattern  *regexp.Regexp
		name     string
		severity int
	}{
		{ad.sqlInjectionPattern, "SQL injection", 9},
		{ad.xssPattern, "XSS", 8},
		{ad.commandInjection, "Command injection", 9},
	}

	for _, det := range detections {
		if det.pattern.MatchString(body) {
			signal.Detected = true
			signal.Severity = max(signal.Severity, det.severity)
			signal.Reason = det.name + " pattern detected in body"
			signal.Recommendations = []string{
				"Block request immediately",
				"Log with full request details",
				"Alert security team",
				"Block IP for 1 hour",
			}
			return signal
		}
	}

	return signal
}

// checkRateLimit detects rapid requests (DDoS-like behavior)
func (ad *AbuseDetector) checkRateLimit(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	key := tenantID + "|" + getClientIP(r)
	history := ad.getOrCreateHistory(key)

	history.mu.Lock()
	defer history.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Second) // Last 1 second

	// Count requests in last second
	validRequests := []time.Time{}
	for _, t := range history.Requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}

	requestsPerSec := len(validRequests)
	if requestsPerSec > ad.MaxRequestsPerSecond {
		signal.Detected = true
		signal.Severity = 7
		signal.Reason = fmt.Sprintf("Rate limit exceeded: %d req/sec > %d", requestsPerSec, ad.MaxRequestsPerSecond)
		signal.Recommendations = []string{
			"Rate limit tenant",
			"Return 429 Too Many Requests",
			"Implement exponential backoff on client",
		}
	}

	return signal
}

// checkVelocity detects suspicious velocity from single IP
func (ad *AbuseDetector) checkVelocity(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	clientIP := getClientIP(r)
	key := "velocity|" + clientIP
	history := ad.getOrCreateHistory(key)

	history.mu.Lock()
	defer history.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute) // Last minute

	// Count requests in last minute from this IP
	validRequests := []time.Time{}
	for _, t := range history.Requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}

	requestsPerMin := len(validRequests)
	if requestsPerMin > ad.VelocityThreshold {
		signal.Detected = true
		signal.Severity = 6
		signal.Reason = fmt.Sprintf("Velocity too high: %d req/min from IP %s", requestsPerMin, clientIP)
		signal.Recommendations = []string{
			"Rate limit IP address",
			"Temporary IP block (15 minutes)",
			"Alert security team",
		}
	}

	return signal
}

// checkUserAgent detects automated/bot access
func (ad *AbuseDetector) checkUserAgent(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	ua := r.Header.Get("User-Agent")

	// Common bot patterns
	botPatterns := []string{
		"sqlmap", "nikto", "nessus", "nmap", "masscan",
		"metasploit", "burp", "zaproxy", "w3af",
		"scrapy", "curl", "wget", "python", "java", "perl",
		"bot", "crawler", "spider", "scanner",
	}

	ua_lower := strings.ToLower(ua)
	for _, pattern := range botPatterns {
		if strings.Contains(ua_lower, pattern) {
			signal.Detected = true
			signal.Severity = 5
			signal.Reason = fmt.Sprintf("Bot/scanner detected: %s", ua)
			signal.Recommendations = []string{
				"Log request details",
				"Block if known bad bot",
				"Require CAPTCHA",
			}
			return signal
		}
	}

	return signal
}

// checkHTTPMethods detects unusual HTTP method combinations
func (ad *AbuseDetector) checkHTTPMethods(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	key := tenantID + "|" + getClientIP(r)
	history := ad.getOrCreateHistory(key)

	history.mu.Lock()
	defer history.mu.Unlock()

	// Detect rapid method changes (sign of port scanning/probing)
	suspiciousMethods := 0
	if r.Method == "HEAD" || r.Method == "OPTIONS" || r.Method == "TRACE" {
		suspiciousMethods++
	}

	if suspiciousMethods > 0 && len(history.Requests) > 5 {
		// Check if they're mixing many different methods
		signal.Detected = true
		signal.Severity = 4
		signal.Reason = fmt.Sprintf("Unusual HTTP method: %s", r.Method)
		signal.Recommendations = []string{"Log request", "Monitor for more patterns"}
	}

	return signal
}

// checkPathTraversal detects directory traversal attempts
func (ad *AbuseDetector) checkPathTraversal(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	path := r.URL.Path
	query := r.URL.RawQuery

	traversalPatterns := []string{
		"../", "..\\", "%2e%2e/", "%2e%2e%5c",
		"....//", "....\\\\",
		"/etc/", "/var/", "/proc/", "/sys/", "/root/",
	}

	for _, pattern := range traversalPatterns {
		if strings.Contains(path, pattern) || strings.Contains(query, pattern) {
			signal.Detected = true
			signal.Severity = 9
			signal.Reason = fmt.Sprintf("Path traversal attempt detected: %s", pattern)
			signal.Recommendations = []string{
				"Block request immediately",
				"Log with full details",
				"Block IP for 24 hours",
				"Alert security team",
			}
			return signal
		}
	}

	return signal
}

// checkBotPatterns detects automated/bot-like patterns
func (ad *AbuseDetector) checkBotPatterns(r *http.Request, tenantID string) AbuseSignal {
	signal := AbuseSignal{Detected: false}

	key := tenantID + "|" + getClientIP(r)
	history := ad.getOrCreateHistory(key)

	history.mu.Lock()
	defer history.mu.Unlock()

	// Check for identical requests (fingerprinting/probing)
	if len(history.Requests) > 10 {
		now := time.Now()
		cutoff := now.Add(-1 * time.Minute)

		recentRequests := 0
		for _, t := range history.Requests {
			if t.After(cutoff) {
				recentRequests++
			}
		}

		// Too many requests from same source in short time
		if recentRequests > 50 {
			signal.Detected = true
			signal.Severity = 7
			signal.Reason = fmt.Sprintf("Bot-like pattern: %d requests in 1 minute", recentRequests)
			signal.Recommendations = []string{
				"Rate limit tenant",
				"Require CAPTCHA",
				"Temporary block",
			}
		}
	}

	return signal
}

// ==================== Helper Functions ====================

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (might be spoofed, but common)
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}

	// Fall back to remote address
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// isSuspiciousUserAgent checks if User-Agent is suspicious
func isSuspiciousUserAgent(ua string) bool {
	if ua == "" {
		return true // Empty is suspicious
	}

	if len(ua) > 500 {
		return true // Excessively long is suspicious
	}

	// Check for null bytes or control characters
	for _, c := range ua {
		if c < 32 && c != '\t' {
			return true
		}
	}

	return false
}

// isTrustedProxy checks if request is from trusted proxy
func isTrustedProxy(r *http.Request) bool {
	trustedProxies := []string{
		"127.0.0.1",
		"::1",
		// Add your trusted proxy IPs
	}

	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	for _, trusted := range trustedProxies {
		if remoteIP == trusted {
			return true
		}
	}

	return false
}

// trackRequest records request in history
func (ad *AbuseDetector) trackRequest(r *http.Request, tenantID string, abusive bool) {
	key := tenantID + "|" + getClientIP(r)
	history := ad.getOrCreateHistory(key)

	history.mu.Lock()
	defer history.mu.Unlock()

	now := time.Now()
	history.Requests = append(history.Requests, now)
	history.LastRequest = now

	if abusive {
		history.SuspiciousCount++
	}

	// Cleanup old entries (keep last 1 minute)
	cutoff := now.Add(-1 * time.Minute)
	validRequests := []time.Time{}
	for _, t := range history.Requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	history.Requests = validRequests
}

// getOrCreateHistory gets or creates history for a key
func (ad *AbuseDetector) getOrCreateHistory(key string) *RequestHistory {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if history, exists := ad.requestHistory[key]; exists {
		return history
	}

	history := &RequestHistory{
		Requests: []time.Time{},
	}
	ad.requestHistory[key] = history
	return history
}

// CleanupOldData removes old tracking data
func (ad *AbuseDetector) CleanupOldData() {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-5 * time.Minute) // Keep 5 minutes of history

	for key, history := range ad.requestHistory {
		history.mu.Lock()

		// Remove old requests
		validRequests := []time.Time{}
		for _, t := range history.Requests {
			if t.After(cutoff) {
				validRequests = append(validRequests, t)
			}
		}
		history.Requests = validRequests

		// Remove old failed attempts
		validFailures := []time.Time{}
		for _, t := range history.FailedAttempts {
			if t.After(cutoff) {
				validFailures = append(validFailures, t)
			}
		}
		history.FailedAttempts = validFailures

		history.mu.Unlock()

		// Delete if empty
		if len(history.Requests) == 0 && len(history.FailedAttempts) == 0 {
			delete(ad.requestHistory, key)
		}
	}
}

// TrackFailedAuth records a failed authentication attempt
func (ad *AbuseDetector) TrackFailedAuth(tenantID string, clientIP string) int {
	key := tenantID + "|" + clientIP
	history := ad.getOrCreateHistory(key)

	history.mu.Lock()
	defer history.mu.Unlock()

	now := time.Now()
	history.FailedAttempts = append(history.FailedAttempts, now)

	// Count failures in last minute
	cutoff := now.Add(-1 * time.Minute)
	validFailures := []time.Time{}
	for _, t := range history.FailedAttempts {
		if t.After(cutoff) {
			validFailures = append(validFailures, t)
		}
	}
	history.FailedAttempts = validFailures

	return len(validFailures)
}

// GetFingerprint generates a request fingerprint for tracking
func GetFingerprint(r *http.Request) string {
	data := r.UserAgent() + r.Header.Get("Accept") + r.Header.Get("Accept-Language")
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
