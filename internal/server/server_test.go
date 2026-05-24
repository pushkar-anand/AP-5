package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/server"
)

type mockOAuthExchanger struct {
	err error
}

func (m *mockOAuthExchanger) Exchange(_ context.Context, _, _ string) error {
	return m.err
}

func newTestRouter(t *testing.T, exchanger server.OAuthExchanger) http.Handler {
	t.Helper()
	return server.NewRouter(exchanger)
}

func TestHealthz(t *testing.T) {
	handler := newTestRouter(t, &mockOAuthExchanger{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want 200", rec.Code)
	}
}

func TestAuthCallback_MissingCode(t *testing.T) {
	handler := newTestRouter(t, &mockOAuthExchanger{})

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?state=user@gmail.com", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAuthCallback_MissingState(t *testing.T) {
	handler := newTestRouter(t, &mockOAuthExchanger{})

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAuthCallback_Success(t *testing.T) {
	handler := newTestRouter(t, &mockOAuthExchanger{})

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=authcode123&state=user@gmail.com", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAuthCallback_ExchangeError(t *testing.T) {
	handler := newTestRouter(t, &mockOAuthExchanger{err: errors.New("token exchange failed")})

	req := httptest.NewRequest(http.MethodGet, "/auth/callback?code=badcode&state=user@gmail.com", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
