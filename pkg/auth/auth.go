// Package auth provides JWT token management for authentication and authorization in the fintech platform.
package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// TokenClaims represents custom JWT claims
type TokenClaims struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

// TokenManager handles JWT creation and validation
type TokenManager struct {
	signingKey   *rsa.PrivateKey
	verifyingKey *rsa.PublicKey
	log          *logrus.Logger
	issuer       string
	expiryHours  int
}

// New creates a new TokenManager with RSA keys
func New(privateKeyPEM, publicKeyPEM string, log *logrus.Logger) (*TokenManager, error) {
	// Parse private key
	privBlock, _ := pem.Decode([]byte(privateKeyPEM))
	if privBlock == nil {
		return nil, fmt.Errorf("failed to parse private key PEM")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(privBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Parse public key
	pubBlock, _ := pem.Decode([]byte(publicKeyPEM))
	if pubBlock == nil {
		return nil, fmt.Errorf("failed to parse public key PEM")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	pubKey, ok := pubKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return &TokenManager{
		signingKey:   privKey,
		verifyingKey: pubKey,
		log:          log,
		issuer:       "fintech-platform",
		expiryHours:  24,
	}, nil
}

// NewToken creates a new JWT token for a tenant
func (tm *TokenManager) NewToken(tenantID, userID, clientID string) (string, error) {
	now := time.Now()
	expiryTime := now.Add(time.Duration(tm.expiryHours) * time.Hour)

	claims := TokenClaims{
		TenantID: tenantID,
		UserID:   userID,
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(tm.signingKey)
	if err != nil {
		tm.log.WithError(err).Error("Failed to sign token")
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	tm.log.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"user_id":    userID,
		"expires_at": expiryTime,
	}).Debug("New token issued")

	return tokenString, nil
}

// ValidateToken validates and parses a JWT token
func (tm *TokenManager) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.verifyingKey, nil
	})

	if err != nil {
		tm.log.WithError(err).Warn("Token parsing failed")
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	if !token.Valid {
		tm.log.Warn("Token validation failed")
		return nil, fmt.Errorf("token is invalid")
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify issuer
	if claims.Issuer != tm.issuer {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Issuer)
	}

	tm.log.WithFields(logrus.Fields{
		"tenant_id": claims.TenantID,
		"user_id":   claims.UserID,
	}).Debug("Token validated")

	return claims, nil
}

// GenerateTestKeys generates RSA keys for testing (in production, use proper key management)
func GenerateTestKeys() (privateKeyPEM, publicKeyPEM string, err error) {
	// Note: In production, use a proper key generation tool or load from secure storage
	// These are example keys for demonstration only
	privateKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/N8r1GfKTN8J0iEqBJRK5Z2BtK8r5D5aE
VO0JnZVN8V8cQN8Y5oPvWvJGmZhPF0V0J8V9K6J8V7Z5K5m0V5K8V9Z3K0Q4J5
Z8V8Z5K3V4K7V8Z9L1R3K4Z6V7Z4K2V3K6V7Z8L0S2J3Z5V6Z3J1U2J2Z4U5Z2
J0T1I1Z3T4Z1I9S0H0Z2S3Z0H8R9G9Z1R2Z9G7Q8F8Z0Q1Z8F6P7E7Y9P0Z7E5
O6D6Y8O9Z6D4N5C5X7N8Z5C3M4B4W6M7Z4B2L3A3V5L6Z3A1K2A2U4K5Z2A0J1
9Z1T0J0Z1I9Y0Y9Z0H8X9X8Z9G7W8W7Z8F6V7V6Z7E5U6U5Z6D4T5T4Z5C3S4S3
Z4B2R3R2Z3A1Q2Q1Z2Y0P1P0ZWQIDAQABAoIBAGQMT+K0t6MnYr3U9GN1lEqV
QvZ9wl5x8z5K2m0v5K8v9Z3k0Q4j5z8v5K3V4k7v8z9L1r3k4z6v7z4k2V3k6v7
z8L0S2j3z5v6z3j1U2j2z4u5z2j0t1i1z3t4z1i9s0h0z2s3z0h8r9g9z1r2z9
g7q8f8z0q1z8f6p7e7y9p0z7e5o6d6y8o9z6d4n5c5x7n8z5c3m4b4w6m7z4b2
l3a3v5l6z3a1k2a2u4k5z2a0j1AoGBAPj7K8v9L1Z0K9V8L1Z5J8U7K0Y5I7T6
J9X4H6S5I8W3G5R4H7V2F4Q3G6U1E3P2F5T0D2O1E4S9C1N0D3R8B0M9C2Q7A9
L8B1P6Z8K7A0O5Y7J6Z9N4X6I5Y8M3W5H4X7L2V4G3W6K1U3F2V5J0T2E1U4I9
S1D0T3H8R0C9S2G7Q9B8R1F6P8A7Q0E5O7Z9P9D4N6Y8O8C3M5X7N7B2L4W6M6
A1K3V5L5Z0J2U4K4Y9I1T3J3X8H0S2I2W7G9R1H1V6F8Q0G0U5E7P9F9T4D6O8
E8S3C5N7D7R2B4M6C6Q1A3L5B5P0Z2K4A4O9Y1J3Z3N8X0I2Y2M7W9H1X1L6V8
G0W0K5U7F9V9J4T6E8U8I3S5D7T7H2R4C6S6G1Q3B5R5F0P2A4Q4E9O1Z3P3D8
N0Y2O2C7M9X1N1B6L8W0M0A5K7V9L9Z4J6U8K8Y3I5T7J7X2H4S6I6W1G3R5H5
V0F2Q4G4U9E1P3F3T8D0O2E2S7C9N1D1R6B8M0C0Q5A7L9B9P4Z1K3A3O8Y0I2
Z2N7X9H1Y1M6W8G0X0L5V7F9W9K4U6E8V8J3T5D7U7I2S4C6T6H1R3B5S5G0Q2
A4R4F9P1Z3Q3E8O0Y2P2D7N9X1O1C6M8W0N0B5L7V9M9A4K6U8L8Z3J5T7K7Y2
I4S6J6X1H3R5I5W0G2Q4H4V9F1P3G3U8E0O2F2T7D9N1E1S6C8M0D0R5B7L9C9
Q4A6P4Z1K3A3O8Y0J2Z2N7X9I1Y1M6W8H0X0L5V7G9W9K4U6F8V8J3T5E7U7I2
S4D6T6H1R3C5S5G0Q2B4R4F9P1Z3Q3E8O0Y2P2D7N9X1O1C6M8W0N0B5L7V9M9
A4K6U8L8Z3J5T7K7Y2I4S6J6X1H3R5I5W0G2Q4H4V9F1P3G3U8E0O2F2T7D9N1
AoGBANx3H5Y0I1Z1J2K3L4M5N6O7P8Q9R0S1T2U3V4W5X6Y7Z8A9B0C1D2E3F4
G5H6I7J8K9L0M1N2O3P4Q5R6S7T8U9V0W1X2Y3Z4A5B6C7D8E9F0G1H2I3J4K5
-----END RSA PRIVATE KEY-----`

	publicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA0Z3VS5JJcds3xfn/N8r1
GfKTN8J0iEqBJRK5Z2BtK8r5D5aEVO0JnZVN8V8cQN8Y5oPvWvJGmZhPF0V0J8V9
K6J8V7Z5K5m0V5K8V9Z3K0Q4J5Z8V8Z5K3V4K7V8Z9L1R3K4Z6V7Z4K2V3K6V7Z8
L0S2J3Z5V6Z3J1U2J2Z4U5Z2J0T1I1Z3T4Z1I9S0H0Z2S3Z0H8R9G9Z1R2Z9G7Q8
F8Z0Q1Z8F6P7E7Y9P0Z7E5O6D6Y8O9Z6D4N5C5X7N8Z5C3M4B4W6M7Z4B2L3A3V5L6
Z3A1K2A2U4K5Z2A0J19Z1T0J0Z1I9Y0Y9Z0H8X9X8Z9G7W8W7Z8F6V7V6Z7E5U6U5Z6
D4T5T4Z5C3S4S3Z4B2R3R2Z3A1Q2Q1Z2Y0P1P0ZQIDAQAB
-----END PUBLIC KEY-----`

	return privateKeyPEM, publicKeyPEM, nil
}
