package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gesellix/go-trmnl/internal/secret"
	"github.com/gesellix/go-trmnl/internal/store"
	"golang.org/x/oauth2"
)

// Sync window: how far back and forward to fetch into the cache. The plugin can
// then show any sub-range of this without re-fetching.
const (
	syncLookback = 24 * time.Hour
	syncHorizon  = 60 * 24 * time.Hour
)

// Service orchestrates calendar accounts: syncing provider events into the
// store cache and serving a merged agenda to the plugin. It is safe for the
// background sync goroutine and request handlers to share one instance.
type Service struct {
	store *store.Store
	box   *secret.Box // encrypts sensitive config fields at rest (may be disabled)

	// newSource builds the provider source for an account. Overridable in tests.
	newSource func(acc Account, deps sourceDeps) (Source, error)
}

// NewService builds the calendar service. box encrypts stored credentials (a
// nil box stores them as plaintext).
func NewService(st *store.Store, box *secret.Box) *Service {
	return &Service{store: st, box: box, newSource: sourceFor}
}

// --- Google OAuth clients ---

// ListOAuthClients returns the configured OAuth clients (secrets stay encrypted
// in the rows and are not needed by callers that only list/label them).
func (s *Service) ListOAuthClients() ([]*store.OAuthClient, error) {
	return s.store.ListOAuthClients()
}

// HasGoogleClients reports whether at least one Google OAuth client exists.
func (s *Service) HasGoogleClients() bool {
	clients, err := s.store.ListOAuthClients()
	return err == nil && len(clients) > 0
}

// CreateOAuthClient stores a Google OAuth client, encrypting its secret.
func (s *Service) CreateOAuthClient(name, clientID, clientSecret string) (int64, error) {
	enc, err := s.box.Encrypt(clientSecret)
	if err != nil {
		return 0, err
	}
	c, err := s.store.CreateOAuthClient(string(ProviderGoogle), name, clientID, enc)
	if err != nil {
		return 0, err
	}
	return c.ID, nil
}

// DeleteOAuthClient removes an OAuth client.
func (s *Service) DeleteOAuthClient(id int64) error { return s.store.DeleteOAuthClient(id) }

// MigrateEncryption encrypts any plaintext stored secrets to the current
// format, leaving already-current values untouched. It is a no-op without a
// configured key. (Pre-scrypt v1 values are no longer readable and are not
// migrated; they surface as an error when used.) Returns the number of rows
// rewritten.
func (s *Service) MigrateEncryption() (int, error) {
	if !s.box.Enabled() {
		return 0, nil
	}
	n := 0

	rows, err := s.store.ListCalendarAccounts()
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		if !s.box.NeedsUpgrade(r.ConfigJSON) {
			continue
		}
		acc, lerr := s.loadAccount(r) // decrypts v1/v2/plaintext
		if lerr != nil {
			return n, fmt.Errorf("calendar: migrate account %d: %w", r.ID, lerr)
		}
		var j string
		switch acc.Provider {
		case ProviderCalDAV:
			if acc.CalDAV.Password == "" {
				continue
			}
			j, lerr = s.marshalCalDAVConfig(acc.CalDAV)
		default:
			if acc.Config.RefreshToken == "" && acc.Config.AccessToken == "" {
				continue
			}
			j, lerr = s.marshalGoogleConfig(acc.Config)
		}
		if lerr != nil {
			return n, lerr
		}
		if lerr = s.store.SetCalendarAccountConfig(r.ID, j); lerr != nil {
			return n, lerr
		}
		n++
	}

	clients, err := s.store.ListOAuthClients()
	if err != nil {
		return n, err
	}
	for _, c := range clients {
		if !s.box.NeedsUpgrade(c.ClientSecret) {
			continue
		}
		plain, lerr := s.box.Decrypt(c.ClientSecret)
		if lerr != nil {
			return n, fmt.Errorf("calendar: migrate oauth client %d: %w", c.ID, lerr)
		}
		if plain == "" {
			continue
		}
		enc, lerr := s.box.Encrypt(plain)
		if lerr != nil {
			return n, lerr
		}
		if lerr = s.store.SetOAuthClientSecret(c.ID, enc); lerr != nil {
			return n, lerr
		}
		n++
	}
	return n, nil
}

