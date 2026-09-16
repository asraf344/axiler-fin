/*
 * Copyright (c) 2026 peek8.io
 *
 * Created Date: Wednesday, September 16th 2026, 9:37:20 am
 * Author: Md. Asraful Haque
 *
 */

package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestNewAndHelperFunctions(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	if d == nil {
		t.Fatal("expected detector")
	}
	if d.MaxRequestsPerSecond == 0 || d.MaxFailedAuthPerMinute == 0 || d.MaxRequestSize == 0 {
		t.Fatal("expected default thresholds to be initialized")
	}
	if d.sqlInjectionPattern == nil || d.xssPattern == nil || d.commandInjection == nil {
		t.Fatal("expected regex patterns to be initialized")
	}

	h1 := http.Header{}
	h1.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	if got := getClientIP(&http.Request{Header: h1, RemoteAddr: "127.0.0.1:1234"}); got != "203.0.113.10" {
		t.Fatalf("expected forwarded IP, got %q", got)
	}
	h2 := http.Header{}
	h2.Set("X-Real-IP", "203.0.113.11")
	if got := getClientIP(&http.Request{Header: h2, RemoteAddr: "127.0.0.1:1234"}); got != "203.0.113.11" {
		t.Fatalf("expected real IP, got %q", got)
	}
	if got := getClientIP(&http.Request{RemoteAddr: "127.0.0.1:1234"}); got != "127.0.0.1" {
		t.Fatalf("expected remote addr IP, got %q", got)
	}

	if !isSuspiciousUserAgent("") {
		t.Fatal("expected empty user-agent to be suspicious")
	}
	if !isSuspiciousUserAgent(strings.Repeat("a", 600)) {
		t.Fatal("expected excessively long user-agent to be suspicious")
	}
	if !isSuspiciousUserAgent("bad\x00ua") {
		t.Fatal("expected control char in user-agent to be suspicious")
	}
	if isSuspiciousUserAgent("Mozilla/5.0") {
		t.Fatal("expected normal user-agent to be safe")
	}

	if !isTrustedProxy(&http.Request{RemoteAddr: "127.0.0.1:1234"}) {
		t.Fatal("expected loopback IP to be a trusted proxy")
	}
	if isTrustedProxy(&http.Request{RemoteAddr: "192.168.1.10:1234"}) {
		t.Fatal("expected non-trusted IP to not be treated as a proxy")
	}

	h3 := http.Header{}
	h3.Set("User-Agent", "UA")
	h3.Set("Accept", "json")
	h3.Set("Accept-Language", "en-US")
	fp := GetFingerprint(&http.Request{Header: h3})
	if fp == "" {
		t.Fatal("expected non-empty fingerprint")
	}
}

func TestCheckRequestSize(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	d.MaxRequestSize = 10
	r := newTestRequest("POST", "/test", "UA", http.Header{}, "body", "127.0.0.1:1234")
	r.ContentLength = d.MaxRequestSize + 1

	signal := d.checkRequestSize(r, "tenant-a")
	if !signal.Detected {
		t.Fatal("expected oversized request to be detected")
	}
	if signal.Severity != 8 {
		t.Fatalf("expected severity 8, got %d", signal.Severity)
	}
	if !strings.Contains(signal.Reason, "exceeds limit") {
		t.Fatalf("unexpected reason: %s", signal.Reason)
	}
}

func TestCheckSuspiciousHeaders(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	d.SuspiciousHeaderThreshold = 1

	t.Run("missing user-agent", func(t *testing.T) {
		r := newTestRequest("GET", "/", "", http.Header{}, "", "127.0.0.1:1234")
		signal := d.checkSuspiciousHeaders(r, "tenant-a")
		if !signal.Detected || !strings.Contains(signal.Reason, "missing User-Agent") {
			t.Fatalf("expected missing User-Agent detection, got %+v", signal)
		}
	})

	t.Run("header injection", func(t *testing.T) {
		r := newTestRequest("GET", "/", "Mozilla/5.0", http.Header{"X-Api-Key": {"<script>alert('x')</script>"}}, "", "127.0.0.1:1234")
		signal := d.checkSuspiciousHeaders(r, "tenant-a")
		if !signal.Detected || !strings.Contains(signal.Reason, "injection in header X-Api-Key") {
			t.Fatalf("expected injection detection, got %+v", signal)
		}
	})

	t.Run("proxy spoofing", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-Forwarded-For", "203.0.113.10")
		headers.Set("X-Real-IP", "203.0.113.11")
		headers.Set("User-Agent", "Mozilla/5.0")
		r := newTestRequest("GET", "/", "Mozilla/5.0", headers, "", "192.168.1.10:1234")
		signal := d.checkSuspiciousHeaders(r, "tenant-a")
		if !signal.Detected || (!strings.Contains(signal.Reason, "multiple proxy headers") && !strings.Contains(signal.Reason, "injection in header")) {
			t.Fatalf("expected suspicious headers detection, got %+v", signal)
		}
	})
}

func TestCheckPayloadInjection(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	body := "SELECT * FROM users WHERE name='admin'; DROP TABLE users;"
	r := newTestRequest("POST", "/login", "Mozilla/5.0", http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}, body, "127.0.0.1:1234")

	signal := d.checkPayloadInjection(r, "tenant-a")
	if !signal.Detected {
		t.Fatal("expected SQL injection payload to be detected")
	}
	if signal.Severity != 9 {
		t.Fatalf("expected SQL injection severity 9, got %d", signal.Severity)
	}

	restored, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read restored body: %v", err)
	}
	if string(restored) != body {
		t.Fatalf("expected request body to be preserved, got %q", string(restored))
	}
}

