package server

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/pushkar-anand/ap-5/internal/llm"
	"github.com/pushkar-anand/build-with-go/http/middleware"
	bwgserver "github.com/pushkar-anand/build-with-go/http/server"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"
)

//go:embed templates
var templateFS embed.FS

// baseTmpl holds only base.html. Each renderTemplate call clones it and parses
// exactly one page template into the clone — preventing {{define "content"}} blocks
// from different pages from overwriting each other in a shared template set.
var baseTmpl = template.Must(
	template.New("base.html").Funcs(template.FuncMap{
		"rupees": func(paise int64) string {
			return fmt.Sprintf("₹%d.%02d", paise/100, paise%100)
		},
	}).ParseFS(templateFS, "templates/base.html"),
)

func renderTemplate(w http.ResponseWriter, name string, data any) {
	t, err := baseTmpl.Clone()
	if err != nil {
		http.Error(w, "template clone error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if t, err = t.ParseFS(templateFS, "templates/"+name); err != nil {
		http.Error(w, "template parse error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}

// OAuthExchanger handles exchanging an OAuth code for a token for a given account.
type OAuthExchanger interface {
	Exchange(ctx context.Context, email, code string) error
}

// PromptSuggester generates a suggested extraction prompt for an email.
type PromptSuggester interface {
	SuggestExtractionPrompt(ctx context.Context, subject, body string) (string, error)
}

// Tester runs the classify + extract pipeline in dry-run mode (no JN-66 import).
type Tester interface {
	Classify(ctx context.Context, subject, body string) (string, error)
	ExtractWithPrompt(ctx context.Context, prompt, subject, body string) (*llm.TransactionData, error)
}

// Server is the AP-5 HTTP server handling health checks, OAuth callbacks, and the review UI.
type Server struct {
	log                 *slog.Logger
	port                int
	oauth               OAuthExchanger
	reviewQueue         ReviewQueue
	ruleStore           RuleStore
	router              HandlerRouter
	registerLearnedRule RuleRegistrar
	suggester           PromptSuggester
	tester              Tester
}

func New(log *slog.Logger, port int, oauth OAuthExchanger) *Server {
	return &Server{log: log, port: port, oauth: oauth}
}

// WithReview attaches the review queue, rule store, router, and rule-registration callback.
func (s *Server) WithReview(q ReviewQueue, rs RuleStore, r HandlerRouter, reg RuleRegistrar) {
	s.reviewQueue = q
	s.ruleStore = rs
	s.router = r
	s.registerLearnedRule = reg
}

// WithLLM attaches the LLM pipeline for prompt suggestion and dry-run testing.
func (s *Server) WithLLM(suggester PromptSuggester, tester Tester) {
	s.suggester = suggester
	s.tester = tester
}

// NewRouter builds the mux with all routes registered — extracted for testability.
func NewRouter(oauth OAuthExchanger) http.Handler {
	s := &Server{log: slog.Default(), oauth: oauth}

	r := mux.NewRouter()
	r.HandleFunc("/healthz", s.handleHealth).Methods(http.MethodGet)
	r.HandleFunc("/auth/callback", s.handleOAuthCallback).Methods(http.MethodGet)
	return r
}

func (s *Server) Serve(ctx context.Context) error {
	r := mux.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(bwglogger.NewHTTPLogger(s.log))

	r.HandleFunc("/healthz", s.handleHealth).Methods(http.MethodGet)
	r.HandleFunc("/auth/callback", s.handleOAuthCallback).Methods(http.MethodGet)

	if s.reviewQueue != nil {
		r.HandleFunc("/review", s.handleReviewList).Methods(http.MethodGet)
		r.HandleFunc("/review/{id}", s.handleReviewDetail).Methods(http.MethodGet)
		r.HandleFunc("/review/{id}/suggest-prompt", s.handleSuggestPrompt).Methods(http.MethodGet)
		r.HandleFunc("/review/{id}/reprocess", s.handleReviewReprocess).Methods(http.MethodPost)
		r.HandleFunc("/review/{id}/teach", s.handleReviewTeach).Methods(http.MethodPost)
		r.HandleFunc("/review/{id}/ignore", s.handleReviewIgnore).Methods(http.MethodPost)
		r.HandleFunc("/rules", s.handleRulesList).Methods(http.MethodGet)
		r.HandleFunc("/rules/{category}/delete", s.handleRulesDelete).Methods(http.MethodPost)
	}
	if s.tester != nil {
		r.HandleFunc("/test", s.handleTestGet).Methods(http.MethodGet)
		r.HandleFunc("/test", s.handleTestPost).Methods(http.MethodPost)
	}

	srv := bwgserver.New(
		r,
		bwgserver.WithHostPort("0.0.0.0", s.port),
		bwgserver.WithLogger(s.log),
		bwgserver.WithReadTimeout(30*time.Second),
		bwgserver.WithWriteTimeout(30*time.Second),
	)

	return srv.Serve(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, `{"status":"ok"}`)
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.URL.Query().Get("code")
	email := r.URL.Query().Get("state") // state = email address set in AuthURL

	if code == "" || email == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	if err := s.oauth.Exchange(ctx, email, code); err != nil {
		s.log.ErrorContext(ctx, "OAuth exchange failed",
			slog.String("email", email),
			slog.Any("error", err),
		)
		http.Error(w, "Authorization failed. Check server logs.", http.StatusInternalServerError)
		return
	}

	s.log.InfoContext(ctx, "OAuth token stored", slog.String("email", email))
	fmt.Fprintf(w, "Authorization successful for %s. You can close this tab.", email)
}
