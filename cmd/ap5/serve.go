package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/pushkar-anand/ap-5/internal/config"
	"github.com/pushkar-anand/ap-5/internal/gmail"
	"github.com/pushkar-anand/ap-5/internal/handlers/creditcard"
	"github.com/pushkar-anand/ap-5/internal/jn66"
	"github.com/pushkar-anand/ap-5/internal/llm"
	"github.com/pushkar-anand/ap-5/internal/router"
	"github.com/pushkar-anand/ap-5/internal/secrets"
	"github.com/pushkar-anand/ap-5/internal/server"
	"github.com/pushkar-anand/ap-5/internal/state"
	bwglogger "github.com/pushkar-anand/build-with-go/logger"
)

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath(), "path to config.yaml")
	dataDir := fs.String("data", defaultDataDir(), "directory for state and secrets")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := bwglogger.New(
		bwglogger.WithLevel(cfg.Log.SlogLevel()),
		bwglogger.WithFormat(logFormat(cfg.Log.Format)),
		bwglogger.WithWriter(os.Stderr),
	)
	slog.SetDefault(log)

	secretStore, err := secrets.NewStore(*dataDir, cfg.Secrets.Backend, cfg.Secrets.EncryptionKey)
	if err != nil {
		log.Error("failed to initialise secret store", slog.Any("error", err))
		os.Exit(1)
	}

	stateStore, err := state.New(*dataDir)
	if err != nil {
		log.Error("failed to initialise state store", slog.Any("error", err))
		os.Exit(1)
	}

	llmClient := llm.New(log, cfg.Ollama.BaseURL, cfg.Ollama.RouterModel, cfg.Ollama.ExtractorModel)

	// Load Gmail OAuth credentials from secret store.
	gmailClientID, err := secretStore.Get("ap5/gmail/client_id")
	if err != nil {
		log.Error("Gmail client_id not found in secret store — run: ap5 auth set-gmail-credentials")
		os.Exit(1)
	}
	gmailClientSecret, err := secretStore.Get("ap5/gmail/client_secret")
	if err != nil {
		log.Error("Gmail client_secret not found in secret store — run: ap5 auth set-gmail-credentials")
		os.Exit(1)
	}

	redirectURL := cfg.Server.BaseURL + "/auth/callback"
	oauthMgr := gmail.NewOAuthManager(log, secretStore, gmailClientID, gmailClientSecret, redirectURL)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start HTTP server (OAuth callback + health).
	srv := server.New(log, cfg.Server.Port, oauthMgr)
	go func() {
		if err := srv.Serve(ctx); err != nil {
			log.Error("server stopped", slog.Any("error", err))
		}
	}()

	// Start one poller per account (sorted for deterministic startup order).
	for _, name := range slices.Sorted(maps.Keys(cfg.Accounts)) {
		account := cfg.Accounts[name]
		email := account.Email

		ts, err := oauthMgr.TokenSource(ctx, email)
		if err != nil {
			if errors.Is(err, secrets.ErrNotFound) {
				log.Info("no OAuth token — visit URL to authorise",
					slog.String("account", name),
					slog.String("email", email),
					slog.String("url", oauthMgr.AuthURL(email)),
				)
				continue
			}
			log.Error("failed to load OAuth token", slog.String("account", name), slog.String("email", email), slog.Any("error", err))
			continue
		}

		if account.JN66Token == "" {
			log.Error("jn66_token not set in config", slog.String("account", name), slog.String("email", email))
			continue
		}

		gmailClient, err := gmail.NewClient(ctx, email, ts)
		if err != nil {
			log.Error("failed to create Gmail client", slog.String("account", name), slog.String("email", email), slog.Any("error", err))
			continue
		}

		jn66Client := jn66.NewClient(cfg.JN66.BaseURL, account.JN66Token)
		accountCache := jn66.NewAccountCache(log, jn66Client)

		ccHandler := creditcard.New(log, llmClient, accountCache, jn66Client)

		r := router.New(log, llmClient)
		r.Register(creditcard.EmailType, ccHandler)

		poller := gmail.NewPoller(log, email, gmailClient, stateStore, r.Route, cfg.Gmail.PollInterval)

		go poller.Poll(ctx)
		log.Info("started poller", slog.String("account", name), slog.String("email", email))
	}

	<-ctx.Done()
	log.Info("shutting down")
}

func logFormat(format string) bwglogger.Format {
	if format == "json" {
		return bwglogger.FormatJSON
	}
	return bwglogger.FormatText
}

func defaultConfigPath() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h + "/.config/ap5/config.yaml"
	}
	return "config.yaml"
}

func defaultDataDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h + "/.config/ap5"
	}
	return ".ap5"
}
