package calendar

import (
	"context"
	"fmt"
)

// Source fetches normalized events for one account over a time window.
type Source interface {
	Fetch(ctx context.Context, w Window) ([]Event, error)
}

// sourceDeps carries the shared dependencies a source may need.
type sourceDeps struct {
	oauth *GoogleOAuth
	// onGoogleToken persists a refreshed Google token for the account.
	onGoogleToken func(accountID int64, cfg GoogleConfig) error
}

// sourceFor builds the Source implementation for an account's provider.
func sourceFor(acc Account, deps sourceDeps) (Source, error) {
	switch acc.Provider {
	case ProviderGoogle:
		if !deps.oauth.Configured() {
			return nil, fmt.Errorf("google OAuth is not configured (set TRMNL_GOOGLE_CLIENT_ID/SECRET)")
		}
		return &googleSource{acc: acc, deps: deps}, nil
	case ProviderCalDAV:
		return &caldavSource{acc: acc}, nil
	default:
		return nil, fmt.Errorf("unknown calendar provider %q", acc.Provider)
	}
}
