package admin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gesellix/go-trmnl/internal/calendar"
)

// oauthStateCookie is the short-lived cookie holding the CSRF state for the
// Google consent round-trip.
const oauthStateCookie = "trmnl_oauth_state"

// oauthRedirectURL derives the Google OAuth callback from the incoming request
// (scheme + host), so it matches whatever URL the admin used to reach the UI.
// This decouples the OAuth redirect from the device-facing base URL: the
// redirect can use a DNS hostname (Google rejects raw private IPs) while
// base-url stays a LAN IP for the device's image fetches.
func oauthRedirectURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	return scheme + "://" + r.Host + "/admin/oauth/google/callback"
}

// requestIsHTTPS reports whether the request reached us over TLS (directly or
// via a terminating proxy). Used to mark cookies Secure only when it would not
// break a plain-HTTP LAN deployment.
func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// CalendarList shows configured calendar accounts and the "add account" action.
func (h *Handler) CalendarList(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	type row struct {
		ID       int64
		Name     string
		Provider string
		Marker   string
		Refresh  string
		LastSync string
		Error    string
	}
	accs, _ := h.cal.Accounts()
	rows := make([]row, 0, len(accs))
	for _, a := range accs {
		last := "never"
		if !a.LastSync.IsZero() {
			last = humanSince(a.LastSync.Unix())
		}
		rows = append(rows, row{
			ID: a.ID, Name: a.Name, Provider: string(a.Provider), Marker: a.Marker,
			Refresh: a.RefreshInterval.String(), LastSync: last, Error: a.LastError,
		})
	}

	type clientRow struct {
		ID       int64
		Name     string
		ClientID string
	}
	clients, _ := h.cal.ListOAuthClients()
	crows := make([]clientRow, 0, len(clients))
	for _, c := range clients {
		crows = append(crows, clientRow{ID: c.ID, Name: c.Name, ClientID: c.ClientID})
	}

	h.render(w, "calendar", map[string]any{
		"Nav":                   "calendar",
		"Accounts":              rows,
		"OAuthClients":          crows,
		"DefaultCalDAVEndpoint": calendar.DefaultCalDAVEndpoint,
		"RedirectURI":           oauthRedirectURL(r),
		"BaseURL":               h.baseURL,
	})
}

// CalendarOAuthClientCreate stores a Google OAuth client from the form.
func (h *Handler) CalendarOAuthClientCreate(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	if clientID == "" || clientSecret == "" {
		http.Error(w, "client ID and client secret are required", http.StatusBadRequest)
		return
	}
	if name == "" {
		name = clientID
	}
	if _, err := h.cal.CreateOAuthClient(name, clientID, clientSecret); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/calendar", http.StatusFound)
}

// CalendarOAuthClientDelete removes a Google OAuth client.
func (h *Handler) CalendarOAuthClientDelete(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.cal.DeleteOAuthClient(id)
	http.Redirect(w, r, "/admin/calendar", http.StatusFound)
}

