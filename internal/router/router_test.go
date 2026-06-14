package router_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/review"
	"github.com/pushkar-anand/ap-5/internal/router"
)

type mockClassifier struct {
	result string
	err    error
}

func (m *mockClassifier) Classify(_ context.Context, _, _ string) (string, error) {
	return m.result, m.err
}

type mockHandler struct {
	calls []*gmail.Message
}

func (m *mockHandler) Handle(_ context.Context, _ string, msg *gmail.Message) error {
	m.calls = append(m.calls, msg)
	return nil
}

type mockQueuer struct {
	items []review.Item
}

func (m *mockQueuer) Add(item review.Item) error {
	m.items = append(m.items, item)
	return nil
}

func newRouter(classifier *mockClassifier) *router.Router {
	return router.New(slog.Default(), classifier)
}

func TestRoute_DispatchesToRegisteredHandler(t *testing.T) {
	classifier := &mockClassifier{result: "credit_card_transaction"}
	h := &mockHandler{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)

	msg := &gmail.Message{ID: "1", Subject: "HDFC Alert", Body: "Spent Rs 500"}
	r.Route(context.Background(), "user@gmail.com", msg)

	if len(h.calls) != 1 {
		t.Errorf("handler called %d times, want 1", len(h.calls))
	}
	if h.calls[0].ID != "1" {
		t.Errorf("wrong message delivered: %+v", h.calls[0])
	}
}

func TestRoute_UnknownType_QueuesToReviewQueue(t *testing.T) {
	classifier := &mockClassifier{result: "other"}
	h := &mockHandler{}
	q := &mockQueuer{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)
	r.SetQueuer(q)

	msg := &gmail.Message{ID: "msg42", Subject: "Newsletter", Body: "Check our offers"}
	r.Route(context.Background(), "user@gmail.com", msg)

	if len(h.calls) != 0 {
		t.Errorf("handler should not be called for unhandled type, called %d times", len(h.calls))
	}
	if len(q.items) != 1 {
		t.Fatalf("queuer should have 1 item, got %d", len(q.items))
	}
	if q.items[0].ID != "msg42" {
		t.Errorf("queued item ID = %q, want msg42", q.items[0].ID)
	}
	if q.items[0].SuggestedType != "other" {
		t.Errorf("queued item SuggestedType = %q, want other", q.items[0].SuggestedType)
	}
}

func TestRoute_UnknownType_NoQueuer_Drops(t *testing.T) {
	classifier := &mockClassifier{result: "other"}
	h := &mockHandler{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)
	// No queuer set — should not panic.

	err := r.Route(context.Background(), "user@gmail.com", &gmail.Message{Subject: "Newsletter"})
	if err != nil {
		t.Errorf("Route should not error for unhandled type without queuer: %v", err)
	}
	if len(h.calls) != 0 {
		t.Errorf("handler should not be called for unhandled type, called %d times", len(h.calls))
	}
}

func TestRoute_ClassifierError_DropsMessage(t *testing.T) {
	classifier := &mockClassifier{err: errors.New("llm unavailable")}
	h := &mockHandler{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)

	err := r.Route(context.Background(), "user@gmail.com", &gmail.Message{Subject: "HDFC Alert"})

	if err == nil {
		t.Error("Route should return error when classifier fails")
	}
	if len(h.calls) != 0 {
		t.Errorf("handler should not be called on classifier error, called %d times", len(h.calls))
	}
}

func TestRoute_MultipleHandlers(t *testing.T) {
	classifier := &mockClassifier{result: "bill_reminder"}
	ccHandler := &mockHandler{}
	billHandler := &mockHandler{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", ccHandler)
	r.Register("bill_reminder", billHandler)

	r.Route(context.Background(), "user@gmail.com", &gmail.Message{Subject: "Bill due"})

	if len(ccHandler.calls) != 0 {
		t.Error("credit card handler should not be called")
	}
	if len(billHandler.calls) != 1 {
		t.Errorf("bill handler called %d times, want 1", len(billHandler.calls))
	}
}

func TestRoute_UnregisteredType_QueuesForReview(t *testing.T) {
	classifier := &mockClassifier{result: "unknown_type"}
	h := &mockHandler{}
	q := &mockQueuer{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)
	r.SetQueuer(q)

	r.Route(context.Background(), "user@gmail.com", &gmail.Message{ID: "u1", Subject: "Weird email"})

	if len(h.calls) != 0 {
		t.Errorf("handler should not be called for unregistered type, called %d times", len(h.calls))
	}
	if len(q.items) != 1 {
		t.Fatalf("expected 1 queued item, got %d", len(q.items))
	}
	if q.items[0].SuggestedType != "unknown_type" {
		t.Errorf("SuggestedType = %q, want unknown_type", q.items[0].SuggestedType)
	}
}

func TestRegisterAndRegisteredTypes(t *testing.T) {
	r := newRouter(&mockClassifier{result: "other"})
	r.Register("credit_card_transaction", &mockHandler{})
	r.Register("bank_account_transaction", &mockHandler{})

	types := r.RegisteredTypes()
	if len(types) != 2 {
		t.Fatalf("RegisteredTypes len = %d, want 2", len(types))
	}
}
