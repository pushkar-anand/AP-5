package router

import (
	"context"
	"log/slog"

	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/llm"
)

// Handler processes emails of a specific type.
type Handler interface {
	Handle(ctx context.Context, email string, msg *gmail.Message) error
}

// Router classifies incoming emails and dispatches to registered handlers.
type Router struct {
	log      *slog.Logger
	llm      llm.Classifier
	handlers map[string]Handler
}

func New(log *slog.Logger, llmClient llm.Classifier) *Router {
	return &Router{
		log:      log,
		llm:      llmClient,
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler for the given email type category.
func (r *Router) Register(emailType string, h Handler) {
	r.handlers[emailType] = h
}

// Route classifies the email and dispatches it to the matching handler.
// Unknown or unhandled types are silently dropped (nil returned).
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

	h, ok := r.handlers[emailType]
	if !ok {
		r.log.DebugContext(ctx, "no handler for type, dropping",
			slog.String("account", email),
			slog.String("subject", msg.Subject),
			slog.String("type", emailType),
		)
		return nil
	}

	r.log.InfoContext(ctx, "routing email",
		slog.String("account", email),
		slog.String("type", emailType),
		slog.String("subject", msg.Subject),
	)

	return h.Handle(ctx, email, msg)
}
