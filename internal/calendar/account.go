package calendar

import (
	"encoding/json"
	"time"

	"github.com/gesellix/go-trmnl/internal/store"
)

// Provider identifies a calendar backend.
type Provider string

const (
	ProviderGoogle Provider = "google"
	ProviderCalDAV Provider = "caldav" // phase 2 (Apple iCloud / generic)
)

// DefaultRefreshInterval is used when an account does not specify one.
const DefaultRefreshInterval = 12 * time.Hour

// Account is the domain view of a configured calendar source. The provider
// specifics (OAuth token, selected calendars, CalDAV endpoint) live in Config,
// shaped per provider.
type Account struct {
	ID              int64
	Provider        Provider
	Name            string
	Marker          string
	RefreshInterval time.Duration
	LastSync        time.Time // zero if never synced
	LastError       string
	Config          GoogleConfig // only the google fields are used for now
}

// GoogleConfig is the provider config stored as JSON in the account row for a
// Google account.
type GoogleConfig struct {
	Email string `json:"email"`
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
		if err := json.Unmarshal([]byte(a.ConfigJSON), &acc.Config); err != nil {
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
