package admin_test

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gesellix/go-trmnl/internal/admin"
	"github.com/gesellix/go-trmnl/internal/calendar"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"
	"github.com/gesellix/go-trmnl/internal/tlscert"
	"golang.org/x/oauth2"

	_ "github.com/gesellix/go-trmnl/internal/plugins/clock"
)

func newAdminServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r := server.New()
	admin.New(st, "http://test.local", dir, admin.Auth{}, nil).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	// Don't follow redirects, so we can assert on 302 Location.
	ts.Client().CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return ts, st
}

func TestAdminPagesRender(t *testing.T) {
	ts, _ := newAdminServer(t)
	for _, path := range []string{"/admin", "/admin/devices", "/admin/screens", "/admin/playlists", "/admin/settings"} {
		resp, err := ts.Client().Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestAdminCreateScreenAndPlaylist(t *testing.T) {
	ts, st := newAdminServer(t)

	post := func(path string, form url.Values) *http.Response {
		resp, err := ts.Client().PostForm(ts.URL+path, form)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		return resp
	}

	resp := post("/admin/screens", url.Values{"plugin_type": {"clock"}, "name": {"My Clock"}})
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("create screen = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/admin/screens/") {
		t.Errorf("redirect = %q", loc)
	}
	scrs, _ := st.ListScreens()
	if len(scrs) != 1 || scrs[0].Name != "My Clock" {
		t.Fatalf("screen not persisted: %+v", scrs)
	}

	// Invalid settings JSON is rejected.
	bad := post("/admin/screens/"+itoa(scrs[0].ID), url.Values{"name": {"x"}, "settings_json": {"{not json"}})
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("bad settings = %d, want 400", bad.StatusCode)
	}

	// Preview renders the screen and records a hash.
	pv := post("/admin/screens/"+itoa(scrs[0].ID)+"/preview", nil)
	pv.Body.Close()
	got, _ := st.GetScreen(scrs[0].ID)
	if !got.RenderedHash.Valid {
		t.Errorf("preview did not record a rendered hash")
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func TestAdminAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r := server.New()
	admin.New(st, "http://test.local", dir, admin.Auth{User: "admin", Password: "s3cret"}, nil).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	ts.Client().CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	// No credentials -> 401 with a challenge.
	resp, _ := ts.Client().Get(ts.URL + "/admin")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-auth status = %d, want 401", resp.StatusCode)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Errorf("missing WWW-Authenticate challenge")
	}

	// Wrong password -> 401.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin", nil)
	req.SetBasicAuth("admin", "wrong")
	resp2, _ := ts.Client().Do(req)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong-pass status = %d, want 401", resp2.StatusCode)
	}

	// Correct credentials -> 200.
	req3, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin", nil)
	req3.SetBasicAuth("admin", "s3cret")
	resp3, _ := ts.Client().Do(req3)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("good-auth status = %d, want 200", resp3.StatusCode)
	}
}

func TestAdminGoogleReconnectStart(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cal := calendar.NewService(st, nil)
	clientID, err := cal.CreateOAuthClient("family", "cid.apps.googleusercontent.com", "csecret")
	if err != nil {
		t.Fatal(err)
	}
	accID, err := cal.CreateGoogleAccount(clientID, "Mom", "M", &oauth2.Token{RefreshToken: "r"},
		"mom@example.com", nil, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	r := server.New()
	admin.New(st, "http://test.local", dir, admin.Auth{}, cal).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	ts.Client().CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	res, err := ts.Client().Get(ts.URL + "/admin/calendar/google/start?account=" + strconv.FormatInt(accID, 10))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || !strings.Contains(res.Header.Get("Location"), "cid.apps.googleusercontent.com") {
		t.Fatalf("status %d, location %q", res.StatusCode, res.Header.Get("Location"))
	}
	var state string
	for _, c := range res.Cookies() {
		if c.Name == "trmnl_oauth_state" {
			state = c.Value
		}
	}
	parts := strings.Split(state, "|")
	if len(parts) != 3 || parts[1] != strconv.FormatInt(clientID, 10) || parts[2] != strconv.FormatInt(accID, 10) {
		t.Errorf("state cookie %q, want nonce|%d|%d", state, clientID, accID)
	}

	// Unknown accounts are rejected.
	res, err = ts.Client().Get(ts.URL + "/admin/calendar/google/start?account=999")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown account: status %d, want 404", res.StatusCode)
	}

	// The account page offers the reconnect button.
	res, err = ts.Client().Get(ts.URL + "/admin/calendar/" + strconv.FormatInt(accID, 10))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), "Reconnect Google account") {
		t.Error("account page lacks the reconnect button")
	}
}

func TestAdminHTTPSInfoAndCADownload(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	src := tlscert.NewLocalCA(filepath.Join(dir, "tls"), []string{"trmnl.fritz.box"})

	r := server.New()
	admin.New(st, "http://test.local", dir, admin.Auth{}, nil).WithHTTPS(":9443", src).Routes(r)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	res, err := ts.Client().Get(ts.URL + "/admin/tls/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(string(body), "-----BEGIN CERTIFICATE-----") {
		t.Fatalf("CA download: status %d, body %.40q", res.StatusCode, body)
	}
	fp, _ := tlscert.Fingerprint(body)

	res, err = ts.Client().Get(ts.URL + "/admin/settings")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(res.Body)
	res.Body.Close()
	host := strings.TrimPrefix(ts.URL, "http://")
	hostname := host[:strings.LastIndex(host, ":")]
	for _, want := range []string{"https://" + hostname + ":9443/admin", fp, "trmnl.fritz.box"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("settings page lacks %q", want)
		}
	}

	// Without HTTPS the CA download does not exist.
	r2 := server.New()
	admin.New(st, "http://test.local", dir, admin.Auth{}, nil).Routes(r2)
	ts2 := httptest.NewServer(r2)
	t.Cleanup(ts2.Close)
	res, err = ts2.Client().Get(ts2.URL + "/admin/tls/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("CA download without HTTPS: status %d, want 404", res.StatusCode)
	}
}

func TestAdminForceRefreshRendersNextScreen(t *testing.T) {
	ts, st := newAdminServer(t)

	p, _ := st.CreatePlugin("clock", "c")
	sc, _ := st.CreateScreen(p.ID, "c", "{}")
	pl, _ := st.CreatePlaylist("default")
	st.AddPlaylistItem(pl.ID, sc.ID)
	d, _ := st.CreateDevice(&store.Device{MAC: "AA:BB:CC:DD:EE:10", APIKey: "k", FriendlyID: "F10"})
	if err := st.UpdateDeviceSettings(d.ID, "", 900, sql.NullInt64{Int64: pl.ID, Valid: true}, "classic", true); err != nil {
		t.Fatal(err)
	}

	resp, err := ts.Client().PostForm(ts.URL+"/admin/devices/"+itoa(d.ID)+"/refresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("refresh = %d, want 302", resp.StatusCode)
	}

	// The device page has something to show without waiting for a poll.
	if _, ok, _ := st.LatestDeviceRender(d.ID); !ok {
		t.Errorf("refresh did not render the device's next screen")
	}
	// Rendering for the page must not skip a screen on the device.
	if got, _ := st.GetDeviceByID(d.ID); got.PlaylistCursor != d.PlaylistCursor {
		t.Errorf("cursor = %d, want %d", got.PlaylistCursor, d.PlaylistCursor)
	}
}
