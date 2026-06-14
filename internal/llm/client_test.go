package llm_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

func mockOllamaServer(t *testing.T, responseContent string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatCompletionMessage{Content: responseContent}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func newTestClient(t *testing.T, srv *httptest.Server) *llm.Client {
	t.Helper()
	return llm.NewWithHTTPClient(slog.Default(), srv.URL+"/v1", "router-model", "extractor-model", srv.Client())
}

func TestClassify_CreditCard(t *testing.T) {
	srv := mockOllamaServer(t, `{"type":"credit_card_transaction"}`)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.Classify(context.Background(), "HDFC Credit Card Alert", "Rs. 500 spent at Amazon")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got != "credit_card_transaction" {
		t.Errorf("Classify = %q, want %q", got, "credit_card_transaction")
	}
}

func TestClassify_Other(t *testing.T) {
	srv := mockOllamaServer(t, `{"type":"other"}`)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.Classify(context.Background(), "Newsletter", "Check out our offers")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got != "other" {
		t.Errorf("Classify = %q, want %q", got, "other")
	}
}

func TestClassify_MalformedJSON_FallsBackToOther(t *testing.T) {
	srv := mockOllamaServer(t, `not json at all`)
	defer srv.Close()

	c := newTestClient(t, srv)

	got, err := c.Classify(context.Background(), "subject", "body")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got != "other" {
		t.Errorf("Classify with bad JSON = %q, want %q", got, "other")
	}
}

func TestClassify_BodyTruncatedAt500Chars(t *testing.T) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) > 0 {
			captured = req.Messages[0].Content
		}
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatCompletionMessage{Content: `{"type":"other"}`}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	longBody := string(make([]byte, 1000))
	_, _ = c.Classify(context.Background(), "subject", longBody)

	// The prompt should contain at most 500 chars of the body
	bodyInPrompt := longBody[:500]
	if len(captured) > 0 && len(captured) > len(bodyInPrompt)+200 {
		t.Error("body was not truncated to 500 chars in classify prompt")
	}
}

func TestExtractTransaction_Success(t *testing.T) {
	payload := `{
		"institution": "HDFC Bank",
		"last_four": "1234",
		"amount_paise": 125000,
		"merchant": "Amazon",
		"date": "2026-05-25",
		"direction": "debit"
	}`
	srv := mockOllamaServer(t, payload)
	defer srv.Close()

	c := newTestClient(t, srv)

	txn, err := c.ExtractTransaction(context.Background(), "HDFC Alert", "Rs 1250 at Amazon")
	if err != nil {
		t.Fatalf("ExtractTransaction: %v", err)
	}
	if txn == nil {
		t.Fatal("ExtractTransaction returned nil")
	}
	if txn.Institution != "HDFC Bank" {
		t.Errorf("Institution = %q, want %q", txn.Institution, "HDFC Bank")
	}
	if txn.LastFour != "1234" {
		t.Errorf("LastFour = %q, want %q", txn.LastFour, "1234")
	}
	if txn.AmountPaise != 125000 {
		t.Errorf("AmountPaise = %d, want 125000", txn.AmountPaise)
	}
	if txn.Merchant != "Amazon" {
		t.Errorf("Merchant = %q", txn.Merchant)
	}
	if txn.Direction != "debit" {
		t.Errorf("Direction = %q", txn.Direction)
	}
}

func TestExtractTransaction_ErrorResponse_ReturnsNil(t *testing.T) {
	srv := mockOllamaServer(t, `{"error":"not a transaction email"}`)
	defer srv.Close()

	c := newTestClient(t, srv)

	txn, err := c.ExtractTransaction(context.Background(), "OTP", "Your OTP is 123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txn != nil {
		t.Errorf("expected nil transaction, got %+v", txn)
	}
}
