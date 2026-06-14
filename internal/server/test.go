package server

import (
	"net/http"
	"slices"

	"github.com/pushkar-anand/ap-5/internal/llm"
)

// testResult holds the dry-run output shown on the /test page.
type testResult struct {
	Subject   string
	Body      string
	Category  string
	Handled   bool
	BuiltIn   bool // true when handled by a built-in handler (no learned rule to extract with)
	Action    string
	Extracted *llm.TransactionData
	Error     string
}

func (s *Server) handleTestGet(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "test.html", nil)
}

func (s *Server) handleTestPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subject := r.FormValue("subject")
	body := r.FormValue("body")

	result := &testResult{Subject: subject, Body: body}

	// Step 1: Classify.
	category, err := s.tester.Classify(ctx, subject, body)
	if err != nil {
		result.Error = "Classification failed: " + err.Error()
		renderTemplate(w, "test.html", map[string]any{"Result": result})
		return
	}
	result.Category = category

	// Step 2: Check if a handler is registered.
	if s.router != nil {
		result.Handled = slices.Contains(s.router.RegisteredTypes(), category)
	}

	if !result.Handled {
		renderTemplate(w, "test.html", map[string]any{"Result": result})
		return
	}

	// Step 3: For learned rules, run extraction as a dry run.
	if s.ruleStore == nil {
		result.BuiltIn = true
		renderTemplate(w, "test.html", map[string]any{"Result": result})
		return
	}
	rule, isLearned := s.ruleStore.Get(category)
	if !isLearned {
		// Built-in handler (e.g. credit_card_transaction) — extraction is internal.
		result.BuiltIn = true
		renderTemplate(w, "test.html", map[string]any{"Result": result})
		return
	}

	result.Action = rule.Action
	extracted, err := s.tester.ExtractWithPrompt(ctx, rule.ExtractionPrompt, subject, body)
	if err != nil {
		result.Error = "Extraction failed: " + err.Error()
	} else {
		result.Extracted = extracted
	}

	renderTemplate(w, "test.html", map[string]any{"Result": result})
}