// oauthForClient builds a GoogleOAuth for the given client row, decrypting its
// secret. redirectURL matters only for the consent + code-exchange path; pass
// "" for token refresh and calendar listing, which do not use it.
func (s *Service) oauthForClient(id int64, redirectURL string) (*GoogleOAuth, error) {
	c, err := s.store.GetOAuthClient(id)
	if err != nil {
		return nil, fmt.Errorf("calendar: oauth client %d: %w", id, err)
	}
	secret, err := s.box.Decrypt(c.ClientSecret)
	if err != nil {
		return nil, err
	}
	return NewGoogleOAuth(c.ClientID, secret, redirectURL), nil
}

// GoogleAuthCodeURL returns the consent URL for a specific OAuth client.
// redirectURL is the callback the caller will handle and must match what is
// registered with Google and used later in ExchangeGoogle.
func (s *Service) GoogleAuthCodeURL(clientID int64, state, redirectURL string) (string, error) {
	oauth, err := s.oauthForClient(clientID, redirectURL)
	if err != nil {
		return "", err
	}
	return oauth.AuthCodeURL(state), nil
}

// ExchangeGoogle swaps an auth code for a token using a specific client and
// returns the token and the account email. redirectURL must match the one used
// in GoogleAuthCodeURL.
func (s *Service) ExchangeGoogle(ctx context.Context, clientID int64, code, redirectURL string) (*oauth2.Token, string, error) {
	oauth, err := s.oauthForClient(clientID, redirectURL)
	if err != nil {
		return nil, "", err
	}
	tok, err := oauth.Exchange(ctx, code)
	if err != nil {
		return nil, "", err
	}
	email, _, err := oauth.AccountInfo(ctx, tok)
	if err != nil {
		return nil, "", err
	}
	return tok, email, nil
}

// Accounts returns all configured accounts as domain values.
func (s *Service) Accounts() ([]Account, error) {
	rows, err := s.store.ListCalendarAccounts()
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(rows))
	for _, r := range rows {
		acc, err := s.loadAccount(r)
		if err != nil {
			return nil, err
		}
		out = append(out, acc)
	}
	return out, nil
}

// Account returns one account by ID.
func (s *Service) Account(id int64) (Account, error) {
	r, err := s.store.GetCalendarAccount(id)
	if err != nil {
		return Account{}, err
	}
	return s.loadAccount(r)
}

// loadAccount converts a store row to a domain Account, decrypting the stored
// token fields.
func (s *Service) loadAccount(r *store.CalendarAccount) (Account, error) {
	acc, err := accountFromStore(r)
	if err != nil {
		return Account{}, err
	}
	switch acc.Provider {
	case ProviderCalDAV:
		if acc.CalDAV.Password, err = s.box.Decrypt(acc.CalDAV.Password); err != nil {
			return Account{}, err
		}
	default:
		if acc.Config.RefreshToken, err = s.box.Decrypt(acc.Config.RefreshToken); err != nil {
			return Account{}, err
		}
		if acc.Config.AccessToken, err = s.box.Decrypt(acc.Config.AccessToken); err != nil {
			return Account{}, err
		}
	}
	return acc, nil
}

