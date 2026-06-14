package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/pushkar-anand/build-with-go/http/middleware"
	bwgserver "github.com/pushkar-anand/build-with-go/http/server"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"
)

// OAuthExchanger handles exchanging an OAuth code for a token for a given account.
type OAuthExchanger interface {
	Exchange(ctx context.Context, email, code string) error
}

// Server is the AP-5 HTTP server handling health checks and OAuth callbacks.
type Server struct {
	log   *slog.Logger
	port  int
	oauth OAuthExchanger
}

func New(log *slog.Logger, port int, oauth OAuthExchanger) *Server {
	return &Server{log: log, port: port, oauth: oauth}
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
