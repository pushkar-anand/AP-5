package learned

import (
	"context"
	"log/slog"

	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/jn66"
	"github.com/pushkar-anand/ap-5/internal/llm"
	"github.com/pushkar-anand/ap-5/internal/rules"
)

type extractor interface {
	ExtractWithPrompt(ctx context.Context, prompt, subject, body string) (*llm.TransactionData, error)
}

type accountResolver interface {
	LookupOrCreate(ctx context.Context, institution, lastFour string) (string, error)
}

type transactionImporter interface {
	Import(ctx context.Context, accountID string, txns []jn66.ImportTransaction) (*jn66.ImportResult, error)
}

// Handler processes emails using a user-taught extraction rule.
type Handler struct {
	log      *slog.Logger
	rule     rules.Rule
	llm      extractor
	accounts accountResolver
	jn66     transactionImporter
}

func New(log *slog.Logger, rule rules.Rule, llmClient extractor, accounts accountResolver, jn66Client transactionImporter) *Handler {
	return &Handler{
		log:      log,
		rule:     rule,
		llm:      llmClient,
		accounts: accounts,
		jn66:     jn66Client,
	}
}

func (h *Handler) Handle(ctx context.Context, email string, msg *gmail.Message) error {
	log := h.log.With(
		slog.String("account", email),
		slog.String("subject", msg.Subject),
		slog.String("category", h.rule.Category),
	)

	if h.rule.Action == rules.ActionLogOnly {
		log.InfoContext(ctx, "learned handler: log-only action, email noted")
		return nil
	}

	txn, err := h.llm.ExtractWithPrompt(ctx, h.rule.ExtractionPrompt, msg.Subject, msg.Body)
	if err != nil {
		log.ErrorContext(ctx, "learned handler: extraction failed", slog.Any("error", err))
		return err
	}
	if txn == nil {
		log.InfoContext(ctx, "learned handler: no data extracted from email")
		return nil
	}

	log.DebugContext(ctx, "learned handler: extracted data",
		slog.String("institution", txn.Institution),
		slog.String("last_four", txn.LastFour),
		slog.String("merchant", txn.Merchant),
		slog.String("date", txn.Date),
		slog.String("direction", txn.Direction),
		slog.Int64("amount_paise", txn.AmountPaise),
	)

	accountID, err := h.accounts.LookupOrCreate(ctx, txn.Institution, txn.LastFour)
	if err != nil {
		log.ErrorContext(ctx, "learned handler: failed to resolve JN-66 account",
			slog.String("institution", txn.Institution),
			slog.String("last_four", txn.LastFour),
			slog.Any("error", err),
		)
		return err
	}

	result, err := h.jn66.Import(ctx, accountID, []jn66.ImportTransaction{
		{
			Date:        txn.Date,
			Description: txn.Merchant,
			AmountPaise: txn.AmountPaise,
			Direction:   txn.Direction,
		},
	})
	if err != nil {
		log.ErrorContext(ctx, "learned handler: failed to import to JN-66", slog.Any("error", err))
		return err
	}

	log.InfoContext(ctx, "learned handler: transaction recorded",
		slog.String("institution", txn.Institution),
		slog.String("last_four", txn.LastFour),
		slog.String("merchant", txn.Merchant),
		slog.Int64("amount_paise", txn.AmountPaise),
		slog.Int("inserted", result.Inserted),
		slog.Int("duplicate", result.Duplicate),
	)
	return nil
}
