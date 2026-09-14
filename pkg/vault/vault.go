package vault

import (
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus"
)

// Client wraps Vault client for secrets and identity
type Client struct {
	client *vault.Client
	log    *logrus.Logger
}

// AppRoleAuth holds AppRole credentials
type AppRoleAuth struct {
	RoleID   string
	SecretID string
}

// New creates a new Vault client
func New(addr string, log *logrus.Logger) (*Client, error) {
	config := vault.DefaultConfig()
	config.Address = addr

	// In production, set TLS config for CA verification
	// For local dev, we accept insecure
	//config.HttpClient.Transport.(*vault.DefaultTransport).TLSConfig.InsecureSkipVerify = true

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	return &Client{
		client: client,
		log:    log,
	}, nil
}

// AuthAppRole authenticates using AppRole method and returns a token
func (c *Client) AuthAppRole(roleID, secretID string) (string, error) {
	start := time.Now()

	payload := map[string]interface{}{
		"role_id":   roleID,
		"secret_id": secretID,
	}

	secret, err := c.client.Logical().Write("auth/approle/login", payload)
	if err != nil {
		c.log.WithError(err).Error("AppRole auth failed")
		return "", fmt.Errorf("approle auth failed: %w", err)
	}

	if secret == nil || secret.Auth == nil {
		return "", fmt.Errorf("no auth info returned from vault")
	}

	c.log.WithFields(logrus.Fields{
		"duration_ms": time.Since(start).Milliseconds(),
		"ttl":         secret.Auth.LeaseDuration,
	}).Info("AppRole authentication successful")

	token := secret.Auth.ClientToken
	c.client.SetToken(token)
	return token, nil
}

// GetSecret retrieves a secret from Vault at path
func (c *Client) GetSecret(path string) (map[string]interface{}, error) {
	secret, err := c.client.Logical().Read(path)
	if err != nil {
		c.log.WithError(err).WithField("path", path).Error("Failed to read secret")
		return nil, fmt.Errorf("failed to read secret at %s: %w", path, err)
	}

	if secret == nil {
		return nil, fmt.Errorf("secret not found at path: %s", path)
	}

	if secret.Data == nil {
		return nil, fmt.Errorf("secret has no data at path: %s", path)
	}

	return secret.Data, nil
}

// GetDatabaseCredentials retrieves dynamic database credentials
func (c *Client) GetDatabaseCredentials(role string) (username, password string, err error) {
	path := fmt.Sprintf("database/creds/%s", role)

	secret, err := c.client.Logical().Read(path)
	if err != nil {
		c.log.WithError(err).WithField("role", role).Error("Failed to get database credentials")
		return "", "", fmt.Errorf("failed to get db creds for role %s: %w", role, err)
	}

	if secret == nil || secret.Data == nil {
		return "", "", fmt.Errorf("no database credentials returned for role: %s", role)
	}

	username, ok := secret.Data["username"].(string)
	if !ok {
		return "", "", fmt.Errorf("invalid username in database credentials")
	}

	password, ok = secret.Data["password"].(string)
	if !ok {
		return "", "", fmt.Errorf("invalid password in database credentials")
	}

	c.log.WithFields(logrus.Fields{
		"role":     role,
		"username": username,
		"ttl":      secret.LeaseDuration,
	}).Debug("Database credentials retrieved")

	return username, password, nil
}

// RenewToken renews the current token
func (c *Client) RenewToken() error {
	secret, err := c.client.Auth().Token().RenewSelf(3600) // Renew for 1 hour
	if err != nil {
		c.log.WithError(err).Error("Token renewal failed")
		return fmt.Errorf("token renewal failed: %w", err)
	}

	c.log.WithField("ttl", secret.Auth.LeaseDuration).Debug("Token renewed")
	return nil
}

// Health checks Vault health status
func (c *Client) Health() error {
	health, err := c.client.Sys().Health()
	if err != nil {
		return fmt.Errorf("vault health check failed: %w", err)
	}

	if health.Sealed {
		return fmt.Errorf("vault is sealed")
	}

	return nil
}

// String safely represents client without exposing token
func (c *Client) String() string {
	return fmt.Sprintf("VaultClient{addr: %s}", c.client.Address())
}
