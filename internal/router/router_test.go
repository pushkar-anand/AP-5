package router_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/pushkar-anand/ap-5/internal/gmail"
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

func (m *mockHandler) Handle(_ context.Context, _ string, msg *gmail.Message) {
	m.calls = append(m.calls, msg)
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

func TestRoute_UnknownTypeIsDropped(t *testing.T) {
	classifier := &mockClassifier{result: "other"}
	h := &mockHandler{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)

	r.Route(context.Background(), "user@gmail.com", &gmail.Message{Subject: "Newsletter"})

	if len(h.calls) != 0 {
		t.Errorf("handler should not be called for 'other' type, called %d times", len(h.calls))
	}
}

func TestRoute_ClassifierError_DropsMessage(t *testing.T) {
	classifier := &mockClassifier{err: errors.New("llm unavailable")}
	h := &mockHandler{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)

	r.Route(context.Background(), "user@gmail.com", &gmail.Message{Subject: "HDFC Alert"})

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

func TestRoute_UnregisteredType_NoHandlerCalled(t *testing.T) {
	classifier := &mockClassifier{result: "unknown_type"}
	h := &mockHandler{}

	r := newRouter(classifier)
	r.Register("credit_card_transaction", h)

	// Should not panic or call any handler
	r.Route(context.Background(), "user@gmail.com", &gmail.Message{Subject: "Weird email"})

	if len(h.calls) != 0 {
		t.Errorf("handler should not be called for unregistered type, called %d times", len(h.calls))
	}
}