func TestCheckRateLimitAndVelocity(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	d.MaxRequestsPerSecond = 1
	d.VelocityThreshold = 1

	key := "tenant-a|127.0.0.1"
	hist := d.getOrCreateHistory(key)
	hist.Requests = []time.Time{time.Now(), time.Now().Add(-200 * time.Millisecond)}
	if signal := d.checkRateLimit(newTestRequest("GET", "/", "UA", http.Header{}, "", "127.0.0.1:1234"), "tenant-a"); !signal.Detected {
		t.Fatal("expected rate limit violation to be detected")
	}

	velocityKey := "velocity|127.0.0.1"
	velHist := d.getOrCreateHistory(velocityKey)
	velHist.Requests = []time.Time{time.Now(), time.Now().Add(-30 * time.Second)}
	if signal := d.checkVelocity(newTestRequest("GET", "/", "UA", http.Header{}, "", "127.0.0.1:1234"), "tenant-a"); !signal.Detected {
		t.Fatal("expected velocity violation to be detected")
	}
}

func TestCheckUserAgentAndHTTPMethods(t *testing.T) {
	d := NewAbuseDetector(logrus.New())

	r1 := newTestRequest("GET", "/", "Mozilla/5.0 (compatible; sqlmap/1.0; +https://sqlmap.org)", http.Header{}, "", "127.0.0.1:1234")
	if signal := d.checkUserAgent(r1, "tenant-a"); !signal.Detected {
		t.Fatal("expected sqlmap user-agent to be considered bot traffic")
	}

	key := "tenant-a|127.0.0.1"
	hist := d.getOrCreateHistory(key)
	hist.Requests = []time.Time{
		time.Now().Add(-30 * time.Second),
		time.Now().Add(-25 * time.Second),
		time.Now().Add(-20 * time.Second),
		time.Now().Add(-15 * time.Second),
		time.Now().Add(-10 * time.Second),
		time.Now().Add(-5 * time.Second),
	}
	if signal := d.checkHTTPMethods(newTestRequest("HEAD", "/", "UA", http.Header{}, "", "127.0.0.1:1234"), "tenant-a"); !signal.Detected {
		t.Fatal("expected unusual HTTP method to be detected")
	}
}

func TestCheckPathTraversalAndBotPatterns(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	r := newTestRequest("GET", "/admin/../../etc/passwd?next=../../var/log/system.log", "UA", http.Header{}, "", "127.0.0.1:1234")
	if signal := d.checkPathTraversal(r, "tenant-a"); !signal.Detected {
		t.Fatal("expected path traversal detection")
	}

	key := "tenant-a|127.0.0.1"
	hist := d.getOrCreateHistory(key)
	now := time.Now()
	for i := 0; i < 60; i++ {
		hist.Requests = append(hist.Requests, now.Add(-time.Duration(i)*time.Second))
	}
	if signal := d.checkBotPatterns(newTestRequest("GET", "/", "UA", http.Header{}, "", "127.0.0.1:1234"), "tenant-a"); !signal.Detected {
		t.Fatal("expected bot-like pattern detection")
	}
}

func TestDetectTracksRequestAndAggregatesSignals(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	d.MaxRequestSize = 1
	d.SuspiciousHeaderThreshold = 1

	r := newTestRequest("POST", "/login?next=../../etc/passwd", "", http.Header{"X-Api-Key": {"<script>alert(1)</script>"}}, "x", "127.0.0.1:1234")
	r.ContentLength = 10
	signal := d.Detect(r, "tenant-a")
	if !signal.Detected {
		t.Fatal("expected aggregate detection from multiple patterns")
	}
	if signal.Severity == 0 {
		t.Fatal("expected non-zero severity")
	}
	if len(signal.Recommendations) == 0 {
		t.Fatal("expected recommendations to be generated")
	}

	history := d.getOrCreateHistory("tenant-a|127.0.0.1")
	if len(history.Requests) == 0 {
		t.Fatal("expected request history to be recorded")
	}
	if history.SuspiciousCount == 0 {
		t.Fatal("expected suspicious request count to increase")
	}
}

func TestTrackFailedAuthAndCleanupOldData(t *testing.T) {
	d := NewAbuseDetector(logrus.New())
	if count := d.TrackFailedAuth("tenant-a", "127.0.0.1"); count != 1 {
		t.Fatalf("expected first failed auth to count as 1, got %d", count)
	}
	if count := d.TrackFailedAuth("tenant-a", "127.0.0.1"); count != 2 {
		t.Fatalf("expected second failed auth to count as 2, got %d", count)
	}

	key := "tenant-a|127.0.0.1"
	history := d.getOrCreateHistory(key)
	history.Requests = []time.Time{time.Now().Add(-10 * time.Minute)}
	history.FailedAttempts = []time.Time{time.Now().Add(-10 * time.Minute)}
	d.CleanupOldData()
	history = d.getOrCreateHistory(key)
	if len(history.Requests) != 0 || len(history.FailedAttempts) != 0 {
		t.Fatal("expected stale history to be removed")
	}
}

func newTestRequest(method, path, userAgent string, headers http.Header, body, remoteAddr string) *http.Request {
	if headers == nil {
		headers = http.Header{}
	}
	if userAgent != "" {
		headers.Set("User-Agent", userAgent)
	}
	r, err := http.NewRequest(method, "http://example.com"+path, strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	r.Header = headers
	r.RemoteAddr = remoteAddr
	if body == "" {
		r.ContentLength = 0
	} else {
		r.ContentLength = int64(len(body))
	}
	return r
}
