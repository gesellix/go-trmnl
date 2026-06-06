package admin_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gesellix/go-trmnl/internal/admin"
	"github.com/gesellix/go-trmnl/internal/server"
	"github.com/gesellix/go-trmnl/internal/store"

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
	admin.New(st, "http://test.local", dir).Routes(r)
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
