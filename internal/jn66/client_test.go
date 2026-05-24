package jn66_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/jn66"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *jn66.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, jn66.NewClient(srv.URL, "test-token")
}

func TestListAccounts_Success(t *testing.T) {
	accounts := []jn66.Account{
		{ID: "uuid-1", Institution: "HDFC Bank", ExternalAccountID: "1234", AccountType: "credit_card"},
		{ID: "uuid-2", Institution: "ICICI Bank", ExternalAccountID: "5678", AccountType: "credit_card"},
	}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/accounts" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or wrong auth header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(accounts)
	})

	got, err := client.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ListAccounts len = %d, want 2", len(got))
	}
	if got[0].ID != "uuid-1" {
		t.Errorf("first account ID = %q", got[0].ID)
	}
}

func TestListAccounts_ServerError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.ListAccounts(context.Background())
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func TestCreateAccount_Created(t *testing.T) {
	created := jn66.Account{ID: "new-uuid", Institution: "Axis Bank", ExternalAccountID: "9999", AccountType: "credit_card"}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	})

	account, alreadyExists, err := client.CreateAccount(context.Background(), "Axis Bank", "Axis ••••9999", "9999")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if alreadyExists {
		t.Error("alreadyExists should be false for 201")
	}
	if account.ID != "new-uuid" {
		t.Errorf("account ID = %q", account.ID)
	}
}

func TestCreateAccount_Conflict(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})

	account, alreadyExists, err := client.CreateAccount(context.Background(), "HDFC Bank", "HDFC ••••1234", "1234")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if !alreadyExists {
		t.Error("alreadyExists should be true for 409")
	}
	if account != nil {
		t.Errorf("account should be nil on conflict, got %+v", account)
	}
}

func TestImport_Success(t *testing.T) {
	result := jn66.ImportResult{Parsed: 1, Inserted: 1}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/import" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	txns := []jn66.ImportTransaction{
		{Date: "2026-05-25", Description: "Amazon", AmountPaise: 50000, Direction: "debit"},
	}

	got, err := client.Import(context.Background(), "account-uuid", txns)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got.Inserted != 1 {
		t.Errorf("Inserted = %d, want 1", got.Inserted)
	}
}

func TestImport_Duplicate(t *testing.T) {
	result := jn66.ImportResult{Parsed: 1, Duplicate: 1}

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	txns := []jn66.ImportTransaction{
		{Date: "2026-05-25", Description: "Amazon", AmountPaise: 50000, Direction: "debit"},
	}

	got, err := client.Import(context.Background(), "account-uuid", txns)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if got.Duplicate != 1 {
		t.Errorf("Duplicate = %d, want 1", got.Duplicate)
	}
}
