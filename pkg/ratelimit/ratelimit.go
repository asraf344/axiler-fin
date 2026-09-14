package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// TenantLimiter tracks rate limits per tenant
type TenantLimiter struct {
	limits map[string]int       // requests per minute per tenant
	tokens map[string]float64   // token bucket
	last   map[string]time.Time // last token refresh time
	mu     sync.RWMutex
}

// New creates a new tenant-aware rate limiter
func New() *TenantLimiter {
	return &TenantLimiter{
		limits: map[string]int{
			"alpha": 1000,
			"beta":  500,
			"gamma": 250,
		},
		tokens: make(map[string]float64),
		last:   make(map[string]time.Time),
	}
}

// Allow checks if a request from tenant is allowed
func (tl *TenantLimiter) Allow(tenant string) bool {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	limit, ok := tl.limits[tenant]
	if !ok {
		// Unknown tenant, deny
		return false
	}

	// Token bucket algorithm: refill at (limit/60) tokens per second
	now := time.Now()
	lastRefresh, ok := tl.last[tenant]
	if !ok {
		lastRefresh = now
		tl.last[tenant] = now
		tl.tokens[tenant] = float64(limit)
	}

	// Elapsed time since last refill
	elapsed := now.Sub(lastRefresh).Seconds()
	tl.last[tenant] = now

	// Refill tokens: limit/60 tokens per second
	refillRate := float64(limit) / 60.0
	tokensToAdd := elapsed * refillRate
	tl.tokens[tenant] = min(float64(limit), tl.tokens[tenant]+tokensToAdd)

	// Check if we have at least 1 token
	if tl.tokens[tenant] >= 1.0 {
		tl.tokens[tenant]--
		return true
	}

	return false
}

// GetStatus returns current token bucket status for a tenant
func (tl *TenantLimiter) GetStatus(tenant string) map[string]interface{} {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	limit := tl.limits[tenant]
	tokens := tl.tokens[tenant]

	return map[string]interface{}{
		"tenant":           tenant,
		"limit_per_minute": limit,
		"current_tokens":   fmt.Sprintf("%.2f", tokens),
		"requests_allowed": tokens >= 1.0,
	}
}

// SetLimit updates the rate limit for a tenant
func (tl *TenantLimiter) SetLimit(tenant string, limit int) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	tl.limits[tenant] = limit
	if _, ok := tl.tokens[tenant]; !ok {
		tl.tokens[tenant] = float64(limit)
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
