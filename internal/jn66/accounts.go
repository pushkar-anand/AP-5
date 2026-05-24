package jn66

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// accountsAPI is the subset of Client used by AccountCache — enables testing.
type accountsAPI interface {
	ListAccounts(ctx context.Context) ([]Account, error)
	CreateAccount(ctx context.Context, institution, name, externalID string) (*Account, bool, error)
}

// AccountCache discovers and caches JN-66 accounts, creating new credit card
// accounts on first encounter using (institution, last_four) as the unique key.
type AccountCache struct {
	mu     sync.Mutex
	log    *slog.Logger
	client accountsAPI
	cache  map[string]string // "institution|last_four" -> account ID
}

func NewAccountCache(log *slog.Logger, client *Client) *AccountCache {
	return &AccountCache{
		log:    log,
		client: client,
		cache:  make(map[string]string),
	}
}

// LookupOrCreate returns the JN-66 account ID for the given card,
// creating it in JN-66 if it does not yet exist.
func (c *AccountCache) LookupOrCreate(ctx context.Context, institution, lastFour string) (string, error) {
	key := cacheKey(institution, lastFour)

	c.mu.Lock()
	defer c.mu.Unlock()

	if id, ok := c.cache[key]; ok {
		return id, nil
	}

	// Refresh from JN-66 and try again.
	if err := c.refresh(ctx); err != nil {
		return "", err
	}

	if id, ok := c.cache[key]; ok {
		return id, nil
	}

	// Still not found — create it.
	name := fmt.Sprintf("%s ••••%s", institution, lastFour)
	account, alreadyExists, err := c.client.CreateAccount(ctx, institution, name, lastFour)
	if err != nil {
		return "", fmt.Errorf("accounts: create %s (%s): %w", institution, lastFour, err)
	}

	if alreadyExists {
		// 409 — another process created it; refresh and retry once.
		if err := c.refresh(ctx); err != nil {
			return "", err
		}
		if id, ok := c.cache[key]; ok {
			return id, nil
		}
		return "", fmt.Errorf("accounts: created account not found after refresh for %s (%s)", institution, lastFour)
	}

	c.log.InfoContext(ctx, "created new JN-66 account",
		slog.String("institution", institution),
		slog.String("last_four", lastFour),
		slog.String("account_id", account.ID),
	)

	c.cache[key] = account.ID
	return account.ID, nil
}

func (c *AccountCache) refresh(ctx context.Context) error {
	accounts, err := c.client.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("accounts: refresh: %w", err)
	}
	for _, a := range accounts {
		if a.AccountType == "credit_card" {
			c.cache[cacheKey(a.Institution, a.ExternalAccountID)] = a.ID
		}
	}
	return nil
}

func cacheKey(institution, lastFour string) string {
	return strings.ToLower(institution) + "|" + lastFour
}