// marshalCalDAVConfig encrypts the password and serializes the config.
func (s *Service) marshalCalDAVConfig(cfg CalDAVConfig) (string, error) {
	var err error
	if cfg.Password, err = s.box.Encrypt(cfg.Password); err != nil {
		return "", err
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// marshalGoogleConfig encrypts the token fields and serializes the config for
// storage.
func (s *Service) marshalGoogleConfig(cfg GoogleConfig) (string, error) {
	var err error
	if cfg.RefreshToken, err = s.box.Encrypt(cfg.RefreshToken); err != nil {
		return "", err
	}
	if cfg.AccessToken, err = s.box.Encrypt(cfg.AccessToken); err != nil {
		return "", err
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DeleteAccount removes an account and its cached events.
func (s *Service) DeleteAccount(id int64) error { return s.store.DeleteCalendarAccount(id) }

func (s *Service) deps() sourceDeps {
	return sourceDeps{
		googleOAuthFor: func(clientID int64) (*GoogleOAuth, error) {
			return s.oauthForClient(clientID, "") // refresh does not use the redirect URI
		},
		onGoogleToken: func(id int64, cfg GoogleConfig) error {
			j, err := s.marshalGoogleConfig(cfg)
			if err != nil {
				return err
			}
			return s.store.SetCalendarAccountConfig(id, j)
		},
	}
}

// SyncDue syncs every account whose refresh interval has elapsed, recording the
// outcome per account. It returns the first error encountered (others are still
// recorded on their accounts) so the caller can log it.
func (s *Service) SyncDue(ctx context.Context, now time.Time) error {
	accs, err := s.Accounts()
	if err != nil {
		return err
	}
	var firstErr error
	for _, acc := range accs {
		if !acc.due(now) {
			continue
		}
		if e := s.runSync(ctx, acc, now); e != nil && firstErr == nil {
			firstErr = e
		}
	}
	return firstErr
}

// SyncAccount forces a resync of one account regardless of its interval.
func (s *Service) SyncAccount(ctx context.Context, id int64) error {
	acc, err := s.Account(id)
	if err != nil {
		return err
	}
	return s.runSync(ctx, acc, time.Now())
}

// runSync fetches and stores one account, recording last_sync/last_error.
func (s *Service) runSync(ctx context.Context, acc Account, now time.Time) error {
	err := s.fetchAndStore(ctx, acc, now)
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_ = s.store.SetCalendarAccountSync(acc.ID, now.Unix(), msg)
	return err
}

func (s *Service) fetchAndStore(ctx context.Context, acc Account, now time.Time) error {
	src, err := s.newSource(acc, s.deps())
	if err != nil {
		return err
	}
	w := Window{From: now.Add(-syncLookback), To: now.Add(syncHorizon)}
	evs, err := src.Fetch(ctx, w)
	if err != nil {
		return err
	}
	rows := make([]store.CalendarEvent, 0, len(evs))
	for _, e := range evs {
		rows = append(rows, store.CalendarEvent{
			AccountID: acc.ID,
			UID:       e.UID,
			Title:     e.Title,
			StartAt:   e.Start.Unix(),
			EndAt:     e.End.Unix(),
			AllDay:    e.AllDay,
			Location:  e.Location,
			Status:    e.Status,
		})
	}
	return s.store.ReplaceCalendarEvents(acc.ID, rows, now.Unix())
}

// Agenda returns the merged, deduplicated events for the given accounts within
// the window, capped at limit (0 = no cap). accountIDs empty = all accounts.
func (s *Service) Agenda(accountIDs []int64, w Window, limit int) ([]Event, error) {
	rows, err := s.store.ListCalendarEvents(accountIDs, w.From.Unix(), w.To.Unix())
	if err != nil {
		return nil, err
	}
	markerByID, err := s.markerByID()
	if err != nil {
		return nil, err
	}
	evs := make([]Event, 0, len(rows))
	for _, r := range rows {
		e := Event{
			UID:       r.UID,
			Title:     r.Title,
			Start:     time.Unix(r.StartAt, 0),
			End:       time.Unix(r.EndAt, 0),
			AllDay:    r.AllDay,
			Location:  r.Location,
			Status:    r.Status,
			AccountID: r.AccountID,
		}
		if m := markerByID[r.AccountID]; m != "" {
			e.Markers = []string{m}
		}
		evs = append(evs, e)
	}
	merged := Merge(evs)
	if limit > 0 && len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// markerByID maps account IDs to their marker badge. It reads store rows
// directly (no token decryption) since rendering an agenda never needs tokens.
func (s *Service) markerByID() (map[int64]string, error) {
	rows, err := s.store.ListCalendarAccounts()
	if err != nil {
		return nil, err
	}
	m := make(map[int64]string, len(rows))
	for _, r := range rows {
		m[r.ID] = r.Marker
	}
	return m, nil
}

// NextWake returns how long the background loop should sleep: the smallest
// account refresh interval, clamped to [1m, 1h]; 15m when there are no accounts.
func (s *Service) NextWake() time.Duration {
	accs, err := s.Accounts()
	if err != nil || len(accs) == 0 {
		return 15 * time.Minute
	}
	smallest := accs[0].RefreshInterval
	for _, a := range accs[1:] {
		if a.RefreshInterval < smallest {
			smallest = a.RefreshInterval
		}
	}
	switch {
	case smallest < time.Minute:
		return time.Minute
	case smallest > time.Hour:
		return time.Hour
	default:
		return smallest
	}
}

// --- Google account management (used by the admin OAuth flow) ---

// CreateGoogleAccount stores a newly authorized Google account, bound to the
// OAuth client it was authorized against.
func (s *Service) CreateGoogleAccount(oauthClientID int64, name, marker string, tok *oauth2.Token, email string, calendarIDs []string, refresh time.Duration) (int64, error) {
	cfg := GoogleConfig{
		OAuthClientID: oauthClientID,
		Email:         email,
		RefreshToken:  tok.RefreshToken,
		AccessToken:   tok.AccessToken,
		TokenType:     tok.TokenType,
		Expiry:        tok.Expiry,
		CalendarIDs:   calendarIDs,
	}
	j, err := s.marshalGoogleConfig(cfg)
	if err != nil {
		return 0, err
	}
	if name == "" {
		name = email
	}
	acc, err := s.store.CreateCalendarAccount(string(ProviderGoogle), name, marker, j, int(refresh/time.Second))
	if err != nil {
		return 0, err
	}
	return acc.ID, nil
}

// UpdateGoogleAccount saves the editable fields and calendar selection for an
// existing Google account, preserving its stored token.
func (s *Service) UpdateGoogleAccount(id int64, name, marker string, calendarIDs []string, refresh time.Duration) error {
	acc, err := s.Account(id)
	if err != nil {
		return err
	}
	cfg := acc.Config
	cfg.CalendarIDs = calendarIDs
	j, err := s.marshalGoogleConfig(cfg)
	if err != nil {
		return err
	}
	return s.store.UpdateCalendarAccount(id, name, marker, j, int(refresh/time.Second))
}

// ListGoogleCalendars lists the calendars available to an existing account,
// for the edit/picker page.
func (s *Service) ListGoogleCalendars(ctx context.Context, id int64) ([]Choice, error) {
	acc, err := s.Account(id)
	if err != nil {
		return nil, err
	}
	oauth, err := s.oauthForClient(acc.Config.OAuthClientID, "") // listing uses the token, not the redirect URI
	if err != nil {
		return nil, err
	}
	_, choices, err := oauth.AccountInfo(ctx, configToToken(acc.Config))
	return choices, err
}

// --- CalDAV account management (Apple iCloud / generic) ---

// CreateCalDAVAccount stores a CalDAV account. endpoint defaults to iCloud when
// empty.
func (s *Service) CreateCalDAVAccount(name, marker, endpoint, username, password string, refresh time.Duration) (int64, error) {
	if endpoint == "" {
		endpoint = DefaultCalDAVEndpoint
	}
	cfg := CalDAVConfig{Endpoint: endpoint, Username: username, Password: password}
	j, err := s.marshalCalDAVConfig(cfg)
	if err != nil {
		return 0, err
	}
	if name == "" {
		name = username
	}
	acc, err := s.store.CreateCalendarAccount(string(ProviderCalDAV), name, marker, j, int(refresh/time.Second))
	if err != nil {
		return 0, err
	}
	return acc.ID, nil
}

// UpdateCalDAVAccount saves the editable fields and calendar selection,
// preserving the stored endpoint and credentials.
func (s *Service) UpdateCalDAVAccount(id int64, name, marker string, calendarPaths []string, refresh time.Duration) error {
	acc, err := s.Account(id)
	if err != nil {
		return err
	}
	cfg := acc.CalDAV
	cfg.CalendarPaths = calendarPaths
	j, err := s.marshalCalDAVConfig(cfg)
	if err != nil {
		return err
	}
	return s.store.UpdateCalendarAccount(id, name, marker, j, int(refresh/time.Second))
}

// ListCalDAVCalendars discovers the calendars available to an existing CalDAV
// account, for the edit/picker page. The choice ID is the calendar path.
func (s *Service) ListCalDAVCalendars(ctx context.Context, id int64) ([]Choice, error) {
	acc, err := s.Account(id)
	if err != nil {
		return nil, err
	}
	src := &caldavSource{acc: acc}
	c, err := src.client()
	if err != nil {
		return nil, err
	}
	cals, err := src.discoverCalendars(ctx, c)
	if err != nil {
		return nil, err
	}
	var choices []Choice
	for _, cal := range cals {
		if !supportsEvents(cal) {
			continue
		}
		name := cal.Name
		if name == "" {
			name = cal.Path
		}
		choices = append(choices, Choice{ID: cal.Path, Summary: name})
	}
	return choices, nil
}
