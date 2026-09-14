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
	metricsReg := prometheus.NewRegistry()
	appMetrics = metrics.New(metricsReg)

	// Initialize token manager (in production, load keys from secure storage)
	var err error
	tokenManager, err = auth.New(
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/N8r1GfKTN8J0iEqBJRK5Z2BtK8r5D5aE\nVO0JnZVN8V8cQN8Y5oPvWvJGmZhPF0V0J8V9K6J8V7Z5K5m0V5K8V9Z3K0Q4J5\nZ8V8Z5K3V4K7V8Z9L1R3K4Z6V7Z4K2V3K6V7Z8L0S2J3Z5V6Z3J1U2J2Z4U5Z2\nJ0T1I1Z3T4Z1I9S0H0Z2S3Z0H8R9G9Z1R2Z9G7Q8F8Z0Q1Z8F6P7E7Y9P0Z7E5\nO6D6Y8O9Z6D4N5C5X7N8Z5C3M4B4W6M7Z4B2L3A3V5L6Z3A1K2A2U4K5Z2A0J1\n9Z1T0J0Z1I9Y0Y9Z0H8X9X8Z9G7W8W7Z8F6V7V6Z7E5U6U5Z6D4T5T4Z5C3S4S3\nZ4B2R3R2Z3A1Q2Q1Z2Y0P1P0ZQIDAQABA\n-----END RSA PRIVATE KEY-----",
		"-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0Z3VS5JJcds3xfn/N8r1\nGfKTN8J0iEqBJRK5Z2BtK8r5D5aEVO0JnZVN8V8cQN8Y5oPvWvJGmZhPF0V0J8V9\nK6J8V7Z5K5m0V5K8V9Z3K0Q4J5Z8V8Z5K3V4K7V8Z9L1R3K4Z6V7Z4K2V3K6V7Z8\nL0S2J3Z5V6Z3J1U2J2Z4U5Z2J0T1I1Z3T4Z1I9S0H0Z2S3Z0H8R9G9Z1R2Z9G7Q8\nF8Z0Q1Z8F6P7E7Y9P0Z7E5O6D6Y8O9Z6D4N5C5X7N8Z5C3M4B4W6M7Z4B2L3A3V5L6\nZ3A1K2A2U4K5Z2A0J19Z1T0J0Z1I9Y0Y9Z0H8X9X8Z9G7W8W7Z8F6V7V6Z7E5U6U5Z6\nD4T5T4Z5C3S4S3Z4B2R3R2Z3A1Q2Q1Z2Y0P1P0ZQIDAQAB\n-----END PUBLIC KEY-----",
		log,
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize token manager")
	}
}

// Credentials holds test credentials for demo tenants
var credentials = map[string]map[string]string{
	"alpha": {
		"api_key": "alpha_key_123",
		"secret":  "alpha_secret_456",
	},
	"beta": {
		"api_key": "beta_key_789",
		"secret":  "beta_secret_012",
	},
	"gamma": {
		"api_key": "gamma_key_345",
		"secret":  "gamma_secret_678",
	},
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
