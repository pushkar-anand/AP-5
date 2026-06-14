package creditcard_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/handlers/creditcard"
	"github.com/pushkar-anand/ap-5/internal/jn66"
	"github.com/pushkar-anand/ap-5/internal/llm"
)

type mockExtractor struct {
	result *llm.TransactionData
	err    error
}

func (m *mockExtractor) ExtractTransaction(_ context.Context, _, _ string) (*llm.TransactionData, error) {
	return m.result, m.err
}

type mockAccountResolver struct {
	accountID string
	err       error
	calls     []struct{ institution, lastFour string }
}

func (m *mockAccountResolver) LookupOrCreate(_ context.Context, institution, lastFour string) (string, error) {
	m.calls = append(m.calls, struct{ institution, lastFour string }{institution, lastFour})
	return m.accountID, m.err
}

type mockImporter struct {
	result *jn66.ImportResult
	err    error
	calls  []string // account IDs
}

func (m *mockImporter) Import(_ context.Context, accountID string, _ []jn66.ImportTransaction) (*jn66.ImportResult, error) {
	m.calls = append(m.calls, accountID)
	return m.result, m.err
}

func newHandler(extractor *mockExtractor, resolver *mockAccountResolver, importer *mockImporter) *creditcard.Handler {
	return creditcard.New(slog.Default(), extractor, resolver, importer)
}

func TestHandle_SuccessfulTransaction(t *testing.T) {
	extractor := &mockExtractor{result: &llm.TransactionData{
		Institution: "HDFC Bank",
		LastFour:    "1234",
		AmountPaise: 125000,
		Merchant:    "Amazon",
		Date:        "2026-05-25",
		Direction:   "debit",
	}}
	resolver := &mockAccountResolver{accountID: "account-uuid"}
	importer := &mockImporter{result: &jn66.ImportResult{Inserted: 1}}

	h := newHandler(extractor, resolver, importer)
	if err := h.Handle(context.Background(), "user@gmail.com", &gmail.Message{Subject: "HDFC Alert", Body: "Spent Rs 1250"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resolver.calls) != 1 {
		t.Errorf("LookupOrCreate called %d times, want 1", len(resolver.calls))
	}
	if resolver.calls[0].institution != "HDFC Bank" || resolver.calls[0].lastFour != "1234" {
		t.Errorf("LookupOrCreate args = %+v", resolver.calls[0])
	}
	if len(importer.calls) != 1 || importer.calls[0] != "account-uuid" {
		t.Errorf("Import called with accountID %v", importer.calls)
	}
}

func TestHandle_ExtractorReturnsNil_NoImport(t *testing.T) {
	extractor := &mockExtractor{result: nil}
	resolver := &mockAccountResolver{}
	importer := &mockImporter{}

	h := newHandler(extractor, resolver, importer)
	if err := h.Handle(context.Background(), "user@gmail.com", &gmail.Message{Subject: "OTP"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(importer.calls) != 0 {
		t.Errorf("Import should not be called when extractor returns nil, called %d times", len(importer.calls))
	}
}

func TestHandle_ExtractorError_NoImport(t *testing.T) {
	extractor := &mockExtractor{err: errors.New("llm error")}
	resolver := &mockAccountResolver{}
	importer := &mockImporter{}

	h := newHandler(extractor, resolver, importer)
	if err := h.Handle(context.Background(), "user@gmail.com", &gmail.Message{Subject: "HDFC Alert"}); err == nil {
		t.Error("expected error when extractor fails")
	}

	if len(importer.calls) != 0 {
		t.Error("Import should not be called when extractor errors")
	}
}

func TestHandle_AccountResolverError_NoImport(t *testing.T) {
	extractor := &mockExtractor{result: &llm.TransactionData{
		Institution: "HDFC Bank", LastFour: "1234",
		AmountPaise: 50000, Merchant: "Zomato", Date: "2026-05-25", Direction: "debit",
	}}
	resolver := &mockAccountResolver{err: errors.New("jn66 unavailable")}
	importer := &mockImporter{}

	h := newHandler(extractor, resolver, importer)
	if err := h.Handle(context.Background(), "user@gmail.com", &gmail.Message{Subject: "Alert"}); err == nil {
		t.Error("expected error when account resolver fails")
	}

	if len(importer.calls) != 0 {
		t.Error("Import should not be called when account resolver errors")
	}
}

func TestHandle_ImportError_DoesNotPanic(t *testing.T) {
	extractor := &mockExtractor{result: &llm.TransactionData{
		Institution: "ICICI Bank", LastFour: "5678",
		AmountPaise: 75000, Merchant: "Swiggy", Date: "2026-05-25", Direction: "debit",
	}}
	resolver := &mockAccountResolver{accountID: "some-id"}
	importer := &mockImporter{err: errors.New("import failed")}

	h := newHandler(extractor, resolver, importer)

	if err := h.Handle(context.Background(), "user@gmail.com", &gmail.Message{Subject: "ICICI Alert"}); err == nil {
		t.Error("expected error when import fails")
	}
}
