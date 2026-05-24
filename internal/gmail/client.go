package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Message is a simplified representation of a Gmail message.
type Message struct {
	ID      string
	Subject string
	Body    string
}

// Client wraps the Gmail API service for a single account.
type Client struct {
	email   string
	service *gmail.Service
}

func NewClient(ctx context.Context, email string, ts oauth2.TokenSource) (*Client, error) {
	svc, err := gmail.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("gmail: new service for %s: %w", email, err)
	}
	return &Client{email: email, service: svc}, nil
}

// HistoryID returns the current historyId for the account.
func (c *Client) HistoryID(ctx context.Context) (uint64, error) {
	profile, err := c.service.Users.GetProfile(c.email).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("gmail: get profile for %s: %w", c.email, err)
	}
	return uint64(profile.HistoryId), nil
}

// NewMessageIDs returns the IDs of messages added since startHistoryID.
func (c *Client) NewMessageIDs(ctx context.Context, startHistoryID uint64) ([]string, uint64, error) {
	var ids []string
	var latestHistoryID uint64

	err := c.service.Users.History.List(c.email).
		StartHistoryId(startHistoryID).
		HistoryTypes("messageAdded").
		Context(ctx).
		Pages(ctx, func(page *gmail.ListHistoryResponse) error {
			if page.HistoryId > latestHistoryID {
				latestHistoryID = page.HistoryId
			}
			for _, h := range page.History {
				for _, m := range h.MessagesAdded {
					ids = append(ids, m.Message.Id)
				}
			}
			return nil
		})
	if err != nil {
		return nil, 0, fmt.Errorf("gmail: list history for %s: %w", c.email, err)
	}

	return ids, latestHistoryID, nil
}

// GetMessage fetches the subject and plain-text body of a message.
func (c *Client) GetMessage(ctx context.Context, id string) (*Message, error) {
	msg, err := c.service.Users.Messages.Get(c.email, id).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("gmail: get message %s: %w", id, err)
	}

	subject := headerValue(msg.Payload.Headers, "Subject")
	body := extractBody(msg.Payload)

	return &Message{ID: id, Subject: subject, Body: body}, nil
}

func headerValue(headers []*gmail.MessagePartHeader, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func extractBody(payload *gmail.MessagePart) string {
	if payload == nil {
		return ""
	}

	// Prefer text/plain parts.
	for _, part := range payload.Parts {
		if part.MimeType == "text/plain" && part.Body != nil && part.Body.Size > 0 {
			return decodeBase64URL(part.Body.Data)
		}
	}

	// Fall back to body of the top-level part.
	if payload.Body != nil && payload.Body.Size > 0 {
		return decodeBase64URL(payload.Body.Data)
	}

	// Recurse into multipart.
	for _, part := range payload.Parts {
		if text := extractBody(part); text != "" {
			return text
		}
	}

	return ""
}

func decodeBase64URL(data string) string {
	b, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		// Try RawURLEncoding (no padding)
		b, err = base64.RawURLEncoding.DecodeString(data)
		if err != nil {
			return data
		}
	}
	return string(b)
}
