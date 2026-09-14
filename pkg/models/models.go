package models

import "time"

// SearchRequest represents a search API request
type SearchRequest struct {
	AccountID string `json:"account_id"`
}

// SearchResponse represents a search API response
type SearchResponse struct {
	AccountID   string    `json:"account_id"`
	AccountName string    `json:"account_name"`
	Balance     float64   `json:"balance"`
	TenantID    string    `json:"tenant_id"`
	Status      string    `json:"status"`
	LastUpdated time.Time `json:"last_updated"`
}

// TransferRequest represents a transfer API request
type TransferRequest struct {
	FromAccount string  `json:"from_account"`
	ToAccount   string  `json:"to_account"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description,omitempty"`
}

// TransferResponse represents a transfer API response
type TransferResponse struct {
	TransferID  string    `json:"transfer_id"`
	Status      string    `json:"status"`
	FromAccount string    `json:"from_account"`
	ToAccount   string    `json:"to_account"`
	Amount      float64   `json:"amount"`
	CreatedAt   time.Time `json:"created_at"`
	Message     string    `json:"message,omitempty"`
}

// AuthRequest represents an authentication request
type AuthRequest struct {
	APIKey string `json:"api_key"`
	Secret string `json:"secret"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	TenantID  string    `json:"tenant_id"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status       string                 `json:"status"`
	Timestamp    time.Time              `json:"timestamp"`
	Service      string                 `json:"service"`
	Version      string                 `json:"version"`
	Dependencies map[string]interface{} `json:"dependencies"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	TraceID   string    `json:"trace_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Account represents a synthetic bank account
type Account struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Owner     string    `json:"owner"`
	Balance   float64   `json:"balance"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Transfer represents a transfer transaction
type Transfer struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	FromAccount string    `json:"from_account"`
	ToAccount   string    `json:"to_account"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"` // initiated, processing, completed, failed
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SyntheticDatabase is an in-memory database for demo purposes
type SyntheticDatabase struct {
	Accounts  map[string]*Account
	Transfers map[string]*Transfer
	Ledger    []*LedgerEntry
}

// LedgerEntry represents a ledger transaction
type LedgerEntry struct {
	ID          string
	TransferID  string
	TenantID    string
	FromAccount string
	ToAccount   string
	Amount      float64
	Timestamp   time.Time
}

// NewSyntheticDatabase creates and initializes a synthetic database
func NewSyntheticDatabase() *SyntheticDatabase {
	db := &SyntheticDatabase{
		Transfers: make(map[string]*Transfer),
		Ledger:    make([]*LedgerEntry, 0),
	}

	// Seed data for tenant Alpha
	db.Accounts = generateAccounts(100)

	return db
}

// GetAccountsByTenant returns all accounts for a tenant (tenant isolation)
func (db *SyntheticDatabase) GetAccountsByTenant(tenantID string) []*Account {
	var accounts []*Account
	for _, acc := range db.Accounts {
		if acc.TenantID == tenantID {
			accounts = append(accounts, acc)
		}
	}
	return accounts
}
