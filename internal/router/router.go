package router

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/llm"
	"github.com/pushkar-anand/ap-5/internal/review"
)

// Handler processes emails of a specific type.
type Handler interface {
	Handle(ctx context.Context, email string, msg *gmail.Message) error
}

// Queuer receives emails that have no registered handler, for later user review.
type Queuer interface {
	Add(item review.Item) error
}

// Router classifies incoming emails and dispatches to registered handlers.
type Router struct {
	log      *slog.Logger
	llm      llm.Classifier
	mu       sync.RWMutex
	handlers map[string]Handler
	queuer   Queuer
}

func New(log *slog.Logger, llmClient llm.Classifier) *Router {
	return &Router{
		log:      log,
		llm:      llmClient,
		handlers: make(map[string]Handler),
	}
}

// SetQueuer attaches a review queue. Unhandled emails are added to it instead of being dropped.
func (r *Router) SetQueuer(q Queuer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queuer = q
}

// Register adds a handler for the given email type category. Safe to call after startup.
func (r *Router) Register(emailType string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[emailType] = h
}

// RegisteredTypes returns a snapshot of all currently registered handler category names.
func (r *Router) RegisteredTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}

// HandleDirect invokes the handler for the given category directly, bypassing classification.
// Used by the review UI to reprocess a queued email with a specific (possibly corrected) handler.
func (r *Router) HandleDirect(ctx context.Context, category, email string, msg *gmail.Message) error {
	r.mu.RLock()
	h, ok := r.handlers[category]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("router: no handler registered for %q", category)
	}
	return h.Handle(ctx, email, msg)
}

// Route classifies the email and dispatches it to the matching handler.
// Emails with no registered handler are added to the review queue (if one is set).
func (r *Router) Route(ctx context.Context, email string, msg *gmail.Message) error {
	emailType, err := r.llm.Classify(ctx, msg.Subject, msg.Body)
	if err != nil {
		r.log.ErrorContext(ctx, "failed to classify email",
			slog.String("account", email),
			slog.String("subject", msg.Subject),
			slog.Any("error", err),
		)
		return err
	}

	r.log.DebugContext(ctx, "classified email",
		slog.String("account", email),
		slog.String("subject", msg.Subject),
		slog.String("type", emailType),
	)

	r.mu.RLock()
	h, ok := r.handlers[emailType]
	q := r.queuer
	r.mu.RUnlock()

	if !ok {
		r.log.InfoContext(ctx, "no handler for email type, queuing for review",
			slog.String("account", email),
			slog.String("subject", msg.Subject),
			slog.String("type", emailType),
		)
		if q != nil {
			if err := q.Add(review.Item{
				ID:            msg.ID,
				Account:       email,
				Subject:       msg.Subject,
				Body:          msg.Body,
				BodyHTML:      msg.BodyHTML,
				SuggestedType: emailType,
			}); err != nil {
				r.log.ErrorContext(ctx, "failed to queue email for review — email will not appear in review UI",
					slog.String("account", email),
					slog.String("subject", msg.Subject),
					slog.Any("error", err),
				)
			}
		}
		return nil
	}

	r.log.InfoContext(ctx, "routing email",
		slog.String("account", email),
		slog.String("type", emailType),
		slog.String("subject", msg.Subject),
	)

	return h.Handle(ctx, email, msg)
}
