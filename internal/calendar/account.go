package calendar

import (
	"encoding/json"
	"time"

	"github.com/gesellix/go-trmnl/internal/store"
)

// Provider identifies a calendar backend.
type Provider string

// Supported calendar backends.
const (
	ProviderGoogle Provider = "google"
	ProviderCalDAV Provider = "caldav" // Apple iCloud / generic CalDAV
)

// DefaultRefreshInterval is used when an account does not specify one.
const DefaultRefreshInterval = 12 * time.Hour

// Account is the domain view of a configured calendar source. The
// provider-specific config (Config for Google, CalDAV for CalDAV) is filled in
// according to Provider.
type Account struct {
	ID              int64
	Provider        Provider
	Name            string
	Marker          string
	RefreshInterval time.Duration
	LastSync        time.Time // zero if never synced
	LastError       string
	Config          GoogleConfig // populated for ProviderGoogle
	CalDAV          CalDAVConfig // populated for ProviderCalDAV
}

// CalDAVConfig is the provider config for a CalDAV account (Apple iCloud or a
// generic server), stored as JSON in the account row.
type CalDAVConfig struct {
	Endpoint string `json:"endpoint"` // e.g. https://caldav.icloud.com
	Username string `json:"username"` // Apple ID / account username
	Password string `json:"password"` // app-specific password (encrypted at rest)
	// CalendarPaths are the calendar collection paths to include. Empty means
	// all discovered calendars that support events.
	CalendarPaths []string `json:"calendar_paths,omitempty"`
}

// GoogleConfig is the provider config stored as JSON in the account row for a
// Google account.
type GoogleConfig struct {
	// OAuthClientID is the oauth_clients row this account authorized against.
	OAuthClientID int64  `json:"oauth_client_id"`
	Email         string `json:"email"`
	// RefreshToken (and the cached AccessToken/Expiry) authorize Calendar API
	// calls. The token is refreshed transparently and persisted back.
	RefreshToken string    `json:"refresh_token"`
	AccessToken  string    `json:"access_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	// CalendarIDs are the calendars to include. Empty means "primary" only.
	CalendarIDs []string `json:"calendar_ids,omitempty"`
}

// accountFromStore converts a store row into a domain Account.
func accountFromStore(a *store.CalendarAccount) (Account, error) {
	acc := Account{
		ID:              a.ID,
		Provider:        Provider(a.Provider),
		Name:            a.Name,
		Marker:          a.Marker,
		RefreshInterval: time.Duration(a.RefreshInterval) * time.Second,
		LastError:       a.LastError.String,
	}
	if acc.RefreshInterval <= 0 {
		acc.RefreshInterval = DefaultRefreshInterval
	}
	if a.LastSync.Valid {
		acc.LastSync = time.Unix(a.LastSync.Int64, 0)
	}
	if a.ConfigJSON != "" {
		var err error
		switch acc.Provider {
		case ProviderCalDAV:
			err = json.Unmarshal([]byte(a.ConfigJSON), &acc.CalDAV)
		default:
			err = json.Unmarshal([]byte(a.ConfigJSON), &acc.Config)
		}
		if err != nil {
			return Account{}, err
		}
	}
	return acc, nil
}

// due reports whether the account should be synced at time now.
func (a Account) due(now time.Time) bool {
	if a.LastSync.IsZero() {
		return true
	}
	return now.Sub(a.LastSync) >= a.RefreshInterval
}
