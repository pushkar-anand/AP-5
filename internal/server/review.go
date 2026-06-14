package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/review"
	"github.com/pushkar-anand/ap-5/internal/rules"
)

func (s *Server) handleReviewList(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "review_list.html", map[string]any{
		"Items": s.reviewQueue.List(),
	})
}

func (s *Server) handleReviewDetail(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	item, ok := s.reviewQueue.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	renderTemplate(w, "review_detail.html", map[string]any{
		"Item":            item,
		"RegisteredTypes": s.router.RegisteredTypes(),
	})
}

func (s *Server) handleReviewReprocess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := mux.Vars(r)["id"]

	item, ok := s.reviewQueue.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	category := r.FormValue("category")
	if category == "" {
		http.Error(w, "category required", http.StatusBadRequest)
		return
	}

	msg := &gmail.Message{ID: item.ID, Subject: item.Subject, Body: item.Body}
	if err := s.router.HandleDirect(ctx, category, item.Account, msg); err != nil {
		s.log.ErrorContext(ctx, "reprocess failed",
			"id", id, "category", category, "error", err)
		renderTemplate(w, "review_detail.html", map[string]any{
			"Item":            item,
			"RegisteredTypes": s.router.RegisteredTypes(),
			"Flash":           "Reprocess failed: " + err.Error(),
			"FlashError":      true,
		})
		return
	}

	_ = s.reviewQueue.Remove(id)
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

func (s *Server) handleReviewTeach(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := mux.Vars(r)["id"]

	item, ok := s.reviewQueue.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	category := r.FormValue("category")
	extractionPrompt := r.FormValue("extraction_prompt")
	action := r.FormValue("action")

	if category == "" || extractionPrompt == "" || action == "" {
		http.Error(w, "category, extraction_prompt, and action are required", http.StatusBadRequest)
		return
	}

	rule := rules.Rule{
		Category:         category,
		ExtractionPrompt: extractionPrompt,
		Action:           action,
	}

	if err := s.ruleStore.Save(rule); err != nil {
		http.Error(w, "failed to save rule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Register the new handler immediately and add the category to the classifier.
	s.registerLearnedRule(rule)

	// Immediately process the queued email with the new handler.
	msg := &gmail.Message{ID: item.ID, Subject: item.Subject, Body: item.Body}
	if err := s.router.HandleDirect(ctx, category, item.Account, msg); err != nil {
		s.log.ErrorContext(ctx, "post-teach processing failed",
			"id", id, "category", category, "error", err)
		// Rule was saved; just warn — don't block the user.
		renderTemplate(w, "review_detail.html", map[string]any{
			"Item":            item,
			"RegisteredTypes": s.router.RegisteredTypes(),
			"Flash":           "Rule saved, but processing this email failed: " + err.Error(),
			"FlashError":      true,
		})
		return
	}

	_ = s.reviewQueue.Remove(id)
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

func (s *Server) handleReviewIgnore(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	_ = s.reviewQueue.Remove(id)
	http.Redirect(w, r, "/review", http.StatusSeeOther)
}

func (s *Server) handleRulesList(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "rules_list.html", map[string]any{
		"Rules": s.ruleStore.List(),
	})
}

func (s *Server) handleRulesDelete(w http.ResponseWriter, r *http.Request) {
	category := mux.Vars(r)["category"]
	_ = s.ruleStore.Delete(category)
	http.Redirect(w, r, "/rules", http.StatusSeeOther)
}

// ReviewQueue is the interface the server uses to manage the email review queue.
type ReviewQueue interface {
	Add(item review.Item) error
	List() []review.Item
	Get(id string) (review.Item, bool)
	Remove(id string) error
}

// RuleStore is the interface the server uses to manage learned rules.
type RuleStore interface {
	Save(rule rules.Rule) error
	List() []rules.Rule
	Get(category string) (rules.Rule, bool)
	Delete(category string) error
}

// HandlerRouter is the router interface the server needs for reprocess + teach flows.
type HandlerRouter interface {
	RegisteredTypes() []string
	HandleDirect(ctx context.Context, category, email string, msg *gmail.Message) error
}

// RuleRegistrar is called by the server when a new rule is taught via the UI.
// Implementations should register the learned handler and add the category to the LLM classifier.
type RuleRegistrar func(rule rules.Rule)
