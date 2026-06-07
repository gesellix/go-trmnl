package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gesellix/go-trmnl/internal/calendar"
)

// oauthStateCookie is the short-lived cookie holding the CSRF state for the
// Google consent round-trip.
const oauthStateCookie = "trmnl_oauth_state"

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
	h.render(w, "calendar", map[string]any{
		"Nav":              "calendar",
		"Accounts":         rows,
		"GoogleConfigured": h.cal.OAuth().Configured(),
		"BaseURL":          h.baseURL,
	})
}

// CalendarGoogleStart redirects the admin to Google's consent screen.
func (h *Handler) CalendarGoogleStart(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil || !h.cal.OAuth().Configured() {
		http.Error(w, "Google OAuth is not configured (set TRMNL_GOOGLE_CLIENT_ID and TRMNL_GOOGLE_CLIENT_SECRET)", http.StatusPreconditionFailed)
		return
	}
	state := randomToken()
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, h.cal.OAuth().AuthCodeURL(state), http.StatusFound)
}

// CalendarGoogleCallback handles Google's redirect: it validates state,
// exchanges the code, creates the account, and sends the admin to the picker.
func (h *Handler) CalendarGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.cal == nil || !h.cal.OAuth().Configured() {
		http.Error(w, "Google OAuth is not configured", http.StatusPreconditionFailed)
		return
	}
	c, err := r.Cookie(oauthStateCookie)
	if err != nil || c.Value == "" || c.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/admin", MaxAge: -1})

	if e := r.URL.Query().Get("error"); e != "" {
		http.Error(w, "Google authorization failed: "+e, http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	tok, err := h.cal.OAuth().Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	email, _, err := h.cal.OAuth().AccountInfo(r.Context(), tok)
	if err != nil {
		http.Error(w, "could not read account info: "+err.Error(), http.StatusBadGateway)
		return
	}
	// Default the marker to the first letter of the local part of the email.
	marker := ""
	if email != "" {
		marker = string([]rune(email)[:1])
	}
	id, err := h.cal.CreateGoogleAccount(email, marker, tok, email, nil, calendar.DefaultRefreshInterval)
	if err != nil {
		http.Error(w, "could not save account: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/calendar/"+i64(id), http.StatusFound)
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

	selected := map[string]bool{}
	for _, cid := range acc.Config.CalendarIDs {
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
	if choices, cerr := h.cal.ListGoogleCalendars(r.Context(), id); cerr != nil {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	marker := r.FormValue("marker")
	refresh := time.Duration(parseHours(r.FormValue("refresh_hours"), 12)) * time.Hour
	calendarIDs := r.Form["calendar_ids"]

	if err := h.cal.UpdateGoogleAccount(id, name, marker, calendarIDs, refresh); err != nil {
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

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// parseHours parses a positive integer hour count, falling back to def.
func parseHours(s string, def int) int {
	if v, ok := parseInt64(s); ok && v > 0 {
		return int(v)
	}
	return def
}
