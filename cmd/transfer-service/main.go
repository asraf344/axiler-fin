package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/asraf344/axiler-fin/pkg/auth"
	appctx "github.com/asraf344/axiler-fin/pkg/context"
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
	db           *models.SyntheticDatabase
	dbLock       sync.RWMutex
	failureMode  bool // Can be set to simulate failure for testing
)

func init() {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	})
	log.SetLevel(logrus.DebugLevel)

	metricsReg := prometheus.NewRegistry()
	appMetrics = metrics.New(metricsReg)

	var err error
	tokenManager, err = auth.New(
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/N8r1GfKTN8J0iEqBJRK5Z2BtK8r5D5aE\nVO0JnZVN8V8cQN8Y5oPvWvJGmZhPF0V0J8V9K6J8V7Z5K5m0V5K8V9Z3K0Q4J5\nZ8V8Z5K3V4K7V8Z9L1R3K4Z6V7Z4K2V3K6V7Z8L0S2J3Z5V6Z3J1U2J2Z4U5Z2\nJ0T1I1Z3T4Z1I9S0H0Z2S3Z0H8R9G9Z1R2Z9G7Q8F8Z0Q1Z8F6P7E7Y9P0Z7E5\nO6D6Y8O9Z6D4N5C5X7N8Z5C3M4B4W6M7Z4B2L3A3V5L6Z3A1K2A2U4K5Z2A0J1\n9Z1T0J0Z1I9Y0Y9Z0H8X9X8Z9G7W8W7Z8F6V7V6Z7E5U6U5Z6D4T5T4Z5C3S4S3\nZ4B2R3R2Z3A1Q2Q1Z2Y0P1P0ZQIDAQABA\n-----END RSA PRIVATE KEY-----",
		"-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0Z3VS5JJcds3xfn/N8r1\nGfKTN8J0iEqBJRK5Z2BtK8r5D5aEVO0JnZVN8V8cQN8Y5oPvWvJGmZhPF0V0J8V9\nK6J8V7Z5K5m0V5K8V9Z3K0Q4J5Z8V8Z5K3V4K7V8Z9L1R3K4Z6V7Z4K2V3K6V7Z8\nL0S2J3Z5V6Z3J1U2J2Z4U5Z2J0T1I1Z3T4Z1I9S0H0Z2S3Z0H8R9G9Z1R2Z9G7Q8\nF8Z0Q1Z8F6P7E7Y9P0Z7E5O6D6Y8O9Z6D4N5C5X7N8Z5C3M4B4W6M7Z4B2L3A3V5L6\nZ3A1K2A2U4K5Z2A0J19Z1T0J0Z1I9Y0Y9Z0H8X9X8Z9G7W8W7Z8F6V7V6Z7E5U6U5Z6\nD4T5T4Z5C3S4S3Z4B2R3R2Z3A1Q2Q1Z2Y0P1P0ZQIDAQAB\n-----END PUBLIC KEY-----",
		log,
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize token manager")
	}

	db = models.NewSyntheticDatabase()
}

func main() {
	r := chi.NewRouter()

	r.Handle("/metrics", promhttp.HandlerFor(metricsReg, promhttp.HandlerOpts{}))
	r.Get("/health", handleHealth)
	r.Post("/api/v1/transfer", authMiddleware(handleTransfer))
	r.Post("/admin/simulate-failure", handleSimulateFailure) // For testing rollback

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	log.WithField("port", port).Info("Transfer service starting")
	http.ListenAndServe(":"+port, r)
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			appMetrics.EdgeAuthFailures.WithLabelValues("missing_token").Inc()
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing authorization"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			appMetrics.EdgeAuthFailures.WithLabelValues("invalid_token_format").Inc()
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid token format"})
			return
		}

		claims, err := tokenManager.ValidateToken(parts[1])
		if err != nil {
			appMetrics.EdgeAuthFailures.WithLabelValues("invalid_token").Inc()
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
			return
		}

		reqCtx := &appctx.RequestContext{
			TenantID:   claims.TenantID,
			UserID:     claims.UserID,
			RequestID:  generateRequestID(),
			TraceID:    generateTraceID(),
			Timestamp:  time.Now().Unix(),
			ReleaseVer: "1.0.0",
		}

		ctx := appctx.NewContextWithRequest(r.Context(), reqCtx)
		r = r.WithContext(ctx)

		w.Header().Set("X-Trace-ID", reqCtx.TraceID)
		w.Header().Set("X-Request-ID", reqCtx.RequestID)

		next(w, r)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	// Simulate health check that can fail
	if failureMode {
		appMetrics.ServiceHealth.WithLabelValues("transfer").Set(0)
		appMetrics.HealthCheckFailures.WithLabelValues("transfer", "simulated_failure").Inc()
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(models.HealthResponse{
			Status:    "unhealthy",
			Timestamp: time.Now(),
			Service:   "transfer",
			Version:   "1.0.0",
		})
		return
	}

	appMetrics.ServiceHealth.WithLabelValues("transfer").Set(1)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Service:   "transfer",
		Version:   "1.0.0",
		Dependencies: map[string]interface{}{
			"database": "ok",
		},
	})
}

func handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req models.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	reqCtx := appctx.FromContext(r.Context())
	log := appctx.ContextLogger(r.Context(), logrus.New())

	dbLock.RLock()
	fromAcc, fromExists := db.Accounts[req.FromAccount]
	toAcc, toExists := db.Accounts[req.ToAccount]
	dbLock.RUnlock()

	// TENANT ISOLATION: Both accounts must belong to the same tenant
	if !fromExists || !toExists {
		appMetrics.TransfersTotal.WithLabelValues(reqCtx.TenantID, "account_not_found").Inc()
		log.WithFields(logrus.Fields{
			"from_account": req.FromAccount,
			"to_account":   req.ToAccount,
		}).Warn("Account not found")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "account not found"})
		return
	}

	// CRITICAL SECURITY CHECK: Verify both accounts belong to the requesting tenant
	if fromAcc.TenantID != reqCtx.TenantID || toAcc.TenantID != reqCtx.TenantID {
		appMetrics.TransfersTotal.WithLabelValues(reqCtx.TenantID, "unauthorized").Inc()
		log.WithFields(logrus.Fields{
			"from_tenant": fromAcc.TenantID,
			"to_tenant":   toAcc.TenantID,
		}).Error("Attempted cross-tenant transfer - SECURITY VIOLATION")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "access denied"})
		return
	}

	// Validation
	if req.Amount <= 0 {
		appMetrics.TransfersTotal.WithLabelValues(reqCtx.TenantID, "invalid_amount").Inc()
		log.WithField("amount", req.Amount).Warn("Invalid transfer amount")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid amount"})
		return
	}

	if fromAcc.Balance < req.Amount {
		appMetrics.TransfersTotal.WithLabelValues(reqCtx.TenantID, "insufficient_balance").Inc()
		log.WithFields(logrus.Fields{
			"from_account": req.FromAccount,
			"balance":      fromAcc.Balance,
			"amount":       req.Amount,
		}).Warn("Insufficient balance")
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(map[string]string{"error": "insufficient balance"})
		return
	}

	// Execute transfer (write locks)
	dbLock.Lock()
	fromAcc.Balance -= req.Amount
	toAcc.Balance += req.Amount
	dbLock.Unlock()

	transferID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
	appMetrics.TransfersTotal.WithLabelValues(reqCtx.TenantID, "success").Inc()
	appMetrics.TransferAmount.WithLabelValues(reqCtx.TenantID).Observe(req.Amount)

	log.WithFields(logrus.Fields{
		"transfer_id":  transferID,
		"from_account": req.FromAccount,
		"to_account":   req.ToAccount,
		"amount":       req.Amount,
	}).Info("Transfer completed")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.TransferResponse{
		TransferID:  transferID,
		Status:      "completed",
		FromAccount: req.FromAccount,
		ToAccount:   req.ToAccount,
		Amount:      req.Amount,
		CreatedAt:   time.Now(),
		Message:     "Transfer successful",
	})
}

// handleSimulateFailure enables/disables failure mode for testing rollback
func handleSimulateFailure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	failureMode = !failureMode
	status := "disabled"
	if failureMode {
		status = "enabled"
		log.Warn("FAILURE MODE ENABLED - Service will report unhealthy")
	} else {
		log.Info("Failure mode disabled")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"failure_mode": failureMode,
		"status":       status,
	})
}

func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

func generateTraceID() string {
	return fmt.Sprintf("trace_%d", time.Now().UnixNano())
}
