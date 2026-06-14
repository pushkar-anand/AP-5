package learned_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/handlers/learned"
	"github.com/pushkar-anand/ap-5/internal/jn66"
	"github.com/pushkar-anand/ap-5/internal/llm"
	"github.com/pushkar-anand/ap-5/internal/rules"
)

type mockExtractor struct {
	result *llm.TransactionData
	err    error
	prompt string // last prompt received
}

func (m *mockExtractor) ExtractWithPrompt(_ context.Context, prompt, _, _ string) (*llm.TransactionData, error) {
	m.prompt = prompt
	return m.result, m.err
}

type mockAccountResolver struct {
	accountID string
	err       error
}

func (m *mockAccountResolver) LookupOrCreate(_ context.Context, _, _ string) (string, error) {
	return m.accountID, m.err
}

type mockImporter struct {
	result *jn66.ImportResult
	err    error
	calls  []string
}

func (m *mockImporter) Import(_ context.Context, accountID string, _ []jn66.ImportTransaction) (*jn66.ImportResult, error) {
	m.calls = append(m.calls, accountID)
	return m.result, m.err
}

func importRule() rules.Rule {
	return rules.Rule{
		Category:         "bank_account_transaction",
		ExtractionPrompt: "Extract bank transaction details.",
		Action:           rules.ActionImportTransaction,
	}
}

func newHandler(rule rules.Rule, ex *mockExtractor, res *mockAccountResolver, imp *mockImporter) *learned.Handler {
	return learned.New(slog.Default(), rule, ex, res, imp)
}

func TestHandle_ImportTransaction_Success(t *testing.T) {
	ex := &mockExtractor{result: &llm.TransactionData{
		Institution: "SBI",
		LastFour:    "9012",
		AmountPaise: 200000,
		Merchant:    "Grocery Store",
		Date:        "2026-06-01",
		Direction:   "debit",
	}}
	res := &mockAccountResolver{accountID: "sbi-account-id"}
	imp := &mockImporter{result: &jn66.ImportResult{Inserted: 1}}

	h := newHandler(importRule(), ex, res, imp)
	if err := h.Handle(context.Background(), "user@example.com", &gmail.Message{Subject: "SBI Alert", Body: "Rs 2000 debited"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ex.prompt != importRule().ExtractionPrompt {
		t.Errorf("extractor received prompt %q, want %q", ex.prompt, importRule().ExtractionPrompt)
	}
	if len(imp.calls) != 1 || imp.calls[0] != "sbi-account-id" {
		t.Errorf("Import called with %v, want [sbi-account-id]", imp.calls)
	}
}

func TestHandle_LogOnly_DoesNotExtractOrImport(t *testing.T) {
	ex := &mockExtractor{}
	imp := &mockImporter{}

	rule := rules.Rule{
		Category:         "newsletter",
		ExtractionPrompt: "irrelevant",
		Action:           rules.ActionLogOnly,
	}
	h := newHandler(rule, ex, &mockAccountResolver{}, imp)

	if err := h.Handle(context.Background(), "user@example.com", &gmail.Message{Subject: "Weekly digest"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ex.prompt != "" {
		t.Error("extractor should not be called for log_only action")
	}
	if len(imp.calls) != 0 {
		t.Error("importer should not be called for log_only action")
	}
}

func TestHandle_ExtractorReturnsNil_NoImport(t *testing.T) {
	ex := &mockExtractor{result: nil}
	imp := &mockImporter{}

	h := newHandler(importRule(), ex, &mockAccountResolver{}, imp)
	if err := h.Handle(context.Background(), "user@example.com", &gmail.Message{Subject: "OTP"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(imp.calls) != 0 {
		t.Error("importer should not be called when extractor returns nil")
	}
}

func TestHandle_ExtractorError_ReturnsError(t *testing.T) {
	ex := &mockExtractor{err: errors.New("llm down")}
	imp := &mockImporter{}

	h := newHandler(importRule(), ex, &mockAccountResolver{}, imp)
	if err := h.Handle(context.Background(), "user@example.com", &gmail.Message{Subject: "SBI Alert"}); err == nil {
		t.Error("expected error when extractor fails")
	}
	if len(imp.calls) != 0 {
		t.Error("importer should not be called when extractor errors")
	}
}

func TestHandle_AccountResolverError_NoImport(t *testing.T) {
	ex := &mockExtractor{result: &llm.TransactionData{Institution: "SBI", LastFour: "1234", AmountPaise: 100, Date: "2026-06-01", Direction: "debit"}}
	res := &mockAccountResolver{err: errors.New("jn66 down")}
	imp := &mockImporter{}

	h := newHandler(importRule(), ex, res, imp)
	if err := h.Handle(context.Background(), "user@example.com", &gmail.Message{Subject: "Alert"}); err == nil {
		t.Error("expected error when account resolver fails")
	}
	if len(imp.calls) != 0 {
		t.Error("importer should not be called when account resolver fails")
	}
}

func TestHandle_ImportError_ReturnsError(t *testing.T) {
	ex := &mockExtractor{result: &llm.TransactionData{Institution: "SBI", LastFour: "1234", AmountPaise: 100, Date: "2026-06-01", Direction: "debit"}}
	res := &mockAccountResolver{accountID: "acct"}
	imp := &mockImporter{err: errors.New("import failed")}

	h := newHandler(importRule(), ex, res, imp)
	if err := h.Handle(context.Background(), "user@example.com", &gmail.Message{Subject: "Alert"}); err == nil {
		t.Error("expected error when import fails")
	}
}
