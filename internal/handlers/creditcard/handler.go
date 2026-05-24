package creditcard

import (
	"context"
	"log/slog"

	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/jn66"
	"github.com/pushkar-anand/ap-5/internal/llm"
)

const EmailType = "credit_card_transaction"

type accountResolver interface {
	LookupOrCreate(ctx context.Context, institution, lastFour string) (string, error)
}

type transactionImporter interface {
	Import(ctx context.Context, accountID string, txns []jn66.ImportTransaction) (*jn66.ImportResult, error)
}

// Handler processes credit card transaction emails and records them in JN-66.
type Handler struct {
	log      *slog.Logger
	llm      llm.Extractor
	accounts accountResolver
	jn66     transactionImporter
}

func New(log *slog.Logger, llmClient llm.Extractor, accounts accountResolver, jn66Client transactionImporter) *Handler {
	return &Handler{
		log:      log,
		llm:      llmClient,
		accounts: accounts,
		jn66:     jn66Client,
	}
}

func (h *Handler) Handle(ctx context.Context, email string, msg *gmail.Message) {
	log := h.log.With(slog.String("account", email), slog.String("subject", msg.Subject))

	txn, err := h.llm.ExtractTransaction(ctx, msg.Subject, msg.Body)
	if err != nil {
		log.ErrorContext(ctx, "failed to extract transaction", slog.Any("error", err))
		return
	}
	if txn == nil {
		log.InfoContext(ctx, "no transaction found in email")
		return
	}

	accountID, err := h.accounts.LookupOrCreate(ctx, txn.Institution, txn.LastFour)
	if err != nil {
		log.ErrorContext(ctx, "failed to resolve JN-66 account",
			slog.String("institution", txn.Institution),
			slog.String("last_four", txn.LastFour),
			slog.Any("error", err),
		)
		return
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
		log.ErrorContext(ctx, "failed to import transaction to JN-66", slog.Any("error", err))
		return
	}

	log.InfoContext(ctx, "transaction recorded",
		slog.String("institution", txn.Institution),
		slog.String("last_four", txn.LastFour),
		slog.String("merchant", txn.Merchant),
		slog.Int64("amount_paise", txn.AmountPaise),
		slog.Int("inserted", result.Inserted),
		slog.Int("duplicate", result.Duplicate),
	)
}
