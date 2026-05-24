package gmail

import (
	"testing"

	"google.golang.org/api/gmail/v1"
)

func TestDecodeBase64URL(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
	}{
		{
			name:  "standard base64url encoded",
			input: "SGVsbG8gV29ybGQ=",
			want:  "Hello World",
		},
		{
			name:  "raw base64url no padding",
			input: "SGVsbG8gV29ybGQ",
			want:  "Hello World",
		},
		{
			name:  "invalid returns input unchanged",
			input: "not-valid-base64!!!",
			want:  "not-valid-base64!!!",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeBase64URL(tc.input)
			if got != tc.want {
				t.Errorf("decodeBase64URL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestHeaderValue(t *testing.T) {
	headers := []*gmail.MessagePartHeader{
		{Name: "Subject", Value: "Hello"},
		{Name: "From", Value: "sender@example.com"},
		{Name: "Content-Type", Value: "text/plain"},
	}

	cases := []struct {
		name string
		want string
	}{
		{"Subject", "Hello"},
		{"subject", "Hello"},   // case-insensitive
		{"SUBJECT", "Hello"},
		{"From", "sender@example.com"},
		{"Missing", ""},
	}

	for _, tc := range cases {
		got := headerValue(headers, tc.name)
		if got != tc.want {
			t.Errorf("headerValue(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestExtractBody_PlainTextPart(t *testing.T) {
	// Base64URL encode "plain text body"
	encoded := "cGxhaW4gdGV4dCBib2R5"

	payload := &gmail.MessagePart{
		MimeType: "multipart/alternative",
		Parts: []*gmail.MessagePart{
			{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: encoded, Size: 15},
			},
			{
				MimeType: "text/html",
				Body:     &gmail.MessagePartBody{Data: "PGh0bWw+PC9odG1sPg==", Size: 13},
			},
		},
	}

	got := extractBody(payload)
	if got != "plain text body" {
		t.Errorf("extractBody = %q, want %q", got, "plain text body")
	}
}

func TestExtractBody_TopLevelBody(t *testing.T) {
	encoded := "aGVsbG8=" // "hello"

	payload := &gmail.MessagePart{
		MimeType: "text/plain",
		Body:     &gmail.MessagePartBody{Data: encoded, Size: 5},
	}

	got := extractBody(payload)
	if got != "hello" {
		t.Errorf("extractBody = %q, want %q", got, "hello")
	}
}

func TestExtractBody_NilPayload(t *testing.T) {
	got := extractBody(nil)
	if got != "" {
		t.Errorf("extractBody(nil) = %q, want empty", got)
	}
}

func TestExtractBody_EmptyBody(t *testing.T) {
	payload := &gmail.MessagePart{
		MimeType: "text/plain",
		Body:     &gmail.MessagePartBody{Size: 0},
	}
	got := extractBody(payload)
	if got != "" {
		t.Errorf("extractBody with empty body = %q, want empty", got)
	}
}

func TestExtractBody_Recursive(t *testing.T) {
	encoded := "bmVzdGVk" // "nested"

	payload := &gmail.MessagePart{
		MimeType: "multipart/mixed",
		Parts: []*gmail.MessagePart{
			{
				MimeType: "multipart/alternative",
				Parts: []*gmail.MessagePart{
					{
						MimeType: "text/plain",
						Body:     &gmail.MessagePartBody{Data: encoded, Size: 6},
					},
				},
			},
		},
	}

	got := extractBody(payload)
	if got != "nested" {
		t.Errorf("extractBody (recursive) = %q, want %q", got, "nested")
	}
}
