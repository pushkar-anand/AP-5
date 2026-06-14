package jn66

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type mockAccountsAPI struct {
	accounts       []Account
	createAccount  *Account
	createConflict bool
	createErr      error
	listErr        error
	listCalls      int
	createCalls    int
}

func (m *mockAccountsAPI) ListAccounts(_ context.Context) ([]Account, error) {
	m.listCalls++
	return m.accounts, m.listErr
}

func (m *mockAccountsAPI) CreateAccount(_ context.Context, institution, name, externalID string) (*Account, bool, error) {
	m.createCalls++
	if m.createErr != nil {
		return nil, false, m.createErr
	}
	if m.createConflict {
		return nil, true, nil
	}
	return m.createAccount, false, nil
}

func newTestCache(api accountsAPI) *AccountCache {
	return &AccountCache{
		log:    slog.Default(),
		client: api,
		cache:  make(map[string]string),
	}
}

func TestLookupOrCreate_FoundAfterRefresh(t *testing.T) {
	api := &mockAccountsAPI{
		accounts: []Account{
			{ID: "uuid-1", Institution: "HDFC Bank", ExternalAccountID: "1234", AccountType: "credit_card"},
		},
	}
	cache := newTestCache(api)

	id, err := cache.LookupOrCreate(context.Background(), "HDFC Bank", "1234")
	if err != nil {
		t.Fatalf("LookupOrCreate: %v", err)
	}
	if id != "uuid-1" {
		t.Errorf("id = %q, want %q", id, "uuid-1")
	}
	if api.createCalls != 0 {
		t.Errorf("CreateAccount should not be called, called %d times", api.createCalls)
	}
}

func TestLookupOrCreate_CreatesWhenMissing(t *testing.T) {
	api := &mockAccountsAPI{
		accounts:      []Account{},
		createAccount: &Account{ID: "new-uuid", Institution: "Axis Bank", ExternalAccountID: "9999"},
	}
	cache := newTestCache(api)

	id, err := cache.LookupOrCreate(context.Background(), "Axis Bank", "9999")
	if err != nil {
		t.Fatalf("LookupOrCreate: %v", err)
	}
	if id != "new-uuid" {
		t.Errorf("id = %q, want %q", id, "new-uuid")
	}
	if api.createCalls != 1 {
		t.Errorf("CreateAccount called %d times, want 1", api.createCalls)
	}
}

func TestLookupOrCreate_CachesAfterFirstLookup(t *testing.T) {
	api := &mockAccountsAPI{
		accounts: []Account{
			{ID: "cached-id", Institution: "ICICI Bank", ExternalAccountID: "5678", AccountType: "credit_card"},
		},
	}
	cache := newTestCache(api)

	_, _ = cache.LookupOrCreate(context.Background(), "ICICI Bank", "5678")
	_, _ = cache.LookupOrCreate(context.Background(), "ICICI Bank", "5678")

	// Second call should hit cache — ListAccounts called only once
	if api.listCalls > 1 {
		t.Errorf("ListAccounts called %d times, want 1 (cache should be used)", api.listCalls)
	}
}

func TestLookupOrCreate_ConflictTriggersRefetch(t *testing.T) {
	callCount := 0
	api := &mockAccountsAPI{
		createConflict: true,
	}
	// Override ListAccounts to return an account on the second call
	api2 := &struct{ mockAccountsAPI }{mockAccountsAPI: *api}
	_ = api2

	// Simulate: first list returns empty, conflict on create, second list returns the account
	dynamicAPI := &dynamicMockAPI{
		listResponses: [][]Account{
			{},
			{{ID: "conflict-uuid", Institution: "SBI", ExternalAccountID: "0000", AccountType: "credit_card"}},
		},
		createConflict: true,
		listCallIndex:  &callCount,
	}
	cache := newTestCache(dynamicAPI)

	id, err := cache.LookupOrCreate(context.Background(), "SBI", "0000")
	if err != nil {
		t.Fatalf("LookupOrCreate: %v", err)
	}
	if id != "conflict-uuid" {
		t.Errorf("id = %q, want %q", id, "conflict-uuid")
	}
}

func TestLookupOrCreate_CreateError(t *testing.T) {
	api := &mockAccountsAPI{
		accounts:  []Account{},
		createErr: errors.New("network error"),
	}
	cache := newTestCache(api)

	_, err := cache.LookupOrCreate(context.Background(), "HDFC Bank", "1234")
	if err == nil {
		t.Error("expected error when CreateAccount fails, got nil")
	}
}

func TestCacheKey_CaseInsensitive(t *testing.T) {
	k1 := cacheKey("HDFC Bank", "1234")
	k2 := cacheKey("hdfc bank", "1234")
	if k1 != k2 {
		t.Errorf("cacheKey not case-insensitive: %q vs %q", k1, k2)
	}
}

// dynamicMockAPI returns different list responses per call — for conflict test.
type dynamicMockAPI struct {
	listResponses  [][]Account
	createConflict bool
	listCallIndex  *int
}

func (d *dynamicMockAPI) ListAccounts(_ context.Context) ([]Account, error) {
	i := *d.listCallIndex
	*d.listCallIndex++
	if i >= len(d.listResponses) {
		return d.listResponses[len(d.listResponses)-1], nil
	}
	return d.listResponses[i], nil
}

func (d *dynamicMockAPI) CreateAccount(_ context.Context, _, _, _ string) (*Account, bool, error) {
	return nil, d.createConflict, nil
}
