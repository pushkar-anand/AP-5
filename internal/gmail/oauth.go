package gmail

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/pushkar-anand/ap-5/internal/secrets"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

var scopes = []string{gmail.GmailReadonlyScope}

// OAuthManager handles Gmail OAuth2 token lifecycle via a SecretStore.
type OAuthManager struct {
	log         *slog.Logger
	store       secrets.Store
	oauthConfig *oauth2.Config
}

func NewOAuthManager(log *slog.Logger, store secrets.Store, clientID, clientSecret, redirectURL string) *OAuthManager {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}
	return &OAuthManager{log: log, store: store, oauthConfig: cfg}
}

// AuthURL returns the URL the user must visit to authorize the given email account.
// state is passed through Google's redirect so we know which account to save the token for.
func (m *OAuthManager) AuthURL(email string) string {
	return m.oauthConfig.AuthCodeURL(email, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange converts an authorization code to a token and persists it.
func (m *OAuthManager) Exchange(ctx context.Context, email, code string) error {
	token, err := m.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("oauth: exchange code for %s: %w", email, err)
	}
	return m.saveToken(email, token)
}

// TokenSource returns a token source for the given email, loading from the store.
// Returns (nil, ErrNotFound) if no token is stored yet.
func (m *OAuthManager) TokenSource(ctx context.Context, email string) (oauth2.TokenSource, error) {
	token, err := m.loadToken(email)
	if err != nil {
		return nil, err
	}

	ts := m.oauthConfig.TokenSource(ctx, token)

	// Wrap to persist refreshed tokens back to the store.
	return &persistingTokenSource{
		email:     email,
		inner:     ts,
		manager:   m,
		ctx:       ctx,
		lastToken: token,
	}, nil
}

func (m *OAuthManager) saveToken(email string, token *oauth2.Token) error {
	b, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("oauth: marshal token: %w", err)
	}
	return m.store.Set(secrets.GmailTokenKey(email), string(b))
}

func (m *OAuthManager) loadToken(email string) (*oauth2.Token, error) {
	raw, err := m.store.Get(secrets.GmailTokenKey(email))
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return nil, fmt.Errorf("oauth: parse token for %s: %w", email, err)
	}
	return &token, nil
}

// persistingTokenSource wraps oauth2.TokenSource and writes refreshed tokens back to the store.
type persistingTokenSource struct {
	email     string
	inner     oauth2.TokenSource
	manager   *OAuthManager
	ctx       context.Context
	lastToken *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	token, err := p.inner.Token()
	if err != nil {
		return nil, err
	}
	if token.AccessToken != p.lastToken.AccessToken {
		if saveErr := p.manager.saveToken(p.email, token); saveErr != nil {
			p.manager.log.WarnContext(p.ctx, "failed to persist refreshed token",
				slog.String("email", p.email), slog.Any("error", saveErr))
		}
		p.lastToken = token
	}
	return token, nil
}
