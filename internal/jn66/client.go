package jn66

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type Account struct {
	ID                string `json:"id"`
	Institution       string `json:"institution"`
	ExternalAccountID string `json:"external_account_id"`
	Name              string `json:"name"`
	AccountType       string `json:"account_type"`
	IsActive          bool   `json:"is_active"`
}

type createAccountRequest struct {
	Institution       string `json:"institution"`
	Name              string `json:"name"`
	AccountType       string `json:"account_type"`
	ExternalAccountID string `json:"external_account_id"`
}

type ImportTransaction struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	AmountPaise int64  `json:"amount_paise"`
	Direction   string `json:"direction"`
	Reference   string `json:"reference,omitempty"`
}

type importRequest struct {
	AccountID    string              `json:"account_id"`
	NoEnrich     bool                `json:"no_enrich"`
	Transactions []ImportTransaction `json:"transactions"`
}

type ImportResult struct {
	Parsed    int `json:"parsed"`
	Inserted  int `json:"inserted"`
	Duplicate int `json:"duplicate"`
	Failed    int `json:"failed"`
}

// ListAccounts returns all accounts for the authenticated user.
func (c *Client) ListAccounts(ctx context.Context) ([]Account, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/accounts", nil)
	if err != nil {
		return nil, fmt.Errorf("jn66: build list accounts request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jn66: list accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jn66: list accounts: unexpected status %d", resp.StatusCode)
	}

	var accounts []Account
	if err := json.NewDecoder(resp.Body).Decode(&accounts); err != nil {
		return nil, fmt.Errorf("jn66: decode accounts: %w", err)
	}
	return accounts, nil
}

// CreateAccount creates a new account. Returns the created account and whether it already existed.
func (c *Client) CreateAccount(ctx context.Context, institution, name, externalID string) (*Account, bool, error) {
	body, _ := json.Marshal(createAccountRequest{
		Institution:       institution,
		Name:              name,
		AccountType:       "credit_card",
		ExternalAccountID: externalID,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/accounts", bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("jn66: build create account request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("jn66: create account: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		var account Account
		if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
			return nil, false, fmt.Errorf("jn66: decode created account: %w", err)
		}
		return &account, false, nil
	case http.StatusConflict:
		return nil, true, nil
	default:
		b, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("jn66: create account: unexpected status %d: %s", resp.StatusCode, b)
	}
}

// Import posts transactions to JN-66.
func (c *Client) Import(ctx context.Context, accountID string, txns []ImportTransaction) (*ImportResult, error) {
	body, _ := json.Marshal(importRequest{
		AccountID:    accountID,
		Transactions: txns,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/import", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jn66: build import request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jn66: import: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jn66: import: unexpected status %d: %s", resp.StatusCode, b)
	}

	var result ImportResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("jn66: decode import result: %w", err)
	}
	return &result, nil
}

func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
}