// CalendarGoogleStart redirects the admin to Google's consent screen for the
// chosen OAuth client (?client=<id>). The state cookie carries a nonce and the
// client id so the callback knows which client to exchange against.
func (h *Handler) CalendarGoogleStart(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	clientID, err := parseInt64Q(r.URL.Query().Get("client"))
	if err != nil {
		http.Error(w, "missing or invalid client", http.StatusBadRequest)
		return
	}
	nonce := randomToken()
	authURL, err := h.cal.GoogleAuthCodeURL(clientID, nonce, oauthRedirectURL(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    nonce + "|" + i64(clientID),
		Path:     "/admin",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

// CalendarGoogleCallback handles Google's redirect: it validates the state
// nonce, exchanges the code via the client recorded in the cookie, creates the
// account, and sends the admin to the picker.
func (h *Handler) CalendarGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	c, err := r.Cookie(oauthStateCookie)
	if err != nil || c.Value == "" {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/admin",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	nonce, clientIDStr, ok := strings.Cut(c.Value, "|")
	if !ok || nonce == "" || nonce != r.URL.Query().Get("state") {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	clientID, err := parseInt64Q(clientIDStr)
	if err != nil {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}

	if e := r.URL.Query().Get("error"); e != "" {
		http.Error(w, "Google authorization failed: "+e, http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	tok, email, err := h.cal.ExchangeGoogle(r.Context(), clientID, code, oauthRedirectURL(r))
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	// Default the marker to the first letter of the email.
	marker := ""
	if email != "" {
		marker = string([]rune(email)[:1])
	}
	id, err := h.cal.CreateGoogleAccount(clientID, email, marker, tok, email, nil, calendar.DefaultRefreshInterval)
	if err != nil {
		http.Error(w, "could not save account: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Sync immediately so the account populates (and any error surfaces), the
	// same way adding a CalDAV account does.
	_ = h.cal.SyncAccount(r.Context(), id)
	http.Redirect(w, r, "/admin/calendar/"+i64(id), http.StatusFound)
}

// parseInt64Q parses a positive int64 from a query/form value.
func parseInt64Q(s string) (int64, error) {
	v, ok := parseInt64(s)
	if !ok || v <= 0 {
		return 0, fmt.Errorf("invalid id %q", s)
	}
	return v, nil
}

// CalendarAccountDetail shows the edit/calendar-picker page for one account.
func (h *Handler) CalendarAccountDetail(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	acc, err := h.cal.Account(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	selectedIDs := acc.Config.CalendarIDs
	if acc.Provider == calendar.ProviderCalDAV {
		selectedIDs = acc.CalDAV.CalendarPaths
	}
	selected := map[string]bool{}
	for _, cid := range selectedIDs {
		selected[cid] = true
	}
	type calRow struct {
		ID       string
		Summary  string
		Primary  bool
		Selected bool
	}
	var cals []calRow
	var listErr string
	var choices []calendar.Choice
	var cerr error
	if acc.Provider == calendar.ProviderCalDAV {
		choices, cerr = h.cal.ListCalDAVCalendars(r.Context(), id)
	} else {
		choices, cerr = h.cal.ListGoogleCalendars(r.Context(), id)
	}
	if cerr != nil {
		listErr = cerr.Error()
	} else {
		for _, c := range choices {
			cals = append(cals, calRow{ID: c.ID, Summary: c.Summary, Primary: c.Primary, Selected: selected[c.ID]})
		}
	}

	h.render(w, "calendar_account", map[string]any{
		"Nav":          "calendar",
		"ID":           acc.ID,
		"Name":         acc.Name,
		"Marker":       acc.Marker,
		"Provider":     string(acc.Provider),
		"Email":        acc.Config.Email,
		"RefreshHours": int(acc.RefreshInterval.Hours()),
		"Calendars":    cals,
		"ListError":    listErr,
		"BaseURL":      h.baseURL,
	})
}

// CalendarAccountUpdate saves name, marker, refresh interval and calendar
// selection for an account.
func (h *Handler) CalendarAccountUpdate(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err = r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	marker := r.FormValue("marker")
	refresh := time.Duration(parseHours(r.FormValue("refresh_hours"), 12)) * time.Hour
	calendarIDs := r.Form["calendar_ids"]

	acc, err := h.cal.Account(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if acc.Provider == calendar.ProviderCalDAV {
		err = h.cal.UpdateCalDAVAccount(id, name, marker, calendarIDs, refresh)
	} else {
		err = h.cal.UpdateGoogleAccount(id, name, marker, calendarIDs, refresh)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Re-sync immediately so the new selection takes effect.
	_ = h.cal.SyncAccount(r.Context(), id)
	http.Redirect(w, r, "/admin/calendar", http.StatusFound)
}

// CalendarAccountSync forces a resync of one account.
func (h *Handler) CalendarAccountSync(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.cal.SyncAccount(r.Context(), id)
	http.Redirect(w, r, "/admin/calendar", http.StatusFound)
}

// CalendarAccountDelete removes an account and its cached events.
func (h *Handler) CalendarAccountDelete(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	id, err := idParam(r, "id")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = h.cal.DeleteAccount(id)
	http.Redirect(w, r, "/admin/calendar", http.StatusFound)
}

// CalendarCalDAVCreate adds an Apple iCloud / generic CalDAV account from the
// form on the calendar list page, then sends the admin to its picker page.
func (h *Handler) CalendarCalDAVCreate(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil {
		http.Error(w, "calendar service unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		http.Error(w, "username and app-specific password are required", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	marker := r.FormValue("marker")
	endpoint := r.FormValue("endpoint")
	refresh := time.Duration(parseHours(r.FormValue("refresh_hours"), 12)) * time.Hour

	id, err := h.cal.CreateCalDAVAccount(name, marker, endpoint, username, password, refresh)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Sync once so a bad credential surfaces immediately as last_error.
	_ = h.cal.SyncAccount(r.Context(), id)
	http.Redirect(w, r, "/admin/calendar/"+i64(id), http.StatusFound)
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// maxRefreshHours bounds a refresh interval at one (leap) year, which also
// keeps the value well within int range on 32-bit builds (e.g. arm).
const maxRefreshHours = 24 * 366

// parseHours parses a positive integer hour count, falling back to def. The
// value is bounded so the int conversion is safe on 32-bit platforms.
func parseHours(s string, def int) int {
	if v, ok := parseInt64(s); ok && v >= 1 && v <= maxRefreshHours {
		return int(v)
	}
	return def
}
