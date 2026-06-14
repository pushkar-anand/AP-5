package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/sashabaranov/go-openai"
)

// Classifier classifies an email into a known type string.
type Classifier interface {
	Classify(ctx context.Context, subject, body string) (string, error)
}

// Extractor extracts structured transaction data from an email.
type Extractor interface {
	ExtractTransaction(ctx context.Context, subject, body string) (*TransactionData, error)
}

type Client struct {
	log            *slog.Logger
	router         *openai.Client
	extractor      *openai.Client
	routerModel    string
	extractorModel string
}

func New(log *slog.Logger, baseURL, routerModel, extractorModel string) *Client {
	return NewWithHTTPClient(log, baseURL, routerModel, extractorModel, nil)
}

// NewWithHTTPClient creates a Client with a custom HTTP client — used in tests.
func NewWithHTTPClient(log *slog.Logger, baseURL, routerModel, extractorModel string, httpClient *http.Client) *Client {
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = baseURL
	if httpClient != nil {
		cfg.HTTPClient = httpClient
	}

	c := openai.NewClientWithConfig(cfg)

	return &Client{
		log:            log,
		router:         c,
		extractor:      c,
		routerModel:    routerModel,
		extractorModel: extractorModel,
	}
}

// Classify asks the router model to classify an email into a known type.
// Returns the type string (e.g. "credit_card_transaction") or "other".
func (c *Client) Classify(ctx context.Context, subject, body string) (string, error) {
	if len(body) > 500 {
		body = body[:500]
	}

	prompt := fmt.Sprintf(`Classify this email into exactly one category. Return JSON: {"type": "<category>"}
Categories: credit_card_transaction, other

Subject: %s
Body: %s`, subject, body)

	resp, err := c.router.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.routerModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return "", fmt.Errorf("llm: classify: %w", err)
	}

	content := resp.Choices[0].Message.Content
	c.log.DebugContext(ctx, "llm classify response", slog.String("subject", subject), slog.String("raw", content))

	var result struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "other", nil
	}

	if result.Type == "" {
		return "other", nil
	}
	return result.Type, nil
}

// ExtractTransaction asks the extractor model to parse a credit card transaction email.
// Returns nil if the email does not contain a recognisable transaction.
func (c *Client) ExtractTransaction(ctx context.Context, subject, body string) (*TransactionData, error) {
	prompt := fmt.Sprintf(`Extract credit card transaction details from this email. Return JSON with these fields:
- institution: bank name (e.g. "HDFC Bank")
- last_four: last 4 digits of the card as a string
- amount_paise: transaction amount in paise as an integer (e.g. 125000 for Rs. 1250.00)
- merchant: merchant or payee name
- date: transaction date in YYYY-MM-DD format
- direction: "debit" or "credit"

If you cannot extract all required fields, return {"error": "reason"}.

Subject: %s
Body: %s`, subject, body)

	resp, err := c.extractor.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.extractorModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("llm: extract transaction: %w", err)
	}

	content := resp.Choices[0].Message.Content
	c.log.DebugContext(ctx, "llm extract response", slog.String("subject", subject), slog.String("raw", content))

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(content), &errResp); err == nil && errResp.Error != "" {
		c.log.DebugContext(ctx, "llm could not extract transaction", slog.String("subject", subject), slog.String("reason", errResp.Error))
		return nil, nil
	}

	var data TransactionData
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("llm: parse transaction response: %w", err)
	}

	return &data, nil
}

type TransactionData struct {
	Institution string `json:"institution"`
	LastFour    string `json:"last_four"`
	AmountPaise int64  `json:"amount_paise"`
	Merchant    string `json:"merchant"`
	Date        string `json:"date"` // YYYY-MM-DD
	Direction   string `json:"direction"`
}
