/*
 * Copyright (c) 2026 peek8.io
 *
 * Created Date: Monday, September 14th 2026, 10:25:30 am
 * Author: Md. Asraful Haque
 *
 */
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/asraf344/axiler-fin/pkg/auth"
	"github.com/asraf344/axiler-fin/pkg/metrics"
	"github.com/asraf344/axiler-fin/pkg/models"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	log          *logrus.Logger
	tokenManager *auth.TokenManager
	metricsReg   *prometheus.Registry
	appMetrics   *metrics.Metrics
)

func init() {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	})
	log.SetLevel(logrus.DebugLevel)

	// Initialize metrics registry
	metricsReg = prometheus.NewRegistry()
	appMetrics = metrics.New(metricsReg)

	// Initialize token manager (in production, load keys from secure storage)
	var err error
	tokenManager, err = auth.New(
		auth.GetRSAPrivateKeyPEM(),
		auth.GetRSAPublicKeyPEM(),
		log,
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize token manager")
	}
}

func main() {
	r := chi.NewRouter()

	// Metrics endpoint
	r.Handle("/metrics", promhttp.HandlerFor(metricsReg, promhttp.HandlerOpts{}))

	// Health check
	r.Get("/health", handleHealth)

	// Authentication endpoint
	r.Post("/auth/login", handleLogin)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.WithField("port", port).Info("Auth service starting")
	http.ListenAndServe(":"+port, r)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	appMetrics.ServiceHealth.WithLabelValues("auth").Set(1)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Service:   "auth",
		Version:   "1.0.0",
		Dependencies: map[string]interface{}{
			"token_manager": "ok",
		},
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req models.AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMetrics.EdgeAuthFailures.WithLabelValues("invalid_request").Inc()
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// Lookup tenant by credentials
	var tenantID string
	for tID, creds := range credentials {
		if creds["api_key"] == req.APIKey && creds["secret"] == req.Secret {
			tenantID = tID
			break
		}
	}

	if tenantID == "" {
		appMetrics.EdgeAuthFailures.WithLabelValues("invalid_credentials").Inc()
		log.WithField("api_key", req.APIKey).Warn("Authentication failed: invalid credentials")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}

	// Create JWT token
	userID := fmt.Sprintf("%s_user", tenantID)
	token, err := tokenManager.NewToken(tenantID, userID, req.APIKey)
	if err != nil {
		appMetrics.EdgeAuthFailures.WithLabelValues("token_generation_failed").Inc()
		log.WithError(err).Error("Failed to generate token")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "token generation failed"})
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)

	log.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("Authentication successful")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.AuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		TenantID:  tenantID,
	})
}
