package calendar

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcal "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// GoogleOAuth wraps the Google OAuth2 client config used both to authorize new
// accounts (the admin consent flow) and to mint API tokens during sync.
type GoogleOAuth struct {
	cfg *oauth2.Config
}

// NewGoogleOAuth builds the OAuth config. redirectURL must match the URI
// registered in the Google Cloud OAuth client. A zero-value (empty client id or
// secret) is returned as a non-nil but unconfigured value; callers should guard
// with Configured.
func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{gcal.CalendarReadonlyScope},
		Endpoint:     google.Endpoint,
	}}
}

// Configured reports whether usable client credentials are present.
func (g *GoogleOAuth) Configured() bool {
	return g != nil && g.cfg.ClientID != "" && g.cfg.ClientSecret != ""
}

// AuthCodeURL returns the consent URL to redirect the admin to. offline access
// + forced consent ensures we always receive a refresh token.
func (g *GoogleOAuth) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"))
}

// Exchange swaps an authorization code for a token (including a refresh token).
func (g *GoogleOAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return g.cfg.Exchange(ctx, code)
}

// CalendarChoice is one selectable calendar within a Google account.
type CalendarChoice struct {
	ID      string
	Summary string
	Primary bool
}

// AccountInfo fetches the account email and the list of available calendars for
// a freshly authorized token, for the post-consent calendar picker.
func (g *GoogleOAuth) AccountInfo(ctx context.Context, tok *oauth2.Token) (email string, choices []CalendarChoice, err error) {
	svc, err := gcal.NewService(ctx, option.WithHTTPClient(g.cfg.Client(ctx, tok)))
	if err != nil {
		return "", nil, err
	}
	list, err := svc.CalendarList.List().Do()
	if err != nil {
		return "", nil, fmt.Errorf("list calendars: %w", err)
	}
	for _, item := range list.Items {
		choices = append(choices, CalendarChoice{ID: item.Id, Summary: item.Summary, Primary: item.Primary})
		if item.Primary {
			email = item.Id // the primary calendar id is the account's email
		}
	}
	return email, choices, nil
}

// configToToken / tokenToConfig bridge the stored GoogleConfig and oauth2.Token.
func configToToken(c GoogleConfig) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
		TokenType:    c.TokenType,
		Expiry:       c.Expiry,
	}
}
