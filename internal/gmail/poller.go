package gmail

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/pushkar-anand/ap-5/internal/state"
)

// MessageHandler is called for each new email message received.
// Returning a non-nil error prevents the history cursor from advancing, causing re-delivery on the next poll.
type MessageHandler func(ctx context.Context, email string, msg *Message) error

// Poller polls Gmail for new messages using the history API.
type Poller struct {
	log      *slog.Logger
	email    string
	client   *Client
	state    *state.Store
	handler  MessageHandler
	interval time.Duration
}

func NewPoller(
	log *slog.Logger,
	email string,
	client *Client,
	state *state.Store,
	handler MessageHandler,
	interval time.Duration,
) *Poller {
	return &Poller{
		log:      log.With(slog.String("account", email)),
		email:    email,
		client:   client,
		state:    state,
		handler:  handler,
		interval: interval,
	}
}

// Poll starts the polling loop. It blocks until ctx is cancelled.
func (p *Poller) Poll(ctx context.Context) {
	if err := p.ensureHistoryID(ctx); err != nil {
		p.log.ErrorContext(ctx, "failed to initialise history ID", slog.Any("error", err))
		return
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.log.InfoContext(ctx, "starting poll loop", slog.Duration("interval", p.interval))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	savedID := p.state.GetHistoryID(p.email)
	if savedID == "" {
		p.log.WarnContext(ctx, "no history ID saved, skipping poll")
		return
	}

	startID, err := strconv.ParseUint(savedID, 10, 64)
	if err != nil {
		p.log.ErrorContext(ctx, "invalid saved history ID", slog.String("value", savedID), slog.Any("error", err))
		return
	}

	ids, latestID, err := p.client.NewMessageIDs(ctx, startID)
	if err != nil {
		p.log.ErrorContext(ctx, "failed to list new messages", slog.Any("error", err))
		return
	}

	p.log.DebugContext(ctx, "poll complete", slog.Int("new_messages", len(ids)))

	allOK := true
	for _, id := range ids {
		msg, err := p.client.GetMessage(ctx, id)
		if err != nil {
			p.log.ErrorContext(ctx, "failed to fetch message", slog.String("id", id), slog.Any("error", err))
			allOK = false
			continue
		}
		p.log.DebugContext(ctx, "processing message", slog.String("id", msg.ID), slog.String("subject", msg.Subject))
		if err := p.handler(ctx, p.email, msg); err != nil {
			p.log.DebugContext(ctx, "message handler failed", slog.String("id", msg.ID), slog.String("subject", msg.Subject), slog.Any("error", err))
			allOK = false
		}
	}

	if allOK && latestID > 0 {
		if err := p.state.SetHistoryID(p.email, fmt.Sprintf("%d", latestID)); err != nil {
			p.log.ErrorContext(ctx, "failed to save history ID", slog.Any("error", err))
		}
	}
}

func (p *Poller) ensureHistoryID(ctx context.Context) error {
	if p.state.GetHistoryID(p.email) != "" {
		return nil
	}

	historyID, err := p.client.HistoryID(ctx)
	if err != nil {
		return err
	}

	return p.state.SetHistoryID(p.email, fmt.Sprintf("%d", historyID))
}
